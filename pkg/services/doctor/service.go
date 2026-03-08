package doctor

import (
	"context"
	"strings"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

type CheckResult struct {
	Name    string
	Status  bool
	Message string
}

type DoctorReport struct {
	System  []CheckResult
	Tools   []CheckResult
	Cluster []CheckResult
	CRDs    []CheckResult
}

type Service interface {
	Run(ctx context.Context) *DoctorReport
}

type doctorService struct {
	runner shared.Runner
}

func NewService(runner shared.Runner) Service {
	return &doctorService{runner: runner}
}

func (s *doctorService) Run(ctx context.Context) *DoctorReport {
	report := &DoctorReport{}

	// System Resources
	report.System = append(report.System, s.checkSystemResources(ctx))

	// Tools
	tools := []string{"kubectl", "helm", "kubeseal", "argocd", "git", "direnv"}
	for _, t := range tools {
		report.Tools = append(report.Tools, s.checkTool(ctx, t))
	}
	report.Tools = append(report.Tools, s.checkDockerPermissions(ctx))

	// Cluster
	report.Cluster = append(report.Cluster, s.checkClusterConnection(ctx))

	// CRDs
	report.CRDs = append(report.CRDs, s.checkCRD(ctx, "servicemonitors.monitoring.coreos.com", "Prometheus Operator"))
	report.CRDs = append(report.CRDs, s.checkCRD(ctx, "clusterissuers.cert-manager.io", "Cert-Manager"))
	report.CRDs = append(report.CRDs, s.checkCRD(ctx, "scaledobjects.keda.sh", "KEDA"))

	return report
}

func (s *doctorService) checkSystemResources(ctx context.Context) CheckResult {
	out, err := s.runner.RunCombinedOutput(ctx, "grep", "MemTotal", "/proc/meminfo")
	if err == nil {
		mem := strings.TrimSpace(strings.Replace(string(out), "MemTotal:", "", 1))
		return CheckResult{Name: "Memória", Status: true, Message: mem}
	}
	// Mac/Other fallback
	return CheckResult{Name: "Memória", Status: false, Message: "Verificação detalhada ignorada (OS não Linux)"}
}

func (s *doctorService) checkTool(ctx context.Context, name string) CheckResult {
	path, err := s.runner.LookPath(name)
	if err != nil {
		return CheckResult{Name: name, Status: false, Message: "Não encontrado"}
	}
	return CheckResult{Name: name, Status: true, Message: path}
}

func (s *doctorService) checkDockerPermissions(ctx context.Context) CheckResult {
	err := s.runner.Run(ctx, "docker", "info")
	if err != nil {
		return CheckResult{Name: "docker", Status: false, Message: "Erro de permissão ou não rodando (tente 'sudo' ou adicione user ao grupo docker)"}
	}
	return CheckResult{Name: "docker", Status: true, Message: "Daemon acessível"}
}

func (s *doctorService) checkClusterConnection(ctx context.Context) CheckResult {
	err := s.runner.Run(ctx, "kubectl", "--insecure-skip-tls-verify", "get", "nodes")
	if err != nil {
		return CheckResult{Name: "Conexão", Status: false, Message: "Falha ao conectar. Dica: Verifique seu KUBECONFIG ou se o cluster está rodando."}
	}
	return CheckResult{Name: "Conexão", Status: true, Message: "OK"}
}

func (s *doctorService) checkCRD(ctx context.Context, crdName, readableName string) CheckResult {
	err := s.runner.Run(ctx, "kubectl", "get", "crd", crdName)
	if err != nil {
		return CheckResult{Name: readableName, Status: false, Message: "Ausente (CRD não instalado)"}
	}
	return CheckResult{Name: readableName, Status: true, Message: "Instalado"}
}
