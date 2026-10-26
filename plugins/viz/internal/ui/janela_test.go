package ui

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"k8s.io/client-go/kubernetes/fake"
)

// A janela abaixo de 60x16 ("aumente a janela"): as camadas que agem ou
// recebem texto fecham, as de leitura ficam, e só ctrl+c e q fazem efeito.

const avisoJanela = "ação cancelada: a janela ficou pequena demais"

var (
	janelaPequena = tea.WindowSizeMsg{Width: 50, Height: 14}
	janelaGrande  = tea.WindowSizeMsg{Width: 120, Height: 40}
)

// TestJanelaPequena_FechaAConfirmacao: com a confirmação de excluir aberta, a
// janela pequena a fecha; digitar o nome e ↵ não exclui nada; a janela de
// volta mostra a lista e o aviso, uma vez só.
func TestJanelaPequena_FechaAConfirmacao(t *testing.T) {
	cs := fake.NewClientset(deploymentDe("payments", "worker"))
	feitas := chamadas(cs)
	m := comJanela(NewModel(&clientComAcesso{MockClient: &MockClient{}, cs: cs}))
	m.activeTab = TabDeployments
	m, _ = atualizar(m, []monitor.Deployment{{Name: "worker", Namespace: "payments"}})
	m, _ = teclas(t, m, "x", "d")
	if m.acao.etapa != confirmando {
		t.Fatalf("a confirmação deveria estar aberta: etapa %d", m.acao.etapa)
	}

	m, _ = atualizar(m, janelaPequena)
	if m.acao.aberta() {
		t.Fatal("a janela pequena deveria fechar a confirmação")
	}
	if !strings.Contains(ansi.Strip(m.View()), "60x16") {
		t.Fatalf("a tela pequena pede para aumentar a janela:\n%s", ansi.Strip(m.View()))
	}
	for _, k := range append(strings.Split("worker", ""), "enter", "x", "d", "2", "r", "esc") {
		var cmd tea.Cmd
		if m, cmd = teclas(t, m, k); cmd != nil {
			t.Fatalf("a tecla %q agiu com a janela pequena", k)
		}
	}
	if len(*feitas) != 0 || m.acao.aberta() || m.activeTab != TabDeployments {
		t.Fatalf("nada deveria acontecer: chamadas %v, etapa %d, aba %s", *feitas, m.acao.etapa, m.activeTab)
	}

	m, cmd := atualizar(m, janelaGrande)
	tela := ansi.Strip(m.View())
	if cmd == nil || !m.toast.Visivel() || !strings.Contains(tela, avisoJanela) {
		t.Fatalf("a janela de volta deveria avisar o que foi cancelado:\n%s", tela)
	}
	if m.acao.aberta() || strings.Contains(tela, "EXCLUIR") || !strings.Contains(tela, "worker") {
		t.Fatalf("a janela de volta mostra a lista, sem a confirmação:\n%s", tela)
	}
	// Outra volta ao tamanho, sem nada cancelado, não avisa de novo.
	m, _ = atualizar(m, janelaPequena)
	if m, cmd = atualizar(m, janelaGrande); cmd != nil || m.cancelouAcao {
		t.Fatal("sem camada cancelada, a janela de volta não avisa")
	}
}

// TestJanelaPequena_CamposVoltamComoNoEsc: os campos /, n e f fecham como no
// esc: a busca volta ao termo de antes, o namespace e o filtro não mudam.
func TestJanelaPequena_CamposVoltamComoNoEsc(t *testing.T) {
	base := buscar(t, telaDaPrancha(t, 120, 30), "web")
	casos := map[string][]string{"busca": {"/", "-", "9"}, "namespace": {"n", "o", "u"}, "filtro": {"f", "a", "p", "p", "="}}
	for nome, ks := range casos {
		t.Run(nome, func(t *testing.T) {
			m, _ := teclas(t, base, ks...)
			if !m.filter.InputMode {
				t.Fatal("o campo deveria estar aberto")
			}
			m, _ = atualizar(m, janelaPequena)
			if m.filter.InputMode || m.filter.InputBuffer != "" || !m.cancelouAcao {
				t.Fatalf("o campo deveria fechar: %+v", m.filter)
			}
			if m.filter.SearchQuery != "web" || m.filter.Namespace != "payments" || m.filter.LabelSelector != "" {
				t.Fatalf("os filtros deveriam ficar como antes do campo: %+v", m.filter)
			}
			if m, _ = atualizar(m, janelaGrande); !strings.Contains(ansi.Strip(m.View()), avisoJanela) {
				t.Fatal("a janela de volta deveria avisar")
			}
		})
	}
}

// TestJanelaPequena_LeituraSobrevive: logs, YAML, ajuda e o detalhe por cima
// ficam abertos e voltam com a janela; com um deles aberto, o q não sai (e o
// esc não fecha nada); sem camada, o q sai; o ctrl+c sai sempre.
func TestJanelaPequena_LeituraSobrevive(t *testing.T) {
	casos := map[string]struct {
		teclas []string
		marca  string
		aberta func(Model) bool
	}{
		"logs":             {[]string{"l"}, "LOGS · ", func(m Model) bool { return m.quadro.aberto }},
		"yaml":             {[]string{"y"}, "YAML · ", func(m Model) bool { return m.quadro.aberto }},
		"ajuda":            {[]string{"?"}, "TECLAS · PODS", func(m Model) bool { return m.ajuda }},
		"detalhe por cima": {[]string{"enter"}, "POD · ", func(m Model) bool { return m.painelPorCima() }},
	}
	volta := tea.WindowSizeMsg{Width: 100, Height: 30} // abaixo de 120 colunas: o detalhe é por cima
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			m, _ := teclas(t, telaDaPrancha(t, 100, 30), c.teclas...)
			if !c.aberta(m) {
				t.Fatal("a camada deveria estar aberta")
			}
			m, _ = atualizar(m, janelaPequena)
			for _, k := range []string{"q", "esc", "j"} {
				var cmd tea.Cmd
				if m, cmd = teclas(t, m, k); cmd != nil {
					t.Fatalf("%q agiu com a janela pequena e a camada %s aberta", k, nome)
				}
			}
			m, cmd := atualizar(m, volta)
			if tela := ansi.Strip(m.View()); !c.aberta(m) || !strings.Contains(tela, c.marca) || cmd != nil || strings.Contains(tela, avisoJanela) {
				t.Fatalf("a camada %s deveria voltar, sem aviso:\n%s", nome, tela)
			}
			if _, cmd := teclas(t, m, "ctrl+c"); cmd == nil {
				t.Fatal("ctrl+c sai")
			}
		})
	}

	m, _ := atualizar(telaDaPrancha(t, 120, 30), janelaPequena)
	if _, cmd := teclas(t, m, "q"); cmd == nil {
		t.Fatal("sem camada, o q sai com a janela pequena")
	}
	if _, cmd := teclas(t, m, "ctrl+c"); cmd == nil {
		t.Fatal("ctrl+c sai com a janela pequena")
	}
}
