//go:build k8s

package backends

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	polarisconfig "github.com/fairwindsops/polaris/pkg/config"
	"github.com/fairwindsops/polaris/pkg/validator"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// TestPolarisBackendName verifica que o nome do backend é "polaris".
func TestPolarisBackendName(t *testing.T) {
	backend := NewPolarisBackend(nil)
	if backend.Name() != "polaris" {
		t.Errorf("esperado Name() = \"polaris\", obtido %q", backend.Name())
	}
}

// TestPolarisBackendIsAvailable verifica que o backend está sempre disponível.
func TestPolarisBackendIsAvailable(t *testing.T) {
	backend := NewPolarisBackend(nil)
	if !backend.IsAvailable() {
		t.Error("esperado IsAvailable() = true, obtido false")
	}
}

// TestPolarisBackendImplementsInterface verifica que PolarisBackend
// satisfaz a interface SecurityBackend em tempo de compilação.
func TestPolarisBackendImplementsInterface(t *testing.T) {
	var _ SecurityBackend = (*PolarisBackend)(nil)
}

// TestClassificarPolaris verifica a categoria e a severidade que cada check do Polaris
// recebe: Security danger é crítico, Security warning é alto ou médio pela tabela, e
// Reliability e Efficiency são médio (danger) ou baixo (warning), nas seções próprias.
func TestClassificarPolaris(t *testing.T) {
	tests := []struct {
		check, categoria, severidade string
		esperaCategoria, esperaSev   string
	}{
		{"priorityClassNotSet", "Reliability", "warning", "reliability", "low"},
		{"tagNotSpecified", "Reliability", "danger", "reliability", "medium"},
		{"memoryLimitsMissing", "Efficiency", "warning", "efficiency", "low"},
		{"cpuLimitsMissing", "Efficiency", "danger", "efficiency", "medium"},
		{"runAsRootAllowed", "Security", "danger", "pod-security", "critical"},
		{"hostPathSet", "Security", "warning", "pod-security", "high"},
		{"procMount", "Security", "warning", "pod-security", "high"},
		{"linuxHardening", "Security", "warning", "pod-security", "medium"},
		{"automountServiceAccountToken", "Security", "warning", "pod-security", "medium"},
		{"clusterrolebindingClusterAdmin", "Security", "danger", "rbac", "critical"},
		{"rolePodExecAttach", "Security", "danger", "rbac", "critical"},
		{"missingNetworkPolicy", "Security", "warning", "network", "medium"},
		{"tlsSettingsMissing", "Security", "warning", "network", "medium"},
		{"sensitiveContainerEnvVar", "Security", "danger", "secrets", "critical"},
		// check de segurança fora da tabela (customizado): alto, como antes
		{"meuCheckCustomizado", "Security", "warning", "pod-security", "high"},
		// sem categoria: tratado como segurança
		{"semCategoria", "", "danger", "pod-security", "critical"},
		{"runAsRootAllowed", "Security", "ignore", "pod-security", "info"},
		{"priorityClassNotSet", "Reliability", "", "reliability", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.check+"/"+tt.categoria+"/"+tt.severidade, func(t *testing.T) {
			cat, sev := classificarPolaris(tt.check, validator.ResultMessage{
				ID: tt.check, Category: tt.categoria, Severity: polarisconfig.Severity(tt.severidade),
			})
			if cat != tt.esperaCategoria || sev != tt.esperaSev {
				t.Errorf("classificarPolaris(%s, %s, %s) = %s/%s, esperado %s/%s",
					tt.check, tt.categoria, tt.severidade, cat, sev, tt.esperaCategoria, tt.esperaSev)
			}
		})
	}
}

// TestMapPolarisResults_CategoriaESeveridadeReais: o achado leva a categoria e a
// severidade do check, e não "pod-security" com danger crítico e warning alto.
func TestMapPolarisResults_CategoriaESeveridadeReais(t *testing.T) {
	audit := validator.AuditData{Results: []validator.Result{resultadoPolaris("Deployment", "api", "payments")}}
	esperado := map[string][2]string{
		"polaris/deploymentMissingReplicas": {"reliability", "low"},
		"polaris/runAsRootAllowed":          {"pod-security", "critical"},
	}
	achados := mapPolarisResults(audit, "payments", nil)
	if len(achados) != len(esperado) {
		t.Fatalf("esperava %d findings, obtido %d: %+v", len(esperado), len(achados), achados)
	}
	for _, f := range achados {
		if e := esperado[f.ID]; f.Category != e[0] || f.Severity != e[1] {
			t.Errorf("%s: %s/%s, esperado %s/%s", f.ID, f.Category, f.Severity, e[0], e[1])
		}
	}
}

// TestFormatRecommendation verifica a formatação de detalhes como recomendação.
func TestFormatRecommendation(t *testing.T) {
	tests := []struct {
		name     string
		details  []string
		expected string
	}{
		{"vazio", nil, ""},
		{"um detalhe", []string{"ajustar limites de CPU"}, "ajustar limites de CPU"},
		{"multiplos detalhes", []string{"limitar CPU", "definir memoria"}, "limitar CPU; definir memoria"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRecommendation(tt.details)
			if got != tt.expected {
				t.Errorf("formatRecommendation(%v) = %q, esperado %q", tt.details, got, tt.expected)
			}
		})
	}
}

// resultadoPolaris monta um resultado do Polaris com um check que falhou no controller e
// outro no contêiner.
func resultadoPolaris(kind, nome, namespace string) validator.Result {
	falha := func(id, categoria string, sev polarisconfig.Severity) validator.ResultMessage {
		return validator.ResultMessage{ID: id, Message: "falhou " + id, Severity: sev, Category: categoria}
	}
	return validator.Result{
		Kind: kind, Name: nome, Namespace: namespace,
		Results: validator.ResultSet{"deploymentMissingReplicas": falha("deploymentMissingReplicas", "Reliability", polarisconfig.SeverityWarning)},
		PodResult: &validator.PodResult{
			ContainerResults: []validator.ContainerResult{{
				Name:    "app",
				Results: validator.ResultSet{"runAsRootAllowed": falha("runAsRootAllowed", "Security", polarisconfig.SeverityDanger)},
			}},
		},
	}
}

// TestMapPolarisResults_SoONamespacePedidoEEmOrdem: os recursos de outros namespaces
// ficam de fora, o recurso que representa o check é o primeiro em ordem alfabética e a
// saída é a mesma em toda rodada.
func TestMapPolarisResults_SoONamespacePedidoEEmOrdem(t *testing.T) {
	audit := validator.AuditData{Results: []validator.Result{
		resultadoPolaris("Deployment", "worker", "payments"),
		resultadoPolaris("Deployment", "coredns", "kube-system"),
		resultadoPolaris("Deployment", "api", "payments"),
		resultadoPolaris("Deployment", "local-path-provisioner", "kube-system"),
		resultadoPolaris("Job", "migrate", "payments"),
	}}
	primeira := mapPolarisResults(audit, "payments", nil)
	if len(primeira) != 2 {
		t.Fatalf("esperava 2 findings (um por check), obtido %d: %+v", len(primeira), primeira)
	}
	for _, f := range primeira {
		if f.Resource != "Deployment/api" {
			t.Errorf("%s: o recurso é o primeiro em ordem, sem contagem, obtido %q", f.ID, f.Resource)
		}
		if want := []string{"Deployment/api", "Deployment/worker", "Job/migrate"}; !reflect.DeepEqual(f.Resources, want) {
			t.Errorf("%s: Resources = %v, esperado a lista inteira %v", f.ID, f.Resources, want)
		}
		if strings.Contains(f.Resource, "coredns") || strings.Contains(f.Resource, "local-path") || f.Namespace != "payments" {
			t.Errorf("%s: recurso de outro namespace no scan de payments: %+v", f.ID, f)
		}
	}
	if primeira[0].ID != "polaris/deploymentMissingReplicas" || primeira[1].ID != "polaris/runAsRootAllowed" {
		t.Errorf("findings fora de ordem: %s, %s", primeira[0].ID, primeira[1].ID)
	}
	for i := 0; i < 20; i++ {
		if outra := mapPolarisResults(audit, "payments", nil); !reflect.DeepEqual(outra, primeira) {
			t.Fatalf("rodada %d diferente:\n%+v\n%+v", i, outra, primeira)
		}
	}
}

// TestPolarisBackend_UsaORestConfigInjetado: o cliente dinâmico sai do config recebido,
// e não do kubeconfig padrão (KUBECONFIG aponta para um arquivo que não existe).
func TestPolarisBackend_UsaORestConfigInjetado(t *testing.T) {
	t.Setenv("KUBECONFIG", t.TempDir()+"/nao-existe")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	chamado := false
	b := NewPolarisBackend(func() (*rest.Config, error) {
		chamado = true
		return &rest.Config{Host: "https://cluster-b.example:6443"}, nil
	})
	if _, err := b.clienteDinamico(); err != nil {
		t.Fatalf("clienteDinamico() com config injetado falhou: %v", err)
	}
	if !chamado {
		t.Error("o config injetado não foi usado")
	}
}

// TestPolarisBackend_ErroDoRestConfigPropaga: se o config do cluster não sai, o scan do
// Polaris falha em vez de auditar outro cluster.
func TestPolarisBackend_ErroDoRestConfigPropaga(t *testing.T) {
	falha := errors.New("contexto b inexistente")
	b := NewPolarisBackend(func() (*rest.Config, error) { return nil, falha })
	_, err := b.ScanCluster(context.Background(), fake.NewSimpleClientset(), "default")
	if !errors.Is(err, falha) {
		t.Fatalf("esperava o erro do config, veio %v", err)
	}
}

// TestMapPolarisResults_RBACDoClusterSoNoEscopo: os checks de RBAC do Polaris sobre
// ClusterRoles e ClusterRoleBindings (Namespace vazio) só entram no scan do namespace
// quando o recorte do RBAC os inclui, e saem sem namespace e com escopo cluster.
func TestMapPolarisResults_RBACDoClusterSoNoEscopo(t *testing.T) {
	falha := validator.ResultMessage{ID: "clusterrolePodExecAttach", Message: "exec", Severity: polarisconfig.SeverityWarning, Category: "Security"}
	papel := func(nome string) validator.Result {
		return validator.Result{Kind: "ClusterRole", Name: nome, Results: validator.ResultSet{"clusterrolePodExecAttach": falha}}
	}
	audit := validator.AuditData{Results: []validator.Result{papel("do-payments"), papel("de-outro")}}

	achados := mapPolarisResults(audit, "payments", func(kind, nome string) bool { return nome == "do-payments" })
	if len(achados) != 1 {
		t.Fatalf("esperava só o ClusterRole do recorte, obtido %+v", achados)
	}
	f := achados[0]
	if f.Resource != "ClusterRole/do-payments" || f.Namespace != "" || f.Scope != "cluster" {
		t.Errorf("ClusterRole no scan de payments: %+v, queria sem namespace e com escopo cluster", f)
	}
}
