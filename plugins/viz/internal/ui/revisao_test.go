package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// Correções da revisão da T019: relógio, seleção, rolagem, filtros e teclas.

// atualizar entrega uma mensagem ao modelo e devolve o modelo e o comando.
func atualizar(m Model, msg tea.Msg) (Model, tea.Cmd) {
	nm, cmd := m.Update(msg)
	return nm.(Model), cmd
}

// tecla monta a KeyMsg de uma tecla especial ou de um caractere.
func tecla(k string) tea.KeyMsg {
	especiais := map[string]tea.KeyType{
		"enter": tea.KeyEnter, "esc": tea.KeyEsc, "pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp,
		"ctrl+c": tea.KeyCtrlC, "tab": tea.KeyTab, "shift+tab": tea.KeyShiftTab, "home": tea.KeyHome, "end": tea.KeyEnd,
		"backspace": tea.KeyBackspace, "up": tea.KeyUp, "down": tea.KeyDown,
	}
	if t, ok := especiais[k]; ok {
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

// muitosPods devolve n pods saudáveis em "loja", na ordem do nome.
func muitosPods(n int) []monitor.Pod {
	pods := make([]monitor.Pod, n)
	for i := range pods {
		pods[i] = monitor.Pod{Name: fmt.Sprintf("p-%02d", i), Namespace: "loja", Status: "Running", Prontos: 1, Containers: 1}
	}
	return pods
}

// linhasNaTela devolve as linhas da tela sem cor.
func linhasNaTela(m Model) []string {
	return strings.Split(ansi.Strip(m.View()), "\n")
}

// selecionadaNaTela diz se o recurso selecionado aparece no corpo da tela.
func selecionadaNaTela(t *testing.T, m Model) bool {
	t.Helper()
	nome, _, _ := m.selectedResourceInfo()
	for _, l := range linhasNaTela(m)[5:] {
		if strings.HasPrefix(strings.TrimSpace(l), nome+" ") {
			return true
		}
	}
	return false
}

// TestEvento_MensagemComVariasLinhas: a mensagem de várias linhas de um evento
// fica numa linha só na tabela e na linha de estado; a seleção continua na tela
// e a moldura com a altura da janela.
func TestEvento_MensagemComVariasLinhas(t *testing.T) {
	m := comJanela(NewModel(&MockClient{}))
	m.activeTab = TabEvents
	m, _ = atualizar(m, []monitor.Event{
		{ID: "a.1", Name: "a", Namespace: "loja", Type: "Warning", Reason: "Failed", Message: "linha1\nlinha2\r\nlinha3"},
		{ID: "b.1", Name: "b", Namespace: "loja", Type: "Normal"},
		{ID: "c.1", Name: "c", Namespace: "loja", Type: "Normal"},
	})
	m, _ = teclas(t, m, "j", "j")
	if nome, _, _ := m.selectedResourceInfo(); nome != "c" {
		t.Fatalf("selecionado = %q", nome)
	}
	linhas := linhasNaTela(m)
	if len(linhas) != m.height {
		t.Fatalf("a moldura tem %d linhas, esperado %d", len(linhas), m.height)
	}
	tela := strings.Join(linhas, "\n")
	for _, quer := range []string{"linha1 linha2 linha3", "Failed em a: linha1 linha2 linha3"} {
		if !strings.Contains(tela, quer) {
			t.Errorf("a tela deveria ter %q:\n%s", quer, tela)
		}
	}
	if !selecionadaNaTela(t, m) {
		t.Errorf("o evento selecionado sumiu da tela:\n%s", tela)
	}
}

// TestAtualizacao_MantemARolagem: a lista que chega de novo não desfaz a
// rolagem do usuário, e o pgdn leva a seleção junto (a ação atinge uma linha
// da tela).
func TestAtualizacao_MantemARolagem(t *testing.T) {
	m := NewModel(&MockClient{})
	m.width, m.height = 120, 20
	m, _ = atualizar(m, muitosPods(50))
	m, _ = atualizar(m, tecla("pgdown"))
	if m.scrollY == 0 || m.selectedIndex == 0 {
		t.Fatalf("pgdn: scrollY %d, seleção %d", m.scrollY, m.selectedIndex)
	}
	if !selecionadaNaTela(t, m) {
		t.Fatalf("depois do pgdn a seleção (%d) está fora da janela (%d)", m.selectedIndex, m.scrollY)
	}
	rolagem, sel := m.scrollY, m.selectedIndex
	m, _ = atualizar(m, muitosPods(50))
	if m.scrollY != rolagem || m.selectedIndex != sel {
		t.Fatalf("a atualização mexeu na rolagem: scrollY %d→%d, seleção %d→%d", rolagem, m.scrollY, sel, m.selectedIndex)
	}
	m, _ = atualizar(m, tecla("pgup"))
	if m.scrollY != 0 || m.selectedIndex != 0 || !selecionadaNaTela(t, m) {
		t.Fatalf("pgup: scrollY %d, seleção %d", m.scrollY, m.selectedIndex)
	}
}

// TestRolagem_AvisaOQueFicouFora: a lista maior que a janela diz quantas
// linhas ficaram acima e abaixo.
func TestRolagem_AvisaOQueFicouFora(t *testing.T) {
	usarModo(t, modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor})
	m := NewModel(&MockClient{})
	m.width, m.height = 100, 20
	m, _ = atualizar(m, muitosPods(40))
	tela := ansi.Strip(m.View())
	if !strings.Contains(tela, "↓ 30 pods abaixo") || strings.Contains(tela, "acima") {
		t.Fatalf("no topo:\n%s", tela)
	}
	m, _ = atualizar(m, tea.KeyMsg{Type: tea.KeyEnd})
	tela = ansi.Strip(m.View())
	if !strings.Contains(tela, "↑ 30 pods acima") || strings.Contains(tela, "abaixo") || !selecionadaNaTela(t, m) {
		t.Fatalf("no fim:\n%s", tela)
	}
	if n := len(strings.Split(tela, "\n")); n != 20 {
		t.Fatalf("altura %d", n)
	}
}

// TestEventos_SelecaoNoMesmoObjeto: vários eventos do mesmo objeto; a seleção
// no terceiro fica nele quando a lista chega de novo, com ou sem ID.
func TestEventos_SelecaoNoMesmoObjeto(t *testing.T) {
	for _, comID := range []bool{true, false} {
		t.Run(fmt.Sprintf("id_%v", comID), func(t *testing.T) {
			eventos := func() []monitor.Event {
				ev := []monitor.Event{
					{ID: "worker.1", Name: "worker", Namespace: "loja", Type: "Warning", Reason: "BackOff"},
					{ID: "worker.2", Name: "worker", Namespace: "loja", Type: "Normal", Reason: "Pulled"},
					{ID: "worker.3", Name: "worker", Namespace: "loja", Type: "Normal", Reason: "Started"},
				}
				if !comID {
					for i := range ev {
						ev[i].ID = ""
					}
				}
				return ev
			}
			m := comJanela(NewModel(&MockClient{}))
			m.activeTab = TabEvents
			m, _ = atualizar(m, eventos())
			m, _ = teclas(t, m, "j", "j")
			m, _ = atualizar(m, eventos())
			if m.selectedIndex != 2 {
				t.Fatalf("a seleção voltou para %d", m.selectedIndex)
			}
			if !comID {
				return
			}
			// Com ID, um evento novo no topo não tira a seleção do mesmo evento.
			novo := append([]monitor.Event{{ID: "worker.4", Name: "worker", Namespace: "loja", Type: "Normal"}}, eventos()...)
			m, _ = atualizar(m, novo)
			if m.selectedIndex != 3 {
				t.Fatalf("com o evento novo a seleção foi para %d", m.selectedIndex)
			}
		})
	}
}

// relogioImediato troca o tea.Tick por um comando que devolve o tique na hora
// e conta quantos foram agendados.
func relogioImediato(m Model, agendados *int) Model {
	m.tique = func(_ time.Duration, f func(time.Time) tea.Msg) tea.Cmd {
		*agendados++
		return func() tea.Msg { return f(horaDaPrancha) }
	}
	return m
}

// TestRelogio_UmaLeituraPorVez: só a leitura do relógio agenda a próxima; as
// leituras avulsas (troca de aba, ação, namespace) não abrem outro relógio, o
// esc sem filtro não lê o cluster e o quadro não para o relógio.
func TestRelogio_UmaLeituraPorVez(t *testing.T) {
	agendados := 0
	m := relogioImediato(comJanela(NewModel(&MockClient{Pods: muitosPods(3)})), &agendados)

	leitura := m.Init()()
	m, cmd := atualizar(m, leitura)
	if agendados != 1 || cmd == nil {
		t.Fatalf("a primeira leitura deveria agendar um tique (agendados %d)", agendados)
	}
	tique := cmd()

	// Cinco trocas de aba: cada uma lê uma vez e não agenda nada.
	for i := 0; i < 5; i++ {
		var c tea.Cmd
		m, c = atualizar(m, tecla("tab"))
		if _, c = atualizar(m, c()); c != nil {
			t.Fatalf("a leitura da troca de aba %d agendou outro relógio", i+1)
		}
	}
	m, _ = atualizar(m, tecla("1"))
	if _, c := atualizar(m, tecla("esc")); c != nil {
		t.Fatal("esc sem filtro não deveria ler o cluster")
	}

	// No quadro, o tique ainda lê e a resposta agenda o próximo.
	m, _ = teclas(t, m, "y")
	m, cmd = atualizar(m, tique)
	if cmd == nil {
		t.Fatal("o tique no quadro deveria ler o cluster")
	}
	m, cmd = atualizar(m, cmd())
	if cmd == nil || agendados != 2 {
		t.Fatalf("a resposta no quadro deveria agendar o próximo tique (agendados %d)", agendados)
	}
	if !m.quadro.aberto {
		t.Fatal("a leitura não deveria fechar o quadro")
	}
}

// TestRelogio_LeSoAAbaAtiva: o tique lê só a aba ativa; as contagens das abas
// de fundo vêm do watch (TestVigia_*), sem rodízio.
func TestRelogio_LeSoAAbaAtiva(t *testing.T) {
	m := telaDaPrancha(t, 140, 32) // Pods ativa; Deployments, StatefulSets, Services e Nodes lidas
	for _, tab := range []ResourceTab{TabPods, TabDeployments} {
		m.activeTab = tab
		_, cmd := atualizar(m, tickMsg(horaDaPrancha))
		msg := cmd().(leituraMsg)
		if len(msg.abas) != 1 || msg.abas[0] != tab || len(msg.respostas) != 1 {
			t.Fatalf("o tique em %s leu %v", tab, msg.abas)
		}
	}
}

// TestErro_ListaQueChegaLimpaOErro: um erro na primeira leitura não prende a
// tela de erro quando a lista chega depois; uma aba de fundo que falha (RBAC)
// não derruba a tela.
func TestErro_ListaQueChegaLimpaOErro(t *testing.T) {
	m := comJanela(NewModel(&MockClient{}))
	m, _ = atualizar(m, fmt.Errorf("timeout"))
	if m.err == nil {
		t.Fatal("sem dados, o erro aparece na hora")
	}
	m, _ = atualizar(m, leituraMsg{respostas: []tea.Msg{muitosPods(2)}})
	if m.err != nil || !m.carregada(TabPods) {
		t.Fatalf("a lista que chegou deveria limpar o erro: %v", m.err)
	}
	m, _ = atualizar(m, leituraMsg{abas: []ResourceTab{TabPods, TabDeployments}, respostas: []tea.Msg{muitosPods(2), fmt.Errorf("forbidden")}})
	if m.err != nil || m.reconnecting {
		t.Fatalf("a falha da aba de fundo derrubou a tela: err %v, reconectando %v", m.err, m.reconnecting)
	}
}

// TestTrocaDeNamespace_AbasVoltamSemContagem: com outro namespace, as listas
// do anterior somem (abas sem número, "lendo …") e uma leitura atrasada do
// namespace anterior é descartada.
func TestTrocaDeNamespace_AbasVoltamSemContagem(t *testing.T) {
	usarModo(t, modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor})
	m := telaDaPrancha(t, 220, 32)
	m, cmd := teclas(t, m, "n", "o", "u", "t", "r", "o", "enter")
	if cmd == nil {
		t.Fatal("o namespace novo deveria ser lido")
	}
	normais, _ := m.linhaAbas()
	quer := "◆ 1 Pods   ◇ 2 Deployments   ◇ 3 StatefulSets   ◇ 4 Services   ◇ 5 Nodes   ◇ 6 Jobs   ◇ 7 Ingresses   ◇ 8 ConfigMaps   ◇ 9 Eventos"
	if got := ansi.Strip(normais); got != quer {
		t.Fatalf("abas depois da troca:\n%q\nesperado:\n%q", got, quer)
	}
	if v := ansi.Strip(m.View()); !strings.Contains(v, "lendo pods de outro") {
		t.Fatalf("a aba ativa deveria estar lendo:\n%s", v)
	}
	m, _ = atualizar(m, leituraMsg{namespace: "payments", respostas: []tea.Msg{podsDaPrancha()}})
	if m.carregada(TabPods) {
		t.Fatal("a leitura atrasada de payments não vale para outro")
	}
	if msg, ok := cmd().(leituraMsg); !ok || msg.namespace != "outro" {
		t.Fatalf("leitura do namespace novo: %#v", msg)
	}

	// O mesmo namespace não apaga nada nem lê de novo.
	m = telaDaPrancha(t, 140, 32)
	if m, cmd = teclas(t, m, "n", "p", "a", "y", "m", "e", "n", "t", "s", "enter"); cmd != nil || !m.carregada(TabDeployments) {
		t.Fatal("o mesmo namespace não deveria apagar as listas")
	}
}

// TestEsc_LimpaFiltrosEMantemONamespace: o esc limpa label, status e busca;
// o namespace é o contexto e fica. A legenda só oferece o esc com filtro.
func TestEsc_LimpaFiltrosEMantemONamespace(t *testing.T) {
	usarModo(t, modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor})
	m := telaDaPrancha(t, 200, 32)
	if strings.Contains(ansi.Strip(m.View()), "limpar filtros") {
		t.Fatal("sem filtro, a legenda não oferece o esc")
	}
	m = buscar(t, m, "api")
	m.filter.LabelSelector = "app=api"
	if !strings.Contains(ansi.Strip(m.View()), "esc limpar filtros") {
		t.Fatal("com filtro, a legenda oferece o esc")
	}
	m, _ = teclas(t, m, "esc")
	if m.filter.Namespace != "payments" || m.algumFiltro() {
		t.Fatalf("depois do esc: %+v", m.filter)
	}
}

// TestFiltroStatus_SoNosPods: o filtro de status vale para os pods; nós, jobs
// e eventos não somem por ele, e a aba sem status não mostra o chip.
func TestFiltroStatus_SoNosPods(t *testing.T) {
	m := telaDaPrancha(t, 140, 32)
	m.filter.StatusFilter = "Running"
	if n := len(m.itensDaAba(TabPods)); n != 5 {
		t.Errorf("pods Running: %d", n)
	}
	if n := len(m.itensDaAba(TabNodes)); n != 3 {
		t.Errorf("os nós não deveriam sumir com status=Running: %d", n)
	}
	m.activeTab = TabNodes
	if len(m.chips()) != 0 {
		t.Errorf("a aba de nós não mostra o chip de status: %v", m.chips())
	}
}

// TestJobs_Classificacao: as condições do Job mandam; o Job de fila de trabalho
// (sem completions) que terminou bem está concluído; o que falhou e ainda tem
// pods ativos está rodando, com alerta.
func TestJobs_Classificacao(t *testing.T) {
	casos := []struct {
		nome         string
		job          monitor.Job
		grupo        grupo
		status, cols string
	}{
		{"fila concluída", monitor.Job{Succeeded: 1}, grupoConcluido, "Concluído", "1"},
		{"condição Complete", monitor.Job{Completions: 3, Succeeded: 2, Concluido: true}, grupoConcluido, "Concluído", "2/3"},
		{"condição Failed", monitor.Job{Completions: 1, Failed: 6, Falhou: true}, grupoFalha, "Falhou", "0/1"},
		{"tenta de novo", monitor.Job{Completions: 1, Active: 1, Failed: 2}, grupoAlerta, "Rodando", "0/1"},
		{"rodando", monitor.Job{Completions: 1, Active: 1}, grupoSaudavel, "Rodando", "0/1"},
		{"falhou sem pods", monitor.Job{Completions: 1, Failed: 1}, grupoFalha, "Falhou", "0/1"},
		{"aguardando", monitor.Job{Completions: 1}, grupoAlerta, "Aguardando", "0/1"},
	}
	for _, c := range casos {
		g, _, st, _ := classificarJob(c.job)
		if g != c.grupo || st != c.status || conclusoes(c.job) != c.cols {
			t.Errorf("%s: grupo %d, status %q, conclusões %q", c.nome, g, st, conclusoes(c.job))
		}
	}
}

// TestOrdemPorNome: Services, Ingresses e ConfigMaps saem ordenados por nome,
// como diz a linha de filtros.
func TestOrdemPorNome(t *testing.T) {
	m := comJanela(NewModel(&MockClient{}))
	m.services = []monitor.Service{{Name: "web", Namespace: "b"}, {Name: "api", Namespace: "b"}, {Name: "api", Namespace: "a"}}
	m.ingresses = []monitor.Ingress{{Name: "z"}, {Name: "a"}}
	m.configmaps = []monitor.ConfigMap{{Name: "m"}, {Name: "b"}}
	if got := strings.Join(nomes(m, TabServices), " "); got != "api api web" {
		t.Errorf("services: %s", got)
	}
	if it := m.itensDaAba(TabServices); it[0].namespace != "a" {
		t.Errorf("no empate do nome, o namespace: %s", it[0].namespace)
	}
	if got := strings.Join(nomes(m, TabIngresses), " ") + " " + strings.Join(nomes(m, TabConfigMaps), " "); got != "a z b m" {
		t.Errorf("ingresses e configmaps: %s", got)
	}
}

// TestCtrlC_SaiDeQualquerLugar: campo, menu, confirmação, ajuda, painel, quadro e lista.
func TestCtrlC_SaiDeQualquerLugar(t *testing.T) {
	base := telaDaPrancha(t, 140, 32)
	niveis := map[string]Model{"lista": base}
	niveis["campo"], _ = teclas(t, base, "/")
	niveis["menu"], _ = teclas(t, base, "x")
	niveis["confirmação"], _ = teclas(t, base, "x", "d")
	niveis["ajuda"], _ = teclas(t, base, "?")
	niveis["quadro"], _ = teclas(t, base, "y")
	niveis["painel"], _ = teclas(t, telaDaPrancha(t, 100, 30), "enter")
	for nome, m := range niveis {
		_, cmd := atualizar(m, tecla("ctrl+c"))
		if cmd == nil {
			t.Errorf("%s: ctrl+c não saiu", nome)
			continue
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s: ctrl+c devolveu %T", nome, cmd())
		}
	}
}

// TestAjuda_InterrogacaoMostraTodasAsTeclas: a legenda da lista é a da
// prancha e corta com "? mais" na tela estreita; o ? mostra todas as teclas
// da aba, agrupadas como na folha Atalhos, e só as que valem nela.
func TestAjuda_InterrogacaoMostraTodasAsTeclas(t *testing.T) {
	usarModo(t, modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor})
	m := telaDaPrancha(t, 100, 30)
	m, _ = teclas(t, m, "2")
	linhas := strings.Split(ansi.Strip(m.View()), "\n")
	legenda := strings.Join(strings.Fields(linhas[len(linhas)-1]), " ")
	if legenda != "↑↓ mover ↵ abrir / buscar f filtrar x ações n namespace ? ajuda q sair" {
		t.Errorf("legenda dos Deployments: %q", legenda)
	}
	m.width = 72
	if !strings.Contains(ansi.Strip(m.View()), "? mais") {
		t.Error("a 72 colunas a legenda corta e oferece ? mais")
	}
	m.width = 100
	m, _ = teclas(t, m, "?")
	tela := ansi.Strip(m.View())
	for _, quer := range []string{"TECLAS · DEPLOYMENTS", "EM TODA TUI", "LISTA · DEPLOYMENTS", "r reiniciar · s escalar · b perguntar ao Bard · c copiar nome · d excluir",
		"filtrar por label", "trocar de aba", "ctrl+c", "sai (só no nível de cima)", "esc fechar", "ctrl+c sair"} {
		if !strings.Contains(tela, quer) {
			t.Errorf("a ajuda deveria ter %q:\n%s", quer, tela)
		}
	}
	for _, nao := range []string{"status e label", "logs", "q sair"} {
		if strings.Contains(tela, nao) {
			t.Errorf("a ajuda dos Deployments não deveria ter %q:\n%s", nao, tela)
		}
	}
	if m, _ = teclas(t, m, "esc"); m.ajuda {
		t.Fatal("esc fecha a ajuda")
	}
	if m, _ = teclas(t, m, "?", "?"); m.ajuda {
		t.Fatal("? fecha a ajuda")
	}
}

// TestFeedback_CanceladaENomesEmPortugues: cancelar não faz nada nem aparece
// como sucesso, a falha sai com ✕, e a recusa de excluir usa o nome do
// recurso em PT-BR.
func TestFeedback_CanceladaENomesEmPortugues(t *testing.T) {
	m := telaDaPrancha(t, 140, 32)
	m, cmd := teclas(t, m, "x", "d", "w", "esc")
	if cmd != nil || m.acao.etapa != noMenu || m.toast.Visivel() {
		t.Fatalf("esc na exclusão volta ao menu sem fazer nada: etapa %d, comando %v", m.acao.etapa, cmd != nil)
	}
	m, _ = teclas(t, m, "esc")
	m, _ = atualizar(m, resultadoAcaoMsg{texto: "não deu para excluir Pod x: proibido", falha: true})
	if fr := ansi.Strip(m.mensagem(nil)); fr != yui.G().Falha+" não deu para excluir Pod x: proibido" {
		t.Errorf("falha: %q", fr)
	}
	m, _ = teclas(t, m, "5", "d")
	if fr := ansi.Strip(m.actionFeedback); fr != yui.G().Alerta+" não dá para excluir nós" {
		t.Errorf("recusa nos nós: %q", fr)
	}
	if got := nomeDoTipo("statefulset"); got != "statefulset" || nomeDoTipo("node") != "nó" {
		t.Errorf("nomeDoTipo: %q %q", got, nomeDoTipo("node"))
	}
}

// TestLabel_InvalidoNaoAplica: um seletor que não é válido não vira filtro.
func TestLabel_InvalidoNaoAplica(t *testing.T) {
	m := telaDaPrancha(t, 140, 32)
	m, _ = teclas(t, m, "f", "a", "p", "p", "=", "=", "=", "enter")
	if m.filter.LabelSelector != "" || !strings.Contains(ansi.Strip(m.actionFeedback), "label inválido") {
		t.Fatalf("label inválido: %q, %q", m.filter.LabelSelector, ansi.Strip(m.actionFeedback))
	}
	if w := lipgloss.Width(m.actionFeedback); w == 0 {
		t.Fatal("o aviso deveria aparecer na linha de estado")
	}
}
