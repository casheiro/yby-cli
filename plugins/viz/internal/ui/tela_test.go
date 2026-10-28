package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	yui "github.com/casheiro/yby-cli/v5/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"
)

// Telas da prancha com os dados de exemplo dela, em tamanho fixo. Para regravar:
// cd plugins/viz && go test -run TestTela ./internal/ui/ -update

// modo é uma variante visual dos goldens.
type modo struct {
	nome   string
	tema   yui.Tema
	ascii  bool
	perfil termenv.Profile
}

// usarModo configura tema, glifos e perfil de cor e restaura tudo no fim do teste.
func usarModo(t *testing.T, md modo) {
	t.Helper()
	escuroAnterior := lipgloss.HasDarkBackground()
	perfilAnterior := lipgloss.ColorProfile()
	opcoesAnteriores := yui.Atual()
	t.Cleanup(func() {
		yui.Configurar(opcoesAnteriores)
		lipgloss.SetHasDarkBackground(escuroAnterior)
		lipgloss.SetColorProfile(perfilAnterior)
	})
	yui.Configurar(yui.Opcoes{Tema: md.tema, ASCII: md.ascii})
	lipgloss.SetColorProfile(md.perfil)
}

// horaDaPrancha é o "atualizado 12:04:31" da folha VizPods.
var horaDaPrancha = time.Date(2026, 10, 25, 12, 4, 31, 0, time.Local)

// clienteStaging é o MockClient no contexto "staging" do kubeconfig.
type clienteStaging struct{ *MockClient }

func (clienteStaging) Contexto() string { return "staging" }

// podDaPrancha monta um pod da folha VizPods; idade e último reinício contam
// para trás a partir da hora da prancha.
func podDaPrancha(nome, status string, prontos, reinicios int32, idade, ultimoReinicio time.Duration, no, ip string) monitor.Pod {
	p := monitor.Pod{
		Name: nome, Namespace: "payments", Status: status, Prontos: prontos, Containers: 1, Reinicios: reinicios,
		CriadoEm: horaDaPrancha.Add(-idade), No: no, IP: ip,
		Labels: map[string]string{"app": strings.SplitN(nome, "-", 2)[0]},
	}
	if ultimoReinicio > 0 {
		p.UltimoReinicio = horaDaPrancha.Add(-ultimoReinicio)
	}
	return p
}

const dia = 24 * time.Hour

// podsDaPrancha vêm na ordem da API (por nome, como a lista do kubectl); a
// tela os põe em "problemas primeiro".
func podsDaPrancha() []monitor.Pod {
	return []monitor.Pod{
		podDaPrancha("api-7f9c4d-x2k2p", "Running", 1, 0, 3*time.Hour, 0, "ip-10-0-1-5", "10.42.0.14"),
		podDaPrancha("api-7f9c4d-m81zq", "Running", 1, 0, 3*time.Hour, 0, "ip-10-0-3-9", "10.42.2.31"),
		podDaPrancha("billing-6c44f-2kq9d", "Running", 1, 1, dia, 20*time.Hour, "ip-10-0-2-17", "10.42.1.8"),
		podDaPrancha("migrate-28311-hx7", "Completed", 0, 0, 2*time.Hour, 0, "ip-10-0-1-5", ""),
		podDaPrancha("redis-0", "Running", 1, 0, 6*dia, 0, "ip-10-0-3-9", "10.42.2.5"),
		podDaPrancha("web-8b77c-5jw4f", "Pending", 0, 0, 4*time.Minute, 0, "", ""),
		podDaPrancha("web-8b77c-9pd2s", "Running", 1, 0, 4*time.Minute, 0, "ip-10-0-2-17", "10.42.1.40"),
		workerDaPrancha(),
	}
}

// workerDaPrancha é o pod do painel da folha VizPods, com os campos do detalhe.
func workerDaPrancha() monitor.Pod {
	p := podDaPrancha("worker-5d8b6-q7tvn", "CrashLoopBackOff", 0, 14, 52*time.Minute, 2*time.Minute, "ip-10-0-2-17", "10.42.1.23")
	p.Motivo, p.Imagens, p.Dono = "Error · código de saída 1", []string{"worker:1.8.2"}, "Deployment worker"
	p.Recursos = monitor.Recursos{CPUPedido: "120m", CPULimite: "500m", MemPedido: "210Mi"}
	return p
}

// telaDaPrancha monta o Viz da folha VizPods: staging ▸ payments, as outras
// abas já lidas (Deployments 4, StatefulSets 1, Services 5, Nodes 3) e os
// pods por último, na aba ativa.
func telaDaPrancha(t *testing.T, largura, altura int) Model {
	t.Helper()
	return telaDaPranchaCom(t, clienteStaging{&MockClient{}}, largura, altura)
}

// telaDaPranchaCom é a tela da prancha com o client dado.
func telaDaPranchaCom(t *testing.T, client monitor.Client, largura, altura int) Model {
	t.Helper()
	m := NewModel(client)
	m.agora = func() time.Time { return horaDaPrancha }
	m.filter.Namespace = "payments"
	ns := "payments"
	msgs := []tea.Msg{
		tea.WindowSizeMsg{Width: largura, Height: altura},
		[]monitor.Deployment{
			{Name: "api", Namespace: ns, Replicas: 3, Ready: 3, Available: 3, Atualizados: 3, CriadoEm: horaDaPrancha.Add(-30 * dia)},
			{Name: "billing", Namespace: ns, Replicas: 1, Ready: 1, Available: 1, Atualizados: 1, CriadoEm: horaDaPrancha.Add(-30 * dia)},
			{Name: "web", Namespace: ns, Replicas: 2, Ready: 1, Available: 1, Atualizados: 2, CriadoEm: horaDaPrancha.Add(-4 * time.Minute)},
			{Name: "worker", Namespace: ns, Replicas: 2, Ready: 0, Available: 0, Atualizados: 2, CriadoEm: horaDaPrancha.Add(-12 * dia)},
		},
		[]monitor.StatefulSet{{Name: "redis", Namespace: ns, Replicas: 1, Ready: 1, Atualizados: 1, CriadoEm: horaDaPrancha.Add(-6 * dia)}},
		[]monitor.Service{
			{Name: "api", Namespace: ns, Type: "ClusterIP"}, {Name: "billing", Namespace: ns, Type: "ClusterIP"},
			{Name: "redis", Namespace: ns, Type: "ClusterIP"}, {Name: "web", Namespace: ns, Type: "LoadBalancer"},
			{Name: "worker", Namespace: ns, Type: "ClusterIP"},
		},
		[]monitor.Node{{Name: "ip-10-0-1-5", Status: "Ready"}, {Name: "ip-10-0-2-17", Status: "Ready"}, {Name: "ip-10-0-3-9", Status: "Ready"}},
		podsDaPrancha(),
	}
	for _, msg := range msgs {
		nm, _ := m.Update(msg)
		m = nm.(Model)
	}
	return m
}

// conferirTela confere as dimensões da tela e compara com o golden.
func conferirTela(t *testing.T, m Model, largura, altura int) string {
	t.Helper()
	tela := m.View()
	linhas := strings.Split(tela, "\n")
	if len(linhas) != altura {
		t.Errorf("altura: %d linhas, esperado %d", len(linhas), altura)
	}
	for i, l := range linhas {
		if w := lipgloss.Width(l); w != largura {
			t.Errorf("linha %d com %d células: %q", i+1, w, l)
		}
	}
	golden.RequireEqual(t, []byte(tela))
	return tela
}

// TestTelaPods reproduz a lista da folha "pods com detalhe" (o painel com os
// eventos e o log está em TestTelaPodsComDetalhe; aqui o client não os lê): masthead com staging ▸ payments e o indicador ao vivo,
// abas com contagem, ordem e resumo, problemas primeiro com o concluído por
// último, a frase do pod com problema e a hora da última leitura.
func TestTelaPods(t *testing.T) {
	escuro := modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor}
	ascii := modo{"ascii", yui.TemaEscuro, true, termenv.TrueColor}
	comum := []string{"VIZ", "staging", "payments", "worker-5d8b6-q7tvn", "atualizado 12:04:31"}
	inteira := append([]string{"8 pods · 2 com problema", "migrate-28311-hx7"}, comum...)
	casos := []struct {
		md              modo
		largura, altura int
		quer            []string
	}{
		{escuro, 100, 30, inteira},
		{escuro, 140, 32, inteira},
		{escuro, 80, 24, inteira}, // a menor largura com as abas completas
		{escuro, 72, 24, inteira}, // compacto: abas numeradas, colunas menos importantes somem
		// O mínimo: somem NÓ, IP e o resumo, e a última linha avisa os pods que ficaram fora.
		{escuro, 60, 16, append([]string{"↓ 2 pods abaixo"}, comum...)},
		{ascii, 100, 30, append([]string{"up/down mover"}, inteira...)},
	}
	for _, c := range casos {
		t.Run(fmt.Sprintf("%s_%dx%d", c.md.nome, c.largura, c.altura), func(t *testing.T) {
			usarModo(t, c.md)
			tela := ansi.Strip(conferirTela(t, telaDaPrancha(t, c.largura, c.altura), c.largura, c.altura))
			for _, quer := range c.quer {
				if !strings.Contains(tela, quer) {
					t.Errorf("a tela deveria mostrar %q", quer)
				}
			}
		})
	}

	// Abaixo de 60x16, a moldura pede uma janela maior.
	t.Run("escuro_59x16", func(t *testing.T) {
		usarModo(t, escuro)
		if tela := ansi.Strip(conferirTela(t, telaDaPrancha(t, 59, 16), 59, 16)); !strings.Contains(tela, "60x16") {
			t.Errorf("a tela deveria pedir o mínimo de 60x16:\n%s", tela)
		}
	})

	// Com todos os namespaces, a coluna NAMESPACE entra por último na tabela.
	t.Run("escuro_todos_120x30", func(t *testing.T) {
		usarModo(t, escuro)
		m := telaDaPrancha(t, 120, 30)
		m.filter.Namespace = ""
		tela := ansi.Strip(conferirTela(t, m, 120, 30))
		if !strings.Contains(tela, "todos os namespaces") || !strings.Contains(tela, "NAMESPACE") {
			t.Errorf("a tela deveria mostrar todos os namespaces e a coluna NAMESPACE:\n%s", tela)
		}
	})
}

// nomes devolve os nomes da lista visível da aba.
func nomes(m Model, tab ResourceTab) []string {
	var n []string
	for _, it := range m.itensDaAba(tab) {
		n = append(n, it.nome)
	}
	return n
}

// TestOrdem_ProblemasPrimeiro: falhas, depois alertas, depois os saudáveis na
// ordem da API e os concluídos por último, em tinta suave.
func TestOrdem_ProblemasPrimeiro(t *testing.T) {
	m := telaDaPrancha(t, 140, 32)
	quer := []string{
		"worker-5d8b6-q7tvn", "web-8b77c-5jw4f", "api-7f9c4d-x2k2p", "api-7f9c4d-m81zq",
		"billing-6c44f-2kq9d", "redis-0", "web-8b77c-9pd2s", "migrate-28311-hx7",
	}
	if got := nomes(m, TabPods); strings.Join(got, " ") != strings.Join(quer, " ") {
		t.Fatalf("ordem dos pods:\n%v\nesperado:\n%v", got, quer)
	}
	_, linhas := m.tabelaDaAba(TabPods, m.itensDaAba(TabPods))
	for i, l := range linhas {
		if l.Concluida != (i == len(linhas)-1) {
			t.Errorf("linha %d (%s): Concluida = %v", i, quer[i], l.Concluida)
		}
	}

	// Deployments: nenhuma réplica pronta (falha) antes de parte delas (alerta).
	if got := strings.Join(nomes(m, TabDeployments), " "); got != "worker web api billing" {
		t.Errorf("ordem dos deployments: %s", got)
	}

	// Reinício na última hora é problema mesmo com o pod Running e pronto;
	// reinício antigo não é.
	m.pods = []monitor.Pod{
		podDaPrancha("a-antigo", "Running", 1, 3, dia, 5*time.Hour, "", ""),
		podDaPrancha("b-recente", "Running", 1, 3, dia, 10*time.Minute, "", ""),
		podDaPrancha("c-meio-pronto", "Running", 0, 0, dia, 0, "", ""),
	}
	if got := strings.Join(nomes(m, TabPods), " "); got != "b-recente c-meio-pronto a-antigo" {
		t.Errorf("reinício recente e contêiner fora do ar primeiro: %s", got)
	}
	if fr := ansi.Strip(m.mensagem(m.itensDaAba(TabPods))); fr != yui.G().Alerta+" b-recente reiniciou 3 vezes, a última há 10m" {
		t.Errorf("frase da linha de estado: %q", fr)
	}

	// Os eventos ficam na ordem do tempo, com ou sem Warning.
	m.events = []monitor.Event{{Name: "x", Type: "Normal"}, {Name: "y", Type: "Warning"}, {Name: "z", Type: "Normal"}}
	if got := strings.Join(nomes(m, TabEvents), " "); got != "x y z" {
		t.Errorf("eventos fora da ordem do tempo: %s", got)
	}
}

// TestSelecao_AcompanhaORecursoQuandoAOrdemMuda: quando um pod muda de estado
// e de lugar na lista, a seleção vai junto com ele.
func TestSelecao_AcompanhaORecursoQuandoAOrdemMuda(t *testing.T) {
	m := telaDaPrancha(t, 140, 32)
	m, _ = teclas(t, m, "j", "j", "j") // api-7f9c4d-m81zq
	if nome, _, _ := m.selectedResourceInfo(); nome != "api-7f9c4d-m81zq" {
		t.Fatalf("selecionado = %s", nome)
	}
	pods := podsDaPrancha()
	pods[1].Status, pods[1].Prontos = "CrashLoopBackOff", 0
	nm, _ := m.Update(pods)
	m = nm.(Model)
	if nome, _, _ := m.selectedResourceInfo(); nome != "api-7f9c4d-m81zq" || m.selectedIndex != 0 {
		t.Fatalf("a seleção ficou em %s (linha %d), esperado api-7f9c4d-m81zq na linha 0 (falha, antes do worker pela ordem da API)", nome, m.selectedIndex)
	}

	// Se o recurso some da lista, a seleção fica dentro dela.
	m, _ = teclas(t, m, "j", "j", "j", "j", "j", "j") // último
	nm, _ = m.Update(podsDaPrancha()[:2])
	m = nm.(Model)
	if m.selectedIndex != 1 {
		t.Fatalf("seleção fora da lista: %d", m.selectedIndex)
	}
}

// TestFiltros_ChipsComNDeM: cada filtro vira um chip e a linha mostra "N de M";
// sem filtro, o resumo da lista. O namespace fica no masthead, não nos chips.
func TestFiltros_ChipsComNDeM(t *testing.T) {
	usarModo(t, modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor})
	m := telaDaPrancha(t, 140, 32)
	linha := func(m Model) string { return ansi.Strip(m.linhaFiltros(m.itensDaAba(m.activeTab))) }

	sem := linha(m)
	for _, quer := range []string{"ordem problemas primeiro", "filtros  nenhum", "/ buscar", "8 pods · 2 com problema"} {
		if !strings.Contains(sem, quer) {
			t.Errorf("sem filtro, a linha deveria ter %q: %q", quer, sem)
		}
	}

	m.filter.StatusFilter = "Running"
	m = buscar(t, m, "api")
	com := linha(m)
	for _, quer := range []string{"[status: Running ✕]", "[busca: api ✕]", "2 de 8"} {
		if !strings.Contains(com, quer) {
			t.Errorf("com filtros, a linha deveria ter %q: %q", quer, com)
		}
	}
	if strings.Contains(com, "payments") {
		t.Errorf("o namespace não é chip: %q", com)
	}
	if w := lipgloss.Width(m.linhaFiltros(m.itensDaAba(m.activeTab))); w > 140 {
		t.Errorf("linha de filtros com %d células", w)
	}

	// O label é filtro da tela: o M é a lista inteira do namespace.
	m.filter = FilterState{Namespace: "payments", LabelSelector: "app=api"}
	if l := linha(m); !strings.Contains(l, "[app=api ✕]") || !strings.Contains(l, "2 de 8") {
		t.Errorf("label vira chip com a chave inteira e conta sobre o total: %q", l)
	}
	if got := strings.Join(nomes(m, TabPods), " "); got != "api-7f9c4d-x2k2p api-7f9c4d-m81zq" {
		t.Errorf("pods com app=api: %s", got)
	}
}

// TestAbas_ContagemEOrdem: as abas seguem a ordem da prancha, a ativa tem ◆ e
// a contagem da lista visível, e a aba ainda não lida fica sem número.
func TestAbas_ContagemEOrdem(t *testing.T) {
	usarModo(t, modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor})
	m := telaDaPrancha(t, 220, 32)
	normais, _ := m.linhaAbas()
	abasTxt := ansi.Strip(normais)
	quer := "◆ 1 Pods 8   ◇ 2 Deployments 4   ◇ 3 StatefulSets 1   ◇ 4 Services 5   ◇ 5 Nodes 3   ◇ 6 Jobs   ◇ 7 Ingresses   ◇ 8 ConfigMaps   ◇ 9 Eventos"
	if abasTxt != quer {
		t.Fatalf("abas:\n%q\nesperado:\n%q", abasTxt, quer)
	}

	// A contagem é a da lista visível: com a busca, a aba conta o que passa.
	m = buscar(t, m, "web")
	normais, _ = m.linhaAbas()
	if !strings.Contains(ansi.Strip(normais), "◆ 1 Pods 2") || !strings.Contains(ansi.Strip(normais), "◇ 2 Deployments 1") {
		t.Errorf("contagem com busca: %q", ansi.Strip(normais))
	}

	// Abaixo de 80 colunas, as abas compactas numeradas.
	m.width = 72
	_, compactas := m.linhaAbas()
	if c := ansi.Strip(compactas); !strings.HasPrefix(c, "1 Pods · 2 Depl") {
		t.Errorf("abas compactas: %q", c)
	}
}

// TestMasthead_SemContextoMostraSoONamespace: sem contexto no kubeconfig, o
// masthead mostra só o namespace; sem namespace escolhido, todos.
func TestMasthead_SemContextoMostraSoONamespace(t *testing.T) {
	usarModo(t, modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor})
	m := comJanela(NewModel(&MockClient{}))
	m.filter.Namespace = "payments"
	if c := ansi.Strip(m.contexto()); c != "payments   ● ao vivo · 2 s" {
		t.Errorf("contexto sem cluster: %q", c)
	}
	m.filter.Namespace = ""
	if c := ansi.Strip(m.contexto()); !strings.HasPrefix(c, "todos os namespaces") {
		t.Errorf("contexto sem namespace: %q", c)
	}
	m.reconnecting = true
	if c := ansi.Strip(m.contexto()); strings.Contains(c, "ao vivo") {
		t.Errorf("reconectando não está ao vivo: %q", c)
	}
}

// TestLinhaDeEstado_Prioridade: a ação em curso e o resultado da última vêm
// antes do problema da lista; a reconexão também.
func TestLinhaDeEstado_Prioridade(t *testing.T) {
	m := telaDaPrancha(t, 140, 32)
	itens := m.itensDaAba(TabPods)
	if fr := ansi.Strip(m.mensagem(itens)); fr != yui.G().Falha+" worker-5d8b6-q7tvn em CrashLoopBackOff, reiniciou 14 vezes" {
		t.Errorf("problema da lista: %q", fr)
	}
	m.reconnecting = true
	if fr := ansi.Strip(m.mensagem(itens)); !strings.Contains(fr, "reconectando") {
		t.Errorf("reconexão: %q", fr)
	}
	m.reconnecting = false
	nm, _ := m.Update(resultadoAcaoMsg{texto: "Deployment api reiniciado"})
	m = nm.(Model)
	if fr := ansi.Strip(m.mensagem(itens)); fr != yui.G().Feito+" Deployment api reiniciado" {
		t.Errorf("resultado da ação: %q", fr)
	}
	// A confirmação fica no corpo; o Toast continua na linha de estado (folha
	// VizConfirmar) até o tempo dele acabar.
	m, _ = teclas(t, m, "x", "d")
	if v := ansi.Strip(m.View()); !strings.Contains(v, "EXCLUIR POD") || !strings.Contains(ansi.Strip(m.mensagem(itens)), "Deployment api reiniciado") {
		t.Errorf("confirmação da ação:\n%s", v)
	}
	m, _ = atualizar(m, yui.ToastSomeMsg{ID: 1})
	if fr := ansi.Strip(m.mensagem(itens)); strings.Contains(fr, "reiniciado") {
		t.Errorf("o Toast deveria sumir no fim do tempo: %q", fr)
	}
}

// TestTelaAjuda: a tela ? com as teclas da aba, em grupos lado a lado quando
// cabem e um embaixo do outro (com rolagem) quando não; e o campo do f aberto,
// com a regra dele na linha de estado.
func TestTelaAjuda(t *testing.T) {
	escuro := modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor}
	for _, tam := range [][2]int{{140, 32}, {72, 24}} {
		t.Run(fmt.Sprintf("ajuda_%dx%d", tam[0], tam[1]), func(t *testing.T) {
			usarModo(t, escuro)
			m, _ := teclas(t, telaDaPrancha(t, tam[0], tam[1]), "?")
			conferirTela(t, m, tam[0], tam[1])
		})
	}
	t.Run("filtro_100x30", func(t *testing.T) {
		usarModo(t, escuro)
		m, _ := teclas(t, telaDaPrancha(t, 100, 30), "f", "a", "p", "p", "=")
		conferirTela(t, m, 100, 30)
	})
	// Abaixo de 80 colunas, a legenda das camadas em que o ? é letra (campo,
	// confirmação) não oferece "? mais"; a do menu, em que o ? abre a ajuda, sim.
	for nome, ks := range map[string][]string{
		"filtro_72x24": {"f", "a", "p", "p", "="}, "confirmacao_72x24": {"x", "d"}, "menu_72x24": {"x"},
	} {
		t.Run(nome, func(t *testing.T) {
			usarModo(t, escuro)
			m, _ := teclas(t, telaDaPrancha(t, 72, 24), ks...)
			tela := strings.Split(ansi.Strip(conferirTela(t, m, 72, 24)), "\n")
			legenda := tela[len(tela)-1]
			if quer := nome == "menu_72x24"; strings.Contains(legenda, "? mais") != quer {
				t.Errorf("? mais na legenda deveria ser %v: %q", quer, legenda)
			}
		})
	}
}
