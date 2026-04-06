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

// StatusReport agrupa o resultado de todas as verificações do cluster.
type StatusReport struct {
	Nodes   ComponentStatus
	ArgoCD  ComponentStatus
	Ingress ComponentStatus
	KEDA    ComponentStatus
	Kepler  ComponentStatus
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
// Nenhum erro individual interrompe a verificação; erros são registrados no campo Message.
func (s *statusService) Check(ctx context.Context) *StatusReport {
	report := &StatusReport{}

	// Nodes
	report.Nodes = s.checkNodes(ctx)

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
			Message:   "Erro ao obter nodes (Cluster rodando?)",
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
			Message:   "Namespace argocd não encontrado ou vazio.",
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
			Message:   "Erro ao obter ingresses.",
		}
	}
	if out == "" {
		return ComponentStatus{
			Available: true,
			Output:    "",
			Message:   "Nenhum ingress encontrado.",
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
			Message:   "KEDA não detectado (CRDs ausentes?)",
		}
	}
	if out == "" {
		return ComponentStatus{
			Available: true,
			Output:    "",
			Message:   "Nenhum ScaledObject encontrado (KEDA ativo mas sem regras).",
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
			Message:   "Sensor Kepler não encontrado no namespace 'kepler'.",
		}
	}
	if strings.Contains(out, "Running") {
		return ComponentStatus{
			Available: true,
			Output:    out,
			Message:   "Sensor Kepler ATIVO e monitorando o cluster.",
		}
	}
	return ComponentStatus{
		Available: true,
		Output:    out,
		Message:   "Sensor Kepler instalado mas não está 'Running'.",
	}
}
