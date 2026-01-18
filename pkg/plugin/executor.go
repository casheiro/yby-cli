package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Executor handles the execution of a plugin process.
type Executor struct {
	Timeout time.Duration
}

// NewExecutor creates a new plugin executor.
func NewExecutor() *Executor {
	return &Executor{
		Timeout: 30 * time.Second, // Default timeout
	}
}

// Run executes the plugin binary at path with the given request payload.
// It writes payload to STDIN and reads response from STDOUT.
func (e *Executor) Run(ctx context.Context, binaryPath string, req interface{}) (*PluginResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath)

	// Prepare STDIN
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plugin request: %w", err)
	}
	cmd.Stdin = bytes.NewReader(reqBytes)

	// Capture STDOUT and STDERR
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("plugin execution failed (%s): %w. Stderr: %s", binaryPath, err, stderr.String())
	}

	// Parse STDOUT
	var resp PluginResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse plugin response: %w. Stdout: %s", err, stdout.String())
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("plugin reported error: %s", resp.Error)
	}

	return &resp, nil
}

// RunInteractive executes the plugin in interactive mode (TUI).
// It passes the request payload via the YBY_PLUGIN_REQUEST environment variable
// and connects the plugin's Stdin/Stdout/Stderr directly to the OS.
func (e *Executor) RunInteractive(ctx context.Context, binaryPath string, req interface{}) error {
	// Interactive plugins typically manage their own timeout or run indefinitely until user exit
	// So we might not want to enforce a strict short timeout, but context cancellation is still good.
	cmd := exec.CommandContext(ctx, binaryPath)

	// Pass payload via Env Var
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal plugin request: %w", err)
	}
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("YBY_PLUGIN_REQUEST=%s", string(reqBytes)))

	// Connect IO
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("interactive plugin execution failed: %w", err)
	}

	return nil
}
