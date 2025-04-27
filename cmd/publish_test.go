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

	// Push the initial commit to master
	pushMasterCmd := exec.Command("git", "push", "origin", "master")
	pushMasterCmd.Dir = localDir
	require.NoError(t, pushMasterCmd.Run())

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
		expectError     bool
		matchError      string
		expectedVersion string
	}{
		{
			name:            "first-release",
			releaseName:     "rel-a",
			expectedVersion: "0.1",
		},
		{
			name:            "version-increment",
			releaseName:     "rel-d",
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
			args := []string{"publish", tt.releaseName}
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
			assert.Contains(t, output.String(), "Created and pushed tag: "+tt.releaseName+"/v"+tt.expectedVersion+".0")

			// Verify remote branch was created
			lsRemoteCmd := exec.Command("git", "ls-remote", "--heads", remoteDir, "release-"+tt.releaseName+"-*")
			branchOutput, err := lsRemoteCmd.Output()
			assert.NoError(t, err)
			assert.Contains(t, string(branchOutput), "release-"+tt.releaseName+"-"+tt.expectedVersion)

			// Verify tag was created and pushed
			lsRemoteTagsCmd := exec.Command("git", "ls-remote", "--tags", remoteDir, tt.releaseName+"/v"+tt.expectedVersion+".0")
			tagOutput, err := lsRemoteTagsCmd.Output()
			assert.NoError(t, err)
			assert.Contains(t, string(tagOutput), tt.releaseName+"/v"+tt.expectedVersion+".0")

			// Verify tag points to the same commit as the branch
			branchRefCmd := exec.Command("git", "ls-remote", remoteDir, "refs/heads/release-"+tt.releaseName+"-"+tt.expectedVersion)
			branchRefOutput, err := branchRefCmd.Output()
			assert.NoError(t, err)
			branchCommit := strings.Split(string(branchRefOutput), "\t")[0]

			tagRefCmd := exec.Command("git", "ls-remote", remoteDir, "refs/tags/"+tt.releaseName+"/v"+tt.expectedVersion+".0")
			tagRefOutput, err := tagRefCmd.Output()
			assert.NoError(t, err)
			tagCommit := strings.Split(string(tagRefOutput), "\t")[0]

			assert.Equal(t, branchCommit, tagCommit, "Tag should point to the same commit as the branch")
		})
	}
}

func TestPublishCommandMultipleVersions(t *testing.T) {
	// Setup test repository
	localDir, remoteDir := setupTestRepo(t)

	// Change to test directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	require.NoError(t, os.Chdir(localDir))

	// Create a test file
	testFile := "test.txt"
	require.NoError(t, os.WriteFile(testFile, []byte("content1"), 0644))

	// Add and commit
	addCmd := exec.Command("git", "add", testFile)
	addCmd.Dir = localDir
	require.NoError(t, addCmd.Run())

	commitCmd := exec.Command("git", "commit", "-m", "Add test file")
	commitCmd.Dir = localDir
	require.NoError(t, commitCmd.Run())

	// First publish
	cmd := NewRootCmd()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)
	cmd.SetArgs([]string{"publish", "test"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, output.String(), "Pushed new release branch: release-test-0.1")
	assert.Contains(t, output.String(), "Created and pushed tag: test/v0.1.0")

	// Modify file and commit
	require.NoError(t, os.WriteFile(testFile, []byte("content2"), 0644))
	addCmd = exec.Command("git", "add", testFile)
	addCmd.Dir = localDir
	require.NoError(t, addCmd.Run())

	commitCmd = exec.Command("git", "commit", "-m", "Update test file")
	commitCmd.Dir = localDir
	require.NoError(t, commitCmd.Run())

	// Second publish
	cmd = NewRootCmd()
	output = &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)
	cmd.SetArgs([]string{"publish", "test"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, output.String(), "Pushed new release branch: release-test-0.2")
	assert.Contains(t, output.String(), "Created and pushed tag: test/v0.2.0")

	// Verify remote branches
	lsRemoteCmd := exec.Command("git", "ls-remote", "--heads", remoteDir, "release-test-*")
	branchOutput, err := lsRemoteCmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(branchOutput), "release-test-0.1")
	assert.Contains(t, string(branchOutput), "release-test-0.2")

	// Verify tags
	lsRemoteTagsCmd := exec.Command("git", "ls-remote", "--tags", remoteDir, "test/v*")
	tagOutput, err := lsRemoteTagsCmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(tagOutput), "test/v0.1.0")
	assert.Contains(t, string(tagOutput), "test/v0.2.0")

	// Verify tag commits are different
	tag1Cmd := exec.Command("git", "ls-remote", remoteDir, "refs/tags/test/v0.1.0")
	tag1Output, err := tag1Cmd.Output()
	require.NoError(t, err)
	tag1Commit := strings.Split(string(tag1Output), "\t")[0]

	tag2Cmd := exec.Command("git", "ls-remote", remoteDir, "refs/tags/test/v0.2.0")
	tag2Output, err := tag2Cmd.Output()
	require.NoError(t, err)
	tag2Commit := strings.Split(string(tag2Output), "\t")[0]

	assert.NotEqual(t, tag1Commit, tag2Commit, "Tags should point to different commits")
}
