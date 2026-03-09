package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompletionCmd_Bash(t *testing.T) {
	var buf bytes.Buffer
	completionCmd.Root().SetOut(&buf)
	completionCmd.SetArgs([]string{})

	err := completionCmd.RunE(completionCmd, []string{"bash"})
	require.NoError(t, err)
}

func TestCompletionCmd_Zsh(t *testing.T) {
	err := completionCmd.RunE(completionCmd, []string{"zsh"})
	require.NoError(t, err)
}

func TestCompletionCmd_Fish(t *testing.T) {
	err := completionCmd.RunE(completionCmd, []string{"fish"})
	require.NoError(t, err)
}

func TestCompletionCmd_Powershell(t *testing.T) {
	err := completionCmd.RunE(completionCmd, []string{"powershell"})
	require.NoError(t, err)
}
