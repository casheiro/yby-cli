package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
)

// Mapa de teclas do Viz (folha Atalhos) num programa de verdade: as teclas
// novas, os níveis de esc, q e ctrl+c, os avisos das teclas antigas e o mouse.

const larguraTeclado, alturaTeclado = 140, 32

// semCor deixa a saída do programa sem cor (o texto sai inteiro, sem códigos
// no meio), com os glifos Unicode.
func semCor(t *testing.T) {
	usarModo(t, modo{"semcor", yui.TemaEscuro, false, termenv.Ascii})
}

// clienteDaPrancha responde às leituras com os dados da folha VizPods, para
// a leitura do Init não apagar a tela montada.
func clienteDaPrancha() clienteStaging {
	ns := "payments"
	return clienteStaging{&MockClient{
		Pods: podsDaPrancha(),
		Deployments: []monitor.Deployment{
			{Name: "api", Namespace: ns, Replicas: 3, Ready: 3, Available: 3},
			{Name: "worker", Namespace: ns, Replicas: 2, Ready: 0},
		},
		Nodes: []monitor.Node{{Name: "ip-10-0-1-5", Status: "Ready"}},
	}}
}

// vizDeTeste é a tela da prancha com o client que responde igual e sem o
// relógio (as leituras só acontecem quando uma tecla pede).
func vizDeTeste(t *testing.T) Model {
	t.Helper()
	m := telaDaPrancha(t, larguraTeclado, alturaTeclado)
	m.client = clienteDaPrancha()
	m.tique = nil
	return m
}

// abrirViz roda o modelo num programa de teste do tamanho da tela.
func abrirViz(t *testing.T, m Model) *teatest.TestModel {
	t.Helper()
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(larguraTeclado, alturaTeclado))
}

// fim espera o programa terminar (sem mandar nada) e devolve o modelo final.
func fim(t *testing.T, tm *teatest.TestModel) Model {
	t.Helper()
	return tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(Model)
}

// fecharViz sai com ctrl+c e devolve o modelo final.
func fecharViz(t *testing.T, tm *teatest.TestModel) Model {
	t.Helper()
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	return fim(t, tm)
}

// esperarTela espera os trechos aparecerem na saída do programa. A saída lida
// é consumida: trechos do mesmo quadro vão na mesma espera.
func esperarTela(t *testing.T, tm *teatest.TestModel, trechos ...string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		for _, trecho := range trechos {
			if !bytes.Contains(b, []byte(trecho)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(10*time.Millisecond))
}

// enviar manda as teclas ao programa: nomes especiais ou um texto, uma tecla
// por caractere.
func enviar(tm *teatest.TestModel, ks ...string) {
	for _, k := range ks {
		switch k {
		case "enter", "esc", "tab", "shift+tab", "pgdown", "pgup", "home", "end", "backspace":
			tm.Send(tecla(k))
		default:
			for _, r := range k {
				tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			}
		}
	}
}

func TestTeclado_BuscaFiltraEnquantoDigita(t *testing.T) {
	semCor(t)
	tm := abrirViz(t, vizDeTeste(t))
	esperarTela(t, tm, "8 pods · 2 com problema")
	enviar(tm, "/", "api")
	esperarTela(t, tm, "[busca: api ✕]", "2 de 8")
	// esc cancela e volta ao valor de antes (nenhum).
	enviar(tm, "esc", "/", "web", "enter")
	esperarTela(t, tm, "[busca: web ✕]")
	// A busca reabre com o termo atual; o esc devolve o termo confirmado.
	enviar(tm, "/", "-9", "esc")
	m := fecharViz(t, tm)
	if m.filter.SearchQuery != "web" || m.filter.InputMode {
		t.Fatalf("busca: %+v", m.filter)
	}
	if got := strings.Join(nomes(m, TabPods), " "); got != "web-8b77c-5jw4f web-8b77c-9pd2s" {
		t.Fatalf("lista com a busca: %s", got)
	}
}

func TestTeclado_NamespaceEFiltro(t *testing.T) {
	semCor(t)
	tm := abrirViz(t, vizDeTeste(t))
	esperarTela(t, tm, "8 pods")
	enviar(tm, "f", "Running", "enter")
	esperarTela(t, tm, "[status: Running ✕]")
	// Com = ou vírgula é label, e troca o status.
	enviar(tm, "f")
	esperarTela(t, tm, "status (Running) ou label")
	enviar(tm, "app=api", "enter")
	esperarTela(t, tm, "[app=api ✕]")
	enviar(tm, "n", "outro", "enter")
	m := fecharViz(t, tm)
	if m.filter.LabelSelector != "app=api" || m.filter.StatusFilter != "" || m.filter.Namespace != "outro" {
		t.Fatalf("filtros: %+v", m.filter)
	}
}

func TestTeclado_RecarregaNaHora(t *testing.T) {
	semCor(t)
	tm := abrirViz(t, vizDeTeste(t))
	esperarTela(t, tm, "8 pods")
	enviar(tm, "2", "r")
	esperarTela(t, tm, "deployments recarregados · reiniciar fica em x ▸ r")
	if m := fecharViz(t, tm); len(m.deployments) != 2 || m.recarregando {
		t.Fatalf("depois do r: %d deployments, recarregando %v", len(m.deployments), m.recarregando)
	}
}

func TestTeclado_AcoesEmX(t *testing.T) {
	semCor(t)
	tm := abrirViz(t, vizDeTeste(t))
	esperarTela(t, tm, "8 pods")
	enviar(tm, "2", "x")
	esperarTela(t, tm, "ações ▸ deployment worker", "r reiniciar  s escalar…  d excluir…  esc voltar")
	enviar(tm, "s")
	esperarTela(t, tm, "escalar deployment worker para quantas réplicas?")
	// esc sobe um nível: da confirmação para o menu, do menu para a lista.
	enviar(tm, "esc")
	esperarTela(t, tm, "ações ▸ deployment worker")
	enviar(tm, "esc")
	m := fecharViz(t, tm)
	if m.actionMode != ActionNone || m.actionTarget != "" {
		t.Fatalf("o esc deveria fechar o menu: modo %d, alvo %q", m.actionMode, m.actionTarget)
	}

	// Nós e eventos não têm ações.
	tm = abrirViz(t, vizDeTeste(t))
	enviar(tm, "5", "x")
	esperarTela(t, tm, "nós não têm ações")
	fecharViz(t, tm)
}

func TestTeclado_TelaDeAjuda(t *testing.T) {
	semCor(t)
	tm := abrirViz(t, vizDeTeste(t))
	esperarTela(t, tm, "8 pods")
	enviar(tm, "?")
	esperarTela(t, tm, "TECLAS · PODS", "filtrar por status e label")
	enviar(tm, "?")
	m := fecharViz(t, tm)
	if m.ajuda {
		t.Fatal("o ? fecha a ajuda")
	}
}

// TestTeclado_Niveis: esc fecha a camada de cima e, sem camada, limpa os
// filtros; q é letra no campo e não sai das camadas; q sai da lista; ctrl+c
// sai de qualquer lugar.
func TestTeclado_Niveis(t *testing.T) {
	semCor(t)

	t.Run("q no campo é letra", func(t *testing.T) {
		tm := abrirViz(t, vizDeTeste(t))
		enviar(tm, "/", "q", "x")
		if m := fecharViz(t, tm); m.filter.InputBuffer != "qx" || !m.filter.InputMode {
			t.Fatalf("o campo deveria ter qx: %+v", m.filter)
		}
	})

	t.Run("q não sai das camadas", func(t *testing.T) {
		tm := abrirViz(t, vizDeTeste(t))
		enviar(tm, "?", "q")
		esperarTela(t, tm, "q sai só da lista; esc volta")
		enviar(tm, "esc", "2", "x", "q", "esc", "y")
		esperarTela(t, tm, "YAML · ")
		enviar(tm, "q", "esc", "/", "z")
		if m := fecharViz(t, tm); m.filter.InputBuffer != "z" || m.ajuda || m.quadro.aberto {
			t.Fatalf("depois das camadas: campo %q, ajuda %v, quadro %v", m.filter.InputBuffer, m.ajuda, m.quadro.aberto)
		}
	})

	t.Run("esc limpa os filtros e q sai da lista", func(t *testing.T) {
		tm := abrirViz(t, vizDeTeste(t))
		enviar(tm, "/", "api", "enter", "f", "Running", "enter")
		esperarTela(t, tm, "esc limpar filtros")
		enviar(tm, "esc", "q")
		m := fim(t, tm) // o q encerrou o programa
		if m.algumFiltro() || m.filter.Namespace != "payments" {
			t.Fatalf("o esc deveria limpar busca e status e manter o namespace: %+v", m.filter)
		}
	})

	// A camada tem de estar aberta (na tela e no modelo) quando o ctrl+c chega.
	for nome, c := range map[string]struct {
		teclas []string
		marca  string
		aberta func(Model) bool
	}{
		"campo":       {[]string{"n"}, "namespace › ", func(m Model) bool { return m.filter.InputMode }},
		"menu":        {[]string{"x"}, "ações ▸ pod", func(m Model) bool { return m.actionMode == ActionMenu }},
		"confirmação": {[]string{"x", "d"}, "digite yes", func(m Model) bool { return m.actionMode == ActionConfirmDelete }},
		"ajuda":       {[]string{"?"}, "TECLAS · PODS", func(m Model) bool { return m.ajuda }},
		"quadro":      {[]string{"l"}, "LOGS · worker-5d8b6-q7tvn", func(m Model) bool { return m.quadro.aberto }},
		"lista":       {nil, "", func(m Model) bool { return !m.ajuda && !m.filter.InputMode && m.actionMode == ActionNone }},
	} {
		t.Run("ctrl+c no "+nome, func(t *testing.T) {
			tm := abrirViz(t, vizDeTeste(t))
			esperarTela(t, tm, "8 pods")
			enviar(tm, c.teclas...)
			if c.marca != "" {
				esperarTela(t, tm, c.marca)
			}
			// Sem o ctrl+c, o FinalModel passaria do tempo.
			if m := fecharViz(t, tm); !c.aberta(m) {
				t.Fatalf("a camada %s não estava aberta no ctrl+c", nome)
			}
		})
	}
}

// TestTeclado_TeclasAntigas: d, s, L e S não agem mais; a linha de estado
// diz onde a ação foi parar.
func TestTeclado_TeclasAntigas(t *testing.T) {
	semCor(t)
	tm := abrirViz(t, vizDeTeste(t))
	esperarTela(t, tm, "8 pods")
	casos := []struct{ teclas, aviso string }{
		{"d", "excluir agora fica em x ▸ d"},
		{"s", "escalar só vale para Deployments e StatefulSets"},
		{"L", "label agora fica em f (app=web)"},
		{"S", "status agora fica em f (Running)"},
		{"2s", "escalar agora fica em x ▸ s"},
		{"5d", "não dá para excluir nós"},
	}
	for _, c := range casos {
		enviar(tm, c.teclas)
		esperarTela(t, tm, c.aviso)
	}
	m := fecharViz(t, tm)
	if m.actionMode != ActionNone || m.filter.InputMode {
		t.Fatalf("as teclas antigas não abrem nada: modo %d, campo %v", m.actionMode, m.filter.InputMode)
	}
}

// TestTeclado_Mouse: a roda rola a lista levando a seleção, o clique numa
// linha a seleciona e o clique numa aba troca de aba.
func TestTeclado_Mouse(t *testing.T) {
	semCor(t)
	m := vizDeTeste(t)
	m.client = clienteStaging{&MockClient{Pods: muitosPods(60)}}
	nm, _ := m.Update(muitosPods(60))
	m = nm.(Model)
	roda := tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress}
	// A roda sozinha: a seleção desce junto com a janela (a primeira linha da tela).
	tm := abrirViz(t, m)
	esperarTela(t, tm, "pods abaixo")
	tm.Send(roda)
	tm.Send(roda)
	esperarTela(t, tm, "6 pods acima")
	if f := fecharViz(t, tm); f.scrollY != 2*passoRoda || f.selectedIndex != f.scrollY {
		t.Fatalf("roda: rolagem %d, seleção %d", f.scrollY, f.selectedIndex)
	}
	// A roda e depois o clique na quinta linha da janela.
	tm = abrirViz(t, m)
	esperarTela(t, tm, "pods abaixo")
	tm.Send(roda)
	tm.Send(roda)
	y := m.primeiraLinhaDeDados() + 4
	tm.Send(tea.MouseMsg{X: 3, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = fecharViz(t, tm)
	if m.scrollY != 2*passoRoda || m.selectedIndex != 2*passoRoda+4 {
		t.Fatalf("roda e clique: rolagem %d, seleção %d", m.scrollY, m.selectedIndex)
	}

	tm = abrirViz(t, vizDeTeste(t))
	esperarTela(t, tm, "8 pods")
	r := regiaoDaAba(t, vizDeTeste(t), TabNodes)
	tm.Send(tea.MouseMsg{X: r.ini + 1, Y: linhaDasAbas, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if m = fecharViz(t, tm); m.activeTab != TabNodes {
		t.Fatalf("o clique na aba Nodes foi para %s", m.activeTab)
	}
}

// regiaoDaAba acha a região da aba na linha desenhada.
func regiaoDaAba(t *testing.T, m Model, aba ResourceTab) regiaoAba {
	t.Helper()
	for _, r := range m.regioesDasAbas() {
		if r.aba == aba {
			return r
		}
	}
	t.Fatalf("a aba %s não está na linha", aba)
	return regiaoAba{}
}

// TestMouse_GeometriaBateComATela: as linhas e as abas que o clique calcula
// são as que a moldura desenhou, em várias larguras.
func TestMouse_GeometriaBateComATela(t *testing.T) {
	for _, md := range []modo{{"escuro", yui.TemaEscuro, false, termenv.TrueColor}, {"ascii", yui.TemaEscuro, true, termenv.Ascii}} {
		for _, tam := range [][2]int{{160, 40}, {140, 32}, {120, 30}, {100, 30}, {80, 24}, {72, 24}, {60, 16}} {
			t.Run(fmt.Sprintf("%s_%dx%d", md.nome, tam[0], tam[1]), func(t *testing.T) {
				usarModo(t, md)
				// O G rola até o fim quando a lista não cabe (60x16): a rolagem do
				// modelo é a que a tela desenha, sem ajuste no teste.
				m, _ := teclas(t, telaDaPrancha(t, tam[0], tam[1]), "G")
				tela := strings.Split(ansi.Strip(m.View()), "\n")
				itens := m.itensDaAba(TabPods)
				for i := 0; i < min(m.linhasDeDados(), len(itens)-m.scrollY); i++ {
					linha := tela[m.primeiraLinhaDeDados()+i]
					if nome := itens[m.scrollY+i].nome; !strings.Contains(linha, nome[:min(10, len(nome))]) {
						t.Errorf("linha %d da tela deveria ser %s: %q", m.primeiraLinhaDeDados()+i, nome, linha)
					}
					nm, _ := m.clique(5, m.primeiraLinhaDeDados()+i)
					if sel := nm.(Model).selectedIndex; sel != m.scrollY+i {
						t.Errorf("clique na linha %d selecionou %d", i, sel)
					}
				}
				// Clicar no cabeçalho ou no aviso de rolagem não muda a seleção.
				for _, y := range []int{m.primeiraLinhaDeDados() - 1, m.primeiraLinhaDeDados() + m.linhasDeDados()} {
					if nm, _ := m.clique(5, y); nm.(Model).selectedIndex != m.selectedIndex {
						t.Errorf("clique fora das linhas (y=%d) mudou a seleção", y)
					}
				}
				abas := []rune(tela[linhaDasAbas])
				regioes := m.regioesDasAbas()
				if len(regioes) == 0 {
					t.Fatal("nenhuma aba clicável")
				}
				for _, r := range regioes {
					trecho := string(abas[r.ini:r.fim])
					if !strings.Contains(trecho, abas2(r.aba)) {
						t.Errorf("região da aba %s é %q", r.aba, trecho)
					}
					nm, cmd := m.clique(r.ini, linhaDasAbas)
					if nm.(Model).activeTab != r.aba || (r.aba != TabPods && cmd == nil) {
						t.Errorf("clique em %q foi para %s", trecho, nm.(Model).activeTab)
					}
				}
			})
		}
	}
}

// abas2 é o começo do título da aba como ele aparece nas abas compactas.
func abas2(t ResourceTab) string {
	r := []rune(abas[t].titulo)
	return fmt.Sprintf("%d %s", int(t)+1, string(r[:min(3, len(r))]))
}

// TestFiltro_RegraDoCampo: com =, !, vírgula ou parênteses é label; senão,
// status; vazio limpa os dois; fora dos pods, o status avisa que não filtra.
func TestFiltro_RegraDoCampo(t *testing.T) {
	for valor, label := range map[string]bool{
		"app=web": true, "tier!=db": true, "a=1,b=2": true, "env in (a,b)": true, "!canary": true, "Running": false, "CrashLoopBackOff": false,
	} {
		if pareceLabel(valor) != label {
			t.Errorf("pareceLabel(%q) = %v", valor, !label)
		}
	}
	m := telaDaPrancha(t, 140, 32)
	m.filter.LabelSelector, m.filter.StatusFilter = "app=api", ""
	m, _ = teclas(t, m, "f", "enter")
	if m.algumFiltro() {
		t.Fatalf("f vazio limpa label e status: %+v", m.filter)
	}
	m, _ = teclas(t, m, "2", "f", "R", "u", "n", "enter")
	if !strings.Contains(ansi.Strip(m.actionFeedback), "o status filtra só a aba Pods") {
		t.Fatalf("status fora dos pods: %q", ansi.Strip(m.actionFeedback))
	}
	// Acentos no campo: o apagar tira a letra inteira.
	m, _ = teclas(t, m, "/", "ã", "backspace")
	if m.filter.InputBuffer != "" {
		t.Fatalf("apagar deveria tirar o ã inteiro: %q", m.filter.InputBuffer)
	}
}

// TestRecarga_FalhaAvisa: o r que não consegue ler diz isso na linha de estado.
func TestRecarga_FalhaAvisa(t *testing.T) {
	m := telaDaPrancha(t, 140, 32)
	m.client = clienteStaging{&MockClient{Err: fmt.Errorf("connection refused")}}
	m, cmd := teclas(t, m, "r")
	if !m.recarregando || !strings.Contains(ansi.Strip(m.View()), "recarregando pods…") {
		t.Fatal("o r mostra que está recarregando")
	}
	m, _ = atualizar(m, cmd())
	if m.recarregando || !strings.Contains(ansi.Strip(m.actionFeedback), "não deu para recarregar: connection refused") {
		t.Fatalf("falha do r: %q", ansi.Strip(m.actionFeedback))
	}
}

// TestAjuda_PorCimaDoMenuEDoDetalhe: o ? abre a ajuda por cima do menu, do
// quadro e do painel, e o esc volta para a camada de baixo.
func TestAjuda_PorCimaDoMenuEDoDetalhe(t *testing.T) {
	m := telaDaPrancha(t, 140, 32)
	m, _ = teclas(t, m, "2", "x", "?")
	if !m.ajuda || m.actionMode != ActionMenu {
		t.Fatalf("? no menu: ajuda %v, modo %d", m.ajuda, m.actionMode)
	}
	if m, _ = teclas(t, m, "esc"); m.ajuda || m.actionMode != ActionMenu {
		t.Fatalf("esc na ajuda volta ao menu: ajuda %v, modo %d", m.ajuda, m.actionMode)
	}
	m, _ = teclas(t, m, "esc", "y")
	if m, _ = teclas(t, m, "?", "esc"); m.ajuda || !m.quadro.aberto {
		t.Fatalf("esc na ajuda volta ao quadro: ajuda %v, quadro %v", m.ajuda, m.quadro.aberto)
	}
	p, _ := teclas(t, telaDaPrancha(t, 100, 30), "enter", "?", "esc")
	if p.ajuda || !p.painelPorCima() {
		t.Fatalf("esc na ajuda volta ao painel: ajuda %v, painel %v", p.ajuda, p.painelPorCima())
	}
}

// TestTeclado_InicioEFim: G vai ao fim da lista e g volta ao início.
func TestTeclado_InicioEFim(t *testing.T) {
	semCor(t)
	m := vizDeTeste(t)
	m.client = clienteStaging{&MockClient{Pods: muitosPods(60)}}
	m, _ = atualizar(m, muitosPods(60))
	tm := abrirViz(t, m)
	esperarTela(t, tm, "pods abaixo")
	enviar(tm, "G")
	esperarTela(t, tm, "p-59", "pods acima")
	if f := fecharViz(t, tm); f.selectedIndex != 59 || f.scrollY != 60-f.linhasDeDados() {
		t.Fatalf("G: seleção %d, rolagem %d", f.selectedIndex, f.scrollY)
	}
	tm = abrirViz(t, m)
	esperarTela(t, tm, "pods abaixo")
	enviar(tm, "G")
	esperarTela(t, tm, "pods acima")
	enviar(tm, "g")
	esperarTela(t, tm, "p-00")
	if f := fecharViz(t, tm); f.selectedIndex != 0 || f.scrollY != 0 {
		t.Fatalf("g: seleção %d, rolagem %d", f.selectedIndex, f.scrollY)
	}
}

// conferirCliques clica em cada linha da tela que mostra um pod e confere
// que a seleção é o pod escrito nela; devolve quantas linhas clicou.
func conferirCliques(t *testing.T, m Model) int {
	t.Helper()
	clicadas := 0
	for y, linha := range linhasNaTela(m) {
		linha = ansi.Truncate(linha, m.larguraDaLista(), "") // só a lista: o painel ao lado repete o nome
		for _, it := range m.itensDaAba(TabPods) {
			if !strings.Contains(linha, it.nome+" ") {
				continue
			}
			clicadas++
			nm, _ := m.clique(5, y)
			if nome, _, _ := nm.(Model).selectedResourceInfo(); nome != it.nome {
				t.Errorf("o clique na linha %q selecionou %s", linha, nome)
			}
		}
	}
	return clicadas
}

// TestMouse_CliqueDepoisDeMudarAJanela: depois do G, aumentar a janela ou
// receber uma lista menor não deixa a rolagem do modelo fora da que a tela
// desenha; o clique seleciona a linha que está na tela.
func TestMouse_CliqueDepoisDeMudarAJanela(t *testing.T) {
	semCor(t)
	base := NewModel(clienteStaging{&MockClient{}})
	base, _ = atualizar(base, tea.WindowSizeMsg{Width: 120, Height: 24})
	base, _ = atualizar(base, muitosPods(30))
	base, _ = teclas(t, base, "G")
	if base.scrollY == 0 {
		t.Fatal("o G deveria rolar a lista de 30 pods em 120x24")
	}

	maior, _ := atualizar(base, tea.WindowSizeMsg{Width: 120, Height: 50})
	if maior.scrollY != 0 {
		t.Errorf("a janela que cabe a lista inteira deveria voltar a rolagem a 0: %d", maior.scrollY)
	}
	if n := conferirCliques(t, maior); n != 30 {
		t.Errorf("janela maior: %d linhas clicadas, esperado 30", n)
	}

	menor, _ := atualizar(base, muitosPods(20))
	if quer := 20 - menor.linhasDeDados(); menor.scrollY != quer {
		t.Errorf("a lista menor deveria rolar até %d: %d", quer, menor.scrollY)
	}
	if n := conferirCliques(t, menor); n != menor.linhasDeDados() {
		t.Errorf("lista menor: %d linhas clicadas, esperado %d", n, menor.linhasDeDados())
	}
}

// TestTeclado_SemListaNaTela: sem conexão (depois das tentativas) ou abaixo
// de 60x16, as teclas que agem sobre a linha selecionada não valem; q, r e
// (sem conexão) ?, n e as abas continuam.
func TestTeclado_SemListaNaTela(t *testing.T) {
	semConexao := telaDaPrancha(t, 140, 32)
	semConexao.client = clienteStaging{&MockClient{Err: fmt.Errorf("connection refused")}}
	for range maxRetries {
		semConexao, _ = atualizar(semConexao, fmt.Errorf("connection refused"))
	}
	if semConexao.err == nil {
		t.Fatal("depois das tentativas, a tela mostra o erro")
	}
	pequena := telaDaPrancha(t, 59, 16)
	for nome, m := range map[string]Model{"sem conexão": semConexao, "janela pequena": pequena} {
		antes := m.selectedIndex
		for _, ks := range [][]string{{"x"}, {"x", "d"}, {"enter"}, {"l"}, {"y"}, {"e"}, {"/"}, {"f"}, {"down"}, {"G"}} {
			nm, cmd := teclas(t, m, ks...)
			if cmd != nil || nm.actionMode != ActionNone || nm.filter.InputMode || nm.selectedIndex != antes {
				t.Errorf("%s: %v agiu sobre a lista escondida (modo %d, campo %v, seleção %d)",
					nome, ks, nm.actionMode, nm.filter.InputMode, nm.selectedIndex)
			}
		}
		if _, cmd := teclas(t, m, "q"); cmd == nil {
			t.Errorf("%s: q deveria sair", nome)
		}
		if _, cmd := teclas(t, m, "r"); cmd == nil {
			t.Errorf("%s: r deveria recarregar", nome)
		}
	}
	if m, _ := teclas(t, semConexao, "n"); !m.filter.InputMode {
		t.Error("sem conexão, n abre o namespace")
	}
	if m, _ := teclas(t, semConexao, "?"); !m.ajuda {
		t.Error("sem conexão, ? abre a ajuda")
	}
	if m, _ := teclas(t, semConexao, "2"); m.activeTab != TabDeployments {
		t.Error("sem conexão, as abas trocam")
	}
	// O mouse também não age: nem a roda nem o clique.
	roda := tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress}
	for nome, m := range map[string]Model{"sem conexão": semConexao, "janela pequena": pequena} {
		nm, _ := atualizar(m, roda)
		nm, _ = atualizar(nm, tea.MouseMsg{X: 5, Y: m.primeiraLinhaDeDados() + 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		if nm.selectedIndex != m.selectedIndex || nm.scrollY != m.scrollY {
			t.Errorf("%s: o mouse mexeu na lista escondida", nome)
		}
	}
}

// TestRecarga_UmaPorVezENaAbaLida: segurar o r não gera uma rajada de
// leituras; a resposta nomeia a aba que foi lida, mesmo depois de trocar de
// aba, e a falha dela não derruba a aba nova.
func TestRecarga_UmaPorVezENaAbaLida(t *testing.T) {
	m := telaDaPrancha(t, 140, 32)
	m, primeira := teclas(t, m, "r")
	if primeira == nil {
		t.Fatal("o r deveria ler")
	}
	for range 4 {
		if _, cmd := teclas(t, m, "r"); cmd != nil {
			t.Fatal("com uma recarga em voo, o r não lê de novo")
		}
	}
	m, _ = teclas(t, m, "2")
	m, _ = atualizar(m, primeira())
	if fr := ansi.Strip(m.actionFeedback); !strings.Contains(fr, "pods recarregados") || strings.Contains(fr, "deployments") {
		t.Fatalf("a recarga dos pods, já em Deployments: %q", fr)
	}

	m = telaDaPrancha(t, 140, 32)
	m.client = clienteStaging{&MockClient{Err: fmt.Errorf("connection refused")}}
	m, falha := teclas(t, m, "r")
	m, _ = teclas(t, m, "2")
	m, _ = atualizar(m, falha())
	if m.err != nil || m.reconnecting || !strings.Contains(ansi.Strip(m.actionFeedback), "não deu para recarregar") {
		t.Fatalf("a falha dos pods, já em Deployments: err %v, reconectando %v, %q", m.err, m.reconnecting, ansi.Strip(m.actionFeedback))
	}
}

// TestRolagem_AjudaEDetalhe: as setas, o fim e a roda rolam a ajuda e o
// quadro; na ajuda de 72x24, rolar mostra as teclas do fim.
func TestRolagem_AjudaEDetalhe(t *testing.T) {
	semCor(t)
	m, _ := teclas(t, telaDaPrancha(t, 72, 24), "?")
	if m.maxRolagemAjuda() == 0 || strings.Contains(ansi.Strip(m.View()), "l y e") {
		t.Fatal("a ajuda de 72x24 não cabe inteira")
	}
	if a, _ := teclas(t, m, "down"); a.ajudaScroll != 1 {
		t.Errorf("↓ na ajuda: rolagem %d", a.ajudaScroll)
	}
	fimDaAjuda, _ := teclas(t, m, "end")
	if fimDaAjuda.ajudaScroll != m.maxRolagemAjuda() || !strings.Contains(ansi.Strip(fimDaAjuda.View()), "l y e") {
		t.Errorf("fim da ajuda: rolagem %d de %d", fimDaAjuda.ajudaScroll, m.maxRolagemAjuda())
	}
	if a, _ := teclas(t, fimDaAjuda, "up"); a.ajudaScroll != m.maxRolagemAjuda()-1 {
		t.Errorf("↑ na ajuda: rolagem %d", a.ajudaScroll)
	}
	roda := tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress}
	if a, _ := atualizar(m, roda); a.ajudaScroll != min(passoRoda, m.maxRolagemAjuda()) {
		t.Errorf("roda na ajuda: rolagem %d", a.ajudaScroll)
	}

	linhas := make([]string, 100)
	for i := range linhas {
		linhas[i] = fmt.Sprintf("linha-%03d: x", i)
	}
	base := telaDaPrancha(t, 120, 30)
	base.client = &clienteComDetalhe{clienteStaging: clienteDaPrancha(), yaml: strings.Join(linhas, "\n")}
	d, pedido := teclas(t, base, "y")
	d, _ = atualizar(d, pedido())
	if d, _ = atualizar(d, roda); d.quadro.scroll != passoRoda || strings.Contains(ansi.Strip(d.View()), "linha-002") {
		t.Errorf("roda no quadro: rolagem %d", d.quadro.scroll)
	}
	if d, _ = teclas(t, d, "down"); d.quadro.scroll != passoRoda+1 {
		t.Errorf("↓ no quadro: rolagem %d", d.quadro.scroll)
	}
	if d, _ = teclas(t, d, "G"); d.quadro.scroll != d.maxRolagemQuadro() || !strings.Contains(ansi.Strip(d.View()), "linha-099") {
		t.Errorf("G no quadro: rolagem %d de %d", d.quadro.scroll, d.maxRolagemQuadro())
	}
	if d, _ = teclas(t, d, "g"); d.quadro.scroll != 0 || !strings.Contains(ansi.Strip(d.View()), "linha-000") {
		t.Errorf("g no quadro: rolagem %d", d.quadro.scroll)
	}
}

// TestMouse_IgnoradoNasCamadas: com campo, menu ou confirmação aberta, a
// roda e o clique (na lista ou nas abas) não mudam nada.
func TestMouse_IgnoradoNasCamadas(t *testing.T) {
	base := telaDaPrancha(t, 140, 32)
	r := regiaoDaAba(t, base, TabNodes)
	eventos := []tea.Msg{
		tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress},
		tea.MouseMsg{X: 5, Y: base.primeiraLinhaDeDados() + 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		tea.MouseMsg{X: r.ini + 1, Y: linhaDasAbas, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
	}
	for nome, ks := range map[string][]string{"campo": {"/"}, "menu": {"x"}, "confirmação": {"x", "d"}} {
		m, _ := teclas(t, base, ks...)
		for _, ev := range eventos {
			nm, cmd := atualizar(m, ev)
			if cmd != nil || nm.selectedIndex != m.selectedIndex || nm.scrollY != m.scrollY || nm.activeTab != m.activeTab {
				t.Errorf("%s: o mouse (%+v) agiu por baixo da camada", nome, ev)
			}
		}
	}
}
