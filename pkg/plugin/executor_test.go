package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper process trick to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	mode := os.Getenv("HELPER_MODE")

	// Read STDIN to simulate parsing PluginRequest
	var req PluginRequest
	decoder := json.NewDecoder(os.Stdin)
	_ = decoder.Decode(&req) // Ignore error in helper, handle outputs below

	var resp PluginResponse
	switch mode {
	case "success":
		resp = PluginResponse{
			Data: map[string]interface{}{"key": "value"},
		}
	case "error":
		resp = PluginResponse{
			Error: "simulated plugin error",
		}
	case "invalid_json":
		fmt.Print("{invalid json")
		os.Exit(0)
	case "crash":
		os.Exit(1)
	}

	respBytes, _ := json.Marshal(resp)
	fmt.Print(string(respBytes))
	os.Exit(0)
}

// fakeExecCommand returns a command that runs the helper process
func fakeExecCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)

	// Pass mode through environment variable, assuming it gets set before fakeExecCommand is called
	// Easiest is to rely on os.Setenv in the test itself
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", fmt.Sprintf("HELPER_MODE=%s", os.Getenv("HELPER_MODE"))}
	return cmd
}

func TestExecutor_Run_Success(t *testing.T) {
	// Setup mock
	oldExec := execCommandContext
	execCommandContext = fakeExecCommand
	defer func() { execCommandContext = oldExec }()

	os.Setenv("HELPER_MODE", "success")
	defer os.Unsetenv("HELPER_MODE")

	executor := NewExecutor()
	req := PluginRequest{Hook: "test_hook"}

	resp, err := executor.Run(context.Background(), "fake-plugin", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	dataMap, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "value", dataMap["key"])
}

func TestExecutor_Run_PluginError(t *testing.T) {
	oldExec := execCommandContext
	execCommandContext = fakeExecCommand
	defer func() { execCommandContext = oldExec }()

	os.Setenv("HELPER_MODE", "error")
	defer os.Unsetenv("HELPER_MODE")

	executor := NewExecutor()
	req := PluginRequest{Hook: "test_hook"}

	resp, err := executor.Run(context.Background(), "fake-plugin", req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "plugin reportou erro: simulated plugin error")
}

func TestExecutor_Run_InvalidJson(t *testing.T) {
	oldExec := execCommandContext
	execCommandContext = fakeExecCommand
	defer func() { execCommandContext = oldExec }()

	os.Setenv("HELPER_MODE", "invalid_json")
	defer os.Unsetenv("HELPER_MODE")

	executor := NewExecutor()
	req := PluginRequest{Hook: "test_hook"}

	resp, err := executor.Run(context.Background(), "fake-plugin", req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "falha ao analisar resposta do plugin")
}

func TestExecutor_Run_Crash(t *testing.T) {
	oldExec := execCommandContext
	execCommandContext = fakeExecCommand
	defer func() { execCommandContext = oldExec }()

	os.Setenv("HELPER_MODE", "crash")
	defer os.Unsetenv("HELPER_MODE")

	executor := NewExecutor()
	req := PluginRequest{Hook: "test_hook"}

	resp, err := executor.Run(context.Background(), "fake-plugin", req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "execução do plugin falhou")
}
