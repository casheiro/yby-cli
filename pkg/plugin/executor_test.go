package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

// mockExecCommandContext creates a fake exec.Cmd that points to our TestHelperProcess
func mockExecCommandContext(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess is the fake binary used by tests.
func TestHelperProcess(t *testing.T) {
	if !testutil.HelperProcessVerifier() {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		return
	}

	cmd := args[0]
	// Handle fake commands based on what the test expects
	switch cmd {
	case "/path/to/success-plugin":
		fmt.Fprintf(os.Stdout, `{"Data": {"key": "value"}}`)
	case "/path/to/error-plugin":
		fmt.Fprintf(os.Stdout, `{"Error": "plugin custom error"}`)
	case "/path/to/crash-plugin":
		fmt.Fprintf(os.Stderr, "panic runtime error")
		os.Exit(1)
	case "/path/to/malformed-json-plugin":
		fmt.Fprintf(os.Stdout, `{"Data": bad-json}`)
	case "/path/to/interactive-plugin":
		// Do nothing, exit 0
		return
	case "/path/to/interactive-crash-plugin":
		os.Exit(1)
	}
}

func TestExecutor_Run(t *testing.T) {
	originalExecCommandContext := execCommandContext
	execCommandContext = mockExecCommandContext
	defer func() { execCommandContext = originalExecCommandContext }()

	executor := NewExecutor()
	executor.SkipTrustCheck = true
	ctx := context.Background()
	req := PluginRequest{Hook: "test"}

	t.Run("Success", func(t *testing.T) {
		resp, err := executor.Run(ctx, "/path/to/success-plugin", req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		dataMap, ok := resp.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "value", dataMap["key"])
	})

	t.Run("Plugin returns explicit error", func(t *testing.T) {
		_, err := executor.Run(ctx, "/path/to/error-plugin", req)
		assert.ErrorContains(t, err, "plugin reportou erro: plugin custom error")
	})

	t.Run("Plugin crashes / non-zero exit", func(t *testing.T) {
		_, err := executor.Run(ctx, "/path/to/crash-plugin", req)
		assert.ErrorContains(t, err, "execução do plugin falhou")
		// O stderr agora está no contexto do YbyError, não inline na mensagem
		var ybyErr *ybyerrors.YbyError
		if errors.As(err, &ybyErr) {
			assert.Contains(t, ybyErr.Context["stderr"], "panic runtime error")
		}
	})

	t.Run("Plugin returns malformed JSON", func(t *testing.T) {
		_, err := executor.Run(ctx, "/path/to/malformed-json-plugin", req)
		assert.ErrorContains(t, err, "falha ao analisar resposta do plugin")
	})

	t.Run("Serialization error", func(t *testing.T) {
		// Provide an unserializable request (e.g. channel)
		badReq := make(chan int)
		_, err := executor.Run(ctx, "/path/to/success-plugin", badReq)
		assert.ErrorContains(t, err, "falha ao serializar requisição do plugin")
	})
}

func TestExecutor_RunInteractive(t *testing.T) {
	originalExecCommandContext := execCommandContext
	execCommandContext = mockExecCommandContext
	defer func() { execCommandContext = originalExecCommandContext }()

	executor := NewExecutor()
	executor.SkipTrustCheck = true
	ctx := context.Background()
	req := PluginRequest{Hook: "command"}

	t.Run("Success", func(t *testing.T) {
		err := executor.RunInteractive(ctx, "/path/to/interactive-plugin", req)
		assert.NoError(t, err)
	})

	t.Run("Plugin crashes", func(t *testing.T) {
		err := executor.RunInteractive(ctx, "/path/to/interactive-crash-plugin", req)
		assert.ErrorContains(t, err, "execução interativa do plugin falhou")
	})

	t.Run("Serialization error", func(t *testing.T) {
		badReq := make(chan int)
		err := executor.RunInteractive(ctx, "/path/to/interactive-plugin", badReq)
		assert.ErrorContains(t, err, "falha ao serializar requisição do plugin")
	})
}
