package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
)

// O b do Viz abre o Bard com a pergunta escrita no campo (yby bard --pergunta): nada é
// enviado até a pessoa apertar ↵.

const perguntaDoViz = "Por que o pod payments/worker-5d8b6-q7tvn está em CrashLoopBackOff (14 reinícios)?"

// perguntaDeEvento é a pergunta do b sobre um evento Warning, com a mensagem longa.
const perguntaDeEvento = `O que significa o evento BackOff no pod payments/worker-5d8b6-q7tvn: "Back-off restarting ` +
	`failed container worker in pod worker-5d8b6-q7tvn_payments(3f1c9a2e-77aa-4e1b-9d0f) aaaaaaaaaaaaaaaaaaaaaaaaaaa ` +
	`bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb cccccccccccccccccc"?`

func TestRascunho_AbreComOCampoPreenchidoSemEnviar(t *testing.T) {
	m := telaDeTeste(t, Config{SessionID: "20261025-100000", Rascunho: perguntaDoViz}, "10:00")

	if got := m.textarea.Value(); got != perguntaDoViz {
		t.Fatalf("campo = %q, esperado a pergunta do Viz", got)
	}
	if len(m.messages) != 0 || m.state != stateIdle {
		t.Fatalf("o rascunho não pode ser enviado sozinho: %d mensagens, estado %v", len(m.messages), m.state)
	}
	if !strings.Contains(m.View(), "CrashLoopBackOff") {
		t.Error("a pergunta deveria aparecer no campo da tela")
	}
}

// campoNaTela devolve as linhas do campo na tela, entre a segunda e a terceira régua,
// sem cor e sem os espaços do fim.
func campoNaTela(t *testing.T, tela string) string {
	t.Helper()
	var reguas []int
	linhas := strings.Split(ansi.Strip(tela), "\n")
	for i, l := range linhas {
		if strings.Trim(l, "─-") == "" && l != "" {
			reguas = append(reguas, i)
		}
	}
	if len(reguas) < 3 {
		t.Fatalf("a tela deveria ter as três réguas:\n%s", ansi.Strip(tela))
	}
	campo := linhas[reguas[1]+1 : reguas[2]]
	for i := range campo {
		campo[i] = strings.TrimRight(campo[i], " ")
	}
	return strings.Join(campo, "\n")
}

// A pergunta aparece inteira no campo, desde o "›", em qualquer largura: o campo cresce
// com as linhas da quebra na tela, não só com as do texto.
func TestRascunho_CampoMostraAPerguntaDesdeOComeco(t *testing.T) {
	semCores(t)
	for _, largura := range []int{60, 64, 80, 100, 120, 160} {
		m := telaDeTeste(t, Config{SessionID: "20261025-100000", Rascunho: perguntaDoViz}, "10:00")
		atualizado, _ := m.Update(tea.WindowSizeMsg{Width: largura, Height: 24})
		m = atualizado.(Model)
		campo := campoNaTela(t, m.View())
		junto := strings.Join(strings.Fields(strings.ReplaceAll(campo, ui.G().Prompt, "")), " ")
		if !strings.HasPrefix(strings.TrimSpace(campo), ui.G().Prompt+" Por que o pod payments/worker") || junto != perguntaDoViz {
			t.Errorf("%d colunas: o campo deveria mostrar a pergunta inteira:\n%s", largura, campo)
		}
	}
}

// A pergunta longa de um evento, maior que o campo, abre mostrando o começo; a vista
// segue o cursor quando a pessoa anda por ela.
func TestRascunho_PerguntaLongaAbreNoComeco(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261025-100000", Rascunho: perguntaDeEvento}, "10:00")
	atualizado, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	m = atualizado.(Model)
	if m.textarea.Height() != alturaMaxCampo {
		t.Fatalf("o campo deveria ir até a altura máxima: %d", m.textarea.Height())
	}
	campo := campoNaTela(t, m.View())
	if !strings.HasPrefix(strings.TrimSpace(campo), ui.G().Prompt+" O que significa o evento BackOff") {
		t.Errorf("o campo deveria abrir no começo da pergunta:\n%s", campo)
	}
	if m.textarea.Value() != perguntaDeEvento {
		t.Errorf("o rascunho segue inteiro no campo: %q", m.textarea.Value())
	}

	// Numa janela larga ela cabe inteira e o cursor volta ao fim.
	atualizado, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 24})
	m = atualizado.(Model)
	campo = campoNaTela(t, m.View())
	if !strings.Contains(campo, ui.G().Prompt+" O que significa") || !strings.Contains(campo, `cccccccccccccccccc"?`) {
		t.Errorf("a 160 colunas a pergunta cabe inteira:\n%s", campo)
	}
}

// O campo também cresce com a quebra de linha do que se digita, como com o ctrl+j.
func TestCampo_CresceComAQuebraNaTela(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261025-100000"}, "10:00")
	atualizado, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	m = atualizado.(Model)
	digitado := "por que o worker reinicia tanto desde o deploy de ontem à noite e o que mudou no configmap dele, e se o banco caiu junto com ele?"
	for _, r := range digitado {
		atualizado, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = atualizado.(Model)
	}
	if m.textarea.Height() < 3 || m.textarea.Height() != m.linhasDoCampo() {
		t.Fatalf("o texto quebra em %d linhas a 60 colunas e o campo deveria acompanhar: %d", m.linhasDoCampo(), m.textarea.Height())
	}
	campo := campoNaTela(t, m.View())
	if !strings.Contains(campo, ui.G().Prompt+" por que o worker") || !strings.Contains(campo, "junto com ele?") {
		t.Errorf("o campo deveria mostrar o que foi digitado:\n%s", campo)
	}
}

// Goldens do Bard aberto pelo b do Viz: a 80x24 (a faixa do painel por cima no Viz) e
// com a pergunta longa de um evento a 120x40.
func TestTelaRascunho(t *testing.T) {
	usarModo(t, modo{"escuro", ui.TemaEscuro, false, termenv.TrueColor})
	casos := []struct {
		nome            string
		pergunta        string
		largura, altura int
	}{
		{"pod_80x24", perguntaDoViz, 80, 24},
		{"evento_120x40", perguntaDeEvento, 120, 40},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			m := telaDeTeste(t, Config{SessionID: "20261025-100000", Cluster: "k3d-yby-local", Namespace: "payments",
				AIModel: "claude-cli", Rascunho: c.pergunta, Voltar: "Viz"}, "10:00")
			atualizado, _ := m.Update(tea.WindowSizeMsg{Width: c.largura, Height: c.altura})
			m = atualizado.(Model)
			tela := m.View()
			linhas := strings.Split(tela, "\n")
			if len(linhas) != c.altura {
				t.Errorf("altura: %d linhas, esperado %d", len(linhas), c.altura)
			}
			for i, l := range linhas {
				if w := lipgloss.Width(l); w != c.largura {
					t.Errorf("linha %d com %d células", i+1, w)
				}
			}
			if !strings.Contains(ansi.Strip(tela), ui.G().Prompt+" Por que o pod") && !strings.Contains(ansi.Strip(tela), ui.G().Prompt+" O que significa") {
				t.Errorf("o começo da pergunta deveria aparecer:\n%s", ansi.Strip(tela))
			}
			golden.RequireEqual(t, []byte(tela))
		})
	}
}

func TestRascunho_EnviaSoNoEnter(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"o worker não alcança o banco"}}
	m := New(p, nil, Config{SessionID: "20261025-100000", Rascunho: perguntaDoViz})
	m.markdown = nil
	m.fluxo.Retry = retryRapido
	tm := abrirBard(t, m)

	esperarTela(t, tm, "CrashLoopBackOff")
	time.Sleep(50 * time.Millisecond) // nada roda sem o ↵
	if n := p.totalChamadas(); n != 0 {
		t.Fatalf("a IA foi chamada %d vez(es) antes do ↵", n)
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	esperarTela(t, tm, "o worker não alcança o banco")
	final := fecharBard(t, tm)

	if len(final.messages) == 0 || final.messages[0].content != perguntaDoViz {
		t.Fatalf("a pergunta enviada deveria ser a do rascunho: %+v", final.messages)
	}
	if !strings.Contains(p.ultimaEntrada(), "worker-5d8b6-q7tvn") {
		t.Errorf("a IA deveria receber a pergunta do rascunho: %q", p.ultimaEntrada())
	}
}

// Sem rascunho, o campo abre vazio, como antes.
func TestRascunho_SemRascunhoCampoVazio(t *testing.T) {
	m := telaDeTeste(t, Config{SessionID: "20261025-100000"}, "10:00")
	if !m.campoVazio() {
		t.Fatalf("campo = %q, esperado vazio", m.textarea.Value())
	}
}

// ctrl+c sai de qualquer lugar: o Run devolve ErrInterrompido (o yby sai com 130 e o Viz
// que abriu o Bard fecha também), no Bard aberto pelo Viz ou sozinho.
func TestRun_CtrlCDevolveInterrompido(t *testing.T) {
	for _, voltar := range []string{"", "Viz"} {
		isolarHome(t)
		m := New(&provedorFalso{}, nil, Config{SessionID: "20261025-100000", Rascunho: perguntaDoViz, Voltar: voltar})
		tm := abrirBard(t, m)
		esperarTela(t, tm, "CrashLoopBackOff")
		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		final := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
		if err := desfecho(final, nil); !errors.Is(err, ErrInterrompido) {
			t.Errorf("voltar %q: ctrl+c deveria dar ErrInterrompido, deu %v", voltar, err)
		}
	}
	if err := desfecho(nil, tea.ErrInterrupted); !errors.Is(err, ErrInterrompido) {
		t.Errorf("o SIGINT do Bubble Tea também é interrupção: %v", err)
	}
	outro := errors.New("terminal")
	if err := desfecho(nil, outro); !errors.Is(err, outro) {
		t.Errorf("outro erro segue: %v", err)
	}
}

// Aberto pelo Viz (--voltar), o esc fora da resposta fecha o Bard sem interrupção (o yby
// sai com 0 e o Viz volta); sozinho, o esc fora da resposta não faz nada.
func TestEsc_VoltaAoInstrumentoQueAbriu(t *testing.T) {
	isolarHome(t)
	m := New(&provedorFalso{}, nil, Config{SessionID: "20261025-100000", Rascunho: perguntaDoViz, Voltar: "Viz"})
	tm := abrirBard(t, m)
	esperarTela(t, tm, "esc voltar ao Viz")
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	final := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
	if err := desfecho(final, nil); err != nil {
		t.Fatalf("o esc volta sem erro: %v", err)
	}

	sozinho := telaDeTeste(t, Config{SessionID: "20261025-100000"}, "10:00")
	if _, cmd := sozinho.Update(tea.KeyMsg{Type: tea.KeyEsc}); cmd != nil {
		t.Error("sem --voltar, o esc fora da resposta não fecha o Bard")
	}
	if strings.Contains(ansi.Strip(sozinho.View()), "voltar ao") {
		t.Error("sem --voltar, a legenda não mostra o esc")
	}
}

// Durante a resposta, o esc do Bard aberto pelo Viz só interrompe a resposta.
func TestEsc_DuranteARespostaSoInterrompe(t *testing.T) {
	m := telaDeTeste(t, Config{SessionID: "20261025-100000", Voltar: "Viz"}, "10:00")
	cancelou := false
	m.state, m.cancelar = stateStreaming, func() { cancelou = true }
	atualizado, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || !cancelou || !atualizado.(Model).interrompendo {
		t.Fatalf("o esc na resposta interrompe e não fecha: cmd=%v cancelou=%v", cmd != nil, cancelou)
	}
}

// A ajuda do Bard aberto pelo Viz lista o esc que volta a ele.
func TestQuadroAjuda_ComVoltar(t *testing.T) {
	semCores(t)
	var texto []string
	for _, l := range quadroAjuda(false, "Viz").linhas {
		texto = append(texto, ansi.Strip(l.chave)+" | "+ansi.Strip(l.valor))
	}
	if !strings.Contains(strings.Join(texto, "\n"), "esc | voltar ao Viz · fora da resposta") {
		t.Errorf("a ajuda deveria ter o esc que volta ao Viz:\n%s", strings.Join(texto, "\n"))
	}
	if v := teclaVoltarA(""); v.Enabled() || key.Matches(tea.KeyMsg{Type: tea.KeyEsc}, v) {
		t.Error("sem instrumento, a tecla fica desligada")
	}
}
