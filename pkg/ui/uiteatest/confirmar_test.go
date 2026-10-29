// Testes de fluxo do ui.Confirmar com teatest.
//
// Ficam num pacote à parte porque o teatest (via x/exp/golden) registra a flag -update,
// que colide com a do helpers_test.go do pacote pkg/ui no mesmo binário de teste.
package uiteatest

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

// hospedeiro embrulha o Confirmar, registra o desfecho e encerra o programa.
type hospedeiro struct {
	filho    tea.Model
	desfecho string // "", "confirmado" ou "cancelado"
	nome     string
}

func (h hospedeiro) Init() tea.Cmd { return h.filho.Init() }

func (h hospedeiro) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case ui.ConfirmadoMsg:
		h.desfecho, h.nome = "confirmado", m.Nome
		return h, tea.Quit
	case ui.CanceladoMsg:
		h.desfecho, h.nome = "cancelado", m.Nome
		return h, tea.Quit
	}
	var cmd tea.Cmd
	h.filho, cmd = h.filho.Update(msg)
	return h, cmd
}

func (h hospedeiro) View() string { return h.filho.View() }

// semCor deixa a saída sem cor e em ASCII durante o teste.
func semCor(t *testing.T) {
	t.Helper()
	anterior := lipgloss.ColorProfile()
	opcoes := ui.Atual()
	t.Cleanup(func() {
		ui.Configurar(opcoes)
		lipgloss.SetColorProfile(anterior)
	})
	ui.Configurar(ui.Opcoes{Tema: ui.TemaEscuro, ASCII: true})
	lipgloss.SetColorProfile(termenv.Ascii)
}

// rodar envia as teclas ao Confirmar e devolve o hospedeiro final. Se nada encerrar o
// programa em 500 ms (caso do nome errado), força o fim para o teste não travar.
func rodar(t *testing.T, filho tea.Model, enviar func(tm *teatest.TestModel)) hospedeiro {
	t.Helper()
	tm := teatest.NewTestModel(t, hospedeiro{filho: filho}, teatest.WithInitialTermSize(80, 24))
	enviar(tm)
	time.AfterFunc(500*time.Millisecond, func() { tm.Send(tea.QuitMsg{}) })
	return tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(hospedeiro)
}

func TestConfirmarDestrutivoNomeErrado(t *testing.T) {
	semCor(t)
	h := rodar(t, ui.NovoConfirmarDestrutivo("excluir pod", "api-7f9c-x2k"), func(tm *teatest.TestModel) {
		tm.Type("api-errado")
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	})
	if h.desfecho != "" {
		t.Fatalf("enter com nome errado não pode confirmar nem cancelar; desfecho = %q", h.desfecho)
	}
}

func TestConfirmarDestrutivoNomeCerto(t *testing.T) {
	semCor(t)
	h := rodar(t, ui.NovoConfirmarDestrutivo("excluir pod", "api-7f9c-x2k"), func(tm *teatest.TestModel) {
		tm.Type("api-7f9c-x2k")
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	})
	if h.desfecho != "confirmado" || h.nome != "api-7f9c-x2k" {
		t.Fatalf("desfecho = %q nome = %q", h.desfecho, h.nome)
	}
}

func TestConfirmarDestrutivoEsc(t *testing.T) {
	semCor(t)
	h := rodar(t, ui.NovoConfirmarDestrutivo("excluir pod", "api"), func(tm *teatest.TestModel) {
		tm.Type("ap")
		tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	})
	if h.desfecho != "cancelado" {
		t.Fatalf("desfecho = %q", h.desfecho)
	}
}

func TestConfirmarReversivel(t *testing.T) {
	semCor(t)
	h := rodar(t, ui.NovoConfirmarReversivel("reiniciar", "reiniciar deployment api?"), func(tm *teatest.TestModel) {
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	})
	if h.desfecho != "confirmado" {
		t.Fatalf("enter deve confirmar: %q", h.desfecho)
	}
	h = rodar(t, ui.NovoConfirmarReversivel("reiniciar", "reiniciar deployment api?"), func(tm *teatest.TestModel) {
		tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	})
	if h.desfecho != "cancelado" {
		t.Fatalf("esc deve cancelar: %q", h.desfecho)
	}
}
