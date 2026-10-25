package status

import (
	"context"
	"strings"
)

// ComponentStatus representa o resultado de uma verificação individual de componente.
type ComponentStatus struct {
	Available bool   // indica se o componente está disponível/acessível
	Output    string // saída textual do comando kubectl
	Message   string // mensagem de erro ou aviso quando não disponível
}

// MensagemKeplerAtivo é a mensagem do Kepler com o sensor rodando.
const MensagemKeplerAtivo = "sensor Kepler ativo e monitorando o cluster"

// StatusReport agrupa o resultado de todas as verificações do cluster.
type StatusReport struct {
	// SemCluster indica que a leitura dos nodes falhou: as demais verificações não rodam.
	SemCluster bool
	Nodes      ComponentStatus
	ArgoCD     ComponentStatus
	Ingress    ComponentStatus
	KEDA       ComponentStatus
	Kepler     ComponentStatus
}

// Service define o contrato para verificação de status do cluster.
type Service interface {
	Check(ctx context.Context) *StatusReport
}

// statusService implementa Service usando um ClusterInspector.
type statusService struct {
	inspector ClusterInspector
}

// NewService cria uma nova instância do serviço de status.
func NewService(inspector ClusterInspector) Service {
	return &statusService{
		inspector: inspector,
	}
}

// Check executa todas as verificações de status do cluster de forma resiliente.
// Erros são registrados no campo Message; só a falha nos nodes interrompe a verificação,
// porque sem acesso ao cluster as demais consultas falhariam pelo mesmo motivo.
func (s *statusService) Check(ctx context.Context) *StatusReport {
	report := &StatusReport{}

	// Nodes
	report.Nodes = s.checkNodes(ctx)
	if !report.Nodes.Available {
		report.SemCluster = true
		return report
	}

	// Argo CD
	report.ArgoCD = s.checkArgoCD(ctx)

	// Ingresses
	report.Ingress = s.checkIngresses(ctx)

	// KEDA
	report.KEDA = s.checkKEDA(ctx)

	// Kepler
	report.Kepler = s.checkKepler(ctx)

	return report
}

func (s *statusService) checkNodes(ctx context.Context) ComponentStatus {
	out, err := s.inspector.GetNodes(ctx)
	if err != nil {
		return ComponentStatus{
			Available: false,
			Output:    out,
			Message:   "sem acesso ao cluster",
		}
	}
	return ComponentStatus{
		Available: true,
		Output:    out,
	}
}

func (s *statusService) checkArgoCD(ctx context.Context) ComponentStatus {
	out, err := s.inspector.GetArgoCDPods(ctx)
	if err != nil {
		return ComponentStatus{
			Available: false,
			Message:   "namespace argocd não encontrado ou vazio",
		}
	}
	return ComponentStatus{
		Available: true,
		Output:    out,
	}
}

func (s *statusService) checkIngresses(ctx context.Context) ComponentStatus {
	out, err := s.inspector.GetIngresses(ctx)
	if err != nil {
		return ComponentStatus{
			Available: false,
			Message:   "erro ao obter os ingresses",
		}
	}
	if out == "" {
		return ComponentStatus{
			Available: true,
			Output:    "",
			Message:   "nenhum ingress encontrado",
		}
	}
	return ComponentStatus{
		Available: true,
		Output:    out,
	}
}

func (s *statusService) checkKEDA(ctx context.Context) ComponentStatus {
	out, err := s.inspector.GetScaledObjects(ctx)
	if err != nil {
		return ComponentStatus{
			Available: false,
			Message:   "KEDA não detectado · CRDs ausentes?",
		}
	}
	if out == "" {
		return ComponentStatus{
			Available: true,
			Output:    "",
			Message:   "nenhum ScaledObject · KEDA ativo, sem regras",
		}
	}
	return ComponentStatus{
		Available: true,
		Output:    out,
	}
}

func (s *statusService) checkKepler(ctx context.Context) ComponentStatus {
	out, err := s.inspector.GetKeplerPods(ctx)
	if err != nil || out == "" {
		return ComponentStatus{
			Available: false,
			Message:   "sensor Kepler não encontrado no namespace kepler",
		}
	}
	if strings.Contains(out, "Running") {
		return ComponentStatus{
			Available: true,
			Output:    out,
			Message:   MensagemKeplerAtivo,
		}
	}
	return ComponentStatus{
		Available: true,
		Output:    out,
		Message:   "sensor Kepler instalado, mas fora do estado Running",
	}
}
