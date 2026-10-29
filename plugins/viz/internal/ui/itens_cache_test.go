package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	yui "github.com/casheiro/yby-cli/v5/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// contarFiltragens liga o contador de filtrarEOrdenar por aba até o fim do teste.
func contarFiltragens(t *testing.T) *[tabCount]int {
	t.Helper()
	var n [tabCount]int
	aoFiltrar = func(tab ResourceTab) { n[tab]++ }
	t.Cleanup(func() { aoFiltrar = nil })
	return &n
}

// total soma as filtragens de todas as abas.
func total(n *[tabCount]int) int {
	s := 0
	for _, v := range n {
		s += v
	}
	return s
}

// modeloComMuitos é o Viz com milhares de pods e eventos, na aba de pods, com
// o painel ao lado.
func modeloComMuitos(t *testing.T) Model {
	t.Helper()
	m := NewModel(&MockClient{})
	m.agora = func() time.Time { return horaDaPrancha }
	m.tique = nil
	m.filter.Namespace = ""
	for _, msg := range []tea.Msg{
		tea.WindowSizeMsg{Width: 160, Height: 45},
		podsDoClusterGrande(podsGrandes),
		eventosDoClusterGrande(eventosGrandes),
	} {
		m, _ = atualizar(m, msg)
	}
	return m
}

// TestItensDaAba_UmaVezPorQuadro: o quadro filtra e ordena cada aba no máximo
// uma vez (a contagem das abas, a lista, o painel e a legenda usam a mesma
// lista), e o quadro seguinte sem mudança nenhuma, ou depois de uma tecla que
// só move a seleção, não filtra de novo.
func TestItensDaAba_UmaVezPorQuadro(t *testing.T) {
	m := modeloComMuitos(t)
	n := contarFiltragens(t)
	_ = m.View()
	for tab, v := range n {
		if v > 1 {
			t.Errorf("o quadro filtrou a aba %s %d vezes; esperava no máximo 1", ResourceTab(tab), v)
		}
	}

	*n = [tabCount]int{}
	_ = m.View()
	m, _ = atualizar(m, tea.KeyMsg{Type: tea.KeyDown})
	_ = m.View()
	if s := total(n); s != 0 {
		t.Errorf("sem mudança na lista nem nos filtros, nada deveria ser filtrado de novo; foram %d filtragens (%v)", s, *n)
	}

	// Uma lista nova de pods refiltra só os pods, uma vez.
	*n = [tabCount]int{}
	m, _ = atualizar(m, podsDoClusterGrande(podsGrandes))
	_ = m.View()
	if n[TabPods] != 1 || total(n) != 1 {
		t.Errorf("a lista nova de pods deveria refiltrar só os pods, uma vez; foi %v", *n)
	}
}

// TestItensDaAba_SegueAsMudancas: a lista guardada nunca fica velha: lista
// nova (inclusive a que cresce no mesmo vetor), busca, label, status, a hora
// (idade e reinício recente) e o modo visual refazem as linhas.
func TestItensDaAba_SegueAsMudancas(t *testing.T) {
	semCor(t)
	agora := horaDaPrancha
	m := NewModel(&MockClient{})
	m.agora = func() time.Time { return agora }
	m.carregadas = 1 << uint(TabPods)
	pods := make([]monitor.Pod, 2, 4)
	pods[0] = monitor.Pod{Name: "api-1", Namespace: "loja", Status: "Running", Prontos: 1, Containers: 1,
		CriadoEm: agora.Add(-50 * time.Second), Labels: map[string]string{"app": "api"}}
	pods[1] = monitor.Pod{Name: "web-1", Namespace: "loja", Status: "Pending", Containers: 1,
		CriadoEm: agora.Add(-time.Hour), Labels: map[string]string{"app": "web"}}
	m.pods = pods
	nomes := func() string {
		var s []string
		for _, it := range m.itensDaAba(TabPods) {
			s = append(s, it.nome)
		}
		return strings.Join(s, ",")
	}
	if got := nomes(); got != "web-1,api-1" {
		t.Fatalf("lista inicial = %q", got)
	}

	// O vetor cresce no lugar (append com capacidade sobrando).
	m.pods = append(m.pods, monitor.Pod{Name: "db-0", Namespace: "loja", Status: "Running", Prontos: 1, Containers: 1})
	if got := nomes(); got != "web-1,api-1,db-0" {
		t.Errorf("depois do append = %q", got)
	}

	m.filter.SearchQuery = "api"
	if got := nomes(); got != "api-1" {
		t.Errorf("com a busca = %q", got)
	}
	m.filter.SearchQuery = ""
	m.filter.LabelSelector = "app=web"
	if got := nomes(); got != "web-1" {
		t.Errorf("com o label = %q", got)
	}
	m.filter.LabelSelector = ""
	m.filter.StatusFilter = "Running"
	if got := nomes(); got != "api-1,db-0" {
		t.Errorf("com o status = %q", got)
	}
	m.filter.StatusFilter = ""

	// A hora anda: a idade do api-1 passa de 50s para 1m.
	idade := func() string { return m.itensDaAba(TabPods)[1].celulas[4] }
	if got := idade(); got != "50s" {
		t.Fatalf("idade = %q, esperava 50s", got)
	}
	agora = agora.Add(10 * time.Second)
	if got := idade(); got != "1m" {
		t.Errorf("a idade deveria andar com a hora: %q", got)
	}

	// O modo visual muda os glifos da coluna de status.
	antes := m.itensDaAba(TabPods)[0].celulas[2]
	anterior := yui.Atual()
	t.Cleanup(func() { yui.Configurar(anterior) })
	yui.Configurar(yui.Opcoes{Tema: anterior.Tema, ASCII: !anterior.ASCII, SemCor: anterior.SemCor})
	if depois := m.itensDaAba(TabPods)[0].celulas[2]; depois == antes {
		t.Errorf("trocar os glifos deveria refazer a célula de status: %q", depois)
	}

	// Uma lista nova (outro vetor, mesmo tamanho) refaz a lista.
	m.pods = []monitor.Pod{{Name: "x-1", Namespace: "loja", Status: "Running"}, {Name: "x-2", Namespace: "loja", Status: "Running"}, {Name: "x-3", Namespace: "loja", Status: "Running"}}
	if got := nomes(); got != "x-1,x-2,x-3" {
		t.Errorf("lista nova = %q", got)
	}
}
