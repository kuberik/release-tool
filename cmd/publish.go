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
		Use:   "publish [name] [paths...]",
		Short: "Publish a release branch",
		Long:  `Publish a release branch with the given name and paths.`,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			paths := args[1:]

			// Get git log for the specified paths
			gitLogCmd := exec.Command("git", append([]string{"log", "--oneline", "--"}, paths...)...)
			output, err := gitLogCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get git log: %v", err)
			} else if len(output) == 0 {
				return fmt.Errorf("no commits found for the specified paths")
			}

			// Get the first commit hash
			lines := strings.Split(string(output), "\n")
			firstCommit := strings.Split(lines[0], " ")[0]

			// Check if the first commit belongs to any release branch
			checkBranchCmd := exec.Command("git", "branch", "-a", "--contains", firstCommit, "--format=%(refname:short)")
			branchOutput, err := checkBranchCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to check branches: %v", err)
			}

			branches := strings.Split(strings.TrimSpace(string(branchOutput)), "\n")
			latestVersion := semver.MustParse("0.0.0")

			for _, branch := range branches {
				branch = strings.TrimSpace(branch)
				if strings.HasPrefix(branch, "origin/") {
					branch = strings.TrimPrefix(branch, "origin/")
				}
				if strings.HasPrefix(branch, "release-"+name+"-") {
					versionStr := strings.TrimPrefix(branch, "release-"+name+"-")
					// Add .0 to make it a valid semver
					version, err := semver.NewVersion(versionStr + ".0")
					if err == nil && version.GreaterThan(latestVersion) {
						latestVersion = version
					}
				}
			}

			// Format as major.minor
			newBranch := fmt.Sprintf("release-%s-%d.%d", name, latestVersion.Major(), latestVersion.Minor() + 1)

			// Push directly to remote
			pushCmd := exec.Command("git", "push", "origin", firstCommit+":refs/heads/"+newBranch)
			if err := pushCmd.Run(); err != nil {
				return fmt.Errorf("failed to push branch: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Pushed new release branch: %s\n", newBranch)
			return nil
		},
	}
}
