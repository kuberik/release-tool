package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
			craneCmd := exec.Command("crane", "manifest", "--insecure", tt.imageName)
			manifestOutput, err := craneCmd.CombinedOutput()
			if err != nil {
				t.Logf("crane manifest output: %s", manifestOutput)
			}
			assert.NoError(t, err)
			assert.NotEmpty(t, manifestOutput)
		})
	}
}
