package tui

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/casheiro/yby-cli/v5/plugins/bard/tools"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// relogioFalso é um relógio que só anda quando o teste manda.
type relogioFalso struct{ t time.Time }

func (r *relogioFalso) agora() time.Time      { return r.t }
func (r *relogioFalso) andar(d time.Duration) { r.t = r.t.Add(d) }
func (r *relogioFalso) em(h, mi int) *relogioFalso {
	r.t = time.Date(2026, 10, 24, h, mi, 0, 0, time.Local)
	return r
}

// atualizar passa a mensagem pelo Update e devolve o modelo.
func atualizar(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	atualizado, _ := m.Update(msg)
	return atualizado.(Model)
}

// etapa entrega uma etapa da rodada em andamento: começa, o relógio anda e ela termina.
func etapa(t *testing.T, m Model, r *relogioFalso, e EtapaMsg, d time.Duration) Model {
	t.Helper()
	e.Rodada = m.rodada
	inicio := e
	inicio.Estado, inicio.Duracao, inicio.Saida = ui.EtapaAtual, 0, ""
	m = atualizar(t, m, inicio)
	r.andar(d)
	if e.Estado == ui.EtapaAtual {
		return m
	}
	e.Duracao = d
	return atualizar(t, m, e)
}

const saidaInvestigate = "Pod: worker-5d8b6-q7tvn\nStatus: CrashLoopBackOff\nReinícios: 14\n" +
	"panic: dial tcp 10.43.12.8:5432: connect: connection refused\n"

const respostaWorker = "O pod **worker-5d8b6-q7tvn** está em CrashLoopBackOff: sai com código 1 logo depois " +
	"de iniciar, porque não consegue falar com o Postgres. O último log mostra:\n\n" +
	"```log\npanic: dial tcp 10.43.12.8:5432: connect: connection refused\n```\n\n" +
	"O Service **postgres** em payments encaminha para a porta 5433, mas o container escuta " +
	"na 5432. Isso começou no deploy de ontem às 18:40"

// trabalhando monta a tela da prancha "trabalhando": uma troca anterior, a pergunta e
// a rodada no meio da resposta, 9 s depois do ↵, com o relógio falso.
func trabalhando(t *testing.T) (Model, *relogioFalso) {
	t.Helper()
	r := (&relogioFalso{}).em(12, 5)
	m := telaDeTeste(t, Config{
		Cluster: "staging", Namespace: "payments", AIModel: "claude-cli", SessionID: "20261024-120000",
	})
	m.agora = r.agora
	m = conversar(t, m, "quais pods estão em payments?",
		"Há 4 pods em payments: **api**, **web** e **redis** rodando e o **worker** reiniciando.", 3*time.Second)
	r.em(12, 7)
	m = perguntar(t, m, "por que o worker fica reiniciando?")
	m = etapa(t, m, r, EtapaMsg{Etapa: EtapaContexto, Rotulo: rotuloContexto, Estado: ui.EtapaFeita, Nota: "3 notas do Synapstor"}, 300*time.Millisecond)
	m = etapa(t, m, r, EtapaMsg{Etapa: EtapaIntencao, Rotulo: rotuloIntencao, Estado: ui.EtapaFeita, Nota: "investigar pod com falha"}, 1800*time.Millisecond)
	m = etapa(t, m, r, EtapaMsg{Etapa: EtapaFerramenta, Rotulo: "sentinel investigate", Estado: ui.EtapaFeita,
		Nota: "worker-5d8b6-q7tvn", Saida: saidaInvestigate}, 4200*time.Millisecond)
	m = etapa(t, m, r, EtapaMsg{Etapa: EtapaResposta, Rotulo: rotuloResposta, Estado: ui.EtapaAtual, Tentativa: 1}, 2700*time.Millisecond)
	for _, trecho := range []string{respostaWorker[:120], respostaWorker[120:]} {
		m = atualizar(t, m, TrechoMsg{Rodada: m.rodada, Texto: trecho})
	}
	return m, r
}

// TestTelaTrabalhando reproduz a prancha "trabalhando": o cabeçalho com o tempo
// corrido, as etapas com resultado e tempo (a resposta contando ao vivo), a resposta
// chegando com o cursor no fim, o campo que aceita a próxima pergunta, a linha de
// estado "◈ gerando resposta · 9 s" e a legenda da resposta.
func TestTelaTrabalhando(t *testing.T) {
	for _, md := range []modo{
		{"escuro", ui.TemaEscuro, false, termenv.TrueColor},
		{"ascii", ui.TemaEscuro, true, termenv.Ascii},
	} {
		t.Run(md.nome, func(t *testing.T) {
			usarModo(t, md)
			m, _ := trabalhando(t)

			tela := conferirTela(t, m)
			limpa := ansi.Strip(tela)
			g, enter := ui.G(), ui.RotuloTecla("↵")
			for _, trecho := range []string{
				g.Andamento + " BARD · trabalhando · 9 s",
				g.Feito + " contexto do projeto       3 notas do Synapstor",
				g.Feito + " entender o pedido         investigar pod com falha",
				g.Feito + " sentinel investigate      worker-5d8b6-q7tvn · " + enter + " ver",
				"0,3 s", "1,8 s", "4,2 s", "2,7 s",
				"às 18:40",
				g.Prompt + " " + textoPlaceholderResp,
				g.Andamento + " gerando resposta · 9 s", "1 ferramenta",
				"esc interromper", "pgup rolar", enter + " ver etapa", "ctrl+c sair",
			} {
				if !strings.Contains(limpa, trecho) {
					t.Errorf("a tela deveria ter %q:\n%s", trecho, limpa)
				}
			}
			if !md.ascii {
				cursor := false
				for _, l := range strings.Split(tela, "\n") {
					if strings.Contains(l, "18:40") && strings.HasSuffix(strings.TrimRight(l, " "), ui.S().Selecionado.Render(" ")) {
						cursor = true
					}
				}
				if !cursor {
					t.Error("o cursor deveria ficar no fim da resposta em andamento")
				}
			}
		})
	}
}

// TestTrabalho_RelogioAnda confere que o tique avança os tempos ao vivo e que um tique
// de outra rodada é ignorado.
func TestTrabalho_RelogioAnda(t *testing.T) {
	semCores(t)
	m, r := trabalhando(t)
	m.eventos = make(chan tea.Msg) // rodada aberta, sem eventos
	r.andar(3 * time.Second)
	m = atualizar(t, m, tiqueMsg{rodada: m.rodada - 1})
	if got := m.mensagemEstado(); got != "◈ gerando resposta · 9 s" {
		t.Errorf("tique de outra rodada não mexe no relógio: %q", got)
	}
	atualizado, cmd := m.Update(tiqueMsg{rodada: m.rodada})
	m = atualizado.(Model)
	if cmd == nil {
		t.Error("depois do tique a leitura dos eventos continua")
	}
	if got := m.mensagemEstado(); got != "◈ gerando resposta · 12 s" {
		t.Errorf("o tique deveria avançar o tempo corrido: %q", got)
	}
	if vista := m.viewport.View(); !strings.Contains(vista, "BARD · trabalhando · 12 s") || !strings.Contains(vista, "5,7 s") {
		t.Errorf("o cabeçalho e a etapa em andamento contam ao vivo:\n%s", vista)
	}
	m.eventos = nil
	if _, cmd := m.Update(tiqueMsg{rodada: m.rodada}); cmd != nil {
		t.Error("sem rodada aberta o tique não pede mais eventos")
	}
}

func TestEsperarEvento(t *testing.T) {
	anterior := intervaloRelogio
	intervaloRelogio = 10 * time.Millisecond
	t.Cleanup(func() { intervaloRelogio = anterior })

	ch := make(chan tea.Msg, 1)
	if msg := esperarEvento(ch, 3)(); msg != (tiqueMsg{rodada: 3}) {
		t.Errorf("sem evento no intervalo, um tique: %#v", msg)
	}
	ch <- TrechoMsg{Rodada: 3, Texto: "a"}
	if msg := esperarEvento(ch, 3)(); msg != (TrechoMsg{Rodada: 3, Texto: "a"}) {
		t.Errorf("o evento chega antes do tique: %#v", msg)
	}
	close(ch)
	if msg := esperarEvento(ch, 3)(); msg != nil {
		t.Errorf("canal fechado não devolve nada: %#v", msg)
	}
}

func TestCronometro(t *testing.T) {
	for d, want := range map[time.Duration]string{
		-time.Second:                          "0 s",
		900 * time.Millisecond:                "0 s",
		9900 * time.Millisecond:               "9 s",
		38*time.Second + 600*time.Millisecond: "38 s",
		72 * time.Second:                      "1 min 12 s",
	} {
		if got := cronometro(d); got != want {
			t.Errorf("cronometro(%v) = %q, esperado %q", d, got, want)
		}
	}
}

func TestEtapasVista_SemNotaNemTempo(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{})
	m.state = stateStreaming
	m.etapas = []EtapaMsg{{Etapa: EtapaContexto, Rotulo: rotuloContexto, Estado: ui.EtapaAtual}}
	m.iniciosEtapas = []time.Time{{}}
	if got := m.etapasVista(60); got != "◈ contexto do projeto" {
		t.Errorf("etapa recém-começada fica sem coluna nem tempo: %q", got)
	}
	// Rótulo maior que a coluna empurra as notas de todas.
	m.etapas = []EtapaMsg{
		{Etapa: EtapaContexto, Rotulo: rotuloContexto, Estado: ui.EtapaFeita, Nota: "ok"},
		{Etapa: EtapaFerramenta, Rotulo: "ferramenta de nome bem comprido", Estado: ui.EtapaFalha, Nota: "falhou"},
	}
	linhas := strings.Split(m.etapasVista(80), "\n")
	if !strings.HasPrefix(linhas[0], "✓ contexto do projeto               ok") || !strings.HasPrefix(linhas[1], "✕ ferramenta de nome bem comprido   falhou") {
		t.Errorf("notas alinhadas pelo rótulo mais largo:\n%s", strings.Join(linhas, "\n"))
	}
}

func TestResumoRodada(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{})
	m.state = stateStreaming
	if got := m.resumoRodada(); got != "nenhuma mensagem" {
		t.Errorf("sem ferramenta, o resumo da sessão: %q", got)
	}
	m.etapas = []EtapaMsg{{Etapa: EtapaFerramenta}, {Etapa: EtapaFerramenta, Rotulo: "outra"}}
	if got := m.resumoRodada(); got != "2 ferramentas" {
		t.Errorf("ferramentas da rodada: %q", got)
	}
}

// copiadora registra o que foi copiado.
type copiadora struct {
	textos []string
	err    error
}

func (c *copiadora) copiar(s string) error {
	c.textos = append(c.textos, s)
	return c.err
}

// executar roda o comando e passa a mensagem dele pelo Update.
func executar(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("esperava um comando")
	}
	return atualizar(t, m, cmd())
}

func TestSaida_EnterAbreDuranteAResposta(t *testing.T) {
	semCores(t)
	m, _ := trabalhando(t)
	c := &copiadora{}
	m.copiar = c.copiar

	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.saida == nil {
		t.Fatal("↵ no campo vazio abre a saída da ferramenta")
	}
	tela := ansi.Strip(m.View())
	for _, trecho := range []string{"✓ sentinel investigate worker-5d8b6-q7tvn · 4 linhas", "SAÍDA · ctrl+y copia",
		"Reinícios: 14", "4 linhas", "esc voltar", "ctrl+y copiar", "› " + textoPlaceholderSaida} {
		if !strings.Contains(tela, trecho) {
			t.Errorf("a saída aberta deveria ter %q:\n%s", trecho, tela)
		}
	}
	if strings.Contains(tela, "pgup rolar") || strings.Contains(tela, textoPlaceholderResp) {
		t.Error("a saída cabe no quadro (pgup não aparece) e o campo não convida a escrever")
	}
	// Os eventos seguem chegando por baixo da camada.
	m = atualizar(t, m, TrechoMsg{Rodada: m.rodada, Texto: "."})
	if m.saida == nil || !strings.HasSuffix(m.parcial, ".") {
		t.Error("a resposta continua chegando com a saída aberta")
	}
	// ctrl+y copia a saída inteira; letras não vão para o campo.
	atualizado, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	m = executar(t, atualizado.(Model), cmd)
	if got := m.mensagemEstado(); got != "✓ saída enviada à área de transferência" {
		t.Errorf("aviso da cópia: %q", got)
	}
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if len(c.textos) != 1 || c.textos[0] != saidaInvestigate || m.textarea.Value() != "" {
		t.Errorf("ctrl+y copia a saída e as letras ficam fora do campo: %q, campo %q", c.textos, m.textarea.Value())
	}
	// esc fecha a camada sem interromper a resposta.
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.saida != nil || m.interrompendo || m.state != stateStreaming {
		t.Errorf("esc fecha só a saída: aberta=%v interrompendo=%v", m.saida != nil, m.interrompendo)
	}
	if m.aviso != nil {
		t.Error("a tecla seguinte apaga o aviso")
	}
}

func TestSaida_EnterComTextoNaoAbre(t *testing.T) {
	semCores(t)
	m, _ := trabalhando(t)
	digitarNoModelo := func(m Model, s string) Model {
		for _, r := range s {
			m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		return m
	}
	m = digitarNoModelo(m, "e o redis?")
	if strings.Contains(ansi.Strip(m.View()), "ver etapa") || strings.Contains(m.viewport.View(), "↵ ver") {
		t.Error("com texto no campo o ↵ não abre a etapa: a dica some")
	}
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.saida != nil {
		t.Error("com texto no campo o ↵ não abre a saída")
	}
}

func TestSaida_DepoisDaResposta(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "11:51")
	saida := strings.TrimSuffix(strings.Repeat("NAMESPACE   NAME   STATUS\n", 40), "\n")
	m = conversar(t, m, "e o storage?", "Tudo certo.", 2*time.Second,
		EtapaMsg{Etapa: EtapaFerramenta, Rotulo: "kubectl get", Nota: "pvc", Estado: ui.EtapaAtual},
		EtapaMsg{Etapa: EtapaFerramenta, Rotulo: "kubectl get", Nota: "pvc", Estado: ui.EtapaFeita, Saida: saida})
	if !strings.Contains(m.viewport.View(), "✓ kubectl get pvc · 40 linhas · ↵ ver saída") {
		t.Fatalf("a última resposta mostra a dica:\n%s", m.viewport.View())
	}
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if strings.Contains(m.viewport.View(), "ver saída") {
		t.Error("com texto no campo a dica some")
	}
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !strings.Contains(m.viewport.View(), "ver saída") {
		t.Error("com o campo vazio de novo a dica volta")
	}

	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.saida == nil {
		t.Fatal("↵ no campo vazio abre a saída da última resposta")
	}
	h := m.alturaSaida()
	if got := m.posicaoSaida(); got != fmt.Sprintf("linhas 1 a %d de 40", h) {
		t.Errorf("posição: %q", got)
	}
	if !strings.Contains(ansi.Strip(m.View()), "pgup rolar") {
		t.Error("a saída passa da altura: pgup aparece")
	}
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.saida.topo != 40-h {
		t.Errorf("pgdn para no fim: topo %d", m.saida.topo)
	}
	m = atualizar(t, m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if m.saida.topo != 40-h-linhasRoda {
		t.Errorf("a roda sobe %d linhas: topo %d", linhasRoda, m.saida.topo)
	}
	m = atualizar(t, m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyPgUp})
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyPgUp})
	if m.saida.topo != 0 {
		t.Errorf("pgup para no começo: topo %d", m.saida.topo)
	}
	// Uma janela mais estreita quebra a saída de novo.
	m = atualizar(t, m, tea.WindowSizeMsg{Width: 70, Height: alturaTela})
	if m.saida.largura != 66 {
		t.Errorf("a saída acompanha a largura: %d", m.saida.largura)
	}
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.saida != nil {
		t.Error("esc fecha a saída")
	}
	if !strings.Contains(m.viewport.View(), "ver saída") {
		t.Error("fechada a saída, a dica volta")
	}

	// Uma pergunta nova tira a dica da resposta anterior.
	m = perguntar(t, m, "e agora?")
	if strings.Contains(m.viewport.View(), "ver saída") {
		t.Error("durante a rodada nova a resposta anterior perde a dica")
	}
}

func TestSaida_SemFerramentaEnterNaoFazNada(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "11:51")
	m = conversar(t, m, "oi", "olá", time.Second,
		EtapaMsg{Etapa: EtapaFerramenta, Rotulo: "kubectl get", Estado: ui.EtapaFalha, Nota: "sem cluster", Saida: "Erro: sem cluster"})
	atualizado, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if atualizado.(Model).saida != nil || cmd != nil || strings.Contains(m.viewport.View(), "ver saída") {
		t.Error("ferramenta que falhou não tem saída para abrir")
	}
}

func TestQuebrarSaida(t *testing.T) {
	got := quebrarSaida("\x1b[1mNAME\x1b[0m    STATUS\r\n\tpod-a  Running   \nabcdefghij\n", 6)
	want := []string{"NAME  ", "  STAT", "US", "    po", "d-a  R", "unning", "abcdef", "ghij"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("quebrarSaida = %q", got)
	}
}

// Depois de uma resposta com comando, uma pergunta que começa com c chega inteira ao
// campo: as letras são do campo, e a cópia é do ctrl+y.
func TestCopiar_LetrasDepoisDeComandoVaoProCampo(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "11:51")
	c := &copiadora{}
	m.copiar = c.copiar
	m = conversar(t, m, "e os volumes?", respostaStorage, time.Second)
	for _, r := range "como apago?" {
		atualizado, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = atualizado.(Model)
		if cmd != nil {
			if _, copia := cmd().(copiadoMsg); copia {
				t.Fatalf("a letra %q não deveria copiar nada", r)
			}
		}
	}
	if got := m.textarea.Value(); got != "como apago?" || len(c.textos) != 0 {
		t.Errorf("o campo fica com a pergunta inteira: %q, cópias %q", got, c.textos)
	}
}

func TestCopiar_CtrlYCopiaOUltimoComando(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "11:51")
	c := &copiadora{}
	m.copiar = c.copiar
	tecla := tea.KeyMsg{Type: tea.KeyCtrlY}

	m = conversar(t, m, "e os volumes?", respostaStorage, time.Second)
	atualizado, cmd := m.Update(tecla)
	m = executar(t, atualizado.(Model), cmd)
	if len(c.textos) != 1 || c.textos[0] != "kubectl delete pv pvc-3f1a9c2e pvc-91cd04b7" || m.textarea.Value() != "" {
		t.Fatalf("ctrl+y copia o comando da última resposta: %q, campo %q", c.textos, m.textarea.Value())
	}
	if got := m.mensagemEstado(); got != "✓ comando enviado à área de transferência" {
		t.Errorf("aviso: %q", got)
	}
	// A tecla seguinte apaga o aviso; com texto no campo o ctrl+y copia de novo sem
	// mexer no campo.
	m = atualizar(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !strings.HasPrefix(m.mensagemEstado(), "✓ resposta em") {
		t.Errorf("a tecla seguinte apaga o aviso: %q", m.mensagemEstado())
	}
	atualizado, cmd = m.Update(tecla)
	m = executar(t, atualizado.(Model), cmd)
	if len(c.textos) != 2 || m.textarea.Value() != "e" {
		t.Errorf("ctrl+y copia quantas vezes for pedido: %d cópias, campo %q", len(c.textos), m.textarea.Value())
	}

	// A falha da cópia vira aviso, e o ctrl+y seguinte tenta de novo.
	m.textarea.Reset()
	m = conversar(t, m, "e o pv?", "```sh\nkubectl get pv\n```", time.Second)
	c.err = errors.New("sem terminal")
	atualizado, cmd = m.Update(tecla)
	m = executar(t, atualizado.(Model), cmd)
	if got := m.mensagemEstado(); got != "✕ não deu para copiar o comando" || c.textos[2] != "kubectl get pv" {
		t.Errorf("falha da cópia: %q, %q", got, c.textos)
	}
	c.err = nil
	atualizado, cmd = m.Update(tecla)
	m = executar(t, atualizado.(Model), cmd)
	if len(c.textos) != 4 || !strings.HasPrefix(m.mensagemEstado(), "✓ comando enviado") {
		t.Errorf("depois da falha o ctrl+y tenta de novo: %q, %q", m.mensagemEstado(), c.textos)
	}

	// Durante a resposta não há comando para copiar.
	m = perguntar(t, m, "e agora?")
	if _, cmd := m.Update(tecla); cmd != nil {
		t.Error("durante a resposta o ctrl+y não copia")
	}
}

// Só o quadro que o ctrl+y copia mostra a tecla: o último comando da última resposta.
// Os outros blocos, as respostas antigas e a resposta em andamento ficam só "COMANDO".
func TestCopiar_SoOQuadroCopiadoLevaATecla(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "11:51")
	c := &copiadora{}
	m.copiar = c.copiar
	rotulos := func() (comTecla, sem int) {
		for _, l := range strings.Split(ansi.Strip(m.viewport.View()), "\n") {
			switch {
			case strings.Contains(l, "COMANDO · ctrl+y copia"):
				comTecla++
			case strings.Contains(l, "COMANDO"):
				sem++
			}
		}
		return comTecla, sem
	}

	m = conversar(t, m, "e os pv?", "Veja:\n\n```sh\nkubectl get pv\n```\n\nE apague:\n\n```sh\nkubectl delete pv x\n```", time.Second)
	if com, sem := rotulos(); com != 1 || sem != 1 {
		t.Fatalf("dois blocos: um com a tecla e um sem (%d com, %d sem):\n%s", com, sem, m.viewport.View())
	}
	vista := ansi.Strip(m.viewport.View())
	if i, j := strings.Index(vista, "COMANDO · ctrl+y"), strings.Index(vista, "kubectl delete pv x"); i < 0 || j < i {
		t.Errorf("a tecla fica no quadro do último comando:\n%s", vista)
	}
	atualizado, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	m = executar(t, atualizado.(Model), cmd)
	if len(c.textos) != 1 || c.textos[0] != "kubectl delete pv x" {
		t.Errorf("ctrl+y copia o bloco que mostra a tecla: %q", c.textos)
	}

	// A resposta em andamento e a anterior ficam sem a tecla.
	m = perguntar(t, m, "e o pvc?")
	m = atualizar(t, m, TrechoMsg{Rodada: m.rodada, Texto: "```sh\nkubectl get pvc\n```"})
	if com, sem := rotulos(); com != 0 || sem != 3 {
		t.Errorf("durante a resposta nenhum quadro mostra a tecla (%d com, %d sem)", com, sem)
	}
	// Uma resposta sem comando deixa a conversa sem nada para copiar.
	atualizado, _ = m.Update(FimMsg{Rodada: m.rodada, Resposta: "não há pvc", Duracao: time.Second})
	m = atualizado.(Model)
	if com, _ := rotulos(); com != 0 {
		t.Error("a resposta antiga não volta a mostrar a tecla")
	}
}

// O aviso da cópia some sozinho no prazo do Toast, sem apagar um aviso mais novo.
func TestAviso_SomeNoPrazo(t *testing.T) {
	semCores(t)
	anterior := duracaoAviso
	duracaoAviso = time.Millisecond
	t.Cleanup(func() { duracaoAviso = anterior })
	m := telaDeTeste(t, Config{}, "11:51")

	atualizado, prazo1 := m.Update(copiadoMsg{oque: "comando"})
	m = atualizado.(Model)
	atualizado, prazo2 := m.Update(copiadoMsg{oque: "saída", err: errors.New("x")})
	m = atualizado.(Model)
	m = executar(t, m, prazo1)
	if got := m.mensagemEstado(); got != "✕ não deu para copiar a saída" {
		t.Errorf("o prazo do primeiro aviso não apaga o segundo: %q", got)
	}
	m = executar(t, m, prazo2)
	if m.aviso != nil || m.mensagemEstado() != "" {
		t.Errorf("no fim do prazo o aviso some: %q", m.mensagemEstado())
	}
}

// O spinner da linha de estado gira durante a resposta e para quando ela termina.
func TestTrabalho_SpinnerGira(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "11:51")
	m.novoSpinner = func(r string) ui.Spinner { return ui.NovoSpinnerCom(r, func() bool { return true }) }
	m = perguntar(t, m, "oi")
	quadros := map[string]bool{strings.Fields(m.mensagemEstado())[0]: true}
	var tique tea.Msg
	for i := 0; i < 2; i++ {
		tique = m.spinner.Init()() // o tique que o spinner da rodada agenda
		atualizado, cmd := m.Update(tique)
		m = atualizado.(Model)
		if cmd == nil {
			t.Fatal("durante a resposta o spinner agenda o próximo quadro")
		}
		quadros[strings.Fields(m.mensagemEstado())[0]] = true
	}
	if len(quadros) < 2 {
		t.Errorf("o spinner deveria trocar de quadro: %v", quadros)
	}
	m.state = stateIdle
	if _, cmd := m.Update(tique); cmd != nil {
		t.Error("fora da resposta o tique do spinner morre")
	}
}

// Na linha que ocupa a largura toda, o cursor da resposta em andamento desce para uma
// linha própria em vez de ficar fora da tela.
func TestTrabalho_CursorNaLinhaCheia(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "11:51")
	m.markdown = nil
	m = perguntar(t, m, "oi")
	cheia := strings.Repeat("a", m.viewport.Width-len(recuo))
	m = atualizar(t, m, TrechoMsg{Rodada: m.rodada, Texto: cheia})
	linhas := strings.Split(m.viewport.View(), "\n")
	cursor := ui.S().Selecionado.Render(" ")
	for i, l := range linhas {
		if strings.Contains(l, cheia) {
			if i+1 >= len(linhas) || strings.TrimSpace(linhas[i+1]) != strings.TrimSpace(recuo+cursor) {
				t.Errorf("o cursor desce para a linha seguinte: %q", linhas[i:])
			}
			return
		}
	}
	t.Fatalf("a resposta deveria aparecer:\n%s", m.viewport.View())
}

func TestCopiar_SemComandoNaoCopia(t *testing.T) {
	semCores(t)
	m := telaDeTeste(t, Config{}, "11:51")
	c := &copiadora{}
	m.copiar = c.copiar
	m = conversar(t, m, "oi", "olá, sem comando", time.Second)
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlY}); cmd != nil || len(c.textos) != 0 {
		t.Error("sem comando na resposta o ctrl+y não faz nada")
	}
	// Com erro no fim da conversa também não há o que copiar.
	m.messages = append(m.messages, chatMessage{role: "error", content: "```bash\nls\n```"})
	if cmd, _ := m.ultimoComando(); cmd != "" {
		t.Error("erro não tem comando para copiar")
	}
	m.messages = append(m.messages, chatMessage{role: "user", content: "e então?"})
	if m.ultimaResposta() != -1 {
		t.Error("a conversa termina numa pergunta: não há última resposta")
	}
}

func TestEscreverOSC52(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("kubectl get pv"))
	env := func(vars map[string]string) func(string) string {
		return func(k string) string { return vars[k] }
	}
	casos := []struct {
		nome   string
		vars   map[string]string
		prefix string
	}{
		{"terminal", nil, "\x1b]52;c;" + b64},
		{"tmux", map[string]string{"TMUX": "/tmp/tmux-1000/default,1,0"}, "\x1bPtmux;\x1b\x1b]52;c;" + b64},
		{"screen", map[string]string{"TERM": "screen-256color"}, "\x1bP\x1b]52;c;" + b64},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var buf bytes.Buffer
			if err := escreverOSC52(&buf, env(c.vars), "kubectl get pv"); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(buf.String(), c.prefix) {
				t.Errorf("sequência %q, esperado o começo %q", buf.String(), c.prefix)
			}
		})
	}
}

// TestTrabalho_NoPrograma passa pelo programa de verdade: o fluxo roda a ferramenta, a
// resposta para no meio, o ↵ abre a saída por cima, esc volta à resposta e o esc
// seguinte interrompe.
func TestTrabalho_NoPrograma(t *testing.T) {
	semCores(t)
	p := &provedorFalso{
		intencao: `{"intent":"investigate_pod","params":{"pod":"worker-1"},"direct":false}`,
		trechos:  []string{"O worker não alcança o Postgres", " e reinicia."},
		travar:   true,
		liberar:  make(chan struct{}),
	}
	defer close(p.liberar)
	m := conversaLonga(t, p)
	chamadas := 0
	m.fluxo.BuscarFerramenta = func(string) *tools.Tool {
		return ferramentaDeTeste("Status: CrashLoopBackOff\nReinícios: 14", nil, &chamadas)
	}
	tm := abrirBard(t, m)
	digitar(tm, "por que o worker reinicia?")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	esperarTela(t, tm, "O worker não alcança o Postgres")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	// A saída do programa é lida uma vez: o quadro e o texto chegam no mesmo quadro.
	esperarTela(t, tm, "Reinícios: 14")
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	esperarTela(t, tm, "esc interromper")
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	esperarTela(t, tm, "resposta interrompida")
	final := fecharBard(t, tm)
	if final.saida != nil || chamadas != 1 {
		t.Errorf("a saída fechou e a ferramenta rodou uma vez: aberta=%v chamadas=%d", final.saida != nil, chamadas)
	}
	ultima := final.messages[len(final.messages)-2]
	if len(ultima.ferramentas) != 1 || ultima.ferramentas[0].saida == "" {
		t.Errorf("a resposta interrompida guarda a ferramenta com a saída: %#v", ultima.ferramentas)
	}
}
