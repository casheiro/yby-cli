package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"
)

// provedorNomeado é o provedorFalso com o Name() de um provider de verdade, para a lista
// do /modelo ter as descrições na largura real.
type provedorNomeado struct {
	provedorFalso
	nome string
}

func (p *provedorNomeado) Name() string { return p.nome }

// abrirModelo digita /modelo, entrega a detecção e devolve a lista aberta.
func abrirModelo(t *testing.T, m Model) Model {
	t.Helper()
	m, cmd := teclarModelo(t, m, append(teclasDe("/modelo"), teclaEnter)...)
	m, _ = rodarComando(t, m, cmd)
	if m.seletor == nil {
		t.Fatal("o /modelo deveria abrir a lista")
	}
	return m
}

// Um modelo digitado fora da lista ganha a linha dele ao reabrir o /modelo, e o ◆ abre
// nela, não no primeiro provider.
func TestComandos_ModeloForaDaListaAbreMarcado(t *testing.T) {
	semCores(t)
	criados := map[string]*provedorFalso{}
	claude, _, lista := modelosDeExemplo(criados)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000", AIModel: "ollama"}, "10:00")
	m.config.Modelos = func(context.Context) []Modelo { return lista }
	m.provider, m.fluxo.Provider = claude, claude

	m = pedirComando(t, m, "/modelo claude-cli/claude-sonnet-4-5")
	if m.config.AIModel != "claude-cli" || m.config.ModeloIA != "claude-sonnet-4-5" {
		t.Fatalf("modelo fora da lista: %q %q %+v", m.config.AIModel, m.config.ModeloIA, m.aviso)
	}
	m = abrirModelo(t, m)
	it := m.seletor.itens[m.seletor.marcado]
	if it.nome != "claude-cli/claude-sonnet-4-5" || it.descricao != "em uso" {
		t.Errorf("o modelo em uso abre marcado: %+v", it)
	}
	if n := strings.Count(ansi.Strip(m.View()), "em uso"); n != 1 {
		t.Errorf("só uma linha em uso, achei %d:\n%s", n, ansi.Strip(m.View()))
	}
}

// A criação que chega durante a resposta espera o fim da rodada: a resposta é do provider
// antigo e a nota da troca vem depois dela.
func TestComandos_ModeloCriadoDuranteAResposta(t *testing.T) {
	semCores(t)
	criados := map[string]*provedorFalso{}
	claude, _, lista := modelosDeExemplo(criados)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000", AIModel: "claude-cli"}, "10:00")
	m.config.Modelos = func(context.Context) []Modelo { return lista }
	m.provider, m.fluxo.Provider = claude, claude

	m, cmd := teclarModelo(t, m, append(teclasDe("/modelo claude-cli/opus"), teclaEnter)...)
	m, criar := rodarComando(t, m, cmd)
	if criar == nil {
		t.Fatal("a troca deveria criar o provider")
	}
	criado := criar()
	m, _ = teclarModelo(t, m, append(teclasDe("oi"), teclaEnter)...)
	if m.state != stateStreaming {
		t.Fatalf("a pergunta deveria abrir a rodada: %v", m.state)
	}

	atualizado, _ := m.Update(criado)
	m = atualizado.(Model)
	if m.config.ModeloIA != "" || m.provider != ai.Provider(claude) || m.fluxo.Provider != ai.Provider(claude) {
		t.Errorf("no meio da resposta a sessão continua no provider antigo: %q", m.config.ModeloIA)
	}
	if strings.Contains(m.contexto(), "opus") {
		t.Errorf("o cabeçalho só muda no fim da rodada: %q", m.contexto())
	}

	atualizado, _ = m.Update(FimMsg{Rodada: m.rodada, Resposta: "olá", Duracao: time.Second})
	m = atualizado.(Model)
	if m.config.ModeloIA != "opus" || m.provider != ai.Provider(criados["claude-cli/opus"]) {
		t.Errorf("no fim da rodada a troca vale: %q", m.config.ModeloIA)
	}
	var papeis []string
	for _, msg := range m.messages {
		papeis = append(papeis, msg.role+":"+msg.content)
	}
	if got := strings.Join(papeis, " | "); !strings.HasSuffix(got, "user:oi | assistant:olá | nota:modelo trocado para claude-cli · opus") {
		t.Errorf("a nota da troca vem depois da resposta: %s", got)
	}
	if strings.Contains(ansi.Strip(m.View()), "trocando para") {
		t.Error("depois da troca a linha de estado não diz mais trocando")
	}
}

// Reabrir o /modelo enquanto o provider é criado não perde a escolha: a criação chega,
// vale, e a lista aberta passa a marcar o modelo novo.
func TestComandos_ModeloReabertoAntesDaCriacao(t *testing.T) {
	semCores(t)
	criados := map[string]*provedorFalso{}
	claude, _, lista := modelosDeExemplo(criados)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000", AIModel: "claude-cli"}, "10:00")
	m.config.Modelos = func(context.Context) []Modelo { return lista }
	m.provider, m.fluxo.Provider = claude, claude

	m, cmd := teclarModelo(t, m, append(teclasDe("/modelo claude-cli/opus"), teclaEnter)...)
	m, criar := rodarComando(t, m, cmd)
	m = abrirModelo(t, m)
	if it := m.seletor.itens[m.seletor.marcado]; it.nome != "claude-cli" {
		t.Errorf("antes da criação, o em uso é o de antes: %+v", it)
	}

	atualizado, _ := m.Update(criar())
	m = atualizado.(Model)
	if m.config.ModeloIA != "opus" || m.provider != ai.Provider(criados["claude-cli/opus"]) {
		t.Fatalf("a escolha não se perde com a lista reaberta: %q", m.config.ModeloIA)
	}
	if it := m.seletor.itens[m.seletor.marcado]; it.nome != "claude-cli/opus" || it.descricao != "em uso" {
		t.Errorf("a lista aberta segue a troca: %+v", it)
	}

	// Uma escolha nova, essa sim, substitui a criação pendente.
	m, _ = teclarModelo(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m, cmd = teclarModelo(t, m, append(teclasDe("/modelo claude-cli/haiku"), teclaEnter)...)
	m, criarHaiku := rodarComando(t, m, cmd)
	m, cmd = teclarModelo(t, m, append(teclasDe("/modelo claude-cli/sonnet"), teclaEnter)...)
	m, criarSonnet := rodarComando(t, m, cmd)
	atualizado, _ = m.Update(criarHaiku())
	m = atualizado.(Model)
	if m.config.ModeloIA != "opus" {
		t.Errorf("a criação substituída é descartada: %q", m.config.ModeloIA)
	}
	atualizado, _ = m.Update(criarSonnet())
	if m = atualizado.(Model); m.config.ModeloIA != "sonnet" {
		t.Errorf("a última escolha vale: %q", m.config.ModeloIA)
	}
}

// TestTelaModelo80 é o /modelo a 80 colunas, a largura padrão do terminal, com os nomes
// reais dos providers e o Ollama sem resposta: o "em uso", o aviso do Ollama e a frase da
// linha de estado não podem sumir no corte.
func TestTelaModelo80(t *testing.T) {
	for _, md := range []modo{
		{"escuro", ui.TemaEscuro, false, termenv.TrueColor},
		{"ascii", ui.TemaEscuro, true, termenv.Ascii},
	} {
		t.Run(md.nome, func(t *testing.T) {
			usarModo(t, md)
			isolarHome(t)
			claude := &provedorNomeado{nome: "Claude Code CLI"}
			lista := []Modelo{
				{Rotulo: "claude-cli", Provider: claude, Modelos: []string{"sonnet", "opus", "haiku"},
					ComModelo: func(context.Context, string) (ai.Provider, string, error) { return nil, "", nil }},
				{Rotulo: "gemini-cli", Provider: &provedorNomeado{nome: "Gemini CLI"}, Modelos: []string{"gemini-2.5-flash", "gemini-2.5-pro"},
					ComModelo: func(context.Context, string) (ai.Provider, string, error) { return nil, "", nil }},
				{Rotulo: "ollama", Provider: &provedorNomeado{nome: "Ollama (Local)"}, Padrao: "llama3", Modelos: []string{"llama3"},
					Aviso:     "o Ollama não respondeu",
					ComModelo: func(context.Context, string) (ai.Provider, string, error) { return nil, "", nil }},
			}
			m := New(claude, nil, Config{SessionID: "20261024-100000", AIModel: "claude-cli", Cluster: "staging", Namespace: "default"})
			m.agora = relogio("10:00")
			m.novoSpinner = func(r string) ui.Spinner { return ui.NovoSpinnerCom(r, func() bool { return false }) }
			m.novoEstado = func(r string) ui.Estado { return ui.NovoEstadoCarregandoCom(r, func() bool { return false }) }
			m.config.Modelos = func(context.Context) []Modelo { return lista }
			atualizado, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m = abrirModelo(t, atualizado.(Model))

			tela := m.View()
			linhas := strings.Split(tela, "\n")
			if len(linhas) != 24 {
				t.Errorf("altura: %d linhas", len(linhas))
			}
			for i, l := range linhas {
				if w := lipgloss.Width(l); w != 80 {
					t.Errorf("linha %d com %d células: %q", i+1, w, l)
				}
			}
			golden.RequireEqual(t, []byte(tela))

			limpa := ansi.Strip(tela)
			for _, trecho := range []string{"em uso · modelo padrão do CLI", "o Ollama não respondeu · llama3", textoDicaModelo,
				"gemini-cli/gemini-2.5-flash", "sessão de hoje"} {
				if !strings.Contains(limpa, trecho) {
					t.Errorf("a 80 colunas falta %q:\n%s", trecho, limpa)
				}
			}
		})
	}
}
