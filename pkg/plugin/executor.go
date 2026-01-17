package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
