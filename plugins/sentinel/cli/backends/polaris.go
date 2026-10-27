//go:build k8s

package backends

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"

	polarisconfig "github.com/fairwindsops/polaris/pkg/config"
	"github.com/fairwindsops/polaris/pkg/kube"
	"github.com/fairwindsops/polaris/pkg/validator"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// PolarisBackend implementa SecurityBackend usando a SDK do Polaris
// para auditoria de segurança de workloads Kubernetes.
type PolarisBackend struct {
	// restConfig entrega o config do mesmo cluster do clientset do scan (kubeconfig e
	// contexto do yby, token cloud), para o cliente dinâmico não cair no kubeconfig padrão.
	restConfig func() (*rest.Config, error)
}

// NewPolarisBackend cria uma nova instância do backend Polaris. restConfig devolve o
// config do cluster que o scan está auditando.
func NewPolarisBackend(restConfig func() (*rest.Config, error)) *PolarisBackend {
	return &PolarisBackend{restConfig: restConfig}
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

	// Criar cliente dinâmico a partir do config do mesmo cluster do clientset
	dynamicClient, err := p.clienteDinamico()
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

// clienteDinamico cria o cliente dinâmico Kubernetes a partir do config injetado.
func (p *PolarisBackend) clienteDinamico() (dynamic.Interface, error) {
	if p.restConfig == nil {
		return nil, fmt.Errorf("backend polaris sem config do cluster")
	}
	restConfig, err := p.restConfig()
	if err != nil {
		return nil, fmt.Errorf("falha ao obter rest config: %w", err)
	}
	return dynamic.NewForConfig(restConfig)
}

// mapPolarisResults converte os resultados da auditoria Polaris para o formato
// unificado de findings do Sentinel.
// Agrupa por check — cada check aparece uma vez com a lista de recursos afetados.
// Só entram os recursos do namespace pedido (e os sem namespace, como um ClusterRole):
// o Polaris pode trazer controllers de outros namespaces mesmo com cfg.Namespace. A
// ordem é determinística: o recurso que representa o check é o primeiro em ordem
// alfabética, e os findings saem em ordem de check e severidade, para duas rodadas
// iguais darem a mesma tela.
func mapPolarisResults(auditData validator.AuditData, namespace string) []Finding {
	// Agrupar: checkID+severity → lista de recursos afetados
	type checkGroup struct {
		id             string
		severity       string
		message        string
		recommendation string
		resources      map[string]bool
		// primeiro é o recurso de onde vêm a mensagem e a recomendação: o primeiro em
		// ordem alfabética, para não depender da ordem dos maps do Polaris.
		primeiro string
	}
	groups := make(map[string]*checkGroup) // chave: checkID+severity

	for _, result := range auditData.Results {
		if result.Namespace != "" && namespace != "" && result.Namespace != namespace {
			continue
		}
		resource := fmt.Sprintf("%s/%s", result.Kind, result.Name)

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
					primeiro:       resource,
				}
				groups[key] = g
			} else if resource < g.primeiro || (resource == g.primeiro && msg.Message < g.message) {
				g.primeiro, g.message, g.recommendation = resource, msg.Message, formatRecommendation(msg.Details)
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

	chaves := make([]string, 0, len(groups))
	for k := range groups {
		chaves = append(chaves, k)
	}
	sort.Strings(chaves)

	// Converter grupos em findings — 1 finding por check
	var findings []Finding
	for _, k := range chaves {
		g := groups[k]
		// Listar recursos afetados, em ordem
		resources := make([]string, 0, len(g.resources))
		for r := range g.resources {
			resources = append(resources, r)
		}
		sort.Strings(resources)

		findings = append(findings, Finding{
			ID:             fmt.Sprintf("polaris/%s", g.id),
			Source:         "polaris",
			Severity:       g.severity,
			Category:       "pod-security",
			Resource:       resources[0],
			Resources:      resources,
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
