package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executeCommand(cmd *cobra.Command, args ...string) (string, error) {
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func TestVersionCmd(t *testing.T) {
	// Setup test repository
	localDir, _ := setupTestRepo(t)

	// Change to test directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	require.NoError(t, os.Chdir(localDir))

	// Create a tag for service-a
	tagCmd := exec.Command("git", "tag", "service-a/v1.2.3")
	err = tagCmd.Run()
	assert.NoError(t, err)

	// Create a new commit
	dummyFile := "dummy.txt"
	err = os.WriteFile(dummyFile, []byte("new content"), 0644)
	require.NoError(t, err)
	addCmd := exec.Command("git", "add", dummyFile)
	require.NoError(t, addCmd.Run())
	commitCmd := exec.Command("git", "commit", "-m", "New commit")
	require.NoError(t, commitCmd.Run())

	// Create a tag for service-b on the new commit
	tagCmd = exec.Command("git", "tag", "service-b/v2.3.4")
	err = tagCmd.Run()
	assert.NoError(t, err)

	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		errContains string
		wantOutput  string
	}{
		{
			name:       "service tagged at HEAD",
			args:       []string{"service-b"},
			wantErr:    false,
			wantOutput: "2.3.4\n",
		},
		{
			name:        "service tagged at previous commit",
			args:        []string{"service-a"},
			wantErr:     true,
			errContains: "current HEAD is not tagged with a version",
		},
		{
			name:        "no version tag",
			args:        []string{"nonexistent-service"},
			wantErr:     true,
			errContains: "current HEAD is not tagged with a version",
		},
		{
			name:        "missing name argument",
			args:        []string{},
			wantErr:     true,
			errContains: "accepts 1 arg(s), received 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewVersionCmd()
			output, err := executeCommand(cmd, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantOutput, output)
			}
		})
	}
}
