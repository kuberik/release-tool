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

	// Helper function to create a dummy commit
	createDummyCommit := func(message string) {
		dummyFile := "dummy.txt"
		require.NoError(t, os.WriteFile(dummyFile, []byte(message), 0644))
		addCmd := exec.Command("git", "add", dummyFile)
		addCmd.Dir = localDir
		require.NoError(t, addCmd.Run())
		commitCmd := exec.Command("git", "commit", "-m", message)
		commitCmd.Dir = localDir
		require.NoError(t, commitCmd.Run())
	}

	// Helper function to run publish command
	runPublish := func(releaseName string) string {
		cmd := NewRootCmd()
		output := &bytes.Buffer{}
		cmd.SetOut(output)
		cmd.SetErr(output)
		cmd.SetArgs([]string{"publish", releaseName})
		require.NoError(t, cmd.Execute())
		return output.String()
	}

	// Step 1: First release (0.1.0)
	createDummyCommit("First release commit")
	output := runPublish("test")
	assert.Contains(t, output, "Pushed new release branch: release-test-0.1")
	assert.Contains(t, output, "Created and pushed tag: test/v0.1.0")

	// Step 2: Second release (0.2.0)
	createDummyCommit("Second release commit")
	output = runPublish("test")
	assert.Contains(t, output, "Pushed new release branch: release-test-0.2")
	assert.Contains(t, output, "Created and pushed tag: test/v0.2.0")

	// Step 3: Checkout release-0.1 and publish (0.1.1)
	checkoutCmd := exec.Command("git", "checkout", "release-test-0.1")
	checkoutCmd.Dir = localDir
	require.NoError(t, checkoutCmd.Run())
	createDummyCommit("Patch for 0.1")
	output = runPublish("test")
	assert.Contains(t, output, "Created and pushed tag: test/v0.1.1")

	// Step 4: Checkout main and publish (0.3.0)
	checkoutCmd = exec.Command("git", "checkout", "master")
	checkoutCmd.Dir = localDir
	require.NoError(t, checkoutCmd.Run())
	createDummyCommit("Third release commit")
	output = runPublish("test")
	assert.Contains(t, output, "Pushed new release branch: release-test-0.3")
	assert.Contains(t, output, "Created and pushed tag: test/v0.3.0")

	// Verify all tags exist
	lsRemoteTagsCmd := exec.Command("git", "ls-remote", "--tags", remoteDir, "test/v*")
	tagOutput, err := lsRemoteTagsCmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(tagOutput), "test/v0.1.0")
	assert.Contains(t, string(tagOutput), "test/v0.1.1")
	assert.Contains(t, string(tagOutput), "test/v0.2.0")
	assert.Contains(t, string(tagOutput), "test/v0.3.0")
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

	// Checkout release branch and make another change
	checkoutCmd := exec.Command("git", "checkout", "release-test-0.2")
	checkoutCmd.Dir = localDir
	require.NoError(t, checkoutCmd.Run())

	require.NoError(t, os.WriteFile(testFile, []byte("content3"), 0644))
	addCmd = exec.Command("git", "add", testFile)
	addCmd.Dir = localDir
	require.NoError(t, addCmd.Run())

	commitCmd = exec.Command("git", "commit", "-m", "Update test file again")
	commitCmd.Dir = localDir
	require.NoError(t, commitCmd.Run())

	// Third publish (patch increment)
	cmd = NewRootCmd()
	output = &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)
	cmd.SetArgs([]string{"publish", "test"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, output.String(), "Created and pushed tag: test/v0.2.1")

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
	assert.Contains(t, string(tagOutput), "test/v0.2.1")

	// Verify tag commits are different
	tag1Cmd := exec.Command("git", "ls-remote", remoteDir, "refs/tags/test/v0.1.0")
	tag1Output, err := tag1Cmd.Output()
	require.NoError(t, err)
	tag1Commit := strings.Split(string(tag1Output), "\t")[0]

	tag2Cmd := exec.Command("git", "ls-remote", remoteDir, "refs/tags/test/v0.2.0")
	tag2Output, err := tag2Cmd.Output()
	require.NoError(t, err)
	tag2Commit := strings.Split(string(tag2Output), "\t")[0]

	tag3Cmd := exec.Command("git", "ls-remote", remoteDir, "refs/tags/test/v0.2.1")
	tag3Output, err := tag3Cmd.Output()
	require.NoError(t, err)
	tag3Commit := strings.Split(string(tag3Output), "\t")[0]

	assert.NotEqual(t, tag1Commit, tag2Commit, "Tags should point to different commits")
	assert.NotEqual(t, tag2Commit, tag3Commit, "Tags should point to different commits")
}
