//go:build k8s

package backends

import (
	"context"
	"fmt"
	"io"
	"log"

	polarisconfig "github.com/fairwindsops/polaris/pkg/config"
	"github.com/fairwindsops/polaris/pkg/kube"
	"github.com/fairwindsops/polaris/pkg/validator"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// PolarisBackend implementa SecurityBackend usando a SDK do Polaris
// para auditoria de segurança de workloads Kubernetes.
type PolarisBackend struct{}

// NewPolarisBackend cria uma nova instância do backend Polaris.
func NewPolarisBackend() *PolarisBackend {
	return &PolarisBackend{}
}

// Name retorna o identificador do backend.
func (p *PolarisBackend) Name() string {
	return "polaris"
}

// IsAvailable retorna true pois o Polaris é uma biblioteca importada,
// sempre disponível quando compilado com a build tag k8s.
func (p *PolarisBackend) IsAvailable() bool {
	return true
}

// ScanCluster executa uma auditoria Polaris no namespace especificado
// e retorna os findings mapeados para o formato unificado.
func (p *PolarisBackend) ScanCluster(ctx context.Context, client kubernetes.Interface, namespace string) ([]Finding, error) {
	// Silenciar logs do Polaris (usa logrus internamente)
	logrus.SetOutput(io.Discard)
	log.SetOutput(io.Discard)

	// Carregar configuração padrão do Polaris
	cfg, err := polarisconfig.MergeConfigAndParseFile("", false)
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar configuração padrão do polaris: %w", err)
	}

	// Definir namespace na configuração para filtrar recursos
	cfg.Namespace = namespace

	// Criar cliente dinâmico a partir do kubeconfig padrão
	dynamicClient, err := buildDynamicClient()
	if err != nil {
		return nil, fmt.Errorf("falha ao criar cliente dinâmico kubernetes: %w", err)
	}

	// Criar resource provider a partir da API do cluster
	resourceProvider, err := kube.CreateResourceProviderFromAPI(ctx, client, "", dynamicClient, cfg)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar resource provider do polaris: %w", err)
	}

	// Executar auditoria
	auditData, err := validator.RunAudit(ctx, cfg, resourceProvider)
	if err != nil {
		return nil, fmt.Errorf("falha ao executar auditoria polaris: %w", err)
	}

	// Mapear resultados para o formato unificado
	return mapPolarisResults(auditData, namespace), nil
}

// buildDynamicClient cria um cliente dinâmico Kubernetes usando as regras
// de carregamento padrão do kubeconfig.
func buildDynamicClient() (dynamic.Interface, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("falha ao obter rest config: %w", err)
	}

	return dynamic.NewForConfig(restConfig)
}

// mapPolarisResults converte os resultados da auditoria Polaris para o formato
// unificado de findings do Sentinel.
// Agrupa por check — cada check aparece uma vez com a lista de recursos afetados.
func mapPolarisResults(auditData validator.AuditData, namespace string) []Finding {
	// Agrupar: checkID+severity → lista de recursos afetados
	type checkGroup struct {
		id             string
		severity       string
		message        string
		recommendation string
		resources      map[string]bool
	}
	groups := make(map[string]*checkGroup) // chave: checkID+severity

	for _, result := range auditData.Results {
		resource := fmt.Sprintf("%s/%s", result.Kind, result.Name)
		resultNamespace := result.Namespace
		if resultNamespace == "" {
			resultNamespace = namespace
		}
		_ = resultNamespace

		// Coletar checks falhados de todos os níveis (controlador, pod, container)
		collectChecks := func(checkID string, msg validator.ResultMessage) {
			if msg.Success {
				return
			}
			sev := mapPolarisSeverity(msg.Severity)
			if sev == "info" {
				return
			}
			key := fmt.Sprintf("%s|%s", checkID, sev)
			g, ok := groups[key]
			if !ok {
				g = &checkGroup{
					id:             checkID,
					severity:       sev,
					message:        msg.Message,
					recommendation: formatRecommendation(msg.Details),
					resources:      make(map[string]bool),
				}
				groups[key] = g
			}
			g.resources[resource] = true
		}

		for checkID, msg := range result.Results {
			collectChecks(checkID, msg)
		}
		if result.PodResult != nil {
			for checkID, msg := range result.PodResult.Results {
				collectChecks(checkID, msg)
			}
			for _, cr := range result.PodResult.ContainerResults {
				for checkID, msg := range cr.Results {
					collectChecks(checkID, msg)
				}
			}
		}
	}

	// Converter grupos em findings — 1 finding por check
	var findings []Finding
	for _, g := range groups {
		// Listar recursos afetados
		var resources []string
		for r := range g.resources {
			resources = append(resources, r)
		}

		resourceStr := resources[0]
		if len(resources) > 1 {
			resourceStr = fmt.Sprintf("%s (+%d)", resources[0], len(resources)-1)
		}

		findings = append(findings, Finding{
			ID:             fmt.Sprintf("polaris/%s", g.id),
			Source:         "polaris",
			Severity:       g.severity,
			Category:       "pod-security",
			Resource:       resourceStr,
			Namespace:      namespace,
			Message:        g.message,
			Recommendation: g.recommendation,
		})
	}

	return findings
}

// mapPolarisSeverity converte a severidade do Polaris para o formato do Sentinel.
// Mapeamento: danger → critical, warning → high, ignore → info.
func mapPolarisSeverity(severity polarisconfig.Severity) string {
	switch severity {
	case polarisconfig.SeverityDanger:
		return "critical"
	case polarisconfig.SeverityWarning:
		return "high"
	default:
		return "info"
	}
}

// formatRecommendation formata os detalhes de um check como recomendação legível.
func formatRecommendation(details []string) string {
	if len(details) == 0 {
		return ""
	}
	rec := ""
	for i, d := range details {
		if i > 0 {
			rec += "; "
		}
		rec += d
	}
	return rec
}
