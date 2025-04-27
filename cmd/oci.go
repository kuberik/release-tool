package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"
)

// getLatestVersionTag returns the version from the latest commit's tag
func getLatestVersionTag(dir string, name string) (string, error) {
	// Check if directory is a git repository
	gitCheckCmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	gitCheckCmd.Dir = dir
	if err := gitCheckCmd.Run(); err != nil {
		// Not a git repository, return default version
		return "0.0.0", nil
	}

	// Get the latest commit's tag
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		// No tag found, get the current commit hash
		hashCmd := exec.Command("git", "rev-parse", "HEAD")
		hashCmd.Dir = dir
		hashOutput, err := hashCmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get commit hash: %v", err)
		}
		return strings.TrimSpace(string(hashOutput)), nil
	}

	tag := strings.TrimSpace(string(output))
	// Look for [name]/v* pattern
	prefix := name + "/v"
	if !strings.HasPrefix(tag, prefix) {
		// No matching tag found, get the current commit hash
		hashCmd := exec.Command("git", "rev-parse", "HEAD")
		hashCmd.Dir = dir
		hashOutput, err := hashCmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get commit hash: %v", err)
		}
		return strings.TrimSpace(string(hashOutput)), nil
	}

	// Extract version from tag
	versionStr := strings.TrimPrefix(tag, prefix)
	// Remove any trailing characters (like ^0)
	versionStr = strings.TrimSuffix(versionStr, "^0")
	return versionStr, nil
}

func NewOciCmd() *cobra.Command {
	var insecure bool
	cmd := &cobra.Command{
		Use:   "oci [release-name] [name] [directory]",
		Short: "Publish a directory as an OCI image",
		Long:  `Publish a directory as an OCI image using crane.`,
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			releaseName := args[0]
			imageName := args[1]
			dir := args[2]

			// Extract the name part from the image reference
			ref, err := name.ParseReference(imageName)
			if err != nil {
				return fmt.Errorf("failed to parse image reference: %v", err)
			}

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

			// Get the latest version tag
			latestVersion, err := getLatestVersionTag(dir, releaseName)
			if err != nil {
				return fmt.Errorf("failed to get latest version tag: %v", err)
			}

			// Create a temporary file for the tarball
			tmpFile, err := os.CreateTemp("", "oci-*.tar.gz")
			if err != nil {
				return fmt.Errorf("failed to create temporary file: %v", err)
			}
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			// Create a gzip writer
			gw := gzip.NewWriter(tmpFile)
			defer gw.Close()

			// Create a tar writer
			tw := tar.NewWriter(gw)
			defer tw.Close()

			// Walk through the directory and add files to the tarball
			err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Skip the root directory
				if path == dir {
					return nil
				}

				// Get the relative path
				relPath, err := filepath.Rel(dir, path)
				if err != nil {
					return fmt.Errorf("failed to get relative path: %v", err)
				}

				// Create tar header
				header, err := tar.FileInfoHeader(info, "")
				if err != nil {
					return fmt.Errorf("failed to create tar header: %v", err)
				}
				header.Name = relPath

				// If it's a regular file, write its contents
				if info.Mode().IsRegular() {
					file, err := os.Open(path)
					if err != nil {
						return fmt.Errorf("failed to open file: %v", err)
					}
					defer file.Close()

					// Read file contents
					content, err := io.ReadAll(file)
					if err != nil {
						return fmt.Errorf("failed to read file contents: %v", err)
					}

					// Replace $(version) with the latest version
					contentStr := string(content)
					contentStr = strings.ReplaceAll(contentStr, "$(version)", latestVersion)

					// Update the header size to match the new content length
					header.Size = int64(len(contentStr))

					// Write the header with updated size
					if err := tw.WriteHeader(header); err != nil {
						return fmt.Errorf("failed to write tar header: %v", err)
					}

					// Write the modified content
					if _, err := tw.Write([]byte(contentStr)); err != nil {
						return fmt.Errorf("failed to write file contents: %v", err)
					}
				}

				return nil
			})

			if err != nil {
				return fmt.Errorf("failed to create tarball: %v", err)
			}

			// Close writers to ensure all data is written
			if err := tw.Close(); err != nil {
				return fmt.Errorf("failed to close tar writer: %v", err)
			}
			if err := gw.Close(); err != nil {
				return fmt.Errorf("failed to close gzip writer: %v", err)
			}
			if err := tmpFile.Close(); err != nil {
				return fmt.Errorf("failed to close temporary file: %v", err)
			}

			// Create a new empty image
			img := empty.Image

			// Add the layer to the image
			layer, err := tarball.LayerFromFile(tmpFile.Name())
			if err != nil {
				return fmt.Errorf("failed to create layer from tarball: %v", err)
			}

			img, err = mutate.Append(img, mutate.Addendum{
				Layer: layer,
			})
			if err != nil {
				return fmt.Errorf("failed to append layer to image: %v", err)
			}

			// Push the image
			opts := []crane.Option{}
			if insecure {
				opts = append(opts, crane.Insecure)
			}

			// Push with latest tag
			if err := crane.Push(img, ref.String(), opts...); err != nil {
				return fmt.Errorf("failed to push image: %v", err)
			}

			// Push with version tag
			versionRef, err := name.NewTag(strings.TrimSuffix(ref.String(), ":latest") + ":" + latestVersion)
			if err != nil {
				return fmt.Errorf("failed to create version tag reference: %v", err)
			}
			if err := crane.Push(img, versionRef.String(), opts...); err != nil {
				return fmt.Errorf("failed to push version tag: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully published directory as OCI image: %s\n", imageName)
			fmt.Fprintf(cmd.OutOrStdout(), "Added version tag: %s\n", versionRef.String())
			return nil
		},
	}

	cmd.Flags().BoolVar(&insecure, "insecure", false, "Allow pushing to insecure registries")
	return cmd
}
