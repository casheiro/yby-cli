package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
)

// execCommandContext is a variable so it can be mocked in tests
var execCommandContext = exec.CommandContext

// Executor handles the execution of a plugin process.
type Executor struct {
	Timeout        time.Duration
	SkipTrustCheck bool // Desabilita verificação de trust (usado em testes e manifest discovery)
}

// NewExecutor creates a new plugin executor.
func NewExecutor() *Executor {
	return &Executor{
		Timeout: 30 * time.Second, // Default timeout
	}
}

// checkTrust verifica se o plugin é confiável antes da execução.
func (e *Executor) checkTrust(binaryPath string) error {
	if e.SkipTrustCheck {
		return nil
	}

	trusted, err := IsTrusted(binaryPath)
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodePlugin, fmt.Sprintf("falha ao verificar confiança do plugin '%s'", binaryPath))
	}

	if !trusted {
		name := filepath.Base(binaryPath)
		return ybyerrors.New(ybyerrors.ErrCodePlugin,
			fmt.Sprintf("plugin '%s' não está na whitelist de confiança ou seu checksum foi alterado", name)).
			WithHint(fmt.Sprintf("Execute 'yby plugin trust %s' para registrá-lo como confiável", name))
	}

	return nil
}

// Run executes the plugin binary at path with the given request payload.
// It writes payload to STDIN and reads response from STDOUT.
func (e *Executor) Run(ctx context.Context, binaryPath string, req interface{}) (*PluginResponse, error) {
	if err := e.checkTrust(binaryPath); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	cmd := execCommandContext(ctx, binaryPath)

	// Prepare STDIN
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, ybyerrors.Wrap(err, ybyerrors.ErrCodePluginRPC, "falha ao serializar requisição do plugin")
	}
	cmd.Stdin = bytes.NewReader(reqBytes)

	// Capture STDOUT and STDERR
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	if err := cmd.Run(); err != nil {
		return nil, ybyerrors.Wrap(err, ybyerrors.ErrCodePlugin,
			fmt.Sprintf("execução do plugin falhou (%s)", binaryPath)).
			WithContext("stderr", stderr.String())
	}

	// Parse STDOUT
	var resp PluginResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, ybyerrors.Wrap(err, ybyerrors.ErrCodePluginRPC, "falha ao analisar resposta do plugin").
			WithContext("stdout", stdout.String())
	}

	if resp.Error != "" {
		return nil, ybyerrors.New(ybyerrors.ErrCodePlugin, fmt.Sprintf("plugin reportou erro: %s", resp.Error))
	}

	return &resp, nil
}

// RunInteractive executes the plugin in interactive mode (TUI).
// It passes the request payload via the YBY_PLUGIN_REQUEST environment variable
// and connects the plugin's Stdin/Stdout/Stderr directly to the OS.
func (e *Executor) RunInteractive(ctx context.Context, binaryPath string, req interface{}) error {
	if err := e.checkTrust(binaryPath); err != nil {
		return err
	}

	// Interactive plugins typically manage their own timeout or run indefinitely until user exit
	// So we might not want to enforce a strict short timeout, but context cancellation is still good.
	cmd := execCommandContext(ctx, binaryPath)

	// Pass payload via Env Var
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodePluginRPC, "falha ao serializar requisição do plugin")
	}
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("YBY_PLUGIN_REQUEST=%s", string(reqBytes)))

	// Connect IO
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute
	if err := cmd.Run(); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodePlugin, "execução interativa do plugin falhou")
	}

	return nil
}
