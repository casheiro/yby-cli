package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// clienteComDetalhe é o client da prancha que também lê eventos, logs e YAML
// (como o K8sClient), com os dados da folha VizPods, e anota cada leitura.
type clienteComDetalhe struct {
	clienteStaging
	eventos  map[string][]monitor.Event // por "Kind/nome"
	logs     map[string]string          // por pod
	anterior map[string]bool            // por pod: o log é o da execução anterior
	yaml     string
	err      error
	leituras []string
}

func (c *clienteComDetalhe) EventosDoObjeto(kind, nome, _ string, limite int) ([]monitor.Event, error) {
	c.leituras = append(c.leituras, "eventos "+kind+"/"+nome)
	e := c.eventos[kind+"/"+nome]
	if limite > 0 && len(e) > limite {
		e = e[:limite]
	}
	return e, c.err
}

func (c *clienteComDetalhe) LogsDoPod(nome, _, conteiner string, _ int64) (string, bool, error) {
	if conteiner != "" {
		nome += "/" + conteiner // o contêiner só entra na anotação quando vem
	}
	c.leituras = append(c.leituras, "logs "+nome)
	return c.logs[nome], c.anterior[nome], c.err
}

func (c *clienteComDetalhe) YAMLDoRecurso(kind, nome, _ string) (string, error) {
	c.leituras = append(c.leituras, "yaml "+kind+"/"+nome)
	return c.yaml, c.err
}

// logDoWorker é o log do pod com problema da prancha; a última linha é a do painel.
const logDoWorker = `2026-10-25T11:12:02Z conectando ao banco em 10.43.12.8:5432
2026-10-25T11:12:03Z tentativa 1 de 3
2026-10-25T11:12:05Z tentativa 2 de 3
2026-10-25T11:12:09Z tentativa 3 de 3
panic: dial tcp 10.43.12.8:5432: connection refused
`

// clienteDetalheDaPrancha responde como a folha VizPods: os eventos e o log
// do worker, dois eventos do Deployment worker.
func clienteDetalheDaPrancha() *clienteComDetalhe {
	evento := func(kind, nome, tipo, razao, msg string, ha time.Duration, vezes int32) monitor.Event {
		return monitor.Event{ID: nome + "." + razao, Name: nome, Namespace: "payments", Kind: kind, Type: tipo, Reason: razao,
			Message: msg, Quantas: vezes, Quando: horaDaPrancha.Add(-ha)}
	}
	return &clienteComDetalhe{
		clienteStaging: clienteStaging{&MockClient{}},
		eventos: map[string][]monitor.Event{
			"Pod/worker-5d8b6-q7tvn": {
				evento("Pod", "worker-5d8b6-q7tvn", "Warning", "BackOff", "reiniciando container", 2*time.Minute, 1),
				evento("Pod", "worker-5d8b6-q7tvn", "Normal", "Pulled", "worker:1.8.2", 52*time.Minute, 1),
			},
			"Deployment/worker": {
				evento("Deployment", "worker", "Normal", "ScalingReplicaSet", "réplicas de worker-5d8b6 para 2", 12*dia, 1),
			},
		},
		logs: map[string]string{"worker-5d8b6-q7tvn": logDoWorker},
		yaml: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: worker-5d8b6-q7tvn\n",
	}
}

// assentarDetalhe entrega ao modelo o fim do atraso do painel e a leitura
// dos extras, como o programa faria quando a seleção para.
func assentarDetalhe(t *testing.T, m Model) Model {
	t.Helper()
	m, cmd := atualizar(m, paradaMsg{id: m.pedidoDetalhe})
	if cmd == nil {
		t.Fatal("a seleção parada deveria ler os extras do painel")
	}
	m, _ = atualizar(m, cmd())
	return m
}

// telaComDetalhe é a tela da prancha com o painel e os extras já lidos;
// abaixo de 120 colunas, depois do ↵.
func telaComDetalhe(t *testing.T, largura, altura int) Model {
	t.Helper()
	m := telaDaPranchaCom(t, clienteDetalheDaPrancha(), largura, altura)
	if largura < yui.LarguraLadoALado {
		m, _ = teclas(t, m, "enter")
	}
	return assentarDetalhe(t, m)
}

// TestTelaPodsComDetalhe reproduz a folha VizPods com o painel: ao lado da
// lista em 140 colunas, por cima dela em 100 (depois do ↵) e no compacto.
func TestTelaPodsComDetalhe(t *testing.T) {
	escuro := modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor}
	painel := []string{"POD · worker-5d8b6-q7tvn", "Error · código de saída 1", "Deployment worker",
		"cpu 120m/500m · mem 210Mi", "ÚLTIMOS EVENTOS", "BackOff reiniciando container", "ÚLTIMA LINHA DE LOG", "panic:"}
	casos := []struct {
		nome            string
		md              modo
		largura, altura int
		quer            []string
	}{
		{"escuro_140x32", escuro, 140, 32, append([]string{"l logs  y yaml  e eventos", "↵ abrir"}, painel...)},
		// Por cima, sem espaço, saem os campos secundários (dono e recursos) e
		// ficam os últimos eventos e a última linha de log.
		{"escuro_100x30_enter", escuro, 100, 30, []string{"l logs", "esc voltar", "ctrl+c sair", painel[0], painel[1],
			"ÚLTIMOS EVENTOS", "BackOff reiniciando container", "Pulled worker:1.8.2", "ÚLTIMA LINHA DE LOG", "panic: dial tcp"}},
		{"escuro_72x24_enter", escuro, 72, 24, []string{"POD · worker-5d8b6-q7tvn", "CrashLoopBackOff", "ÚLTIMOS EVENTOS",
			"BackOff reiniciando container", "ÚLTIMA LINHA DE LOG", "panic: dial tcp"}},
		{"ascii_140x32", modo{"ascii", yui.TemaEscuro, true, termenv.TrueColor}, 140, 32, []string{"+ POD · worker-5d8b6-q7tvn", "! BackOff"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarModo(t, c.md)
			tela := ansi.Strip(conferirTela(t, telaComDetalhe(t, c.largura, c.altura), c.largura, c.altura))
			for _, quer := range c.quer {
				if !strings.Contains(tela, quer) {
					t.Errorf("a tela deveria mostrar %q", quer)
				}
			}
		})
	}
}

// TestTelaQuadroDeLogs: l abre o log inteiro num quadro que ocupa o corpo.
func TestTelaQuadroDeLogs(t *testing.T) {
	usarModo(t, modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor})
	m, pedido := teclas(t, telaComDetalhe(t, 140, 32), "l")
	m, _ = atualizar(m, pedido())
	tela := ansi.Strip(conferirTela(t, m, 140, 32))
	for _, quer := range []string{"LOGS · worker-5d8b6-q7tvn", "últimas 100 linhas do log · em payments", "panic: dial tcp", "esc voltar", "y yaml"} {
		if !strings.Contains(tela, quer) {
			t.Errorf("o quadro de logs deveria mostrar %q", quer)
		}
	}
}

// TestTelaQuadroDeLogs_anterior: com o log atual vazio, o quadro diz que o
// texto é da execução anterior.
func TestTelaQuadroDeLogs_anterior(t *testing.T) {
	usarModo(t, modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor})
	c := clienteDetalheDaPrancha()
	c.anterior = map[string]bool{"worker-5d8b6-q7tvn": true}
	m := telaDaPranchaCom(t, c, 140, 32)
	m, pedido := teclas(t, m, "l")
	m, _ = atualizar(m, pedido())
	tela := ansi.Strip(conferirTela(t, m, 140, 32))
	if !strings.Contains(tela, "execução anterior: últimas 100 linhas do log") {
		t.Error("o quadro deveria dizer que o log é da execução anterior")
	}
}

// TestPainel_AcompanhaASelecaoECarrega: ao lado, o painel mostra a linha
// selecionada na hora, com os extras carregando até a seleção parar.
func TestPainel_AcompanhaASelecaoECarrega(t *testing.T) {
	semCor(t)
	m := telaDaPranchaCom(t, clienteDetalheDaPrancha(), 140, 32)
	tela := ansi.Strip(m.View())
	if !strings.Contains(tela, "POD · worker-5d8b6-q7tvn") || !strings.Contains(tela, "lendo eventos") || !strings.Contains(tela, "lendo o log") {
		t.Fatalf("antes de a seleção parar, o painel carrega os extras:\n%s", tela)
	}
	m, _ = teclas(t, m, "j")
	if tela = ansi.Strip(m.View()); !strings.Contains(tela, "POD · web-8b77c-5jw4f") || !strings.Contains(tela, "Pending") {
		t.Fatalf("o painel deveria acompanhar a seleção:\n%s", tela)
	}
	// O Deployment não tem log: só os eventos.
	m, _ = teclas(t, m, "2")
	m, _ = atualizar(m, []monitor.Deployment{{Name: "worker", Namespace: "payments", Replicas: 2, Estrategia: "RollingUpdate", Seletor: "app=worker"}})
	m = assentarDetalhe(t, m)
	tela = ansi.Strip(m.View())
	for _, quer := range []string{"DEPLOYMENT · worker", "estratégia", "app=worker", "ScalingReplicaSet", "y yaml  e eventos"} {
		if !strings.Contains(tela, quer) {
			t.Errorf("painel do Deployment deveria mostrar %q:\n%s", quer, tela)
		}
	}
	if strings.Contains(tela, "ÚLTIMA LINHA DE LOG") || strings.Contains(tela, "l logs") {
		t.Errorf("Deployment não tem log:\n%s", tela)
	}
}

// agendador troca o tea.Tick por um que responde na hora e guarda o atraso
// pedido.
type agendador struct{ atrasos []time.Duration }

func (a *agendador) tique(d time.Duration, f func(time.Time) tea.Msg) tea.Cmd {
	a.atrasos = append(a.atrasos, d)
	return func() tea.Msg { return f(horaDaPrancha) }
}

// TestPainel_LeSoQuandoASelecaoPara: cada troca de linha agenda um atraso
// novo; o atraso de uma linha que já ficou para trás não lê nada, e o da
// linha em que a seleção parou lê os extras dela uma vez.
func TestPainel_LeSoQuandoASelecaoPara(t *testing.T) {
	c := clienteDetalheDaPrancha()
	m := telaDaPranchaCom(t, c, 140, 32)
	a := &agendador{}
	m.tique = a.tique
	var atrasos []tea.Cmd
	for range 3 {
		var cmd tea.Cmd
		m, cmd = teclas(t, m, "j")
		if cmd == nil {
			t.Fatal("trocar de linha deveria agendar o atraso do painel")
		}
		atrasos = append(atrasos, cmd)
	}
	if len(a.atrasos) != 3 || a.atrasos[0] != atrasoDetalhe || a.atrasos[2] != atrasoDetalhe {
		t.Fatalf("cada troca de linha agenda um atraso de %v: %v", atrasoDetalhe, a.atrasos)
	}
	for i, atraso := range atrasos[:2] {
		if _, cmd := atualizar(m, atraso()); cmd != nil {
			t.Errorf("o atraso %d, de uma linha que ficou para trás, leu os extras", i)
		}
	}
	m, ler := atualizar(m, atrasos[2]())
	if ler == nil {
		t.Fatal("o atraso da linha em que a seleção parou deveria ler os extras")
	}
	m, _ = atualizar(m, ler())
	if got := strings.Join(c.leituras, ", "); got != "eventos Pod/api-7f9c4d-m81zq, logs api-7f9c4d-m81zq" {
		t.Fatalf("leituras: %s", got)
	}

	// Com os extras no cache, voltar ao recurso não agenda nada; passada a
	// validade, o relógio lê de novo.
	m, _ = teclas(t, m, "k")
	m, cmd := teclas(t, m, "j")
	if cmd != nil || !m.extrasValidos(m.alvoDetalhe) {
		t.Fatal("o recurso com extras no cache não deveria agendar leitura")
	}
	m.agora = func() time.Time { return horaDaPrancha.Add(validadeExtras) }
	_, cmd = atualizar(m, tickMsg(horaDaPrancha))
	lote, ok := cmd().(tea.BatchMsg)
	if !ok || len(lote) != 2 {
		t.Fatalf("o relógio com os extras vencidos deveria ler a lista e os extras: %T", cmd())
	}
	c.leituras = nil
	if _, ok := lote[1]().(extrasMsg); !ok || strings.Join(c.leituras, ", ") != "eventos Pod/api-7f9c4d-m81zq, logs api-7f9c4d-m81zq" {
		t.Fatalf("a renovação deveria ler os extras do recurso na tela: %v", c.leituras)
	}
}

// TestPainel_AtrasoVelhoNaoIgnoraOCache: voltar a um recurso com os extras no
// cache invalida o atraso pendente do recurso de antes.
func TestPainel_AtrasoVelhoNaoIgnoraOCache(t *testing.T) {
	c := clienteDetalheDaPrancha()
	m := assentarDetalhe(t, telaDaPranchaCom(t, c, 140, 32)) // worker lido
	m, atraso := teclas(t, m, "j")                           // web, sem cache: agenda
	if atraso == nil {
		t.Fatal("o web sem extras deveria agendar a leitura")
	}
	m, _ = teclas(t, m, "k") // de volta ao worker, com cache
	c.leituras = nil
	if _, cmd := atualizar(m, atraso()); cmd != nil || len(c.leituras) != 0 {
		t.Fatalf("o atraso do web não deveria reler o worker do cache: %v", c.leituras)
	}
}

// TestPainel_RReleOsExtras: o r relê os extras do painel mesmo no cache e com
// uma leitura em andamento; a resposta de antes do r é descartada.
func TestPainel_RReleOsExtras(t *testing.T) {
	semCor(t)
	c := clienteDetalheDaPrancha()
	m := assentarDetalhe(t, telaDaPranchaCom(t, c, 140, 32))
	c.leituras = nil
	m, cmd := teclas(t, m, "r")
	if cmd == nil {
		t.Fatal("o r deveria ler de novo")
	}
	for _, msg := range cmd().(tea.BatchMsg) {
		if msg != nil {
			msg()
		}
	}
	if got := strings.Join(c.leituras, ", "); !strings.Contains(got, "eventos Pod/worker-5d8b6-q7tvn, logs worker-5d8b6-q7tvn") {
		t.Fatalf("o r deveria reler os extras do worker: %s", got)
	}

	// Com uma leitura em andamento, o r lê de novo e só a resposta nova vale.
	m, ler := atualizar(m, paradaMsg{id: m.pedidoDetalhe})
	if ler != nil {
		t.Fatal("o atraso com o r em andamento não deveria ler de novo")
	}
	velho := m.esquecerExtras()
	c.logs["worker-5d8b6-q7tvn"] = "nova linha\n"
	novo := m.esquecerExtras()
	antes := velho().(extrasMsg)
	antes.extras.log = "linha de antes do r"
	m, _ = atualizar(m, antes)
	if _, ok := m.extras[m.alvoDetalhe]; ok {
		t.Fatal("a resposta de antes do r não deveria entrar no cache")
	}
	m, _ = atualizar(m, novo())
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "nova linha") || m.lendoExtras != "" {
		t.Fatalf("a resposta do r deveria aparecer e soltar a leitura:\n%s", tela)
	}
}

// TestPainel_EscComPainelForaDaTela: o esc só fecha o painel que está na
// tela; com a lista vazia pela busca ou com a janela larga, ele limpa os
// filtros já no primeiro toque.
func TestPainel_EscComPainelForaDaTela(t *testing.T) {
	semCor(t)
	m := telaDaPranchaCom(t, clienteDetalheDaPrancha(), 100, 30)
	m, _ = teclas(t, m, "enter")
	vazia := buscar(t, m, "nadaassim")
	if vazia.painelAberto() {
		t.Fatal("sem linhas, o painel não está na tela")
	}
	if vazia, _ = teclas(t, vazia, "esc"); vazia.filter.SearchQuery != "" || vazia.detalheAberto {
		t.Errorf("um esc deveria limpar a busca e fechar o painel: busca %q, aberto %v", vazia.filter.SearchQuery, vazia.detalheAberto)
	}

	larga, _ := atualizar(m, tea.WindowSizeMsg{Width: 140, Height: 32})
	larga = buscar(t, larga, "web")
	if larga, _ = teclas(t, larga, "esc"); larga.filter.SearchQuery != "" {
		t.Errorf("ao lado, o esc deveria limpar a busca: %q", larga.filter.SearchQuery)
	}
	if estreita, _ := atualizar(larga, tea.WindowSizeMsg{Width: 100, Height: 30}); estreita.painelAberto() {
		t.Error("voltando a estreitar, o painel começa fechado")
	}
}

// TestPainel_PorCimaNosExtremos: em 60x16 e 80x24, o ↵ abre o painel e a
// seleção continua na lista embaixo dele.
func TestPainel_PorCimaNosExtremos(t *testing.T) {
	semCor(t)
	for _, tam := range [][2]int{{60, 16}, {80, 24}} {
		m := telaDaPranchaCom(t, clienteDetalheDaPrancha(), tam[0], tam[1])
		m, _ = teclas(t, m, "enter", "j", "j")
		m = assentarDetalhe(t, m)
		tela := ansi.Strip(m.View())
		if !m.painelPorCima() || !strings.Contains(tela, "POD · api-7f9c4d-x2k2p") || !selecionadaNaTela(t, m) {
			t.Errorf("%dx%d: o painel e a seleção deveriam estar na tela:\n%s", tam[0], tam[1], tela)
		}
		if len(strings.Split(tela, "\n")) != tam[1] {
			t.Errorf("%dx%d: a tela deveria ter %d linhas", tam[0], tam[1], tam[1])
		}
	}
}

// TestPainel_LogDoConteiner: o painel e o quadro pedem o log do contêiner do
// pod, e o quadro diz qual é quando o pod tem mais de um.
func TestPainel_LogDoConteiner(t *testing.T) {
	semCor(t)
	c := clienteDetalheDaPrancha()
	m := telaDaPranchaCom(t, c, 140, 32)
	pods := podsDaPrancha()
	for i := range pods {
		pods[i].Conteiner, pods[i].Containers = "app", 2
	}
	m, _ = atualizar(m, pods)
	m = assentarDetalhe(t, m)
	m, pedido := teclas(t, m, "l")
	m, _ = atualizar(m, pedido())
	if got := strings.Join(c.leituras, ", "); got != "eventos Pod/worker-5d8b6-q7tvn, logs worker-5d8b6-q7tvn/app, logs worker-5d8b6-q7tvn/app" {
		t.Errorf("leituras: %s", got)
	}
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "últimas 100 linhas do log de app · em payments") {
		t.Errorf("o quadro deveria dizer de qual contêiner é o log:\n%s", tela)
	}
}

// TestPainel_DescartaResultadoDeOutroRecurso: os extras que chegam depois que
// a seleção saiu do recurso não entram no cache nem no painel.
func TestPainel_DescartaResultadoDeOutroRecurso(t *testing.T) {
	semCor(t)
	m := telaDaPranchaCom(t, clienteDetalheDaPrancha(), 140, 32)
	m, ler := atualizar(m, paradaMsg{id: m.pedidoDetalhe}) // worker
	velho := ler()
	m, _ = teclas(t, m, "j") // web, antes de a resposta do worker chegar
	m, _ = atualizar(m, velho)
	if _, ok := m.extras["pod|payments/worker-5d8b6-q7tvn"]; ok {
		t.Fatal("os extras do worker chegaram depois da troca e não deveriam entrar no cache")
	}
	if tela := ansi.Strip(m.View()); strings.Contains(tela, "BackOff reiniciando") || !strings.Contains(tela, "lendo eventos") {
		t.Fatalf("o painel do web não mostra os eventos do worker:\n%s", tela)
	}
	// A leitura do worker não prende a do web.
	if m, _ = atualizar(m, paradaMsg{id: m.pedidoDetalhe}); m.lendoExtras != m.alvoDetalhe {
		t.Fatalf("o web deveria estar sendo lido: %q, alvo %q", m.lendoExtras, m.alvoDetalhe)
	}
}

// TestPainel_PorCima: abaixo de 120 colunas, ↵ abre o painel por cima da
// lista e esc fecha; q não sai dele; ↵ de novo abre o YAML; a lista continua
// rolando por baixo e o clique acerta as linhas dela.
func TestPainel_PorCima(t *testing.T) {
	semCor(t)
	m := telaDaPranchaCom(t, clienteDetalheDaPrancha(), 100, 30)
	if strings.Contains(ansi.Strip(m.View()), "POD · ") {
		t.Fatal("em 100 colunas o painel só abre com ↵")
	}
	m, _ = teclas(t, m, "enter")
	if !m.painelPorCima() || !strings.Contains(ansi.Strip(m.View()), "POD · worker-5d8b6-q7tvn") {
		t.Fatal("o ↵ deveria abrir o painel por cima da lista")
	}
	if q, cmd := teclas(t, m, "q"); cmd != nil || !strings.Contains(ansi.Strip(q.actionFeedback), "q sai só da lista") {
		t.Errorf("q no painel: %q", ansi.Strip(q.actionFeedback))
	}
	tela := linhasNaTela(m)
	for i := 0; i < min(m.linhasDeDados(), m.itemCount()); i++ {
		y := m.primeiraLinhaDeDados() + i
		nome := m.itensDaAba(TabPods)[i].nome
		if !strings.Contains(tela[y], nome) {
			t.Errorf("a linha %d da tela deveria ser %s: %q", y, nome, tela[y])
		}
		if nm, _ := m.clique(5, y); nm.(Model).selectedIndex != i {
			t.Errorf("o clique na linha %d selecionou %d", y, nm.(Model).selectedIndex)
		}
	}
	j, _ := teclas(t, m, "j")
	if !strings.Contains(ansi.Strip(j.View()), "POD · web-8b77c-5jw4f") {
		t.Error("com o painel por cima, ↓ move a seleção e o painel acompanha")
	}
	if y, cmd := teclas(t, m, "enter"); !y.quadro.aberto || y.quadro.tipo != quadroYAML || cmd == nil {
		t.Error("↵ no painel deveria abrir o YAML")
	}
	if f, _ := teclas(t, m, "esc"); f.painelAberto() {
		t.Error("esc deveria fechar o painel")
	}

	// A lista encolhe embaixo do painel: a seleção no fim continua na tela.
	longa, _ := atualizar(m, muitosPods(60))
	longa, _ = teclas(t, longa, "esc", "G", "enter")
	if !longa.painelPorCima() || !selecionadaNaTela(t, longa) {
		t.Errorf("com o painel aberto, a seleção (linha %d, rolagem %d) saiu da tela", longa.selectedIndex, longa.scrollY)
	}
}

// TestPainel_AoLado: ↵ abre o YAML (na aba Eventos, os eventos do objeto), l
// fora dos pods avisa, e o clique no painel não seleciona linha.
func TestPainel_AoLado(t *testing.T) {
	c := clienteDetalheDaPrancha()
	m := telaDaPranchaCom(t, c, 140, 32)
	y, pedido := teclas(t, m, "enter")
	if !y.quadro.aberto || y.quadro.tipo != quadroYAML {
		t.Fatal("ao lado, ↵ abre o YAML")
	}
	pedido()
	if got := c.leituras[len(c.leituras)-1]; got != "yaml pod/worker-5d8b6-q7tvn" {
		t.Errorf("leitura do ↵: %s", got)
	}
	if nm, _ := m.clique(130, m.primeiraLinhaDeDados()+3); nm.(Model).selectedIndex != m.selectedIndex {
		t.Error("o clique no painel não deveria selecionar a linha da lista")
	}
	d, _ := teclas(t, m, "2", "l")
	if d.quadro.aberto || !strings.Contains(ansi.Strip(d.actionFeedback), "logs só existem para pods") {
		t.Errorf("l nos deployments: %q", ansi.Strip(d.actionFeedback))
	}

	e := m
	e.events = []monitor.Event{{ID: "worker.BackOff", Name: "worker-5d8b6-q7tvn", Namespace: "payments", Kind: "Pod", Type: "Warning", Reason: "BackOff"}}
	e.carregadas |= 1 << uint(TabEvents)
	e, _ = teclas(t, e, "9")
	e, pedido = teclas(t, e, "enter")
	pedido()
	if e.quadro.tipo != quadroEventos || c.leituras[len(c.leituras)-1] != "eventos Pod/worker-5d8b6-q7tvn" {
		t.Errorf("↵ num evento abre os eventos do objeto: %s, %v", e.quadro.tipo, c.leituras)
	}
	e, pedido = teclas(t, e, "y")
	pedido()
	if c.leituras[len(c.leituras)-1] != "yaml event/worker.BackOff" {
		t.Errorf("y num evento lê o YAML do Event: %v", c.leituras)
	}
}

// TestQuadro_DescartaRespostaVelha: o conteúdo de um quadro fechado ou
// trocado não aparece no quadro novo; o do quadro na tela, sim.
func TestQuadro_DescartaRespostaVelha(t *testing.T) {
	semCor(t)
	m := telaComDetalhe(t, 140, 32)
	m, yaml := teclas(t, m, "y")
	m, eventos := teclas(t, m, "e") // troca de conteúdo antes de o YAML chegar
	m, _ = atualizar(m, yaml())
	if !m.quadro.carregando || m.quadro.tipo != quadroEventos {
		t.Fatal("o YAML que chegou depois da troca não deveria preencher o quadro de eventos")
	}
	m, _ = atualizar(m, eventos())
	tela := ansi.Strip(m.View())
	if m.quadro.carregando || !strings.Contains(tela, "EVENTOS · worker-5d8b6-q7tvn") || !strings.Contains(tela, "BackOff reiniciando container") {
		t.Fatalf("os eventos deveriam aparecer:\n%s", tela)
	}
	m, logs := teclas(t, m, "l")
	m, _ = teclas(t, m, "esc")
	if m, _ = atualizar(m, logs()); m.quadro.aberto {
		t.Fatal("o log que chegou depois do esc não deveria reabrir o quadro")
	}
}

// TestQuadro_RRecarrega: r no quadro lê o conteúdo de novo, mantém a rolagem e
// descarta a resposta do pedido de antes.
func TestQuadro_RRecarrega(t *testing.T) {
	semCor(t)
	c := clienteDetalheDaPrancha()
	m := telaComDetalhe(t, 140, 32)
	m.client = c
	m, pedido := teclas(t, m, "l")
	m, _ = atualizar(m, pedido())
	m, antigo := teclas(t, m, "r")
	if antigo == nil || !m.quadro.carregando {
		t.Fatal("r no quadro deveria ler de novo")
	}
	c.logs["worker-5d8b6-q7tvn"] = logDoWorker + "reiniciando o worker\n"
	m, novo := teclas(t, m, "r")
	m, _ = atualizar(m, antigo())
	if !m.quadro.carregando {
		t.Fatal("a resposta do r anterior não deveria preencher o quadro")
	}
	m, _ = atualizar(m, novo())
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "reiniciando o worker") || !strings.Contains(tela, "LOGS · worker-5d8b6-q7tvn") {
		t.Fatalf("o quadro deveria mostrar o log novo:\n%s", tela)
	}
}

// TestQuadro_AvisoDeRolagem: o conteúdo maior que o corpo rola com o aviso de
// linhas acima e abaixo no topo, no meio e no fim; a janela que cresce traz
// a rolagem para o máximo novo.
func TestQuadro_AvisoDeRolagem(t *testing.T) {
	semCor(t)
	c := clienteDetalheDaPrancha()
	var yaml []string
	for i := range 50 {
		yaml = append(yaml, fmt.Sprintf("linha-%02d: valor", i))
	}
	c.yaml = strings.Join(yaml, "\n")
	m := telaDaPranchaCom(t, c, 80, 24)
	m, pedido := teclas(t, m, "y")
	m, _ = atualizar(m, pedido())
	v := m.linhasVisiveisDoQuadro()
	casos := []struct {
		teclas []string
		quer   []string
		nao    string
	}{
		{nil, []string{"linha-00", fmt.Sprintf("↓ %d linhas abaixo", 50-(v-1))}, "acima"},
		{[]string{"pgdown"}, []string{fmt.Sprintf("↑ %d linhas acima", v-1), "linhas abaixo"}, "linha-00"}, // a página é a janela menos a linha do aviso
		{[]string{"G"}, []string{"linha-49", fmt.Sprintf("↑ %d linhas acima", 50-(v-1))}, "abaixo"},
	}
	for _, cs := range casos {
		n, _ := teclas(t, m, cs.teclas...)
		tela := ansi.Strip(n.View())
		for _, q := range cs.quer {
			if !strings.Contains(tela, q) {
				t.Errorf("%v: o quadro deveria mostrar %q:\n%s", cs.teclas, q, tela)
			}
		}
		if strings.Contains(tela, cs.nao) {
			t.Errorf("%v: o quadro não deveria mostrar %q:\n%s", cs.teclas, cs.nao, tela)
		}
	}

	fim, _ := teclas(t, m, "G")
	alta, _ := atualizar(fim, tea.WindowSizeMsg{Width: 80, Height: 40})
	if alta.quadro.scroll != alta.maxRolagemQuadro() {
		t.Fatalf("a rolagem %d deveria vir para o máximo novo %d", alta.quadro.scroll, alta.maxRolagemQuadro())
	}
	if subiu, _ := teclas(t, alta, "up"); subiu.quadro.scroll != alta.quadro.scroll-1 {
		t.Errorf("o primeiro ↑ depois de crescer a janela deveria rolar: %d", subiu.quadro.scroll)
	}
}

// TestQuadro_ErroEVazio: sem leitor, com erro do cluster ou sem conteúdo, o
// quadro diz isso no lugar do conteúdo.
func TestQuadro_ErroEVazio(t *testing.T) {
	semCor(t)
	casos := map[string]struct {
		client monitor.Client
		tecla  string
		quer   string
	}{
		"sem leitor": {clienteStaging{&MockClient{}}, "y", "não deu para ler o YAML"},
		"erro":       {&clienteComDetalhe{clienteStaging: clienteStaging{&MockClient{}}, err: fmt.Errorf("forbidden")}, "l", "forbidden"},
		"vazio":      {&clienteComDetalhe{clienteStaging: clienteStaging{&MockClient{}}}, "e", "nenhum evento deste recurso"},
	}
	for nome, c := range casos {
		m, pedido := teclas(t, telaDaPranchaCom(t, c.client, 120, 30), c.tecla)
		m, _ = atualizar(m, pedido())
		if tela := ansi.Strip(m.View()); !strings.Contains(tela, c.quer) {
			t.Errorf("%s: o quadro deveria dizer %q:\n%s", nome, c.quer, tela)
		}
	}
}

// TestPainel_CamposPorTipo: cada tipo mostra os campos que fazem sentido.
func TestPainel_CamposPorTipo(t *testing.T) {
	semCor(t)
	m := telaDaPrancha(t, 140, 32)
	casos := []struct {
		obj  any
		quer []string
	}{
		{monitor.StatefulSet{Name: "redis", Replicas: 1, Ready: 1, Servico: "redis-headless", Imagens: []string{"redis:7"}}, []string{"serviço   redis-headless", "prontos   ● 1/1"}},
		{monitor.Service{Name: "web", Type: "LoadBalancer", ClusterIP: "10.43.0.7", Externos: "1.2.3.4", Ports: "80/TCP", Seletor: "app=web"}, []string{"externos  1.2.3.4", "portas    80/TCP"}},
		{monitor.Node{Name: "n1", Status: "Ready", Version: "v1.31.2", CPUCapacity: "4", MemoryCapacity: "15Gi", Papeis: "control-plane"}, []string{"capacidade", "cpu 4 · mem 15Gi", "papéis    control-plane"}},
		{monitor.Job{Name: "migrate", Completions: 1, Succeeded: 1}, []string{"conclusões", "1/1"}},
		{monitor.Ingress{Name: "web", Class: "traefik", Hosts: "loja.exemplo", Endereco: "10.43.0.9"}, []string{"hosts     loja.exemplo", "endereço  10.43.0.9"}},
		{monitor.ConfigMap{Name: "cfg", Keys: 2, Chaves: []string{"LOG_LEVEL", "app.yaml"}, DataSize: "1.2Ki"}, []string{"chaves    2 · LOG_LEVEL, app.yaml"}},
		{monitor.Event{Name: "worker", Kind: "Pod", Type: "Warning", Reason: "BackOff", Quantas: 14, Quando: horaDaPrancha.Add(-2 * time.Minute)}, []string{"objeto    Pod worker", "vezes     14", "idade     2m"}},
	}
	for _, c := range casos {
		var linhas []string
		for _, cp := range m.camposDoItem(itemLista{obj: c.obj, tipo: yui.StatusRodando, status: "Concluído"}) {
			linhas = append(linhas, ansi.Strip(cp.rotulo+cp.valor))
		}
		todas := strings.Join(linhas, "\n")
		for _, quer := range c.quer {
			if !strings.Contains(todas, quer) {
				t.Errorf("%T deveria ter %q:\n%s", c.obj, quer, todas)
			}
		}
	}
}

// TestUltimaLinhaDeLog: a última linha com texto, sem cores nem \r; a
// primeira palavra que diz falha sai destacada.
func TestUltimaLinhaDeLog(t *testing.T) {
	if got := ultimaLinha("a\r\n\x1b[31mERROR\x1b[0m falhou\r\n\n  \n"); got != "ERROR falhou" {
		t.Errorf("última linha: %q", got)
	}
	usarModo(t, modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor})
	if l := linhaDeLog("panic: x"); ansi.Strip(l) != "panic: x" || !strings.HasPrefix(l, yui.S().Erro.Render("panic:")) {
		t.Errorf("panic deveria sair em vermelho: %q", l)
	}
	if l := linhaDeLog("ok tudo certo"); strings.Contains(l, yui.S().Erro.Render("ok")) {
		t.Errorf("linha comum sem destaque: %q", l)
	}
	if partes := duasLinhas(strings.Repeat("palavra ", 20), 20); len(partes) != 2 || !strings.HasSuffix(ansi.Strip(partes[1]), "…") {
		t.Errorf("a linha longa fica em duas, com reticências: %q", partes)
	}
}

// TestTextoDoEvento: a razão não se repete quando a mensagem já começa por ela.
func TestTextoDoEvento(t *testing.T) {
	casos := []struct{ razao, msg, quer string }{
		{"Created", "Created container: worker", "Created container: worker"},
		{"BackOff", "Back-off restarting failed container", "BackOff Back-off restarting failed container"},
		{"Pulled", "worker:1.8.2", "Pulled worker:1.8.2"},
		{"", "sem razão", "sem razão"},
	}
	for _, c := range casos {
		if got := textoDoEvento(monitor.Event{Reason: c.razao, Message: c.msg}); got != c.quer {
			t.Errorf("textoDoEvento(%q, %q) = %q, quer %q", c.razao, c.msg, got, c.quer)
		}
	}
}
