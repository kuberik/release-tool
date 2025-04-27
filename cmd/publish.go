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

			// Get current commit hash
			headCmd := exec.Command("git", "rev-parse", "HEAD")
			headOutput, err := headCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get current commit: %v", err)
			}
			currentCommit := strings.TrimSpace(string(headOutput))

			// Get current branch name
			branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
			branchOutput, err := branchCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get current branch: %v", err)
			}
			currentBranch := strings.TrimSpace(string(branchOutput))

			// Check if we're on a release branch
			isReleaseBranch := strings.HasPrefix(currentBranch, "release-"+name+"-")

			// Get latest version from git history
			logCmd := exec.Command("git", "log", "--pretty=format:%D", "--simplify-by-decoration", "HEAD")
			logOutput, err := logCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get git log: %v", err)
			}

			// Parse tags and find latest version
			latestVersion := semver.MustParse("0.0.0")
			lines := strings.Split(string(logOutput), "\n")
		find_loop:
			for _, line := range lines {
				if line == "" {
					continue
				}
				// Extract tags from git log output (format: tag: name/v1.2.3)
				tags := strings.Split(strings.TrimSpace(line), ", ")
				for _, tag := range tags {
					tag := strings.TrimSpace(tag)
					if strings.HasPrefix(tag, "tag: "+name+"/v") {
						versionStr := strings.TrimPrefix(tag, "tag: "+name+"/v")
						version, err := semver.NewVersion(versionStr)
						if err == nil {
							latestVersion = version
							break find_loop
						}
					}
				}
			}

			var newVersion *semver.Version

			if isReleaseBranch {
				// For patch releases, increment from the current version's patch
				newVersion = semver.MustParse(fmt.Sprintf("%d.%d.%d", latestVersion.Major(), latestVersion.Minor(), latestVersion.Patch()+1))

				// Push the current branch
				pushCmd := exec.Command("git", "push", "origin", currentBranch)
				if err := pushCmd.Run(); err != nil {
					return fmt.Errorf("failed to push branch: %v", err)
				}
			} else {
				newVersion = semver.MustParse(fmt.Sprintf("%d.%d.%d", latestVersion.Major(), latestVersion.Minor()+1, latestVersion.Patch()))
				newBranch := fmt.Sprintf("release-%s-%d.%d", name, newVersion.Major(), newVersion.Minor())
				pushCmd := exec.Command("git", "push", "origin", currentCommit+":refs/heads/"+newBranch)
				if err := pushCmd.Run(); err != nil {
					return fmt.Errorf("failed to push branch: %v", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Pushed new release branch: %s\n", newBranch)
			}

			// Create and push a tag for this release
			tagName := fmt.Sprintf("%s/v%d.%d.%d", name, newVersion.Major(), newVersion.Minor(), newVersion.Patch())
			tagCmd := exec.Command("git", "tag", "-f", tagName, currentCommit)
			if err := tagCmd.Run(); err != nil {
				return fmt.Errorf("failed to create tag: %v", err)
			}

			// Push the tag
			pushTagCmd := exec.Command("git", "push", "-f", "origin", tagName)
			if err := pushTagCmd.Run(); err != nil {
				return fmt.Errorf("failed to push tag: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created and pushed tag: %s\n", tagName)
			return nil
		},
	}
}
