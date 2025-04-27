package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version [name]",
		Short: "Get the version of the current HEAD commit",
		Long:  `Get the version of the current HEAD commit if it's tagged, otherwise throw an error.`,
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

			// Get tags pointing at current commit
			tagCmd := exec.Command("git", "tag", "--points-at", currentCommit, name+"/v*")
			output, err := tagCmd.Output()
			if err != nil || len(output) == 0 {
				return fmt.Errorf("current HEAD is not tagged with a version")
			}

			// Parse the version from the tag
			tag := strings.TrimSpace(string(output))
			versionStr := strings.TrimPrefix(tag, name+"/v")
			version, err := semver.NewVersion(versionStr)
			if err != nil {
				return fmt.Errorf("failed to parse version from tag: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%d.%d.%d\n", version.Major(), version.Minor(), version.Patch())
			return nil
		},
	}
}
