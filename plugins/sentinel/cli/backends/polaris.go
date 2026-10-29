//go:build k8s

package backends

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/escopo"
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
	// Silenciar logs do Polaris (usa logrus internamente) só durante a chamada: o
	// processo é o do plugin inteiro e o resto do scan precisa do log de volta.
	defer silenciarLogs()()

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

	// Os recursos sem namespace (ClusterRoles e ClusterRoleBindings dos checks de RBAC)
	// passam pelo recorte do escopo: no escopo namespace, só os que valem no namespace.
	dentro := func(string, string) bool { return true }
	var falhaDoRecorte error
	if escopo.De(ctx) == escopo.Namespace && temRecursoSemNamespace(auditData) {
		recorte, err := escopo.Ler(ctx, client, namespace)
		if err != nil {
			// Sem o recorte, não há como saber quais valem no namespace: ficam de fora, e
			// o scan diz que essa parte não rodou.
			falhaDoRecorte = err
		}
		dentro = recorte.Inclui
	}

	// Mapear resultados para o formato unificado
	findings := mapPolarisResults(auditData, namespace, dentro)
	if falhaDoRecorte != nil {
		return findings, &FalhaParcial{Falhas: []string{"RBAC do cluster"}, Causa: falhaDoRecorte}
	}
	return findings, nil
}

// temRecursoSemNamespace diz se a auditoria trouxe algum recurso sem namespace.
func temRecursoSemNamespace(auditData validator.AuditData) bool {
	for _, r := range auditData.Results {
		if r.Namespace == "" {
			return true
		}
	}
	return false
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

// silenciarLogs manda o logrus e o log padrão para io.Discard e devolve a função que
// restaura os destinos de antes.
func silenciarLogs() func() {
	logrusAntes := logrus.StandardLogger().Out
	logAntes := log.Writer()
	logrus.SetOutput(io.Discard)
	log.SetOutput(io.Discard)
	return func() {
		logrus.SetOutput(logrusAntes)
		log.SetOutput(logAntes)
	}
}

// mapPolarisResults converte os resultados da auditoria Polaris para o formato
// unificado de findings do Sentinel.
// Agrupa por check — cada check aparece uma vez com a lista de recursos afetados.
// Só entram os recursos do namespace pedido, que o Polaris pode misturar com controllers
// de outros namespaces mesmo com cfg.Namespace, e os sem namespace (um ClusterRole) que
// dentro aceita; estes saem sem namespace e com escopo cluster. dentro nil aceita
// todos. A ordem é determinística: o recurso que representa o check é o primeiro em ordem
// alfabética, e os findings saem em ordem de check e severidade, para duas rodadas
// iguais darem a mesma tela.
func mapPolarisResults(auditData validator.AuditData, namespace string, dentro func(kind, nome string) bool) []Finding {
	// Agrupar: checkID+severity → lista de recursos afetados
	type checkGroup struct {
		id             string
		escopo         escopo.Escopo
		category       string
		severity       string
		message        string
		recommendation string
		resources      map[string]bool
		// primeiro é o recurso de onde vêm a mensagem e a recomendação: o primeiro em
		// ordem alfabética, para não depender da ordem dos maps do Polaris.
		primeiro string
	}
	groups := make(map[string]*checkGroup) // chave: checkID+severity+escopo

	for _, result := range auditData.Results {
		if result.Namespace != "" && namespace != "" && result.Namespace != namespace {
			continue
		}
		esc := escopo.Namespace
		if result.Namespace == "" {
			if dentro != nil && !dentro(result.Kind, result.Name) {
				continue
			}
			esc = escopo.Cluster
		}
		resource := fmt.Sprintf("%s/%s", result.Kind, result.Name)

		// Coletar checks falhados de todos os níveis (controlador, pod, container)
		collectChecks := func(checkID string, msg validator.ResultMessage) {
			if msg.Success {
				return
			}
			cat, sev := classificarPolaris(checkID, msg)
			if sev == "info" {
				return
			}
			key := fmt.Sprintf("%s|%s|%s", checkID, sev, esc)
			g, ok := groups[key]
			if !ok {
				g = &checkGroup{
					id:             checkID,
					escopo:         esc,
					category:       cat,
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

		ns := namespace
		if g.escopo == escopo.Cluster {
			ns = ""
		}
		findings = append(findings, Finding{
			ID:             fmt.Sprintf("polaris/%s", g.id),
			Source:         "polaris",
			Severity:       g.severity,
			Category:       g.category,
			Resource:       resources[0],
			Resources:      resources,
			Namespace:      ns,
			Message:        g.message,
			Recommendation: g.recommendation,
			Scope:          string(g.escopo),
		})
	}

	return findings
}

// categoriasDeSeguranca dá a seção dos checks de Security do Polaris que não são do pod:
// RBAC, rede e secrets ficam junto dos achados do OPA e dos checks internos. Os outros
// checks de Security ficam em "pod-security".
var categoriasDeSeguranca = map[string]string{
	"clusterrolebindingClusterAdmin":      "rbac",
	"clusterrolebindingPodExecAttach":     "rbac",
	"clusterrolePodExecAttach":            "rbac",
	"rolebindingClusterAdminClusterRole":  "rbac",
	"rolebindingClusterAdminRole":         "rbac",
	"rolebindingClusterRolePodExecAttach": "rbac",
	"rolebindingRolePodExecAttach":        "rbac",
	"rolePodExecAttach":                   "rbac",
	"missingNetworkPolicy":                "network",
	"tlsSettingsMissing":                  "network",
	"sensitiveContainerEnvVar":            "secrets",
	"sensitiveConfigmapContent":           "secrets",
}

// avisosDeSegurancaMedios são os checks de Security que o Polaris marca como warning e
// que são endurecimento (defesa em profundidade), não uma porta aberta: saem como médio.
// Os outros warnings de Security, inclusive os que não estão aqui (hostPathSet,
// hostProcess, procMount e os checks customizados), saem como alto.
var avisosDeSegurancaMedios = map[string]bool{
	"automountServiceAccountToken": true,
	"hostPortSet":                  true,
	"insecureCapabilities":         true,
	"linuxHardening":               true,
	"missingNetworkPolicy":         true,
	"notReadOnlyRootFilesystem":    true,
	"tlsSettingsMissing":           true,
}

// classificarPolaris dá a categoria e a severidade do Sentinel a um check do Polaris,
// pela categoria do próprio check (ResultMessage.Category):
//   - Reliability e Efficiency vão para "reliability" e "efficiency", com danger como
//     médio e warning como baixo: não são falhas de segurança;
//   - Security (e o check sem categoria) vai para a seção de categoriasDeSeguranca, com
//     danger como crítico e warning como alto ou médio (avisosDeSegurancaMedios).
//
// ignore (ou severidade vazia) vira info, que o scan descarta.
func classificarPolaris(checkID string, msg validator.ResultMessage) (categoria, severidade string) {
	switch msg.Category {
	case "Reliability", "Efficiency":
		categoria = strings.ToLower(msg.Category)
		switch msg.Severity {
		case polarisconfig.SeverityDanger:
			return categoria, "medium"
		case polarisconfig.SeverityWarning:
			return categoria, "low"
		}
		return categoria, "info"
	}

	categoria = "pod-security"
	if c, ok := categoriasDeSeguranca[checkID]; ok {
		categoria = c
	}
	switch msg.Severity {
	case polarisconfig.SeverityDanger:
		return categoria, "critical"
	case polarisconfig.SeverityWarning:
		if avisosDeSegurancaMedios[checkID] {
			return categoria, "medium"
		}
		return categoria, "high"
	}
	return categoria, "info"
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
