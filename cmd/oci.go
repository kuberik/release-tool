package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"
)

func NewOciCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "oci [name] [directory]",
		Short: "Publish a directory as an OCI image",
		Long:  `Publish a directory as an OCI image using crane.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageName := args[0]
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

				// Write header
				if err := tw.WriteHeader(header); err != nil {
					return fmt.Errorf("failed to write tar header: %v", err)
				}

				// If it's a regular file, write its contents
				if info.Mode().IsRegular() {
					file, err := os.Open(path)
					if err != nil {
						return fmt.Errorf("failed to open file: %v", err)
					}
					defer file.Close()

					if _, err := io.Copy(tw, file); err != nil {
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

			// Parse the image reference
			ref, err := name.ParseReference(imageName)
			if err != nil {
				return fmt.Errorf("failed to parse image reference: %v", err)
			}

			// Push the image
			if err := crane.Push(img, ref.String()); err != nil {
				return fmt.Errorf("failed to push image: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully published directory as OCI image: %s\n", imageName)
			return nil
		},
	}
}
