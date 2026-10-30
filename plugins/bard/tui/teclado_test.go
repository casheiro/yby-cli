package tui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
)

// Teclado e rolagem do Bard (coluna "Texto" dos atalhos): o teclado é do campo e só
// pgup, pgdn e a roda do mouse rolam a conversa.

// conversaLonga monta o Bard com uma conversa de várias telas, no fim.
func conversaLonga(t *testing.T, p *provedorFalso) Model {
	t.Helper()
	m := telaDeTeste(t, Config{SessionID: "20261024-100000"}, "10:00")
	m.markdown = nil
	if p != nil {
		m.fluxo.Provider = p
		m.fluxo.Retry = retryRapido
	}
	for i := 1; i <= 3; i++ {
		var resposta strings.Builder
		for l := 1; l <= 30; l++ {
			fmt.Fprintf(&resposta, "resposta %d, linha %d\n", i, l)
		}
		m = conversar(t, m, fmt.Sprintf("pergunta %d", i), resposta.String(), time.Second)
	}
	if !m.viewport.AtBottom() || m.viewport.TotalLineCount() < 3*m.viewport.Height {
		t.Fatalf("a conversa deveria ter várias telas e estar no fim: %d linhas", m.viewport.TotalLineCount())
	}
	return m
}

// abrirBard roda o modelo num programa de teste do tamanho das telas.
func abrirBard(t *testing.T, m Model) *teatest.TestModel {
	t.Helper()
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(larguraTela, alturaTela))
}

// fecharBard sai com ctrl+c e devolve o modelo final. Ele não prova que o Bard ficou
// aberto até ali (depois de um tea.Quit o ctrl+c é ignorado e o modelo volta do mesmo
// jeito): quem precisa provar digita depois da tecla e confere o campo.
func fecharBard(t *testing.T, tm *teatest.TestModel) Model {
	t.Helper()
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	return tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(Model)
}

// esperarTela espera o trecho aparecer na saída do programa.
func esperarTela(t *testing.T, tm *teatest.TestModel, trecho string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(trecho))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(10*time.Millisecond))
}

// digitar manda uma tecla por caractere. O tm.Type do teatest manda um byte por
// tecla e parte os acentos.
func digitar(tm *teatest.TestModel, texto string) {
	for _, r := range texto {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// fundo é o deslocamento do fim da conversa.
func fundo(m Model) int {
	return max(m.viewport.TotalLineCount()-m.viewport.Height, 0)
}

// pagina é quanto o pgup anda com a conversa rolada: a altura menos as linhas que o
// aviso das mensagens anteriores cobre no topo e as que o das linhas abaixo cobre no pé.
func pagina(m Model) int {
	return m.viewport.Height - 2*m.linhasAviso()
}

func TestTeclado_LetrasNaoRolam(t *testing.T) {
	semCores(t)
	tm := abrirBard(t, conversaLonga(t, nil))
	tm.Send(tea.KeyMsg{Type: tea.KeyPgUp})
	// No viewport padrão j k d u f b e espaço rolam; aqui são só texto.
	digitar(tm, "jkdufb ")
	m := fecharBard(t, tm)

	if want := fundo(m) - pagina(m); m.viewport.YOffset != want {
		t.Errorf("as letras não deveriam mudar a rolagem: deslocamento %d, esperado %d", m.viewport.YOffset, want)
	}
	if got := m.textarea.Value(); got != "jkdufb " {
		t.Errorf("as letras deveriam ir para o campo: %q", got)
	}
}

func TestTeclado_PgupPgdnRolam(t *testing.T) {
	semCores(t)
	tm := abrirBard(t, conversaLonga(t, nil))
	tm.Send(tea.KeyMsg{Type: tea.KeyPgUp})
	tm.Send(tea.KeyMsg{Type: tea.KeyPgUp})
	tm.Send(tea.KeyMsg{Type: tea.KeyPgDown})
	m := fecharBard(t, tm)

	if want := fundo(m) - pagina(m); m.viewport.YOffset != want {
		t.Errorf("pgup, pgup, pgdn deveriam deixar a conversa uma página acima do fim: %d, esperado %d", m.viewport.YOffset, want)
	}
	if m.textarea.Value() != "" {
		t.Errorf("pgup e pgdn não escrevem no campo: %q", m.textarea.Value())
	}
}

func TestTeclado_RodaDoMouseRola(t *testing.T) {
	semCores(t)
	roda := func(b tea.MouseButton) tea.MouseMsg {
		return tea.MouseMsg{Button: b, Action: tea.MouseActionPress}
	}
	tm := abrirBard(t, conversaLonga(t, nil))
	tm.Send(roda(tea.MouseButtonWheelUp))
	tm.Send(roda(tea.MouseButtonWheelUp))
	tm.Send(roda(tea.MouseButtonWheelDown))
	m := fecharBard(t, tm)

	if want := fundo(m) - m.viewport.MouseWheelDelta; m.viewport.YOffset != want {
		t.Errorf("a roda deveria rolar %d linhas por passo: %d, esperado %d", m.viewport.MouseWheelDelta, m.viewport.YOffset, want)
	}
}

func TestTeclado_SetaEditaUltimaPergunta(t *testing.T) {
	semCores(t)
	tm := abrirBard(t, conversaLonga(t, nil))
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	m := fecharBard(t, tm)
	if got := m.textarea.Value(); got != "pergunta 3" {
		t.Errorf("↑ no campo vazio deveria trazer a última pergunta: %q", got)
	}
	if !m.viewport.AtBottom() {
		t.Error("↑ não rola a conversa")
	}

	// Com texto no campo, ↑ é do cursor e não troca o rascunho.
	tm = abrirBard(t, conversaLonga(t, nil))
	digitar(tm, "rascunho")
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	m = fecharBard(t, tm)
	if got := m.textarea.Value(); got != "rascunho" {
		t.Errorf("↑ com texto no campo não deveria trocar o rascunho: %q", got)
	}

	// Sem pergunta feita, ↑ não faz nada.
	tm = abrirBard(t, telaDeTeste(t, Config{}, "10:00"))
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	if m = fecharBard(t, tm); m.textarea.Value() != "" {
		t.Errorf("sem pergunta, ↑ deixa o campo vazio: %q", m.textarea.Value())
	}
}

func TestTeclado_NovaLinha(t *testing.T) {
	semCores(t)
	tm := abrirBard(t, telaDeTeste(t, Config{}, "10:00"))
	digitar(tm, "linha um")
	// Mede ctrl+j e alt+↵ (ESC + CR), as teclas de nova linha que chegam ao Bubble Tea
	// v1. O ⇧↵ de um terminal real não é reproduzível aqui: chega como ↵ puro e envia.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	digitar(tm, "linha dois")
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlJ})
	digitar(tm, "três")
	m := fecharBard(t, tm)

	if got := m.textarea.Value(); got != "linha um\nlinha dois\ntrês" {
		t.Errorf("ctrl+j e alt+↵ deveriam quebrar a linha no campo: %q", got)
	}
	if len(m.messages) != 0 || m.state != stateIdle {
		t.Errorf("a nova linha não envia: %d mensagens, estado %d", len(m.messages), m.state)
	}
	if m.textarea.Height() != 3 || m.viewport.Height != ui.AlturaCorpoTexto(alturaTela, 3) {
		t.Errorf("o campo deveria crescer com as linhas: campo %d, histórico %d", m.textarea.Height(), m.viewport.Height)
	}
	if linhas := strings.Split(m.View(), "\n"); len(linhas) != alturaTela {
		t.Errorf("a tela deveria manter a altura: %d linhas", len(linhas))
	}

	// O campo cresce até alturaMaxCampo e volta ao tamanho mínimo ao enviar.
	m.textarea.SetValue(strings.Repeat("x\n", 10) + "fim")
	m.ajustarCampo()
	if m.textarea.Height() != alturaMaxCampo {
		t.Errorf("o campo para em %d linhas: %d", alturaMaxCampo, m.textarea.Height())
	}
	atualizado, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = atualizado.(Model)
	if cmd == nil || m.textarea.Height() != alturaCampo || m.viewport.Height != ui.AlturaCorpoTexto(alturaTela, alturaCampo) {
		t.Errorf("↵ envia e o campo volta a %d linhas: %d", alturaCampo, m.textarea.Height())
	}
	if m.cancelar != nil {
		m.cancelar()
	}
	if got := m.messages[0].content; got != strings.Repeat("x\n", 10)+"fim" {
		t.Errorf("a pergunta vai com as quebras: %q", got)
	}
}

// No limite de altura, a linha nova (a do cursor) aparece no campo na hora, sem
// esperar a tecla seguinte; o mesmo vale para a pergunta longa trazida pelo ↑.
func TestTeclado_NovaLinhaNoLimiteMostraCursor(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "10:00")
	teclar := func(k tea.KeyMsg) {
		t.Helper()
		atualizado, _ := m.Update(k)
		m = atualizado.(Model)
		_ = m.View() // como o programa, que desenha entre as teclas
	}
	for i := 1; i <= alturaMaxCampo; i++ {
		if i > 1 {
			teclar(tea.KeyMsg{Type: tea.KeyCtrlJ})
		}
		for _, r := range fmt.Sprintf("linha %d", i) {
			teclar(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	teclar(tea.KeyMsg{Type: tea.KeyCtrlJ})
	campo := ansi.Strip(m.campo())
	if strings.Contains(campo, "linha 1") || !strings.Contains(campo, "linha 6") {
		t.Errorf("o campo deveria rolar até a linha nova do cursor:\n%s", campo)
	}
	if ultima := strings.Split(campo, "\n")[alturaMaxCampo-1]; strings.TrimSpace(ultima) != "" {
		t.Errorf("a última linha do campo é a nova, ainda vazia: %q", ultima)
	}

	// ↑ com uma pergunta mais alta que o campo mostra o fim dela, onde fica o cursor.
	m = telaDeTeste(t, Config{}, "10:00")
	m.messages = append(m.messages, chatMessage{role: "user", content: "p1\np2\np3\np4\np5\np6\np7\np8"})
	_ = m.View()
	teclar(tea.KeyMsg{Type: tea.KeyUp})
	if campo := ansi.Strip(m.campo()); !strings.Contains(campo, "p8") || strings.Contains(campo, "p1") {
		t.Errorf("o campo deveria mostrar o fim da pergunta, com o cursor:\n%s", campo)
	}
}

func TestTeclado_EscInterrompeSemFechar(t *testing.T) {
	semCores(t)
	p := &provedorFalso{trechos: []string{"começo da resposta", " resto"}, travar: true, liberar: make(chan struct{})}
	defer close(p.liberar)
	tm := abrirBard(t, conversaLonga(t, p))
	digitar(tm, "o que houve?")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	esperarTela(t, tm, "começo da resposta")
	// Durante a resposta o campo aceita a próxima pergunta.
	digitar(tm, "próxima")
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	esperarTela(t, tm, "resposta interrompida")
	m := fecharBard(t, tm)

	if m.state != stateIdle || m.cancelar != nil {
		t.Errorf("depois do esc o Bard volta a aceitar perguntas: estado %d", m.state)
	}
	ultimas := m.messages[len(m.messages)-2:]
	if ultimas[0].content != "começo da resposta" || ultimas[1].role != "nota" {
		t.Errorf("a resposta interrompida fica com o que chegou e a nota: %#v", ultimas)
	}
	if got := m.textarea.Value(); got != "próxima" {
		t.Errorf("o texto escrito durante a resposta fica no campo: %q", got)
	}
}

func TestTeclado_EscParadoNaoFecha(t *testing.T) {
	semCores(t)
	tm := abrirBard(t, telaDeTeste(t, Config{}, "10:00"))
	digitar(tm, "antes")
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	// Se o esc fechasse o Bard, o texto de depois não chegaria ao campo.
	digitar(tm, " depois")
	esperarTela(t, tm, "antes depois")
	m := fecharBard(t, tm)
	if got := m.textarea.Value(); got != "antes depois" {
		t.Errorf("esc com o Bard parado não fecha nem mexe no campo: %q", got)
	}
	if len(m.messages) != 0 || m.state != stateIdle {
		t.Errorf("esc parado não envia nem abre rodada: %d mensagens, estado %d", len(m.messages), m.state)
	}
}

func TestTeclado_EnterDuranteRespostaNaoEnvia(t *testing.T) {
	semCores(t)
	p := &provedorFalso{trechos: []string{"parcial"}, travar: true, liberar: make(chan struct{})}
	defer close(p.liberar)
	inicial := telaDeTeste(t, Config{}, "10:00")
	inicial.fluxo.Provider = p
	tm := abrirBard(t, inicial)
	digitar(tm, "primeira")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	esperarTela(t, tm, "parcial")
	digitar(tm, "segunda")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	m := fecharBard(t, tm)
	if m.rodada != 1 || m.textarea.Value() != "segunda" {
		t.Errorf("↵ durante a resposta não abre outra rodada: rodada %d, campo %q", m.rodada, m.textarea.Value())
	}
}

func TestTeclado_CtrlCSaiDuranteResposta(t *testing.T) {
	semCores(t)
	abrirRodada := func(p *provedorFalso) *teatest.TestModel {
		m := telaDeTeste(t, Config{}, "10:00")
		m.fluxo.Provider = p
		tm := abrirBard(t, m)
		digitar(tm, "oi")
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
		esperarTela(t, tm, "parcial")
		return tm
	}

	// Sai na hora, sem esperar a rodada: o provider que ignora o cancelamento segue
	// preso e o programa termina mesmo assim (fecharBard falha se ele não terminar).
	preso := &provedorFalso{trechos: []string{"parcial"}, travar: true, ignorarCtx: true, liberar: make(chan struct{})}
	defer close(preso.liberar)
	fecharBard(t, abrirRodada(preso))

	// Sair cancela o contexto da rodada: o provider deixa de esperar a resposta.
	p := &provedorFalso{trechos: []string{"parcial"}, travar: true, cancelado: make(chan struct{})}
	fecharBard(t, abrirRodada(p))
	select {
	case <-p.cancelado:
	case <-time.After(5 * time.Second):
		t.Error("ctrl+c deveria cancelar o contexto da rodada")
	}
}

// rolarAcima simula o pgup no modelo, fora do programa de teste.
func rolarAcima(t *testing.T, m Model) Model {
	t.Helper()
	atualizado, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	return atualizado.(Model)
}

// perguntar abre a rodada como o ↵ e devolve o modelo com a resposta em andamento.
func perguntar(t *testing.T, m Model, pergunta string) Model {
	t.Helper()
	m.textarea.SetValue(pergunta)
	atualizado, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = atualizado.(Model)
	if m.cancelar != nil {
		m.cancelar() // o teste entrega os eventos à mão
	}
	m.eventos = nil
	return m
}

func TestMensagensNovas(t *testing.T) {
	semCores(t)
	m := conversaLonga(t, nil)
	if n := m.mensagensNovas(); n != 0 {
		t.Fatalf("no fim não há mensagens novas: %d", n)
	}

	// Rolar para cima não cria mensagens novas: o que está abaixo já foi lido.
	m = rolarAcima(t, m)
	if n := m.mensagensNovas(); n != 0 || strings.Contains(m.corpo(), "nova") {
		t.Fatalf("o que já passou pela tela não é novo: %d", n)
	}
	for i := 0; i < 3; i++ {
		atualizado, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = atualizado.(Model)
	}

	// Pergunta no fim, rola para cima durante a resposta: o parcial é novo.
	m = perguntar(t, m, "e agora?")
	if !m.viewport.AtBottom() {
		t.Fatal("↵ leva a conversa para o fim")
	}
	m = rolarAcima(t, m)
	// A resposta em andamento já passou pela tela no fim: sem texto novo, não é nova.
	if n := m.mensagensNovas(); n != 0 {
		t.Errorf("rolar sem chegar texto não cria mensagem nova: %d", n)
	}
	atualizado, _ := m.Update(TrechoMsg{Rodada: m.rodada, Texto: "chegando"})
	m = atualizado.(Model)
	if n := m.mensagensNovas(); n != 1 {
		t.Errorf("a resposta em andamento abaixo da janela é nova: %d", n)
	}
	desloc := m.viewport.YOffset
	atualizado, _ = m.Update(FimMsg{Rodada: m.rodada, Resposta: "resposta final", Duracao: time.Second})
	m = atualizado.(Model)
	if m.viewport.YOffset != desloc {
		t.Errorf("a resposta não puxa a conversa para o fim: %d, antes %d", m.viewport.YOffset, desloc)
	}
	if n := m.mensagensNovas(); n != 1 {
		t.Errorf("a resposta pronta continua sendo uma mensagem nova: %d", n)
	}
	linhas := strings.Split(m.corpo(), "\n")
	if len(linhas) != m.viewport.Height || ansi.Strip(linhas[len(linhas)-1]) != "↓ 1 mensagem nova" || linhas[len(linhas)-2] != "" {
		t.Errorf("o aviso fica no pé do histórico, depois de uma linha em branco: %q", linhas[len(linhas)-2:])
	}

	// Descendo, o aviso some quando o começo da resposta aparece na janela, e não
	// volta ao rolar de novo depois que a conversa passa pelo fim.
	for i := 0; m.mensagensNovas() > 0; i++ {
		if i > 5 {
			t.Fatal("o pgdn deveria chegar ao começo da resposta")
		}
		atualizado, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = atualizado.(Model)
	}
	if ini := m.inicios[len(m.inicios)-1]; ini < m.viewport.YOffset || ini >= m.viewport.YOffset+m.viewport.Height {
		t.Errorf("o aviso só some com o começo da resposta na janela: começo %d, janela %d+%d", ini, m.viewport.YOffset, m.viewport.Height)
	}
	for i := 0; !m.viewport.AtBottom() && i < 5; i++ {
		atualizado, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = atualizado.(Model)
	}
	if !m.viewport.AtBottom() || m.mensagensNovas() != 0 {
		t.Fatalf("no fim a mensagem foi lida: fim=%v novas=%d", m.viewport.AtBottom(), m.mensagensNovas())
	}
	m = rolarAcima(t, m)
	if n := m.mensagensNovas(); n != 0 {
		t.Errorf("a mensagem lida não volta a ser nova: %d", n)
	}
}

// Descendo página por página com o aviso no pé, toda linha da conversa passa pela
// tela: o aviso cobre duas linhas, e o pgdn anda só as que apareceram.
func TestMensagensNovas_PaginarMostraTudo(t *testing.T) {
	semCores(t)
	m := conversaLonga(t, nil)
	m = perguntar(t, m, "e agora?")
	m = rolarAcima(t, m)
	m = rolarAcima(t, m)
	var resposta strings.Builder
	for l := 1; l <= 30; l++ {
		fmt.Fprintf(&resposta, "resposta nova, linha %d\n", l)
	}
	atualizado, _ := m.Update(FimMsg{Rodada: m.rodada, Resposta: resposta.String(), Duracao: time.Second})
	m = atualizado.(Model)
	if m.mensagensNovas() == 0 {
		t.Fatal("a resposta chegou abaixo da janela: deveria haver o aviso")
	}

	// As linhas sob o aviso do topo passaram pela tela na página de cima.
	inicio := m.viewport.YOffset + m.linhasAviso()
	vistas := map[string]bool{}
	for passo := 0; ; passo++ {
		for _, l := range strings.Split(ansi.Strip(m.corpo()), "\n") {
			vistas[strings.TrimSpace(l)] = true
		}
		if m.viewport.AtBottom() {
			break
		}
		if passo > 50 {
			t.Fatal("o pgdn não chegou ao fim")
		}
		atualizado, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = atualizado.(Model)
	}
	conteudo := strings.Split(ansi.Strip(strings.TrimRight(m.historico, "\n")), "\n")
	for i, l := range conteudo[inicio:] {
		if l = strings.TrimSpace(l); l != "" && !vistas[l] {
			t.Errorf("a linha %d nunca apareceu na tela: %q", inicio+i, l)
		}
	}
}

func TestMensagensNovas_PluralENotas(t *testing.T) {
	semCores(t)
	m := conversaLonga(t, nil)
	m = rolarAcima(t, m)
	lidas := m.lidas
	// Duas respostas e uma nota chegam com a conversa rolada: as notas não contam.
	m.messages = append(m.messages,
		chatMessage{role: "assistant", content: "uma", hora: m.agora()},
		chatMessage{role: "nota", content: "resposta interrompida"},
		chatMessage{role: "error", content: "falhou", hora: m.agora()})
	m.updateViewport()
	if m.lidas != lidas {
		t.Fatalf("rolada para cima, a conversa não marca nada como lido: %d, antes %d", m.lidas, lidas)
	}
	if n := m.mensagensNovas(); n != 2 {
		t.Errorf("duas mensagens novas (a nota não conta): %d", n)
	}
	if !strings.Contains(m.corpo(), "↓ 2 mensagens novas") {
		t.Errorf("aviso no plural:\n%s", m.corpo())
	}
}

// TestTelaMensagensNovas mostra a conversa rolada para cima com a resposta que chegou
// abaixo da janela: o aviso "↓ 1 mensagem nova" no pé do histórico.
func TestTelaMensagensNovas(t *testing.T) {
	for _, md := range []modo{
		{"escuro", ui.TemaEscuro, false, termenv.TrueColor},
		{"ascii", ui.TemaEscuro, true, termenv.Ascii},
	} {
		t.Run(md.nome, func(t *testing.T) {
			usarModo(t, md)
			m := telaDeTeste(t, Config{
				Cluster: "staging", Namespace: "payments", AIModel: "claude-cli", SessionID: "20261024-120000",
			}, "12:05", "12:05", "12:07", "12:07", "12:08")
			m = conversar(t, m, "quais pods estão em payments?", respostaStorage, 3*time.Second)
			m = perguntar(t, m, "por que o worker fica reiniciando?")
			m = rolarAcima(t, m)
			atualizado, _ := m.Update(FimMsg{Rodada: m.rodada, Duracao: 9 * time.Second,
				Resposta: "O pod **worker-5d8b6-q7tvn** está em CrashLoopBackOff: não consegue falar com o Postgres."})
			m = atualizado.(Model)

			tela := conferirTela(t, m)
			limpa := ansi.Strip(tela)
			seta := "↓"
			if md.ascii {
				seta = "v"
			}
			if !strings.Contains(limpa, seta+" 1 mensagem nova") {
				t.Errorf("a tela deveria avisar a mensagem nova:\n%s", limpa)
			}
		})
	}
}

// Com a conversa rolada, o topo da janela avisa as mensagens que começam acima dela,
// como na prancha "trabalhando"; no começo da conversa o aviso some.
func TestMensagensAnteriores(t *testing.T) {
	semCores(t)
	m := conversaLonga(t, nil)
	linhas := strings.Split(m.corpo(), "\n")
	n := m.mensagensAnteriores()
	if n == 0 || ansi.Strip(linhas[0]) != fmt.Sprintf("↑ %d mensagens anteriores", n) || linhas[1] != "" {
		t.Fatalf("o aviso abre a janela, seguido de uma linha em branco: %q", linhas[:2])
	}
	topo := m.viewport.YOffset + m.linhasAviso()
	for i, ini := range m.inicios {
		if acima := ini < topo; acima != (i < n) {
			t.Errorf("a mensagem %d começa na linha %d (topo %d): contada=%v", i, ini, topo, i < n)
		}
	}
	m.viewport.GotoTop()
	if m.mensagensAnteriores() != 0 || strings.Contains(m.corpo(), "anterior") {
		t.Error("no começo da conversa não há aviso")
	}
	// As notas não contam, e a janela baixa demais não tem lugar para os dois avisos.
	m.messages = append(m.messages, chatMessage{role: "nota", content: "resposta interrompida"})
	m.updateViewport()
	m.viewport.GotoBottom()
	if got := m.mensagensAnteriores(); got != n {
		t.Errorf("a nota não conta: %d, antes %d", got, n)
	}
	m.viewport.Height = 4
	if m.mensagensAnteriores() != 0 {
		t.Error("numa janela de 4 linhas os avisos não cabem com a conversa")
	}
}

// Subindo página por página com o aviso no topo, toda linha da conversa passa pela tela.
// Lida do começo, uma conversa longa avisa quantas linhas faltam embaixo, mesmo sem
// mensagem nova (o resto da resposta não ficava escondido sem aviso), e o pgdn passa por
// todas até o fim, onde o aviso some.
func TestLinhasAbaixo_PaginarParaBaixoMostraTudo(t *testing.T) {
	semCores(t)
	m := conversaLonga(t, nil)
	m.viewport.GotoTop()
	m.marcarLidas()
	if m.mensagensNovas() != 0 {
		t.Fatalf("nada é novo: %d", m.mensagensNovas())
	}
	esperado := m.viewport.TotalLineCount() - m.viewport.Height + m.linhasAviso()
	if corpo := ansi.Strip(m.corpo()); !strings.Contains(corpo, fmt.Sprintf("↓ %d linhas abaixo", esperado)) {
		t.Fatalf("faltou o aviso das %d linhas abaixo:\n%s", esperado, corpo)
	}
	vistas := map[string]bool{}
	for passo := 0; ; passo++ {
		for _, l := range strings.Split(ansi.Strip(m.corpo()), "\n") {
			vistas[strings.TrimSpace(l)] = true
		}
		if m.viewport.AtBottom() {
			break
		}
		if passo > 50 {
			t.Fatal("o pgdn não chegou ao fim")
		}
		atualizado, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = atualizado.(Model)
	}
	if strings.Contains(ansi.Strip(m.corpo()), "abaixo") {
		t.Errorf("no fim não há aviso de linhas abaixo:\n%s", ansi.Strip(m.corpo()))
	}
	for i, l := range strings.Split(ansi.Strip(strings.TrimRight(m.historico, "\n")), "\n") {
		if l = strings.TrimSpace(l); l != "" && !vistas[l] {
			t.Errorf("a linha %d nunca apareceu na tela: %q", i, l)
		}
	}
}

func TestMensagensAnteriores_PaginarMostraTudo(t *testing.T) {
	semCores(t)
	m := conversaLonga(t, nil)
	vistas := map[string]bool{}
	for passo := 0; ; passo++ {
		for _, l := range strings.Split(ansi.Strip(m.corpo()), "\n") {
			vistas[strings.TrimSpace(l)] = true
		}
		if m.viewport.YOffset == 0 {
			break
		}
		if passo > 50 {
			t.Fatal("o pgup não chegou ao começo")
		}
		m = rolarAcima(t, m)
	}
	for i, l := range strings.Split(ansi.Strip(strings.TrimRight(m.historico, "\n")), "\n") {
		if l = strings.TrimSpace(l); l != "" && !vistas[l] {
			t.Errorf("a linha %d nunca apareceu na tela: %q", i, l)
		}
	}
}

// O ctrl+s solta o mouse para o terminal (o arraste volta a selecionar texto), diz isso
// na linha de estado e na legenda, e o segundo ctrl+s o traz de volta para a roda. Vale
// também com a saída da ferramenta aberta, sem fechá-la.
// temMsg diz se o comando (ou um dos comandos do lote) devolve uma mensagem do tipo de
// quer.
func temMsg(cmd tea.Cmd, quer tea.Msg) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	if lote, ok := msg.(tea.BatchMsg); ok {
		for _, c := range lote {
			if temMsg(c, quer) {
				return true
			}
		}
		return false
	}
	return fmt.Sprintf("%T", msg) == fmt.Sprintf("%T", quer)
}

func TestCtrlS_SoltaEDevolveOMouse(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "10:00")
	ctrlS := tea.KeyMsg{Type: tea.KeyCtrlS}

	atualizado, cmd := m.Update(ctrlS)
	m = atualizado.(Model)
	if !temMsg(cmd, tea.DisableMouse()) {
		t.Fatal("o ctrl+s deveria soltar o mouse")
	}
	if m.textarea.Cursor.Mode() != cursor.CursorStatic {
		t.Error("com o mouse solto, o cursor do campo não pisca: nada muda na tela durante a seleção")
	}
	if !strings.Contains(ansi.Strip(m.mensagemEstado()), "arraste com o mouse para selecionar") {
		t.Errorf("a linha de estado deveria explicar a seleção: %q", ansi.Strip(m.mensagemEstado()))
	}
	legenda := ui.KeyHintsCom(200, ui.DefaultKeys().Global.SairForcado, m.teclas()...)
	if !strings.Contains(legenda, "ctrl+s rolar com o mouse") {
		t.Errorf("a legenda deveria mostrar como voltar: %q", legenda)
	}

	m.saida = &vistaSaida{}
	atualizado, cmd = m.Update(ctrlS)
	m = atualizado.(Model)
	if !temMsg(cmd, tea.EnableMouseCellMotion()) {
		t.Fatal("o segundo ctrl+s deveria devolver o mouse para a roda")
	}
	if m.textarea.Cursor.Mode() != cursor.CursorBlink {
		t.Error("de volta à roda, o cursor do campo volta a piscar")
	}
	if m.saida == nil {
		t.Error("o ctrl+s não fecha a saída aberta")
	}
	if strings.Contains(ansi.Strip(m.mensagemEstado()), "arraste") {
		t.Errorf("de volta à roda, a linha de estado não fala de seleção: %q", ansi.Strip(m.mensagemEstado()))
	}
}

// Com o mouse solto, a roda chega como ↑ e ↓ no Windows Terminal e no GNOME Terminal: elas
// rolam a conversa e não trazem a última pergunta para o campo. O ctrl+s não apaga o aviso
// da linha de estado.
func TestCtrlS_SetasRolamComOMouseSolto(t *testing.T) {
	semCores(t)
	m := conversaLonga(t, nil)
	m.aviso = &avisoEstado{texto: "comando copiado", ok: true}
	atualizado, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = atualizado.(Model)
	if m.aviso == nil {
		t.Error("o ctrl+s não deveria apagar o aviso")
	}
	fim := m.viewport.YOffset
	for i := 0; i < 3; i++ {
		atualizado, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = atualizado.(Model)
	}
	if m.viewport.YOffset != fim-3 {
		t.Errorf("três ↑ deveriam subir três linhas: %d, esperado %d", m.viewport.YOffset, fim-3)
	}
	if m.textarea.Value() != "" {
		t.Errorf("o ↑ não deveria trazer a última pergunta: %q", m.textarea.Value())
	}
	atualizado, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = atualizado.(Model)
	if m.viewport.YOffset != fim-2 {
		t.Errorf("o ↓ deveria descer uma linha: %d, esperado %d", m.viewport.YOffset, fim-2)
	}
	legenda := ui.KeyHintsCom(200, ui.DefaultKeys().Global.SairForcado, m.teclas()...)
	if strings.Contains(legenda, "editar última") {
		t.Errorf("com o mouse solto o ↑ rola: %q", legenda)
	}
}

// Com o mouse solto, os blocos de código saem sem as bordas dos lados: a linha selecionada
// com o mouse é só o código. O número de linhas não muda, e o segundo ctrl+s traz as bordas.
func TestCtrlS_CodigoSemBordas(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{SessionID: "20261030-100000"}, "10:00")
	resposta := "Confira:\n\n```bash\nkubectl get pods -n web\n```\n\n```yaml\nsecurityContext:\n  runAsNonRoot: true\n```\n"
	m = conversar(t, m, "como confiro?", resposta, time.Second)
	linhaCom := func(m Model, texto string) string {
		for _, l := range strings.Split(ansi.Strip(m.viewport.View()), "\n") {
			if strings.Contains(l, texto) {
				return strings.TrimRight(l, " ")
			}
		}
		t.Fatalf("a conversa não mostra %q:\n%s", texto, ansi.Strip(m.viewport.View()))
		return ""
	}
	if l := linhaCom(m, "kubectl get pods"); !strings.HasSuffix(l, "│") {
		t.Fatalf("com o mouse preso, o comando fica no quadro com bordas: %q", l)
	}
	antes := m.viewport.TotalLineCount()

	atualizado, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = atualizado.(Model)
	for _, texto := range []string{"kubectl get pods -n web", "runAsNonRoot: true"} {
		if l := linhaCom(m, texto); strings.Contains(l, "│") || !strings.HasSuffix(l, texto) {
			t.Errorf("com o mouse solto, a linha do código sai sem bordas: %q", l)
		}
	}
	if l := linhaCom(m, "COMANDO"); !strings.Contains(l, "ctrl+y copia") {
		t.Errorf("o quadro aberto mantém a tecla da cópia: %q", l)
	}
	if depois := m.viewport.TotalLineCount(); depois != antes {
		t.Errorf("o ctrl+s não muda o número de linhas: %d, antes %d", depois, antes)
	}

	atualizado, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = atualizado.(Model)
	if l := linhaCom(m, "kubectl get pods"); !strings.HasSuffix(l, "│") {
		t.Errorf("o segundo ctrl+s traz as bordas de volta: %q", l)
	}
}
