package testutil

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteCommand_Success(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	child := &cobra.Command{
		Use: "hello",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Print("olá mundo")
			return nil
		},
	}
	root.AddCommand(child)

	output, err := ExecuteCommand(root, "hello")
	require.NoError(t, err)
	assert.Contains(t, output, "olá mundo")
}

func TestExecuteCommand_Error(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	child := &cobra.Command{
		Use: "fail",
		RunE: func(cmd *cobra.Command, args []string) error {
			return assert.AnError
		},
	}
	root.AddCommand(child)

	_, err := ExecuteCommand(root, "fail")
	assert.Error(t, err)
}

func TestExecuteCommand_Help(t *testing.T) {
	root := &cobra.Command{Use: "test", Short: "teste da CLI"}
	output, err := ExecuteCommand(root, "--help")
	require.NoError(t, err)
	assert.Contains(t, output, "teste da CLI")
}
