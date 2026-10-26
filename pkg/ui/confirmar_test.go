package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

// Os testes de fluxo com teatest ficam em pkg/ui/uiteatest (conflito da flag -update).

func TestConfirmarEdicaoEView(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	var m tea.Model = NovoConfirmarDestrutivo("excluir pod", "api")
	enviar := func(k tea.KeyMsg) { m, _ = m.Update(k) }
	enviar(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("apx")})
	for i := 0; i < 4; i++ { // o último apaga com o campo já vazio
		enviar(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	enviar(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("api")})
	c := m.(Confirmar)
	if c.Digitado() != "api" || !c.Liberado() {
		t.Fatalf("digitado = %q", c.Digitado())
	}
	if !strings.Contains(m.View(), "enter confirmar") {
		t.Errorf("dica de confirmação ausente:\n%s", m.View())
	}
	m, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 10})
	if m.(Confirmar).largura != 72 {
		t.Error("largura deve ser limitada a 72")
	}
	if m.Init() != nil {
		t.Error("Init deve ser nil")
	}
}

func TestConfirmarMensagens(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	var m tea.Model = NovoConfirmarDestrutivo("excluir pod", "api")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("enter sem o nome não deve emitir mensagem")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("api")})
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if msg, ok := cmd().(ConfirmadoMsg); !ok || msg.Nome != "api" {
		t.Errorf("esperava ConfirmadoMsg{api}")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if _, ok := cmd().(CanceladoMsg); !ok {
		t.Errorf("esperava CanceladoMsg")
	}

	// Reversível ignora texto digitado.
	var r tea.Model = NovoConfirmarReversivel("reiniciar", "reiniciar?")
	r, _ = r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zzz")})
	if r.(Confirmar).Digitado() != "" {
		t.Error("reversível não aceita digitação")
	}
	_, cmd = r.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := cmd().(ConfirmadoMsg); !ok {
		t.Errorf("enter reversível deve confirmar")
	}
}

func TestConfirmarGolden(t *testing.T) {
	etapasPorModo(t, "confirmar_destrutivo", func() string {
		var m tea.Model = NovoConfirmarDestrutivo("excluir pod", "api-7f9c-x2k")
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("api")})
		return m.View()
	})
	etapasPorModo(t, "confirmar_reversivel", func() string {
		return NovoConfirmarReversivel("reiniciar", "reiniciar o deployment api?").View()
	})
	// O destrutivo completo da folha VizConfirmar, com o nome digitado pela metade.
	etapasPorModo(t, "confirmar_detalhado", func() string {
		var m tea.Model = NovoConfirmarDestrutivo("excluir deployment", "worker").
			ComDetalhes(S().Negrito.Render("worker")+" em payments · 2 réplicas",
				"Os pods param e o Deployment some do cluster. Se ele vem do Argo CD, a próxima sincronização cria de novo.", "excluir")
		m, _ = m.Update(tea.WindowSizeMsg{Width: 60})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("work")})
		return m.View()
	})
}

// TestConfirmarDetalhado: a contagem ao lado do campo e o botão de confirmar seguem o
// que foi digitado; resumo e explicação vazios ficam de fora.
func TestConfirmarDetalhado(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var m tea.Model = NovoConfirmarDestrutivo("excluir pod", "api").ComDetalhes("", "", "")
	digitar := func(txt string) string {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(txt)})
		return m.View()
	}
	v := m.View()
	if linhas := strings.Split(v, "\n"); !strings.Contains(v, "faltam 3 letras") || !strings.Contains(v, "[ ↵ confirmar ]") ||
		!strings.HasPrefix(linhas[1], "│ Digite api para confirmar") {
		t.Errorf("vazio:\n%s", v)
	}
	if v = digitar("ap"); !strings.Contains(v, "falta 1 letra") {
		t.Errorf("uma letra:\n%s", v)
	}
	if v = digitar("i"); !strings.Contains(v, "✓ confere") || !m.(Confirmar).Liberado() {
		t.Errorf("confere:\n%s", v)
	}
	if v = digitar("x"); !strings.Contains(v, "▲ não confere") || m.(Confirmar).Liberado() {
		t.Errorf("não confere:\n%s", v)
	}
	etapasLarguraMax(t, v, larguraConfirmarPadrao)
}
