package logs

import (
	"context"
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// Service define o contrato para operações de logs de pods.
type Service interface {
	// ListPods retorna os nomes dos pods no namespace fornecido.
	ListPods(ctx context.Context, namespace string) ([]string, error)
	// ListContainers retorna os nomes dos containers de um pod.
	ListContainers(ctx context.Context, namespace, pod string) ([]string, error)
	// GetLogs retorna os logs de um pod/container.
	GetLogs(ctx context.Context, opts LogOptions) (string, error)
	// StreamLogs executa kubectl logs com --follow, conectando a saída ao terminal.
	StreamLogs(ctx context.Context, opts LogOptions) error
	// DetectNamespace tenta detectar o namespace a partir do nome do pod.
	DetectNamespace(ctx context.Context, podName string) (string, error)
}

// LogOptions configura a busca de logs.
type LogOptions struct {
	Namespace string
	Pod       string
	Container string
	Follow    bool
	Tail      int
}

// logsService implementa Service usando shared.Runner + kubectl.
type logsService struct {
	runner shared.Runner
}

// NewService cria uma nova instância do serviço de logs.
func NewService(runner shared.Runner) Service {
	return &logsService{runner: runner}
}

// ListPods retorna os nomes dos pods no namespace fornecido.
func (s *logsService) ListPods(ctx context.Context, namespace string) ([]string, error) {
	out, err := s.runner.RunCombinedOutput(ctx, "kubectl", "get", "pods",
		"-n", namespace, "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeExec, "falha ao listar pods").
			WithHint(fmt.Sprintf("Verifique se o namespace '%s' existe e se kubectl está configurado.", namespace))
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	return strings.Fields(raw), nil
}

// ListContainers retorna os nomes dos containers de um pod.
func (s *logsService) ListContainers(ctx context.Context, namespace, pod string) ([]string, error) {
	out, err := s.runner.RunCombinedOutput(ctx, "kubectl", "get", "pod", pod,
		"-n", namespace, "-o", "jsonpath={.spec.containers[*].name}")
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeExec, fmt.Sprintf("falha ao listar containers do pod '%s'", pod)).
			WithHint("Verifique se o pod existe e está acessível.")
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	return strings.Fields(raw), nil
}

// GetLogs retorna os logs de um pod/container como string.
func (s *logsService) GetLogs(ctx context.Context, opts LogOptions) (string, error) {
	args := s.buildKubectlArgs(opts)

	out, err := s.runner.RunCombinedOutput(ctx, "kubectl", args...)
	if err != nil {
		return "", errors.Wrap(err, errors.ErrCodeExec, fmt.Sprintf("falha ao obter logs do pod '%s'", opts.Pod)).
			WithHint("Verifique se o pod está em execução com: kubectl get pods -n " + opts.Namespace)
	}

	return string(out), nil
}

// StreamLogs executa kubectl logs com --follow, conectando a saída ao terminal.
func (s *logsService) StreamLogs(ctx context.Context, opts LogOptions) error {
	opts.Follow = true
	args := s.buildKubectlArgs(opts)

	err := s.runner.Run(ctx, "kubectl", args...)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeExec, fmt.Sprintf("falha ao acompanhar logs do pod '%s'", opts.Pod)).
			WithHint("O stream pode ter sido interrompido. Verifique a conexão com o cluster.")
	}

	return nil
}

// DetectNamespace tenta detectar o namespace de um pod pesquisando em todos os namespaces.
func (s *logsService) DetectNamespace(ctx context.Context, podName string) (string, error) {
	out, err := s.runner.RunCombinedOutput(ctx, "kubectl", "get", "pods",
		"--all-namespaces", "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\t\"}{.metadata.namespace}{\"\\n\"}{end}")
	if err != nil {
		return "", errors.Wrap(err, errors.ErrCodeExec, "falha ao buscar pods no cluster").
			WithHint("Verifique se kubectl está configurado corretamente.")
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == podName {
			return parts[1], nil
		}
	}

	// Busca parcial (prefixo)
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && strings.HasPrefix(parts[0], podName) {
			return parts[1], nil
		}
	}

	return "", errors.New(errors.ErrCodeValidation, fmt.Sprintf("pod '%s' não encontrado em nenhum namespace", podName)).
		WithHint("Verifique o nome do pod com: kubectl get pods --all-namespaces")
}

// buildKubectlArgs constrói os argumentos do kubectl logs.
func (s *logsService) buildKubectlArgs(opts LogOptions) []string {
	args := []string{"logs", opts.Pod, "-n", opts.Namespace}

	if opts.Container != "" {
		args = append(args, "-c", opts.Container)
	}

	if opts.Follow {
		args = append(args, "--follow")
	}

	if opts.Tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", opts.Tail))
	}

	return args
}
