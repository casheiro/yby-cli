package doctor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/services/shared"
)

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

	// Tools obrigatórios
	tools := []string{"kubectl", "helm", "argocd", "git", "direnv"}
	for _, t := range tools {
		report.Tools = append(report.Tools, s.checkTool(ctx, t))
	}
	report.Tools = append(report.Tools, s.checkDockerPermissions(ctx))

	// Tools opcionais por estratégia de secrets
	report.Tools = append(report.Tools, s.checkOptionalTool(ctx, "kubeseal", "Sealed Secrets"))
	report.Tools = append(report.Tools, s.checkOptionalTool(ctx, "sops", "SOPS"))
	report.Tools = append(report.Tools, s.checkOptionalTool(ctx, "age", "age (SOPS)"))

	// Cluster
	report.Cluster = append(report.Cluster, s.checkClusterConnection(ctx))

	// CRDs
	report.CRDs = append(report.CRDs, s.checkCRD(ctx, "servicemonitors.monitoring.coreos.com", "Prometheus Operator"))
	report.CRDs = append(report.CRDs, s.checkCRD(ctx, "clusterissuers.cert-manager.io", "Cert-Manager"))
	report.CRDs = append(report.CRDs, s.checkCRD(ctx, "scaledobjects.keda.sh", "KEDA"))

	// Cloud Providers
	report.Cloud = s.checkCloudProviders(ctx)

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

func (s *doctorService) checkOptionalTool(ctx context.Context, name, label string) CheckResult {
	path, err := s.runner.LookPath(name)
	if err != nil {
		return CheckResult{Name: label, Status: false, Message: "Não encontrado (opcional)"}
	}
	return CheckResult{Name: label, Status: true, Message: path}
}

func (s *doctorService) checkCloudProviders(ctx context.Context) []CheckResult {
	providers := cloud.Detect(ctx, s.runner)
	if len(providers) == 0 {
		return []CheckResult{
			{Name: "Cloud Providers", Status: true, Message: "Nenhum provider cloud detectado"},
		}
	}

	var results []CheckResult
	for _, p := range providers {
		version, err := p.CLIVersion(ctx)
		if err != nil {
			results = append(results, CheckResult{
				Name:    p.Name(),
				Status:  false,
				Message: "CLI detectado mas não foi possível obter versão",
			})
		} else {
			results = append(results, CheckResult{
				Name:    p.Name(),
				Status:  true,
				Message: fmt.Sprintf("CLI %s instalado", version),
			})
		}

		// Verificar kubelogin para Azure
		if p.Name() == "azure" {
			type kubeloginChecker interface {
				IsKubeloginAvailable(ctx context.Context) bool
			}
			if checker, ok := p.(kubeloginChecker); ok {
				if checker.IsKubeloginAvailable(ctx) {
					results = append(results, CheckResult{
						Name:    "azure kubelogin",
						Status:  true,
						Message: "kubelogin instalado (necessário para autenticação AAD/Entra ID)",
					})
				} else {
					results = append(results, CheckResult{
						Name:    "azure kubelogin",
						Status:  false,
						Message: "kubelogin não encontrado. Instale para autenticação AAD/Entra ID em clusters AKS",
					})
				}
			}
		}

		// Validar credenciais com timeout de 10s
		credCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		credStatus, err := p.ValidateCredentials(credCtx)
		cancel()

		if err != nil {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("%s credenciais", p.Name()),
				Status:  false,
				Message: "Não foi possível validar credenciais",
			})
			continue
		}

		if credStatus.Authenticated {
			msg := fmt.Sprintf("Autenticado como %s", credStatus.Identity)
			if credStatus.ExpiresAt != nil && credStatus.ExpiresAt.Before(time.Now()) {
				results = append(results, CheckResult{
					Name:    fmt.Sprintf("%s credenciais", p.Name()),
					Status:  false,
					Message: "Token expirado. Execute 'yby cloud refresh'",
				})
			} else {
				results = append(results, CheckResult{
					Name:    fmt.Sprintf("%s credenciais", p.Name()),
					Status:  true,
					Message: msg,
				})
			}
		} else {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("%s credenciais", p.Name()),
				Status:  false,
				Message: "Não autenticado. Execute 'yby cloud auth'",
			})
		}
	}

	return results
}

func (s *doctorService) checkCRD(ctx context.Context, crdName, readableName string) CheckResult {
	err := s.runner.Run(ctx, "kubectl", "get", "crd", crdName)
	if err != nil {
		return CheckResult{Name: readableName, Status: false, Message: "Ausente (CRD não instalado)"}
	}
	return CheckResult{Name: readableName, Status: true, Message: "Instalado"}
}
