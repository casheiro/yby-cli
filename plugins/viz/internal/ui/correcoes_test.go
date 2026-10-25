package ui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

// clientComAcesso é o MockClient com acesso direto à API (clientset falso),
// como o K8sClient, para exercitar as ações.
type clientComAcesso struct {
	*MockClient
	cs kubernetes.Interface
}

func (c *clientComAcesso) Clientset() kubernetes.Interface { return c.cs }
func (c *clientComAcesso) RESTConfig() *rest.Config        { return nil }

// teclas manda cada tecla ao modelo e devolve o modelo e o último comando.
func teclas(t *testing.T, m Model, ks ...string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range ks {
		var nm tea.Model
		nm, cmd = m.Update(tecla(k))
		m = nm.(Model)
	}
	return m, cmd
}

// buscar aplica a busca por nome pela tecla /.
func buscar(t *testing.T, m Model, termo string) Model {
	t.Helper()
	ks := append([]string{"/"}, strings.Split(termo, "")...)
	m, _ = teclas(t, m, append(ks, "enter")...)
	return m
}

// TestSelecao_ComBuscaUsaListaDaTela: com a busca ativa, a linha selecionada
// é contada na lista que aparece na tela, não na lista completa.
func TestSelecao_ComBuscaUsaListaDaTela(t *testing.T) {
	m := NewModel(&MockClient{})
	m.activeTab = TabDeployments
	m.deployments = []monitor.Deployment{
		{Name: "api", Namespace: "loja"},
		{Name: "web", Namespace: "loja"},
		{Name: "worker", Namespace: "fila"},
	}
	m = buscar(t, m, "wor")

	nome, ns, kind := m.selectedResourceInfo()
	if nome != "worker" || ns != "fila" || kind != "deployment" {
		t.Fatalf("selecionado = %s/%s (%s), esperado fila/worker (deployment)", ns, nome, kind)
	}

	// Os pods somam o filtro de status e a busca.
	m.activeTab = TabPods
	m.pods = []monitor.Pod{
		{Name: "api-1", Namespace: "loja", Status: "Running"},
		{Name: "api-2", Namespace: "loja", Status: "Pending"},
		{Name: "worker-1", Namespace: "fila", Status: "Pending"},
	}
	m.filter.SearchQuery = ""
	m.filter.StatusFilter = "Pending"
	m = buscar(t, m, "worker")
	if nome, _, _ := m.selectedResourceInfo(); nome != "worker-1" {
		t.Fatalf("pod selecionado = %q, esperado worker-1", nome)
	}
}

// chamadas registra as chamadas à API (recurso/subrecurso e verbo; no update
// do scale, as réplicas gravadas: "update deployments/scale=3") e responde ao
// scale com um Scale vazio, que o clientset falso não monta sozinho.
func chamadas(cs *fake.Clientset) *[]string {
	var feitas []string
	cs.PrependReactor("*", "*", func(a k8stesting.Action) (bool, runtime.Object, error) {
		rec := a.GetResource().Resource
		if a.GetSubresource() != "" {
			rec += "/" + a.GetSubresource()
		}
		if u, ok := a.(k8stesting.UpdateAction); ok {
			if s, ok := u.GetObject().(*autoscalingv1.Scale); ok {
				rec += "=" + strconv.Itoa(int(s.Spec.Replicas))
			}
		}
		feitas = append(feitas, a.GetVerb()+" "+rec)
		if a.GetSubresource() == "scale" {
			return true, &autoscalingv1.Scale{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "dados"}}, nil
		}
		return false, nil, nil
	})
	return &feitas
}

// TestListaVazia_PorFiltroNaoMostraCarregando: depois que a lista chegou,
// uma lista vazia diz que nada passa nos filtros ou que não há recursos, e
// "lendo pods" fica só para antes da primeira resposta.
func TestListaVazia_PorFiltroNaoMostraCarregando(t *testing.T) {
	m := comJanela(NewModel(&MockClient{}))
	if v := m.View(); !strings.Contains(v, "lendo pods") {
		t.Fatalf("antes da primeira resposta deveria mostrar que está lendo:\n%s", v)
	}

	nm, _ := m.Update([]monitor.Pod{{Name: "api-1", Namespace: "loja"}, {Name: "api-2", Namespace: "loja"}})
	m = buscar(t, nm.(Model), "nada")
	v := m.View()
	if strings.Contains(v, "lendo") || !strings.Contains(v, "nenhum pod com esse filtro") || !strings.Contains(v, "0 de 2") {
		t.Fatalf("com filtro sem resultado:\n%s", v)
	}

	nm, _ = m.Update([]monitor.Pod{})
	m = nm.(Model)
	m.filter = FilterState{}
	v = m.View()
	if strings.Contains(v, "lendo") || !strings.Contains(v, "nenhum pod no cluster") {
		t.Fatalf("com a lista vazia no cluster:\n%s", v)
	}
}
