package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// Comandos / do Bard (prancha "comandos" e o /contexto da prancha "tema kraft").

var teclaEnter = tea.KeyMsg{Type: tea.KeyEnter}

// teclasDe devolve uma tecla por caractere do texto.
func teclasDe(texto string) []tea.KeyMsg {
	var teclas []tea.KeyMsg
	for _, r := range texto {
		teclas = append(teclas, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return teclas
}

// teclarModelo passa as teclas pelo Update e devolve o modelo e o comando da última.
func teclarModelo(t *testing.T, m Model, teclas ...tea.KeyMsg) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range teclas {
		var atualizado tea.Model
		atualizado, cmd = m.Update(k)
		m = atualizado.(Model)
	}
	return m, cmd
}

// rodarComando executa o comando devolvido pelo Update e entrega a mensagem dele.
func rodarComando(t *testing.T, m Model, cmd tea.Cmd) (Model, tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("o comando deveria devolver um tea.Cmd")
	}
	atualizado, proximo := m.Update(cmd())
	return atualizado.(Model), proximo
}

func contextoDeExemplo() InfoContexto {
	return InfoContexto{
		Cluster: "staging · eks · us-east-1", Projeto: "yby-cli · topologia standard", Embeddings: "embeddings locais",
		Synapstor: true, Notas: 12, NotasEm: time.Date(2026, 10, 22, 12, 0, 0, 0, time.Local),
	}
}

func ferramentasDeExemplo() []tools.Situacao {
	return []tools.Situacao{
		{Nome: "atlas_blueprint", Familia: "atlas", Disponivel: true, Descricao: "Obtém o blueprint do projeto via plugin Atlas"},
		{Nome: "kubectl_get", Familia: "kubectl", Disponivel: true, Descricao: "Executa kubectl get para listar recursos Kubernetes"},
		{Nome: "kubectl_logs", Familia: "kubectl", Disponivel: true, Descricao: "Mostra os logs de um pod."},
		{Nome: "sentinel_scan", Familia: "sentinel", Disponivel: true, Descricao: "Scan de segurança do namespace"},
		{Nome: "deploy_status", Familia: "deploy", Externa: true, Disponivel: true, Descricao: "Status do deploy"},
		{Nome: "~/.yby/tools/backup.yaml", Externa: true, Invalida: true, Nota: "falta o campo command"},
	}
}

// TestTelaComandos reproduz a prancha "comandos": a / no campo abre a lista dos 8
// comandos sobre ele, com /sessões marcado, a conversa apagada atrás e a legenda da lista.
func TestTelaComandos(t *testing.T) {
	for _, md := range []modo{
		{"escuro", ui.TemaEscuro, false, termenv.TrueColor},
		{"ascii", ui.TemaEscuro, true, termenv.Ascii},
	} {
		t.Run(md.nome, func(t *testing.T) {
			usarModo(t, md)
			m := telaDeTeste(t, Config{
				Cluster: "staging", Namespace: "default", AIModel: "claude-cli", SessionID: "20261024-114500",
			}, "11:51", "11:52")
			saida := strings.TrimSuffix(strings.Repeat("NAMESPACE NAME STATUS VOLUME\n", 14), "\n")
			m = conversar(t, m, "qual a situação do meu storage?", respostaStorage, 21*time.Second,
				EtapaMsg{Etapa: EtapaFerramenta, Rotulo: "kubectl get", Nota: "pvc,pv -A", Estado: ui.EtapaFeita,
					Duracao: 4 * time.Second, Saida: saida + "\n"})
			m, _ = teclarModelo(t, m, teclasDe("/")...)

			limpa := ansi.Strip(conferirTela(t, m))
			for _, trecho := range []string{"COMANDOS", "/sessões", "/limpar", "/contexto", "/ferramentas", "/modelo",
				"/copiar", "/exportar", "/ajuda", "8 comandos", textoDicaComandos, "sessão de hoje · 2 mensagens", "VOCÊ · 11:51"} {
				if !strings.Contains(limpa, trecho) {
					t.Errorf("a tela deveria ter %q:\n%s", trecho, limpa)
				}
			}
			legenda := "↑↓ escolher  ⇥ completar  ↵ executar  esc fechar"
			if md.ascii {
				legenda = "up/down escolher  tab completar  enter executar  esc fechar"
			}
			if !strings.Contains(limpa, legenda) {
				t.Errorf("legenda da lista %q:\n%s", legenda, limpa)
			}
		})
	}
}

func TestComandosFiltrados(t *testing.T) {
	nomes := func(filtro string) []string {
		var n []string
		for _, c := range comandosFiltrados(filtro) {
			n = append(n, c.nome)
		}
		return n
	}
	casos := map[string][]string{
		"se":    {"sessões"},
		"SESSO": {"sessões"},
		"c":     {"contexto", "copiar"},
		"cl":    nil, // o apelido /clear só vale por extenso
		"x":     nil,
	}
	for filtro, want := range casos {
		if got := nomes(filtro); !reflect.DeepEqual(got, want) {
			t.Errorf("filtro %q: %v, esperado %v", filtro, got, want)
		}
	}
	if got := len(comandosFiltrados("")); got != 8 {
		t.Errorf("sem filtro: %d comandos, esperado 8", got)
	}
}

func TestComandoDigitado(t *testing.T) {
	casos := []struct {
		texto, nome, args string
		ok                bool
	}{
		{"/clear", "limpar", "", true},
		{"/sessoes", "sessões", "", true},
		{"/session 20261023-090000", "sessões", "20261023-090000", true},
		{"/exportar  notas/storage ", "exportar", "notas/storage", true},
		{"/Ajuda", "ajuda", "", true},
		{"/etc/hosts está certo?", "", "", false},
		{"por que /sessões?", "", "", false},
	}
	for _, c := range casos {
		nome, args, ok := comandoDigitado(c.texto)
		if nome != c.nome || args != c.args || ok != c.ok {
			t.Errorf("%q: (%q, %q, %v), esperado (%q, %q, %v)", c.texto, nome, args, ok, c.nome, c.args, c.ok)
		}
	}
}

func TestComandos_TeatestFiltrar(t *testing.T) {
	semCores(t)
	tm := abrirBard(t, telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00"))
	digitar(tm, "/se")
	esperarTela(t, tm, "1 comando")
	m := fecharBard(t, tm)
	lista := m.comandosVisiveis()
	if len(lista) != 1 || lista[0].nome != "sessões" {
		t.Fatalf("/se deveria mostrar só /sessões: %v", lista)
	}
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "/sessões") || strings.Contains(tela, "/limpar") {
		t.Errorf("a lista filtrada:\n%s", tela)
	}
}

func TestComandos_TeatestTabCompleta(t *testing.T) {
	semCores(t)
	tm := abrirBard(t, telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00"))
	digitar(tm, "/fe")
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	esperarTela(t, tm, "/ferramentas")
	m := fecharBard(t, tm)
	if got := m.textarea.Value(); got != "/ferramentas" {
		t.Errorf("⇥ deveria completar o comando: %q", got)
	}
	if len(m.messages) != 0 {
		t.Errorf("⇥ só completa, não executa: %d mensagens", len(m.messages))
	}
}

func TestComandos_TeatestEnterExecuta(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000", Contexto: contextoDeExemplo, Ferramentas: ferramentasDeExemplo}, "10:00")
	tm := abrirBard(t, m)
	digitar(tm, "/")
	esperarTela(t, tm, "8 comandos")
	// ↓ ↓ marca o terceiro, /contexto; ↑ e ↓ de novo voltam para ele.
	for _, k := range []tea.KeyType{tea.KeyDown, tea.KeyDown, tea.KeyUp, tea.KeyDown} {
		tm.Send(tea.KeyMsg{Type: k})
	}
	tm.Send(teclaEnter)
	esperarTela(t, tm, "CONTEXTO DESTA SESSÃO")
	final := fecharBard(t, tm)
	if final.textarea.Value() != "" || final.comandosVisiveis() != nil {
		t.Errorf("depois de executar, o campo fica vazio e a lista fecha: %q", final.textarea.Value())
	}
	if n := len(final.messages); n != 2 || final.messages[0].content != "/contexto" || !final.messages[0].comando ||
		final.messages[1].role != "quadro" {
		t.Fatalf("o comando e o quadro deveriam entrar na conversa: %+v", final.messages)
	}
}

func TestComandos_TeatestEscFecha(t *testing.T) {
	semCores(t)
	tm := abrirBard(t, telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00"))
	digitar(tm, "/mo")
	esperarTela(t, tm, "1 comando")
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	// Depois do esc o Bard segue aberto e as letras voltam a ser do campo.
	digitar(tm, "oi")
	esperarTela(t, tm, "oi")
	m := fecharBard(t, tm)
	if got := m.textarea.Value(); got != "oi" {
		t.Errorf("esc deveria limpar a barra e o filtro: %q", got)
	}
	if m.comandosVisiveis() != nil {
		t.Error("a lista deveria estar fechada")
	}
}

func TestComandos_NaRespostaNaoAbre(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	m.state = stateStreaming
	m, _ = teclarModelo(t, m, append(teclasDe("/limpar"), teclaEnter)...)
	if m.comandosVisiveis() != nil || m.textarea.Value() != "/limpar" || m.config.SessionID != "20261024-100000" {
		t.Errorf("durante a resposta a / é texto e o ↵ espera: campo %q, sessão %s", m.textarea.Value(), m.config.SessionID)
	}
}

func TestComandos_BarraQueNaoEComandoViraPergunta(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	m, _ = teclarModelo(t, m, append(teclasDe("/etc/hosts está certo?"), teclaEnter)...)
	if m.cancelar != nil {
		defer m.cancelar()
	}
	if m.state != stateStreaming || len(m.messages) != 1 || m.messages[0].comando || m.messages[0].content != "/etc/hosts está certo?" {
		t.Errorf("a pergunta deveria ir para a IA: estado %d, %+v", m.state, m.messages)
	}
}

func TestComandos_ComArgumentosPeloEnter(t *testing.T) {
	semCores(t)
	dir := t.TempDir()
	m := telaDeTeste(t, Config{SessionID: "20261024-100000", Diretorio: dir}, "10:00")
	m = conversar(t, m, "oi", "olá", time.Second)
	// Com espaço a lista fecha; o ↵ reconhece o comando e passa o argumento.
	m, _ = teclarModelo(t, m, append(teclasDe("/exportar notas/oi"), teclaEnter)...)
	if _, err := os.Stat(filepath.Join(dir, "notas", "oi.md")); err != nil {
		t.Errorf("o /exportar com nome deveria gravar notas/oi.md: %v", err)
	}
	if m.textarea.Value() != "" {
		t.Errorf("o campo deveria esvaziar: %q", m.textarea.Value())
	}
}

func TestComandos_InterrogacaoAbreAjuda(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	m, _ = teclarModelo(t, m, teclasDe("?")...)
	if len(m.messages) != 2 || m.messages[1].quadro == nil || m.messages[1].quadro.rotulo != "ajuda" {
		t.Fatalf("? no campo vazio abre a ajuda: %+v", m.messages)
	}
	tela := ansi.Strip(m.View())
	for _, trecho := range []string{"AJUDA", "ctrl+y", "exemplos", "por que o pod api está reiniciando?"} {
		if !strings.Contains(tela, trecho) {
			t.Errorf("a ajuda deveria ter %q:\n%s", trecho, tela)
		}
	}
	// No meio do texto, ? é só texto.
	m, _ = teclarModelo(t, m, teclasDe("tudo bem?")...)
	if m.textarea.Value() != "tudo bem?" || len(m.messages) != 2 {
		t.Errorf("? no texto: campo %q, %d mensagens", m.textarea.Value(), len(m.messages))
	}
}

func TestComandos_Limpar(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00", "10:01", "10:30")
	m = conversar(t, m, "oi", "olá", time.Second)
	m, _ = teclarModelo(t, m, append(teclasDe("/limpar"), teclaEnter)...)
	if len(m.messages) != 1 || m.messages[0].content != "conversa nova; a anterior fica em /sessões" {
		t.Fatalf("/limpar deixa só a nota: %+v", m.messages)
	}
	if m.config.SessionID != "20261024-103000" || m.ultimoFim != nil {
		t.Errorf("/limpar abre outra sessão: %s", m.config.SessionID)
	}
	tela := ansi.Strip(m.View())
	if strings.Contains(tela, "olá") || !strings.Contains(tela, "nenhuma mensagem") {
		t.Errorf("a conversa anterior sai da tela:\n%s", tela)
	}
	// Sem conversa, a nota não fala da anterior.
	m, _ = teclarModelo(t, m, append(teclasDe("/limpar"), teclaEnter)...)
	if m.messages[0].content != "conversa nova" {
		t.Errorf("nota sem conversa anterior: %q", m.messages[0].content)
	}
}

// historicoFalso faz o papel do history.go: duas sessões salvas.
func historicoFalso(pedidas *[]string) (func() ([]Sessao, error), func(string) (Retomada, error)) {
	inicio := time.Date(2026, 10, 23, 9, 10, 0, 0, time.Local)
	sessoes := func() ([]Sessao, error) {
		return []Sessao{
			{ID: "20261024-100000", Mensagens: 2, Inicio: time.Date(2026, 10, 24, 10, 0, 0, 0, time.Local), Primeira: "oi"},
			{ID: "20261023-091000", Mensagens: 2, Inicio: inicio, Primeira: "qual a situação do meu storage?\nsegunda linha"},
		}, nil
	}
	retomar := func(id string) (Retomada, error) {
		*pedidas = append(*pedidas, id)
		if id != "20261023-091000" {
			return Retomada{}, nil
		}
		return Retomada{SystemPrompt: "sistema com a sessão", Historico: HistoricoPrompt{Mensagens: 2, Sessoes: 1}, Mensagens: []MensagemSalva{
			{Papel: "user", Texto: "qual a situação do meu storage?", Hora: inicio},
			{Papel: "assistant", Texto: "seis volumes", Hora: inicio.Add(time.Minute)},
		}}, nil
	}
	return sessoes, retomar
}

func TestComandos_SessoesRetoma(t *testing.T) {
	semCores(t)
	var pedidas []string
	sessoes, retomar := historicoFalso(&pedidas)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000", SystemPrompt: "sistema", Sessoes: sessoes, RetomarSessao: retomar}, "10:00")
	m, _ = teclarModelo(t, m, append(teclasDe("/se"), teclaEnter)...)
	if m.seletor == nil || len(m.seletor.itens) != 2 {
		t.Fatalf("/sessões abre a lista das conversas: %+v", m.seletor)
	}
	tela := ansi.Strip(m.View())
	for _, trecho := range []string{"SESSÕES", "hoje 10:00", "esta conversa · 2 mensagens · oi", "ontem 09:10",
		"2 mensagens · qual a situação do meu storage?", "2 conversas", "↵ retomar", "esc volta para a conversa"} {
		if !strings.Contains(tela, trecho) {
			t.Errorf("a lista deveria ter %q:\n%s", trecho, tela)
		}
	}
	// As letras não vão para o campo com a lista aberta.
	m, _ = teclarModelo(t, m, teclasDe("x")...)
	if m.textarea.Value() != "" {
		t.Errorf("letra com a lista aberta: %q", m.textarea.Value())
	}
	// ↵ na conversa atual só fecha.
	m, _ = teclarModelo(t, m, teclaEnter)
	if m.seletor != nil || len(pedidas) != 0 {
		t.Fatalf("↵ na conversa atual fecha sem retomar: %v", pedidas)
	}
	m, _ = teclarModelo(t, m, append(teclasDe("/sessões"), teclaEnter, tea.KeyMsg{Type: tea.KeyDown}, teclaEnter)...)
	if !reflect.DeepEqual(pedidas, []string{"20261023-091000"}) || m.seletor != nil {
		t.Fatalf("↵ retoma a marcada: %v", pedidas)
	}
	if m.config.SessionID != "20261023-091000" || m.fluxo.SystemPrompt != "sistema com a sessão" {
		t.Errorf("a sessão e o prompt passam a ser os da retomada: %s %q", m.config.SessionID, m.fluxo.SystemPrompt)
	}
	if n := len(m.messages); n != 3 || m.messages[2].content != "conversa de 23/10 retomada" {
		t.Fatalf("mensagens retomadas: %+v", m.messages)
	}
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "seis volumes") || !strings.Contains(tela, "sessão de 23/10 · 2 mensagens") {
		t.Errorf("a conversa retomada na tela:\n%s", tela)
	}
}

func TestComandos_SessoesDiretoEFalhas(t *testing.T) {
	semCores(t)
	var pedidas []string
	sessoes, retomar := historicoFalso(&pedidas)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000", Sessoes: sessoes, RetomarSessao: retomar}, "10:00")

	// /sessões <id> retoma direto, sem abrir a lista.
	m, _ = teclarModelo(t, m, append(teclasDe("/sessões 20261023-091000"), teclaEnter)...)
	if m.seletor != nil || m.config.SessionID != "20261023-091000" {
		t.Errorf("/sessões <id> retoma direto: %s", m.config.SessionID)
	}
	// Uma sessão que não está no histórico vira aviso e não mexe na conversa.
	m, _ = teclarModelo(t, m, append(teclasDe("/sessões 1999"), teclaEnter)...)
	if m.aviso == nil || m.aviso.texto != "a conversa 1999 não está no histórico" || m.config.SessionID != "20261023-091000" {
		t.Errorf("sessão inexistente: %+v", m.aviso)
	}

	// Sem histórico configurado, o comando avisa.
	m = telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	m, _ = teclarModelo(t, m, append(teclasDe("/sessões"), teclaEnter)...)
	if m.aviso == nil || m.aviso.ok || !strings.Contains(ansi.Strip(m.View()), "o histórico de conversas não está disponível") {
		t.Errorf("sem histórico: %+v", m.aviso)
	}

	// Com o histórico vazio, a lista mostra o estado vazio.
	m = telaDeTeste(t, Config{SessionID: "20261024-100000",
		Sessoes: func() ([]Sessao, error) { return nil, nil }, RetomarSessao: retomar}, "10:00")
	m, _ = teclarModelo(t, m, append(teclasDe("/sessões"), teclaEnter)...)
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "nenhuma conversa salva nos últimos 30 dias") || strings.Contains(tela, "↵ retomar") {
		t.Errorf("histórico vazio:\n%s", tela)
	}
	m, _ = teclarModelo(t, m, teclaEnter, tea.KeyMsg{Type: tea.KeyEsc})
	if m.seletor != nil {
		t.Error("esc fecha a lista vazia")
	}

	// Um erro de leitura vira aviso.
	m = telaDeTeste(t, Config{SessionID: "20261024-100000",
		Sessoes: func() ([]Sessao, error) { return nil, errors.New("disco") }, RetomarSessao: retomar}, "10:00")
	m, _ = teclarModelo(t, m, append(teclasDe("/sessões"), teclaEnter)...)
	if m.seletor != nil || m.aviso == nil || m.aviso.texto != "não deu para ler o histórico de conversas" {
		t.Errorf("erro do histórico: %+v", m.aviso)
	}
}

func TestComandos_ModeloTroca(t *testing.T) {
	semCores(t)
	claude, gemini := &provedorFalso{}, &provedorFalso{}
	detectados := func() []Modelo {
		return []Modelo{{Rotulo: "claude-cli", Provider: claude}, {Rotulo: "gemini-cli", Provider: gemini}, {Rotulo: "nada"}}
	}
	m := telaDeTeste(t, Config{SessionID: "20261024-100000", AIModel: "claude-cli"}, "10:00")
	m.config.Modelos = func(ctxDet context.Context) []Modelo { return detectados() }
	m.provider, m.fluxo.Provider = claude, claude

	m, cmd := teclarModelo(t, m, append(teclasDe("/modelo"), teclaEnter)...)
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "procurando providers de IA") || !strings.Contains(tela, "MODELO") {
		t.Errorf("enquanto detecta:\n%s", tela)
	}
	m, _ = rodarComando(t, m, cmd)
	if m.seletor == nil || len(m.seletor.itens) != 2 || m.seletor.marcado != 0 {
		t.Fatalf("os providers detectados, com o atual marcado: %+v", m.seletor)
	}
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "claude-cli") || !strings.Contains(tela, "falso · em uso") || !strings.Contains(tela, "2 providers") {
		t.Errorf("lista de providers:\n%s", tela)
	}
	// Uma detecção antiga não mexe na lista.
	atualizado, _ := m.Update(modelosMsg{pedido: m.pedidos - 1})
	m = atualizado.(Model)
	if len(m.seletor.itens) != 2 {
		t.Fatal("a detecção antiga deveria ser descartada")
	}

	m, _ = teclarModelo(t, m, tea.KeyMsg{Type: tea.KeyDown}, teclaEnter)
	if m.seletor != nil || m.fluxo.Provider != ai.Provider(gemini) || m.provider != ai.Provider(gemini) || m.config.AIModel != "gemini-cli" {
		t.Fatalf("↵ troca o provider: %s", m.config.AIModel)
	}
	if ultima := m.messages[len(m.messages)-1]; ultima.content != "provider trocado para gemini-cli" {
		t.Errorf("a troca fica na conversa: %+v", ultima)
	}
	if !strings.Contains(m.contexto(), "gemini-cli") {
		t.Errorf("o cabeçalho mostra o provider novo: %q", m.contexto())
	}

	// /modelo <nome> troca direto quando a detecção chega.
	m, cmd = teclarModelo(t, m, append(teclasDe("/modelo claude-cli"), teclaEnter)...)
	m, _ = rodarComando(t, m, cmd)
	if m.seletor != nil || m.config.AIModel != "claude-cli" || m.fluxo.Provider != ai.Provider(claude) {
		t.Errorf("/modelo claude-cli: %s", m.config.AIModel)
	}
	// Um provider que não foi detectado fecha a lista e o aviso aparece na tela.
	m, cmd = teclarModelo(t, m, append(teclasDe("/modelo openai"), teclaEnter)...)
	m, _ = rodarComando(t, m, cmd)
	if m.seletor != nil || m.aviso == nil || m.aviso.texto != "nenhum provider openai disponível" {
		t.Errorf("provider que não existe: seletor %+v, aviso %+v", m.seletor, m.aviso)
	}
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "✕ nenhum provider openai disponível") || strings.Contains(tela, "MODELO") {
		t.Errorf("o aviso de falha deveria estar na linha de estado:\n%s", tela)
	}
}

// TestComandos_ModeloSpinnerGira: enquanto a detecção não chega, o spinner do
// "procurando providers de IA" anda com os tiques que passam pelo Update.
func TestComandos_ModeloSpinnerGira(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	m.novoEstado = func(r string) ui.Estado { return ui.NovoEstadoCarregandoCom(r, func() bool { return true }) }
	m.config.Modelos = func(context.Context) []Modelo { return nil }
	m, cmd := teclarModelo(t, m, append(teclasDe("/modelo"), teclaEnter)...)
	lote, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("o /modelo dispara a detecção e o spinner juntos: %T", cmd())
	}
	var tique tea.Msg
	for _, c := range lote {
		if c == nil {
			continue
		}
		if msg, ok := c().(ui.SpinnerTickMsg); ok {
			tique = msg
		}
	}
	if tique == nil {
		t.Fatal("o spinner do /modelo deveria pedir um tique")
	}
	antes := m.quadroSeletor()
	atualizado, proximo := m.Update(tique)
	m = atualizado.(Model)
	if proximo == nil || m.quadroSeletor() == antes {
		t.Errorf("o tique deveria girar o spinner e pedir o próximo:\n%s", m.quadroSeletor())
	}
	// Com a lista preenchida, o tique morre.
	atualizado, _ = m.Update(modelosMsg{pedido: m.pedidos})
	m = atualizado.(Model)
	if _, proximo := m.Update(tique); proximo != nil {
		t.Error("sem o procurando, o tique não pede outro")
	}
}

func TestComandos_ModeloSemDeteccao(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	m, _ = teclarModelo(t, m, append(teclasDe("/modelo"), teclaEnter)...)
	if m.seletor != nil || m.aviso == nil || m.aviso.texto != "a troca de provider não está disponível" {
		t.Errorf("sem detecção: %+v", m.aviso)
	}
	m.config.Modelos = func(context.Context) []Modelo { return nil }
	m, cmd := teclarModelo(t, m, append(teclasDe("/modelo"), teclaEnter)...)
	m, _ = rodarComando(t, m, cmd)
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "nenhum provider de IA disponível") {
		t.Errorf("nenhum provider:\n%s", tela)
	}
}

func TestComandos_Copiar(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	var copiado string
	m.copiar = func(texto string) error { copiado = texto; return nil }

	m, _ = teclarModelo(t, m, append(teclasDe("/copiar"), teclaEnter)...)
	if m.aviso == nil || m.aviso.texto != "ainda não há resposta para copiar" {
		t.Errorf("sem resposta: %+v", m.aviso)
	}
	m = conversar(t, m, "oi", "**olá**, tudo certo", time.Second)
	m, cmd := teclarModelo(t, m, append(teclasDe("/copiar"), teclaEnter)...)
	m, _ = rodarComando(t, m, cmd)
	if copiado != "**olá**, tudo certo" {
		t.Errorf("/copiar manda a resposta em markdown: %q", copiado)
	}
	if !strings.Contains(ansi.Strip(m.View()), "resposta enviada à área de transferência") {
		t.Errorf("aviso da cópia:\n%s", ansi.Strip(m.View()))
	}
}

func TestComandos_Exportar(t *testing.T) {
	semCores(t)
	dir := t.TempDir()
	m := telaDeTeste(t, Config{SessionID: "20261024-114500", Cluster: "staging", Namespace: "default", AIModel: "claude-cli",
		Diretorio: dir, Sanitizar: func(s string) string { return strings.ReplaceAll(s, "sk-segredo", "[REDACTED]") }}, "11:50", "11:51", "11:52")

	m, _ = teclarModelo(t, m, append(teclasDe("/exportar"), teclaEnter)...)
	if m.aviso == nil || m.aviso.texto != "a conversa ainda não tem mensagens para exportar" {
		t.Errorf("conversa vazia: %+v", m.aviso)
	}
	m = conversar(t, m, "minha chave é sk-segredo", "Guarde-a fora do repositório.", time.Second,
		EtapaMsg{Etapa: EtapaFerramenta, Rotulo: "kubectl get", Nota: "pods", Estado: ui.EtapaFeita, Saida: "x\n"})
	m, _ = teclarModelo(t, m, append(teclasDe("/ajuda"), teclaEnter)...)
	m, _ = teclarModelo(t, m, append(teclasDe("/exportar"), teclaEnter)...)

	dados, err := os.ReadFile(filepath.Join(dir, "bard-20261024-114500.md"))
	if err != nil {
		t.Fatalf("o /exportar deveria gravar no diretório do projeto: %v", err)
	}
	want := "# Conversa com o Bard · 24/10/2026 11:45\n\nstaging · default · claude-cli\n\n" +
		"## Você · 11:51\n\nminha chave é [REDACTED]\n\n" +
		"## Bard · 11:52\n\n- ferramenta: `kubectl get pods`\n\nGuarde-a fora do repositório.\n"
	if string(dados) != want {
		t.Errorf("markdown exportado:\n%s\nesperado:\n%s", dados, want)
	}
	if m.aviso == nil || !m.aviso.ok || m.aviso.texto != "conversa salva em bard-20261024-114500.md" {
		t.Errorf("aviso do /exportar: %+v", m.aviso)
	}

	// Um diretório que não dá para criar vira aviso de falha.
	bloqueio := filepath.Join(dir, "arquivo")
	if err := os.WriteFile(bloqueio, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _ = teclarModelo(t, m, append(teclasDe("/exportar arquivo/conversa.md"), teclaEnter)...)
	if m.aviso == nil || m.aviso.ok || m.aviso.texto != "não deu para salvar a conversa em arquivo/conversa.md" {
		t.Errorf("falha ao gravar: %+v", m.aviso)
	}
}

func TestComandos_FerramentasEContexto(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-120000", AIModel: "claude-cli", Ferramentas: ferramentasDeExemplo}, "12:00")
	m, _ = teclarModelo(t, m, append(teclasDe("/ferramentas"), teclaEnter)...)
	tela := ansi.Strip(m.View())
	for _, trecho := range []string{"FERRAMENTAS", "✓ kubectl get", "executa kubectl get para listar recursos Kubernetes",
		"mostra os logs de um pod", "✓ deploy status", "status do deploy · externa", "✕ ~/.yby/tools/backup.yaml", "falta o campo command"} {
		if !strings.Contains(tela, trecho) {
			t.Errorf("/ferramentas deveria ter %q:\n%s", trecho, tela)
		}
	}

	// Sem o plugin ou sem Synapstor, o /contexto diz o que falta.
	m.config.Ferramentas = func() []tools.Situacao {
		return []tools.Situacao{{Nome: "kubectl_get", Familia: "kubectl", Nota: "kubectl não está no PATH"}}
	}
	m, _ = teclarModelo(t, m, append(teclasDe("/contexto"), teclaEnter)...)
	tela = ansi.Strip(m.View())
	for _, trecho := range []string{"cluster       sem cluster", "namespace     sem namespace", "provider      claude-cli",
		"notas         sem Synapstor", "ferramentas   ▲ kubectl", "histórico     nenhuma conversa anterior"} {
		if !strings.Contains(tela, trecho) {
			t.Errorf("/contexto sem nada deveria ter %q:\n%s", trecho, tela)
		}
	}
	if strings.Contains(tela, "│  projeto") {
		t.Errorf("sem projeto, a linha some:\n%s", tela)
	}
}

func TestResumoFerramentas(t *testing.T) {
	semCores(t)
	lista := []tools.Situacao{
		{Nome: "outra_coisa", Familia: "outra", Disponivel: true},
		{Nome: "atlas_blueprint", Familia: "atlas", Disponivel: true},
		{Nome: "kubectl_get", Familia: "kubectl", Disponivel: true},
		{Nome: "kubectl_logs", Familia: "kubectl", Nota: "kubectl não está no PATH"},
		{Nome: "a.yaml", Externa: true, Invalida: true},
		{Nome: "b.yaml", Externa: true, Invalida: true},
	}
	if got, want := resumoFerramentas(lista), "▲ kubectl  ✓ atlas  ✓ outra  ▲ 2 externas inválidas"; got != want {
		t.Errorf("resumo: %q, esperado %q", got, want)
	}
	if got := resumoFerramentas(nil); got != "nenhuma" {
		t.Errorf("sem ferramentas: %q", got)
	}
	m := telaDeTeste(t, Config{Ferramentas: func() []tools.Situacao { return nil }}, "10:00")
	if q := m.quadroFerramentas(); len(q.linhas) != 1 || !strings.Contains(q.linhas[0].chave, "nenhuma ferramenta registrada") {
		t.Errorf("quadro sem ferramentas: %+v", q)
	}
}

func TestHaQuanto(t *testing.T) {
	agora := time.Date(2026, 10, 24, 12, 0, 0, 0, time.Local)
	casos := map[time.Duration]string{
		30 * time.Second: "agora há pouco",
		5 * time.Minute:  "há 5 min",
		time.Hour:        "há 1 hora",
		3 * time.Hour:    "há 3 horas",
		30 * time.Hour:   "ontem",
		50 * time.Hour:   "há 2 dias",
	}
	for d, want := range casos {
		if got := haQuanto(agora.Add(-d), agora); got != want {
			t.Errorf("%v: %q, esperado %q", d, got, want)
		}
	}
}

func TestDataSessao(t *testing.T) {
	agora := time.Date(2026, 10, 24, 12, 0, 0, 0, time.Local)
	casos := []struct {
		t    time.Time
		want string
	}{
		{time.Time{}, "sem data"},
		{time.Date(2026, 10, 24, 9, 5, 0, 0, time.Local), "hoje 09:05"},
		{time.Date(2026, 10, 23, 18, 20, 0, 0, time.Local), "ontem 18:20"},
		{time.Date(2026, 9, 2, 8, 0, 0, 0, time.Local), "02/09 08:00"},
		{time.Date(2025, 12, 31, 8, 0, 0, 0, time.Local), "31/12/2025"},
	}
	for _, c := range casos {
		if got := dataSessao(c.t, agora); got != c.want {
			t.Errorf("%v: %q, esperado %q", c.t, got, c.want)
		}
	}
}

// TestQuadroLista_JanelaSegueOMarcado: com mais itens que a altura, o marcado aparece.
func TestQuadroLista_JanelaSegueOMarcado(t *testing.T) {
	semCores(t)
	var itens []itemLista
	for _, n := range []string{"um", "dois", "tres", "quatro", "cinco", "seis"} {
		itens = append(itens, itemLista{nome: n, descricao: "item " + n})
	}
	q := ansi.Strip(quadroLista("teste", itens, 5, 40, 5))
	linhas := strings.Split(q, "\n")
	if len(linhas) != 5 || !strings.Contains(q, "◆ seis") || strings.Contains(q, "um ") {
		t.Errorf("janela de 3 itens terminando no marcado:\n%s", q)
	}
	if q := ansi.Strip(quadroLista("teste", itens, 0, 40, 5)); !strings.Contains(q, "◆ um") || strings.Contains(q, "quatro") {
		t.Errorf("janela no começo:\n%s", q)
	}
}

func TestJuntarDireita(t *testing.T) {
	if got := juntarDireita("› /se        ", "1 comando", 20); got != "› /se      1 comando" || len([]rune(got)) != 20 {
		t.Errorf("%q", got)
	}
	if got := juntarDireita("abcdefghijklmnopqrstuvwxyz", "fim", 12); got != "abcdefg  fim" {
		t.Errorf("corta a esquerda: %q", got)
	}
}

// TestComandos_ExportarNaoSobrescreve: um arquivo do projeto com o nome pedido não é
// truncado; o bard-<sessão>.md e o que o /exportar já gravou podem ser regravados.
func TestComandos_ExportarNaoSobrescreve(t *testing.T) {
	semCores(t)
	dir := t.TempDir()
	ler := func(nome string) string {
		t.Helper()
		dados, err := os.ReadFile(filepath.Join(dir, nome))
		if err != nil {
			t.Fatal(err)
		}
		return string(dados)
	}
	for nome, conteudo := range map[string]string{"README.md": "# projeto\n", "bard-20261024-100000.md": "antiga\n"} {
		if err := os.WriteFile(filepath.Join(dir, nome), []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := telaDeTeste(t, Config{SessionID: "20261024-100000", Diretorio: dir}, "10:00")
	m = conversar(t, m, "oi", "olá", time.Second)

	m, _ = teclarModelo(t, m, append(teclasDe("/exportar README"), teclaEnter)...)
	if m.aviso == nil || m.aviso.ok || m.aviso.texto != "já existe README.md; use outro nome" {
		t.Errorf("arquivo do projeto: %+v", m.aviso)
	}
	if got := ler("README.md"); got != "# projeto\n" {
		t.Errorf("o README.md não pode mudar: %q", got)
	}
	if !strings.Contains(ansi.Strip(m.View()), "✕ já existe README.md; use outro nome") {
		t.Errorf("o aviso na tela:\n%s", ansi.Strip(m.View()))
	}
	// Com outra caixa: no Windows é o mesmo arquivo e a recusa vale; em outros sistemas
	// vira outro arquivo. Nos dois casos o README.md fica como estava.
	m, _ = teclarModelo(t, m, append(teclasDe("/exportar readme"), teclaEnter)...)
	if got := ler("README.md"); got != "# projeto\n" {
		t.Errorf("o README.md não pode mudar com outra caixa: %q", got)
	}

	// O que o /exportar gravou nesta execução pode ser gravado de novo.
	m, _ = teclarModelo(t, m, append(teclasDe("/exportar notas"), teclaEnter)...)
	m = conversar(t, m, "e agora?", "tudo certo", time.Second)
	m, _ = teclarModelo(t, m, append(teclasDe("/exportar notas"), teclaEnter)...)
	if m.aviso == nil || !m.aviso.ok || !strings.Contains(ler("notas.md"), "tudo certo") {
		t.Errorf("reexportar notas.md: %+v", m.aviso)
	}
	// O bard-<sessão>.md é o nome do próprio Bard: reexportar regrava.
	m, _ = teclarModelo(t, m, append(teclasDe("/exportar"), teclaEnter)...)
	if m.aviso == nil || !m.aviso.ok || !strings.Contains(ler("bard-20261024-100000.md"), "tudo certo") {
		t.Errorf("reexportar bard-<sessão>.md: %+v", m.aviso)
	}
}

// TestComandos_ContextoContaOHistoricoDoPrompt: a linha histórico do /contexto mostra o
// que foi no system prompt: o da abertura, o da sessão retomada e, depois do /limpar, o
// da abertura de novo. A conversa atual não entra na conta.
func TestComandos_ContextoContaOHistoricoDoPrompt(t *testing.T) {
	semCores(t)
	var pedidas []string
	sessoes, retomar := historicoFalso(&pedidas)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000", SystemPrompt: "sistema", Sessoes: sessoes, RetomarSessao: retomar,
		Historico: HistoricoPrompt{Mensagens: 20, Sessoes: 3}}, "10:00")
	historico := func() string {
		t.Helper()
		m, _ = teclarModelo(t, m, append(teclasDe("/contexto"), teclaEnter)...)
		ultimo := m.messages[len(m.messages)-1].quadro
		return ansi.Strip(ultimo.linhas[len(ultimo.linhas)-1].valor)
	}
	m = conversar(t, m, "oi", "olá", time.Second)
	if got := historico(); got != "últimas 20 mensagens de 3 sessões" {
		t.Errorf("na abertura: %q", got)
	}
	m, _ = teclarModelo(t, m, append(teclasDe("/sessões 20261023-091000"), teclaEnter)...)
	if got := historico(); got != "últimas 2 mensagens de 1 sessão" {
		t.Errorf("depois de retomar: %q", got)
	}

	// O /limpar volta ao prompt da abertura: a conversa nova não leva a sessão retomada.
	m, _ = teclarModelo(t, m, append(teclasDe("/limpar"), teclaEnter)...)
	if m.fluxo.SystemPrompt != "sistema" || m.config.SystemPrompt != "sistema" {
		t.Errorf("o /limpar volta ao prompt da abertura: %q", m.fluxo.SystemPrompt)
	}
	if got := historico(); got != "últimas 20 mensagens de 3 sessões" {
		t.Errorf("depois do /limpar: %q", got)
	}
}

// TestComandos_EditarUltimaIgnoraComandos: o ↑ no campo vazio traz a última pergunta à
// IA, não o último comando /.
func TestComandos_EditarUltimaIgnoraComandos(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	m, _ = teclarModelo(t, m, append(teclasDe("/ajuda"), teclaEnter)...)
	if m.podeEditarUltima() {
		t.Error("só com comandos não há pergunta para editar")
	}
	m = conversar(t, m, "oi", "olá", time.Second)
	m, _ = teclarModelo(t, m, append(teclasDe("/ajuda"), teclaEnter, tea.KeyMsg{Type: tea.KeyUp})...)
	if got := m.textarea.Value(); got != "oi" || m.comandosVisiveis() != nil {
		t.Errorf("o ↑ traz a última pergunta: %q", got)
	}
}

// TestComandos_MarcadoVoltaAoInicio: a lista que fecha porque o campo esvaziou reabre com
// /sessões marcado, como na prancha.
func TestComandos_MarcadoVoltaAoInicio(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	apagar := tea.KeyMsg{Type: tea.KeyBackspace}
	m, _ = teclarModelo(t, m, append(teclasDe("/"), tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyDown}, apagar)...)
	m, _ = teclarModelo(t, m, teclasDe("/")...)
	if lista := m.comandosVisiveis(); lista[m.comandoMarcado(lista)].nome != "sessões" {
		t.Errorf("ao reabrir, o marcado é /sessões: %q", m.comandoSel)
	}
	// Um filtro que não combina com nada também fecha a lista.
	m, _ = teclarModelo(t, m, append([]tea.KeyMsg{{Type: tea.KeyDown}}, teclasDe("x")...)...)
	m, _ = teclarModelo(t, m, apagar)
	if lista := m.comandosVisiveis(); lista[m.comandoMarcado(lista)].nome != "sessões" {
		t.Errorf("depois de um filtro sem comandos, o marcado é /sessões: %q", m.comandoSel)
	}
}

// TestComandos_AjudaDuranteResposta: com o campo vazio, o ? abre a ajuda numa camada sem
// mexer na rodada; o esc fecha a camada sem interromper a resposta.
func TestComandos_AjudaDuranteResposta(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	m.state = stateStreaming
	interrompeu := false
	m.cancelar = func() { interrompeu = true }
	m, _ = teclarModelo(t, m, teclasDe("?")...)
	if !m.ajuda || len(m.messages) != 0 {
		t.Fatalf("o ? abre a camada sem entrar na conversa: %v %+v", m.ajuda, m.messages)
	}
	tela := ansi.Strip(m.View())
	for _, trecho := range []string{"AJUDA", "esc fechar", "esc volta para a conversa", "interromper · durante a resposta",
		"nas listas", "↑↓ escolher  ⇥ completar  ↵ executar  esc fechar"} {
		if !strings.Contains(tela, trecho) {
			t.Errorf("a camada da ajuda deveria ter %q:\n%s", trecho, tela)
		}
	}
	if strings.Contains(tela, "exemplos") {
		t.Errorf("a camada tem só as teclas:\n%s", tela)
	}
	// As letras não vão para o campo com a camada aberta; o esc fecha sem interromper.
	m, _ = teclarModelo(t, m, append(teclasDe("x"), tea.KeyMsg{Type: tea.KeyEsc})...)
	if m.ajuda || m.textarea.Value() != "" || interrompeu || m.interrompendo {
		t.Errorf("esc fecha a ajuda e a resposta segue: ajuda %v, campo %q, interrompeu %v", m.ajuda, m.textarea.Value(), interrompeu)
	}
	// Com texto no campo, o ? é só texto.
	m, _ = teclarModelo(t, m, teclasDe("e?")...)
	if m.ajuda || m.textarea.Value() != "e?" {
		t.Errorf("? no meio do texto: ajuda %v, campo %q", m.ajuda, m.textarea.Value())
	}
}

// TestQuadroLista_NegritoSoNoNome: no item marcado, só o nome vai em negrito, como na
// prancha; o glifo e a descrição ficam na tinta invertida sem negrito.
func TestQuadroLista_NegritoSoNoNome(t *testing.T) {
	usarModo(t, modo{tema: ui.TemaEscuro, perfil: termenv.TrueColor})
	q := quadroLista("teste", []itemLista{{nome: "/sessões", descricao: "lista as conversas"}}, 0, 60, 5)
	linha := strings.Split(q, "\n")[1]
	s := ui.S()
	if !strings.Contains(linha, s.Selecionado.Render("/sessões")) || !strings.Contains(linha, s.Selecionado.Bold(false).Render(ui.G().Ativo+" ")) {
		t.Errorf("negrito só no nome: %q", linha)
	}
	if strings.Contains(linha, s.Selecionado.Render(ui.G().Ativo+" /sessões")) {
		t.Errorf("o glifo não vai em negrito: %q", linha)
	}
}

// TestQuadroAjuda_TeclasDoKeymap: as teclas do /ajuda vêm dos mesmos bindings da legenda.
func TestQuadroAjuda_TeclasDoKeymap(t *testing.T) {
	semCores(t)
	q := quadroAjuda(true)
	var linhas []string
	for _, l := range q.linhas {
		linhas = append(linhas, ansi.Strip(l.chave)+" | "+ansi.Strip(l.valor))
	}
	texto := strings.Join(linhas, "\n")
	for _, trecho := range []string{"ctrl+j | nova linha", "/ | comandos · no campo vazio", "? | ajuda · no campo vazio",
		"↑ | editar última", "pgup pgdn | rolar", "↵ | ver etapa", "esc | interromper", "ctrl+y | copiar",
		"nas listas | ↑↓ escolher  ⇥ completar  ↵ executar  esc fechar", "ctrl+c | sair", "exemplos | por que o pod api está reiniciando?"} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("a ajuda deveria ter %q:\n%s", trecho, texto)
		}
	}
	if len(quadroAjuda(false).linhas) != len(q.linhas)-len(exemplosPerguntas)-1 {
		t.Error("sem exemplos, só as teclas")
	}
}
