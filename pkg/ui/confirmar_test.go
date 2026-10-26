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
}
