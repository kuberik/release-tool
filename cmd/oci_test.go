package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kuberik/release-tool/cmd/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOciCommand(t *testing.T) {
	// Start a local registry
	registry := testhelpers.LocalRegistry()
	defer registry.Close()

	// Create a temporary directory for testing
	testDir := t.TempDir()

	// Create some test files
	testFiles := []struct {
		path    string
		content string
	}{
		{"file1.txt", "content1"},
		{"file2.txt", "content2"},
		{"dir/file3.txt", "content3"},
	}

	for _, file := range testFiles {
		// Create directory if needed
		dir := filepath.Dir(file.path)
		if dir != "." {
			require.NoError(t, os.MkdirAll(filepath.Join(testDir, dir), 0755))
		}

		// Create file
		require.NoError(t, os.WriteFile(filepath.Join(testDir, file.path), []byte(file.content), 0644))
	}

	// Test cases
	tests := []struct {
		name        string
		imageName   string
		dir         string
		expectError bool
		matchError  string
	}{
		{
			name:      "valid-directory",
			imageName: strings.TrimPrefix(registry.URL, "http://") + "/test/image:latest",
			dir:       testDir,
		},
		{
			name:        "non-existent-directory",
			imageName:   strings.TrimPrefix(registry.URL, "http://") + "/test/image:latest",
			dir:         "/non/existent/dir",
			expectError: true,
			matchError:  "failed to copy directory contents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new command for each test
			cmd := NewRootCmd()

			// Capture command output
			output := &bytes.Buffer{}
			cmd.SetOut(output)
			cmd.SetErr(output)

			// Prepare command arguments
			args := []string{"oci", tt.imageName, tt.dir}
			cmd.SetArgs(args)

			// Execute command
			err := cmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, output.String(), tt.matchError)
				return
			}

			assert.NoError(t, err)
			assert.Contains(t, output.String(), "Successfully published directory as OCI image: "+tt.imageName)

			// Verify the image exists in the registry
			ref, err := name.ParseReference(tt.imageName)
			require.NoError(t, err)

			// Pull the image
			img, err := crane.Pull(ref.String())
			require.NoError(t, err)

			// Get the manifest
			manifest, err := img.Manifest()
			require.NoError(t, err)
			require.Len(t, manifest.Layers, 1, "Expected exactly one layer")

			// Create a temporary directory for extraction
			extractDir := t.TempDir()

			// Get the layer
			layer, err := img.LayerByDigest(manifest.Layers[0].Digest)
			require.NoError(t, err)

			// Read and extract the layer
			rc, err := layer.Uncompressed()
			require.NoError(t, err)
			defer rc.Close()

			// Create a temporary file for the tar
			tarFile, err := os.CreateTemp("", "layer-*.tar")
			require.NoError(t, err)
			defer os.Remove(tarFile.Name())

			// Copy the layer content to the tar file
			_, err = io.Copy(tarFile, rc)
			require.NoError(t, err)
			err = tarFile.Close()
			require.NoError(t, err)

			// Extract the tar file
			err = exec.Command("tar", "-xf", tarFile.Name(), "-C", extractDir).Run()
			require.NoError(t, err)

			// Verify all test files exist in the extracted image
			for _, file := range testFiles {
				path := filepath.Join(extractDir, file.path)
				content, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, file.content, string(content))
			}
		})
	}
}
