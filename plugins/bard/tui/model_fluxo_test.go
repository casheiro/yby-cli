package tui

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
	tea "github.com/charmbracelet/bubbletea"
)

// salvas registra as chamadas a SaveMessage.
type salvas struct {
	mu    sync.Mutex
	lista []string
}

func (s *salvas) salvar(role, content, _ string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lista = append(s.lista, role+": "+content)
}

func (s *salvas) todas() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.lista...)
}

// modeloComFluxo cria um modelo dimensionado com o provider falso.
func modeloComFluxo(t *testing.T, p *provedorFalso, s *salvas) Model {
	t.Helper()
	isolarHome(t)
	cfg := Config{SystemPrompt: "sistema", SessionID: "sessao"}
	if s != nil {
		cfg.SaveMessage = s.salvar
	}
	m := New(p, nil, cfg)
	m.fluxo.Retry = retryRapido
	m.fluxo.BuscarFerramenta = func(string) *tools.Tool { return nil }
	atualizado, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return atualizado.(Model)
}

// enviar digita a pergunta e aperta enter, devolvendo o comando do primeiro evento.
func enviar(t *testing.T, m Model, pergunta string) (Model, tea.Cmd) {
	t.Helper()
	m.textarea.SetValue(pergunta)
	atualizado, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter deveria iniciar a rodada")
	}
	return atualizado.(Model), cmd
}

// rodar executa os comandos de evento até o FimMsg; aoEvento pode alterar o
// modelo depois de cada evento (ex.: apertar esc).
func rodar(t *testing.T, m Model, cmd tea.Cmd, aoEvento func(Model, tea.Msg) Model) Model {
	t.Helper()
	prazo := time.Now().Add(5 * time.Second)
	for cmd != nil {
		if time.Now().After(prazo) {
			t.Fatal("a rodada não terminou")
		}
		msg := cmd()
		if msg == nil {
			break
		}
		atualizado, proximo := m.Update(msg)
		m = atualizado.(Model)
		cmd = proximo
		if aoEvento != nil {
			m = aoEvento(m, msg)
		}
		if _, fim := msg.(FimMsg); fim {
			break
		}
	}
	return m
}

func TestModel_RodadaCompleta(t *testing.T) {
	s := &salvas{}
	p := &provedorFalso{trechos: []string{"Olá", ", ", "mundo"}}
	m, cmd := enviar(t, modeloComFluxo(t, p, s), "oi")

	if m.state != stateStreaming || m.rodada != 1 || m.cancelar == nil {
		t.Fatalf("enter deveria abrir a rodada 1: estado=%d rodada=%d", m.state, m.rodada)
	}

	var parciais []string
	var viuEtapa bool
	m = rodar(t, m, cmd, func(m Model, msg tea.Msg) Model {
		switch msg.(type) {
		case TrechoMsg:
			parciais = append(parciais, m.parcial)
			if !strings.Contains(m.viewport.View(), m.parcial) {
				t.Errorf("o trecho deveria aparecer no histórico: %q\n%q", m.parcial, m.viewport.View())
			}
		case EtapaMsg:
			if m.etapaAtual() != "" {
				viuEtapa = true
			}
		}
		return m
	})

	if got := strings.Join(parciais, "|"); got != "Olá|Olá, |Olá, mundo" {
		t.Errorf("a resposta deveria crescer a cada trecho: %q", got)
	}
	if !viuEtapa {
		t.Error("o modelo deveria acompanhar a etapa em andamento")
	}
	if m.state != stateIdle || m.cancelar != nil || m.eventos != nil || m.parcial != "" {
		t.Errorf("a rodada deveria terminar limpa: estado=%d parcial=%q", m.state, m.parcial)
	}
	if len(m.messages) != 2 || m.messages[1].role != "assistant" || m.messages[1].content != "Olá, mundo" {
		t.Fatalf("mensagens inesperadas: %#v", m.messages)
	}
	if len(m.etapas) != 3 {
		t.Errorf("esperava 3 etapas registradas (contexto, intenção, resposta), vieram %d", len(m.etapas))
	}
	for _, e := range m.etapas {
		if e.Estado != ui.EtapaFeita {
			t.Errorf("etapa %q deveria terminar feita: %d", e.Rotulo, e.Estado)
		}
	}
	if got := strings.Join(s.todas(), "\n"); got != "user: oi\nassistant: Olá, mundo" {
		t.Errorf("histórico salvo inesperado: %q", got)
	}
}

func TestModel_EscInterrompeSemFechar(t *testing.T) {
	s := &salvas{}
	p := &provedorFalso{trechos: []string{"primeiro", " segundo"}, travar: true, liberar: make(chan struct{})}
	defer close(p.liberar)
	m, cmd := enviar(t, modeloComFluxo(t, p, s), "oi")

	apertou := false
	m = rodar(t, m, cmd, func(m Model, msg tea.Msg) Model {
		if _, ok := msg.(TrechoMsg); ok && !apertou {
			apertou = true
			atualizado, c := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			if c != nil {
				if _, sair := c().(tea.QuitMsg); sair {
					t.Fatal("esc não deveria fechar o Bard")
				}
			}
			m = atualizado.(Model)
			if !m.interrompendo || !strings.Contains(m.viewport.View(), "Interrompendo") {
				t.Error("esc deveria marcar a rodada como interrompendo")
			}
		}
		return m
	})

	if !apertou {
		t.Fatal("nenhum trecho chegou antes do esc")
	}
	if m.state != stateIdle {
		t.Fatal("o Bard deveria voltar a aceitar mensagens")
	}
	n := len(m.messages)
	if n != 3 || m.messages[1].content != "primeiro" || m.messages[2].role != "nota" || m.messages[2].content != "resposta interrompida" {
		t.Fatalf("mensagens inesperadas: %#v", m.messages)
	}
	if got := strings.Join(s.todas(), "\n"); got != "user: oi" {
		t.Errorf("resposta interrompida não vai ao histórico: %q", got)
	}

	// Uma nova pergunta abre a rodada seguinte normalmente.
	p2 := &provedorFalso{trechos: []string{"de novo"}}
	m.fluxo.Provider = p2
	m, cmd = enviar(t, m, "outra")
	m = rodar(t, m, cmd, nil)
	if m.rodada != 2 || m.messages[len(m.messages)-1].content != "de novo" {
		t.Errorf("a rodada 2 deveria responder: rodada=%d %#v", m.rodada, m.messages)
	}
}

func TestModel_RetryDescartaParcial(t *testing.T) {
	m := modeloComFluxo(t, &provedorFalso{}, nil)
	m.state = stateStreaming
	m.rodada = 4
	m.parcial = "texto da tentativa falha"

	atualizado, cmd := m.Update(EtapaMsg{Rodada: 4, Etapa: EtapaResposta, Rotulo: rotuloResposta, Estado: ui.EtapaAtual, Tentativa: 2})
	m = atualizado.(Model)
	if cmd != nil {
		t.Error("sem canal aberto não há próximo evento a pedir")
	}
	if m.parcial != "" {
		t.Errorf("a nova tentativa deveria descartar o parcial: %q", m.parcial)
	}
	if len(m.etapas) != 1 || m.etapas[0].Tentativa != 2 {
		t.Errorf("etapa não registrada: %#v", m.etapas)
	}

	// O estado mais recente da mesma etapa substitui o anterior.
	atualizado, _ = m.Update(EtapaMsg{Rodada: 4, Etapa: EtapaResposta, Rotulo: rotuloResposta, Estado: ui.EtapaFeita})
	m = atualizado.(Model)
	if len(m.etapas) != 1 || m.etapas[0].Estado != ui.EtapaFeita {
		t.Errorf("a etapa deveria ser atualizada no lugar: %#v", m.etapas)
	}
}

func TestModel_IgnoraEventosDeOutraRodada(t *testing.T) {
	m := modeloComFluxo(t, &provedorFalso{}, nil)
	m.state = stateStreaming
	m.rodada = 2

	for _, msg := range []tea.Msg{
		EtapaMsg{Rodada: 1, Etapa: EtapaContexto, Estado: ui.EtapaAtual},
		TrechoMsg{Rodada: 1, Texto: "velho"},
		FimMsg{Rodada: 1, Resposta: "velho"},
	} {
		atualizado, cmd := m.Update(msg)
		m = atualizado.(Model)
		if cmd != nil {
			t.Errorf("evento de outra rodada não deveria pedir o próximo: %T", msg)
		}
	}
	if m.state != stateStreaming || m.parcial != "" || len(m.messages) != 0 || len(m.etapas) != 0 {
		t.Errorf("eventos de outra rodada deveriam ser ignorados: %#v", m)
	}
}

func TestModel_EnterDuranteRodadaNaoReenvia(t *testing.T) {
	m := modeloComFluxo(t, &provedorFalso{}, nil)
	m.state = stateStreaming
	m.textarea.SetValue("mais uma")
	atualizado, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || atualizado.(Model).rodada != 0 {
		t.Error("enter durante a resposta não deveria abrir outra rodada")
	}
}

func TestModel_CtrlCCancelaRodada(t *testing.T) {
	p := &provedorFalso{trechos: []string{"a", "b"}, travar: true, liberar: make(chan struct{})}
	defer close(p.liberar)
	m, cmd := enviar(t, modeloComFluxo(t, p, nil), "oi")
	// Lê até o primeiro trecho e então sai.
	for {
		msg := cmd()
		atualizado, proximo := m.Update(msg)
		m, cmd = atualizado.(Model), proximo
		if _, ok := msg.(TrechoMsg); ok {
			break
		}
	}
	_, sair := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if sair == nil {
		t.Fatal("ctrl+c deveria sair")
	}
	// O cancelamento faz o fluxo terminar com FimMsg interrompido.
	m = rodar(t, m, cmd, nil)
	if m.state != stateIdle || m.messages[len(m.messages)-1].role != "nota" {
		t.Errorf("a rodada deveria ter sido interrompida: %#v", m.messages)
	}
}

func TestModel_TrechoNaoRenderizaOHistoricoDeNovo(t *testing.T) {
	m := modeloComFluxo(t, &provedorFalso{}, nil)
	chamadas := 0
	m.markdown = func(s string) (string, error) {
		chamadas++
		return "**" + s + "**", nil
	}
	for i := 0; i < 20; i++ {
		m.messages = append(m.messages,
			chatMessage{role: "user", content: fmt.Sprintf("pergunta %d", i)},
			chatMessage{role: "assistant", content: fmt.Sprintf("# resposta %d", i)})
	}
	m.state = stateStreaming
	m.rodada = 1
	m.updateViewport()
	if chamadas != 20 {
		t.Fatalf("cada resposta deveria passar pelo markdown uma vez: %d", chamadas)
	}

	for i := 0; i < 200; i++ {
		atualizado, _ := m.Update(TrechoMsg{Rodada: 1, Texto: "tok "})
		m = atualizado.(Model)
	}
	if chamadas != 20 {
		t.Errorf("os trechos não deveriam renderizar o histórico de novo: %d chamadas", chamadas)
	}
	if !strings.Contains(m.viewport.View(), "tok tok") || !strings.Contains(m.historico, "**# resposta 19**") {
		t.Error("o viewport deveria juntar o histórico e o parcial")
	}

	atualizado, _ := m.Update(FimMsg{Rodada: 1, Resposta: "final"})
	m = atualizado.(Model)
	if chamadas != 21 {
		t.Errorf("o fim deveria renderizar só a resposta nova: %d chamadas", chamadas)
	}

	// Largura nova refaz o histórico inteiro.
	atualizado, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = atualizado.(Model)
	if chamadas != 42 {
		t.Errorf("a largura nova deveria renderizar tudo de novo: %d chamadas", chamadas)
	}
}

func TestModel_NovaTentativaTiraTextoFalhoDaTela(t *testing.T) {
	m := modeloComFluxo(t, &provedorFalso{}, nil)
	m.state = stateStreaming
	m.rodada = 1
	atualizado, _ := m.Update(TrechoMsg{Rodada: 1, Texto: "lixo"})
	m = atualizado.(Model)
	atualizado, _ = m.Update(EtapaMsg{Rodada: 1, Etapa: EtapaResposta, Rotulo: rotuloResposta, Estado: ui.EtapaAtual, Tentativa: 2, Nota: "tentando de novo"})
	m = atualizado.(Model)
	if m.parcial != "" || strings.Contains(m.viewport.View(), "lixo") {
		t.Errorf("o anúncio da nova tentativa deveria tirar o texto falho da tela: %q", m.parcial)
	}
}
