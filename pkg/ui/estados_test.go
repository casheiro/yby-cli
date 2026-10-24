package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

func TestEstadosTextosDiferentes(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	semTTY := func() bool { return false }
	vazio := NovoEstadoVazio("nenhum pod com esse filtro", "esc limpa")
	carregando := NovoEstadoCarregandoCom("lendo pods de payments", semTTY)
	erro := NovoEstadoErro("sem acesso ao cluster", "r tenta de novo")

	casos := []struct {
		m    Estado
		tipo TipoEstado
		view string
	}{
		{vazio, EstadoVazio, "○ nenhum pod com esse filtro · esc limpa"},
		{carregando, EstadoCarregando, "◈ lendo pods de payments"},
		{erro, EstadoErro, "✕ sem acesso ao cluster · r tenta de novo"},
	}
	vistos := map[string]bool{}
	for _, c := range casos {
		if c.m.Tipo() != c.tipo {
			t.Errorf("tipo = %v, esperado %v", c.m.Tipo(), c.tipo)
		}
		if got := c.m.View(); got != c.view {
			t.Errorf("view = %q, esperado %q", got, c.view)
		}
		vistos[c.m.View()] = true
	}
	if len(vistos) != 3 {
		t.Error("os três estados devem ter textos diferentes")
	}
}

func TestEstadoCarregandoAnima(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	m := NovoEstadoCarregandoCom("lendo", func() bool { return true })
	if m.Init() == nil {
		t.Fatal("em TTY o carregando deve iniciar a animação")
	}
	antes := m.View()
	var modelo tea.Model = m
	modelo, cmd := modelo.Update(SpinnerTickMsg{id: m.spinner.id})
	if cmd == nil || modelo.View() == antes {
		t.Error("o tick deve avançar o quadro e reagendar")
	}
}

func TestEstadoSemAnimacao(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	for _, m := range []Estado{NovoEstadoVazio("a", ""), NovoEstadoErro("b", "")} {
		if m.Init() != nil {
			t.Error("estado estático não deve ter Init")
		}
		if novo, cmd := m.Update(SpinnerTickMsg{}); cmd != nil || novo.View() != m.View() {
			t.Error("estado estático ignora mensagens")
		}
		if strings.Contains(m.View(), "·") {
			t.Errorf("sem dica não deve haver separador: %q", m.View())
		}
	}
	if NovoEstadoCarregando("x").View() == "" {
		t.Error("view vazia")
	}
}

func TestEstadosGolden(t *testing.T) {
	etapasPorModo(t, "estados", func() string {
		return strings.Join([]string{
			NovoEstadoCarregandoCom("lendo pods de payments", func() bool { return false }).View(),
			NovoEstadoVazio("nenhum pod com esse filtro", "esc limpa").View(),
			NovoEstadoErro("sem acesso ao cluster", "r tenta de novo").View(),
		}, "\n")
	})
}
