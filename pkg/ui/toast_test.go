package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

// toastAgendadorInstantaneo registra a duração pedida e entrega a mensagem na hora.
func toastAgendadorInstantaneo(pedida *time.Duration) Agendador {
	return func(d time.Duration, msg tea.Msg) tea.Cmd {
		*pedida = d
		return func() tea.Msg { return msg }
	}
}

func TestToastSomeEm6s(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	var pedida time.Duration
	m := NovoToast().ComAgendador(toastAgendadorInstantaneo(&pedida))
	if m.Visivel() || m.View() != "" || m.Init() != nil {
		t.Fatal("o Toast deve nascer oculto")
	}
	m, cmd := m.Mostrar(ToastOk, "Deployment api reiniciado · 3 réplicas")
	if pedida != 6*time.Second {
		t.Errorf("duração = %v, esperado 6 s", pedida)
	}
	if !strings.Contains(m.View(), "v Deployment api reiniciado") {
		t.Errorf("view: %q", m.View())
	}
	novo, _ := m.Update(cmd())
	if novo.(Toast).Visivel() || novo.View() != "" {
		t.Error("o Toast deveria ter sumido")
	}
}

func TestToastNovaMensagemSubstituiTempoAntigo(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	var pedida time.Duration
	m := NovoToast().ComAgendador(toastAgendadorInstantaneo(&pedida)).ComDuracao(time.Second)
	m, cmd1 := m.Mostrar(ToastOk, "primeiro")
	m, _ = m.Mostrar(ToastErro, "sem permissão para escalar · RBAC")
	if pedida != time.Second {
		t.Errorf("duração = %v", pedida)
	}
	// O tempo da primeira mensagem não pode esconder a segunda.
	novo, _ := m.Update(cmd1())
	if !novo.(Toast).Visivel() {
		t.Fatal("tempo antigo escondeu a mensagem nova")
	}
	if !strings.Contains(novo.View(), "x sem permissão") {
		t.Errorf("view: %q", novo.View())
	}
}

func TestToastTiposEPadrao(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	m, _ := NovoToast().ComAgendador(func(time.Duration, tea.Msg) tea.Cmd { return nil }).Mostrar(ToastAviso, "token perto de expirar")
	if !strings.HasPrefix(m.View(), "! token") {
		t.Errorf("view: %q", m.View())
	}
	// O agendador padrão devolve um comando (tea.Tick).
	if agendadorPadrao(time.Millisecond, ToastSomeMsg{ID: 1}) == nil {
		t.Error("agendador padrão deve devolver comando")
	}
	if _, cmd := NovoToast().Mostrar(ToastOk, "x"); cmd == nil {
		t.Error("Mostrar com o agendador padrão deve devolver comando")
	}
}

func TestToastGolden(t *testing.T) {
	etapasPorModo(t, "toast", func() string {
		nada := func(time.Duration, tea.Msg) tea.Cmd { return nil }
		var linhas []string
		for _, c := range []struct {
			tipo  TipoToast
			texto string
		}{
			{ToastOk, "Deployment api reiniciado · 3 réplicas"},
			{ToastAviso, "token expira em 4 min"},
			{ToastErro, "sem permissão para escalar · RBAC"},
		} {
			m, _ := NovoToast().ComAgendador(nada).Mostrar(c.tipo, c.texto)
			linhas = append(linhas, m.View())
		}
		return strings.Join(linhas, "\n")
	})
}
