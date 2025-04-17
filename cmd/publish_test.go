package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRepo(t *testing.T) (string, string) {
	// Create a temporary directory for the remote
	remoteDir := t.TempDir()

	// Initialize bare repository for remote
	initRemoteCmd := exec.Command("git", "init", "--bare")
	initRemoteCmd.Dir = remoteDir
	require.NoError(t, initRemoteCmd.Run())

	// Create a temporary directory for the local repository
	localDir := t.TempDir()

	// Initialize git repository
	cmds := []*exec.Cmd{
		exec.Command("git", "init"),
		exec.Command("git", "config", "user.name", "Test User"),
		exec.Command("git", "config", "user.email", "test@example.com"),
		// Add local remote
		exec.Command("git", "remote", "add", "origin", remoteDir),
	}

	for _, cmd := range cmds {
		cmd.Dir = localDir
		require.NoError(t, cmd.Run())
	}

	// Create some test files and commits
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
			require.NoError(t, os.MkdirAll(filepath.Join(localDir, dir), 0755))
		}

		// Create file
		require.NoError(t, os.WriteFile(filepath.Join(localDir, file.path), []byte(file.content), 0644))

		// Add and commit
		addCmd := exec.Command("git", "add", file.path)
		addCmd.Dir = localDir
		require.NoError(t, addCmd.Run())

		commitCmd := exec.Command("git", "commit", "-m", "Add "+file.path)
		commitCmd.Dir = localDir
		require.NoError(t, commitCmd.Run())
	}

	// Add some more commits for version increment testing
	additionalFiles := []struct {
		path    string
		content string
	}{
		{"newfile1.txt", "new content 1"},
		{"newfile2.txt", "new content 2"},
	}

	for _, file := range additionalFiles {
		require.NoError(t, os.WriteFile(filepath.Join(localDir, file.path), []byte(file.content), 0644))

		addCmd := exec.Command("git", "add", file.path)
		addCmd.Dir = localDir
		require.NoError(t, addCmd.Run())

		commitCmd := exec.Command("git", "commit", "-m", "Add "+file.path)
		commitCmd.Dir = localDir
		require.NoError(t, commitCmd.Run())
	}

	// Create an existing release branch (0.1.0)
	firstCommitCmd := exec.Command("git", "rev-list", "-n1", "HEAD")
	firstCommitCmd.Dir = localDir
	firstCommit, err := firstCommitCmd.Output()
	require.NoError(t, err)

	pushCmd := exec.Command("git", "push", "origin", strings.TrimSpace(string(firstCommit))+":refs/heads/release-rel-d-0.1")
	pushCmd.Dir = localDir
	require.NoError(t, pushCmd.Run())

	return localDir, remoteDir
}

func TestPublishCommand(t *testing.T) {
	// Setup test repository
	localDir, remoteDir := setupTestRepo(t)

	// Change to test directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	require.NoError(t, os.Chdir(localDir))

	// Test cases
	tests := []struct {
		name            string
		releaseName     string
		paths           []string
		expectError     bool
		matchError      string
		expectedVersion string
	}{
		{
			name:            "file",
			releaseName:     "rel-a",
			paths:           []string{"file1.txt"},
			expectedVersion: "0.1",
		},
		{
			name:            "dir",
			releaseName:     "rel-b",
			paths:           []string{"dir/"},
			expectError:     false,
			expectedVersion: "0.1",
		},
		{
			name:            "non-existent-file",
			releaseName:     "rel-c",
			paths:           []string{"nonexistent.txt"},
			expectError:     true,
			matchError:      "no commits found for the specified paths",
		},
		{
			name:            "version-increment",
			releaseName:     "rel-d",
			paths:           []string{"newfile1.txt"},
			expectError:     false,
			expectedVersion: "0.2",
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
			args := append([]string{"publish", tt.releaseName}, tt.paths...)
			cmd.SetArgs(args)

			// Execute command
			err := cmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, output.String(), tt.matchError)
				return
			}

			assert.NoError(t, err)
			assert.Contains(t, output.String(), "Pushed new release branch: release-"+tt.releaseName+"-")

			// Verify remote branch was created
			lsRemoteCmd := exec.Command("git", "ls-remote", "--heads", remoteDir, "release-"+tt.releaseName+"-*")
			branchOutput, err := lsRemoteCmd.Output()
			assert.NoError(t, err)

			assert.Contains(t, string(branchOutput), "release-"+tt.releaseName+"-"+tt.expectedVersion)
		})
	}
}
