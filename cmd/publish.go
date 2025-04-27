package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func NewPublishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "publish [name]",
		Short: "Publish a release branch",
		Long:  `Publish a release branch with the given name.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Get git log
			gitLogCmd := exec.Command("git", "log", "--oneline")
			output, err := gitLogCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get git log: %v", err)
			} else if len(output) == 0 {
				return fmt.Errorf("no commits found")
			}

			// Get the first commit hash
			lines := strings.Split(string(output), "\n")
			firstCommit := strings.Split(lines[0], " ")[0]

			// Get all remote branches
			lsRemoteCmd := exec.Command("git", "ls-remote", "--heads", "origin", "release-"+name+"-*")
			remoteOutput, err := lsRemoteCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to list remote branches: %v", err)
			}

			// Parse existing versions
			latestVersion := semver.MustParse("0.0.0")
			remoteBranches := strings.Split(string(remoteOutput), "\n")
			for _, branch := range remoteBranches {
				if branch == "" {
					continue
				}
				parts := strings.Split(branch, "\t")
				if len(parts) != 2 {
					continue
				}
				branchName := strings.TrimPrefix(parts[1], "refs/heads/")
				if strings.HasPrefix(branchName, "release-"+name+"-") {
					versionStr := strings.TrimPrefix(branchName, "release-"+name+"-")
					// Add .0 to make it a valid semver
					version, err := semver.NewVersion(versionStr + ".0")
					if err == nil && version.GreaterThan(latestVersion) {
						latestVersion = version
					}
				}
			}

			// Increment the minor version
			newVersion := semver.MustParse(fmt.Sprintf("%d.%d.0", latestVersion.Major(), latestVersion.Minor()+1))
			newBranch := fmt.Sprintf("release-%s-%d.%d", name, newVersion.Major(), newVersion.Minor())

			// Push directly to remote
			pushCmd := exec.Command("git", "push", "origin", firstCommit+":refs/heads/"+newBranch)
			if err := pushCmd.Run(); err != nil {
				return fmt.Errorf("failed to push branch: %v", err)
			}

			// Create and push a tag for this release
			tagName := fmt.Sprintf("%s/v%d.%d.0", name, newVersion.Major(), newVersion.Minor())
			tagCmd := exec.Command("git", "tag", tagName, firstCommit)
			if err := tagCmd.Run(); err != nil {
				return fmt.Errorf("failed to create tag: %v", err)
			}

			// Push the tag
			pushTagCmd := exec.Command("git", "push", "origin", tagName)
			if err := pushTagCmd.Run(); err != nil {
				return fmt.Errorf("failed to push tag: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Pushed new release branch: %s\n", newBranch)
			fmt.Fprintf(cmd.OutOrStdout(), "Created and pushed tag: %s\n", tagName)
			return nil
		},
	}
}
