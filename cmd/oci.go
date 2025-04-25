package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func NewOciCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "oci [name] [directory]",
		Short: "Publish a directory as an OCI image",
		Long:  `Publish a directory as an OCI image using crane.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir := args[1]

			// Check if directory exists
			if !filepath.IsAbs(dir) {
				var err error
				dir, err = filepath.Abs(dir)
				if err != nil {
					return fmt.Errorf("failed to get absolute path: %v", err)
				}
			}

			// Check if directory exists and is accessible
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return fmt.Errorf("failed to copy directory contents: directory does not exist")
			}

			// Check if crane is installed
			if _, err := exec.LookPath("crane"); err != nil {
				return fmt.Errorf("crane not found in PATH. Please install it first: https://github.com/google/go-containerregistry/tree/main/cmd/crane")
			}

			// Create a temporary directory for the OCI image
			tempDir, err := os.MkdirTemp("", "oci-*")
			if err != nil {
				return fmt.Errorf("failed to create temporary directory: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Create a tarball of the directory
			tarPath := filepath.Join(tempDir, "layer.tar")
			tarCmd := exec.Command("tar", "-czf", tarPath, "-C", dir, ".")
			if output, err := tarCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to create tarball: %v: %s", err, output)
			}

			// Create the OCI image using crane
			craneArgs := []string{"append"}
			if strings.HasPrefix(name, "localhost:") || strings.HasPrefix(name, "127.0.0.1:") {
				craneArgs = append(craneArgs, "--insecure")
			}
			craneArgs = append(craneArgs, "--new_layer", tarPath, "--new_tag", name)
			craneCmd := exec.Command("crane", craneArgs...)
			if output, err := craneCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to create OCI image: %v: %s", err, output)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully published directory as OCI image: %s\n", name)
			return nil
		},
	}
}
