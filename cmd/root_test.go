package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCmd_Flags(t *testing.T) {
	// Re-initialize or reset flags to test behavior
	cmd := rootCmd

	// Test setting string flags
	err := cmd.ParseFlags([]string{"--context", "test-env", "--log-level", "debug", "--log-format", "json"})
	assert.NoError(t, err)

	ctxFlag, _ := cmd.Flags().GetString("context")
	assert.Equal(t, "test-env", ctxFlag)

	lvlFlag, _ := cmd.Flags().GetString("log-level")
	assert.Equal(t, "debug", lvlFlag)

	fmtFlag, _ := cmd.Flags().GetString("log-format")
	assert.Equal(t, "json", fmtFlag)
}

func TestExecuteCommand(t *testing.T) {
	// Capture output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Command should return error or print help when called with invalid subcommand
	rootCmd.SetArgs([]string{"invalid-command-that-doesnt-exist"})

	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestExecuteRootHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Plataforma de Engenharia")
}
