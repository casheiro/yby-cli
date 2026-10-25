package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// semCores deixa a saída sem ANSI, para comparar texto.
func semCores(t *testing.T) {
	t.Helper()
	usarModo(t, modo{tema: ui.TemaEscuro, perfil: termenv.Ascii})
}

func TestContexto(t *testing.T) {
	semCores(t)
	casos := []struct {
		cfg  Config
		want string
	}{
		{Config{Cluster: "staging", Namespace: "default", AIModel: "claude-cli"}, "staging ▸ default   claude-cli"},
		{Config{Namespace: "payments", AIModel: "ollama"}, "payments   ollama"},
		{Config{Cluster: "k3d-local"}, "k3d-local"},
		{Config{AIModel: "gemini"}, "gemini"},
		{Config{}, ""},
	}
	for _, c := range casos {
		m := New(nil, nil, c.cfg)
		if got := m.contexto(); got != c.want {
			t.Errorf("contexto(%+v) = %q, esperado %q", c.cfg, got, c.want)
		}
	}
}

func TestResumoSessao(t *testing.T) {
	semCores(t)
	m := New(nil, nil, Config{SessionID: "20261024-090000"})
	m.agora = relogio("18:00")
	if got := m.resumoSessao(); got != "sessão de hoje · nenhuma mensagem" {
		t.Errorf("sessão vazia: %q", got)
	}
	m.messages = []chatMessage{{role: "user"}, {role: "nota"}, {role: "error"}}
	if got := m.resumoSessao(); got != "sessão de hoje · 1 mensagem" {
		t.Errorf("notas e erros não contam: %q", got)
	}
	m.config.SessionID = "20261020-090000"
	m.messages = append(m.messages, chatMessage{role: "assistant"})
	if got := m.resumoSessao(); got != "sessão de 20/10 · 2 mensagens" {
		t.Errorf("sessão de outro dia: %q", got)
	}
	m.config.SessionID = ""
	if got := m.resumoSessao(); got != "2 mensagens" {
		t.Errorf("sem sessão: %q", got)
	}
}

func TestMensagemEstado(t *testing.T) {
	semCores(t)
	m := New(nil, nil, Config{})
	if got := m.mensagemEstado(); got != "" {
		t.Errorf("antes da primeira pergunta: %q", got)
	}
	m.state = stateStreaming
	if got := m.mensagemEstado(); got != "◈ trabalhando" {
		t.Errorf("trabalhando: %q", got)
	}
	m.etapas = []EtapaMsg{{Etapa: EtapaContexto, Rotulo: rotuloContexto, Estado: ui.EtapaFeita}, {Etapa: EtapaResposta, Rotulo: rotuloResposta, Estado: ui.EtapaAtual}}
	if got := m.mensagemEstado(); got != "◈ gerar resposta" {
		t.Errorf("etapa atual: %q", got)
	}
	m.interrompendo = true
	if got := m.mensagemEstado(); got != "◈ interrompendo" {
		t.Errorf("interrompendo: %q", got)
	}
	m.state = stateIdle
	for _, c := range []struct {
		fim  FimMsg
		want string
	}{
		{FimMsg{Duracao: 21 * time.Second}, "✓ resposta em 21 s"},
		{FimMsg{Err: errors.New("429")}, "✕ a resposta falhou"},
		{FimMsg{Interrompida: true}, "▲ resposta interrompida"},
	} {
		m.ultimoFim = &c.fim
		if got := m.mensagemEstado(); got != c.want {
			t.Errorf("fim %+v: %q, esperado %q", c.fim, got, c.want)
		}
	}
}

func TestLinhaFerramenta(t *testing.T) {
	semCores(t)
	casos := []struct {
		f    ferramentaUsada
		want string
	}{
		{ferramentaUsada{rotulo: "kubectl get", alvo: "pvc,pv -A", linhas: 14}, "✓ kubectl get pvc,pv -A · 14 linhas"},
		{ferramentaUsada{rotulo: "atlas blueprint", linhas: 1}, "✓ atlas blueprint · 1 linha"},
		{ferramentaUsada{rotulo: "kubectl logs", alvo: "worker"}, "✓ kubectl logs worker"},
		{ferramentaUsada{rotulo: "kubectl get", falhou: true, nota: "sem acesso ao cluster"}, "✕ kubectl get · sem acesso ao cluster"},
		{ferramentaUsada{rotulo: "kubectl get", falhou: true}, "✕ kubectl get"},
	}
	for _, c := range casos {
		if got := linhaFerramenta(c.f); got != c.want {
			t.Errorf("linhaFerramenta(%+v) = %q, esperado %q", c.f, got, c.want)
		}
	}
}

func TestFerramentasDaRodada(t *testing.T) {
	m := New(nil, nil, Config{})
	m.etapas = []EtapaMsg{
		{Etapa: EtapaContexto, Rotulo: rotuloContexto, Estado: ui.EtapaFeita},
		{Etapa: EtapaFerramenta, Rotulo: "kubectl get", Nota: "pods", Estado: ui.EtapaFeita, Saida: "a\nb\n"},
		{Etapa: EtapaFerramenta, Rotulo: "kubectl logs", Nota: "sem permissão", Estado: ui.EtapaFalha, Saida: "Erro: x"},
		{Etapa: EtapaFerramenta, Rotulo: "sentinel scan", Estado: ui.EtapaAtual},
	}
	got := m.ferramentasDaRodada()
	if len(got) != 2 {
		t.Fatalf("só as ferramentas que terminaram: %+v", got)
	}
	if got[0].alvo != "pods" || got[0].linhas != 2 || got[0].falhou {
		t.Errorf("ferramenta feita: %+v", got[0])
	}
	if !got[1].falhou || got[1].nota != "sem permissão" || got[1].alvo != "" || got[1].linhas != 0 {
		t.Errorf("ferramenta com falha: %+v", got[1])
	}
	if contarLinhas("  \n") != 0 || contarLinhas("um") != 1 {
		t.Error("contarLinhas")
	}
}

func TestCampo(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "10:00")
	linhas := strings.Split(m.campo(), "\n")
	if len(linhas) != alturaCampo || !strings.HasPrefix(linhas[0], "›  Pergunte sobre o cluster") {
		t.Errorf("campo vazio: %q", linhas)
	}
	m.textarea.SetValue("qual pod reinicia?")
	linhas = strings.Split(m.campo(), "\n")
	if len(linhas) != alturaCampo || !strings.HasPrefix(linhas[0], "› qual pod reinicia?") || strings.HasPrefix(linhas[1], "›") {
		t.Errorf("campo com texto: o prompt só na primeira linha: %q", linhas)
	}
	m.state = stateStreaming
	if got := strings.Split(m.campo(), "\n")[0]; got != "›" {
		t.Errorf("campo durante a resposta: %q", got)
	}
}

func TestTeclas(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "10:00")
	legenda := func() string {
		return ui.KeyHintsCom(200, ui.DefaultKeys().Global.SairForcado, m.teclas()...)
	}
	// Só as teclas que já funcionam: ⇧↵ e ↑ (T013), / e ? (T015) ficam de fora.
	semPendentes := func(got string) bool {
		for _, k := range []string{"⇧↵", "/ comandos", "↑", "? ajuda"} {
			if strings.Contains(got, k) {
				return false
			}
		}
		return true
	}
	if got := legenda(); !strings.HasPrefix(got, "↵ enviar   ") || !semPendentes(got) {
		t.Errorf("legenda ociosa: %q", got)
	}
	m.state = stateStreaming
	if got := legenda(); !strings.HasPrefix(got, "esc interromper   ") || !semPendentes(got) {
		t.Errorf("legenda durante a resposta: %q", got)
	}
	// Com a conversa maior que a tela, pgup aparece.
	m.state = stateIdle
	m.messages = append(m.messages, chatMessage{role: "assistant", content: strings.Repeat("linha\n", 40)})
	m.markdown = nil
	m.updateViewport()
	if got := legenda(); !strings.Contains(got, "pgup rolar") {
		t.Errorf("legenda com rolagem: %q", got)
	}
}

func TestRenderizarMensagem_ErroENota(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "10:00")
	erro := m.renderizarMensagem(chatMessage{role: "error", content: "o provider recusou\nsegunda linha", hora: m.agora(), duracao: 3 * time.Second,
		ferramentas: []ferramentaUsada{{rotulo: "kubectl get", alvo: "pods", linhas: 3}}})
	want := "✕ BARD · 10:00 · 1 ferramenta · 3,0 s\n  ✓ kubectl get pods · 3 linhas\n\n  o provider recusou\n  segunda linha\n\n"
	if erro != want {
		t.Errorf("bloco de erro:\n%q\nesperado\n%q", erro, want)
	}
	if got := m.renderizarMensagem(chatMessage{role: "nota", content: "resposta interrompida"}); got != "  resposta interrompida\n\n" {
		t.Errorf("nota: %q", got)
	}
}

func TestCorpoResposta_MarkdownFalhaMostraTextoCru(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "10:00")
	m.markdown = func(string, int) (string, error) { return "", errors.New("quebrou") }
	if got := m.corpoResposta("**oi**\n\ntchau", 40); got != "  **oi**\n\n  tchau\n" {
		t.Errorf("texto cru: %q", got)
	}
}

func TestParcialAparecePorBaixo(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "10:00")
	m.state = stateStreaming
	m.parcial = "Primeira parte da resposta"
	m.updateViewport()
	vista := m.viewport.View()
	if !strings.Contains(vista, "◈ BARD · trabalhando") || !strings.Contains(vista, "  Primeira parte da resposta") {
		t.Errorf("parcial: %q", vista)
	}
}

func TestMarkdownCache(t *testing.T) {
	semCores(t)
	c := &markdownCache{}
	a, err := c.renderizar("**oi**", 40)
	if err != nil || a != "oi" {
		t.Fatalf("render: %q %v", a, err)
	}
	r := c.r
	if _, _ = c.renderizar("x", 40); c.r != r {
		t.Error("a mesma largura reaproveita o renderizador")
	}
	if _, _ = c.renderizar("x", 60); c.r == r || c.largura != 60 {
		t.Error("largura nova pede outro renderizador")
	}
}

// TestTelaJanelaPequena garante que o Bard pede uma janela maior abaixo de 60x16.
func TestTelaJanelaPequena(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "10:00")
	atualizado, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 12})
	tela := atualizado.(Model).View()
	if !strings.Contains(tela, "Aumente a janela") {
		t.Errorf("janela pequena:\n%s", tela)
	}
	for _, l := range strings.Split(tela, "\n") {
		if lipgloss.Width(ansi.Strip(l)) > 50 {
			t.Errorf("linha passa da janela: %q", l)
		}
	}
}
