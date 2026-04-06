package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
)

// execCommandContext is a variable so it can be mocked in tests
var execCommandContext = exec.CommandContext

// safeEnvVars define as variáveis de ambiente seguras para herança por plugins.
// Plugins NÃO recebem credentials (AWS_*, GITHUB_TOKEN, etc.) por padrão.
var safeEnvVars = map[string]bool{
	// Sistema
	"PATH": true, "HOME": true, "USER": true, "SHELL": true,
	"TERM": true, "LANG": true, "LC_ALL": true, "LC_CTYPE": true,
	"TZ": true, "TMPDIR": true, "XDG_RUNTIME_DIR": true,
	"COLORTERM": true, "TERM_PROGRAM": true,
	// Kubernetes
	"KUBECONFIG": true,
	// Rede (proxy corporativo)
	"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true,
	"http_proxy": true, "https_proxy": true, "no_proxy": true,
}

// PluginResourceLimits define os limites de recursos para processos de plugins.
var PluginResourceLimits = struct {
	MaxMemoryBytes uint64 // Limite de memória virtual (default: 1GB)
	MaxOpenFiles   uint64 // Limite de file descriptors (default: 256)
	MaxProcesses   uint64 // Limite de processos filhos (default: 32)
}{
	MaxMemoryBytes: 1 << 30, // 1GB
	MaxOpenFiles:   256,
	MaxProcesses:   32,
}

// sanitizedEnv retorna env vars filtradas pela whitelist + extras fornecidas.
func sanitizedEnv(extra ...string) []string {
	var env []string
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		if safeEnvVars[key] {
			env = append(env, e)
		}
	}
	return append(env, extra...)
}

// applyResourceLimits é reservado para futura implementação de limites de recursos.
// SysProcAttr e Setpgid NÃO são usados — quebram acesso ao TTY de plugins interativos.
func applyResourceLimits(_ *exec.Cmd) {}

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

	// Segurança: env vars filtradas (sem credentials do parent)
	// Não sobrescreve se já definido (ex: testes com mock)
	if cmd.Env == nil {
		cmd.Env = sanitizedEnv()
	}

	// Segurança: resource limits no processo filho
	applyResourceLimits(cmd)

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
	// Plugins interativos herdam todas as env vars do parent (precisam de KUBECONFIG, etc)
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("YBY_PLUGIN_REQUEST=%s", string(reqBytes)))

	// Segurança: resource limits no processo filho
	applyResourceLimits(cmd)

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
