package ui

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	yerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	yui "github.com/casheiro/yby-cli/v5/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Ligação Viz → Bard: o b monta o pedido sobre a linha selecionada e roda o yby.

// binDoCore é o yby localizado nos testes. Não termina em "yby": um exec.Command("yby")
// direto, sem o localizar, teria outro Path.
const binDoCore = "/opt/core/bin-do-core"

// bardFalso guarda o comando que o b mandou rodar, sem rodar nada.
type bardFalso struct {
	execs []execBard
	fim   tea.ExecCallback
}

// ligarBardFalso troca a ligação do modelo pela falsa, com o yby em binYby.
func ligarBardFalso(m Model, binYby string, errLocalizar error) (Model, *bardFalso) {
	f := &bardFalso{}
	m.bard = ligacaoBard{
		localizar: func() (string, error) { return binYby, errLocalizar },
		executar: func(c tea.ExecCommand, fim tea.ExecCallback) tea.Cmd {
			f.execs = append(f.execs, c.(execBard))
			f.fim = fim
			return func() tea.Msg { return nil }
		},
		ambiente: func() []string {
			return []string{"PATH=/bin", `YBY_PLUGIN_REQUEST={"hook":"command","args":["--namespace","payments"]}`, "YBY_BIN=/opt/yby", "KUBECONFIG=/padrao"}
		},
	}
	return m, f
}

// argsDoBard são os args do yby (sem o executável) do último b.
func (f *bardFalso) argsDoBard(t *testing.T) []string {
	t.Helper()
	if len(f.execs) == 0 {
		t.Fatal("o b não abriu o Bard")
	}
	return f.execs[len(f.execs)-1].cmd.Args[1:]
}

func TestBard_PedidoMontado(t *testing.T) {
	semCor(t)
	casos := []struct {
		nome   string
		passos []string // teclas antes do b
		quer   string
	}{
		{"pod com problema", nil, "Por que o pod payments/worker-5d8b6-q7tvn está em CrashLoopBackOff (14 reinícios)?"},
		{"pod pendente", []string{"j"}, "Por que o pod payments/web-8b77c-5jw4f está em Pending há 4m?"},
		{"pod saudável", []string{"/", "r", "e", "d", "i", "s", "enter"}, "Explique o estado do pod payments/redis-0."},
		{"deployment com problema", []string{"2"}, "Por que o deployment payments/worker está sem réplicas prontas (0/2)?"},
		{"deployment pela metade", []string{"2", "j"}, "Por que o deployment payments/web está com 1/2 réplicas prontas?"},
		{"deployment saudável", []string{"2", "/", "a", "p", "i", "enter"}, "Explique o estado do deployment payments/api."},
		{"nó", []string{"5"}, "Explique o estado do nó ip-10-0-1-5."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			m, f := ligarBardFalso(telaDaPrancha(t, 140, 32), binDoCore, nil)
			m, _ = teclas(t, m, c.passos...)
			m, cmd := teclas(t, m, "b")
			if cmd == nil {
				t.Fatal("o b deveria devolver o comando que abre o Bard")
			}
			ns := "payments" // o do recurso; no nó, o do Viz
			quer := []string{"bard", "--pergunta", c.quer, "--voltar", "Viz", "--context", "staging", "--namespace", ns}
			if got := f.argsDoBard(t); strings.Join(got, "\x00") != strings.Join(quer, "\x00") {
				t.Fatalf("args do yby:\n got %q\nquer %q", got, quer)
			}
			e := f.execs[0]
			if e.cmd.Path != binDoCore || e.cmd.Args[0] != binDoCore {
				t.Errorf("o yby rodado deveria ser o localizado (o YBY_BIN do core): %q %q", e.cmd.Path, e.cmd.Args[0])
			}
			for _, kv := range e.cmd.Env {
				if strings.HasPrefix(kv, "YBY_PLUGIN_REQUEST=") {
					t.Errorf("o pedido do core ao Viz não vai ao Bard: %q", kv)
				}
			}
			if !contem(e.cmd.Env, "KUBECONFIG=/padrao") || !contem(e.cmd.Env, "YBY_BIN=/opt/yby") {
				t.Errorf("o resto do ambiente segue: %q", e.cmd.Env)
			}
			if m.toast.Visivel() {
				t.Errorf("abrir o Bard não mostra toast")
			}
		})
	}
}

// contem diz se a lista tem o texto.
func contem(lista []string, s string) bool {
	for _, x := range lista {
		if x == s {
			return true
		}
	}
	return false
}

// Lista vazia: o b não abre nada e avisa na linha de estado.
func TestBard_ListaVaziaAvisa(t *testing.T) {
	semCor(t)
	m, f := ligarBardFalso(telaDaPrancha(t, 140, 32), binDoCore, nil)
	m, _ = teclas(t, m, "/", "z", "z", "z", "enter")
	m, cmd := teclas(t, m, "b")
	if cmd != nil || len(f.execs) != 0 {
		t.Fatalf("sem linha, o b não abre o Bard: %d execs", len(f.execs))
	}
	if !strings.Contains(ansi.Strip(m.View()), "nada selecionado para perguntar ao Bard") {
		t.Fatalf("a linha de estado deveria avisar:\n%s", ansi.Strip(m.View()))
	}
}

// Sem yby (nem YBY_BIN nem PATH), um toast em PT-BR explica.
func TestBard_SemYbyMostraToast(t *testing.T) {
	semCor(t)
	m, f := ligarBardFalso(telaDaPrancha(t, 140, 32), "", errSemYby)
	m, _ = teclas(t, m, "b")
	if len(f.execs) != 0 {
		t.Fatal("sem yby, nada roda")
	}
	tela := ansi.Strip(m.View())
	if !strings.Contains(tela, "não achei o yby para abrir o Bard") {
		t.Fatalf("o toast deveria explicar:\n%s", tela)
	}
}

// O --kubeconfig do Viz vai ao Bard no KUBECONFIG, no lugar do herdado. O client vem
// dentro do RetryClient, como no main.go.
func TestBard_KubeconfigDoViz(t *testing.T) {
	semCor(t)
	cliente := monitor.NewRetryClient(clienteComKubeconfig{clienteStaging{&MockClient{}}})
	m, f := ligarBardFalso(telaDaPranchaCom(t, cliente, 140, 32), binDoCore, nil)
	teclas(t, m, "b")
	env := f.execs[0].cmd.Env
	if !contem(env, "KUBECONFIG=/tmp/outro.yaml") || contem(env, "KUBECONFIG=/padrao") {
		t.Fatalf("KUBECONFIG do Bard: %q", env)
	}
}

type clienteComKubeconfig struct{ clienteStaging }

func (clienteComKubeconfig) Kubeconfig() string { return "/tmp/outro.yaml" }

// O b vale também com o painel por cima (abaixo de 120 colunas, depois do ↵).
func TestBard_ComPainelPorCima(t *testing.T) {
	semCor(t)
	m, f := ligarBardFalso(telaDaPrancha(t, 100, 30), binDoCore, nil)
	m, _ = teclas(t, m, "enter")
	if !m.painelPorCima() {
		t.Fatal("o ↵ deveria abrir o painel por cima")
	}
	teclas(t, m, "b")
	if args := f.argsDoBard(t); args[2] != "Por que o pod payments/worker-5d8b6-q7tvn está em CrashLoopBackOff (14 reinícios)?" {
		t.Fatalf("pergunta: %q", args[2])
	}
}

func TestPerguntaSobre(t *testing.T) {
	semCor(t)
	agora := horaDaPrancha
	pod := func(p monitor.Pod) itemLista { return itemLista{nome: p.Name, namespace: p.Namespace, obj: p} }
	naoPronto := podDaPrancha("api-1", "Running", 0, 0, time.Hour, 0, "", "")
	reiniciou := podDaPrancha("billing-1", "Running", 1, 3, dia, 5*time.Minute, "", "")
	umReinicio := podDaPrancha("worker-1", "Error", 0, 1, time.Hour, time.Minute, "", "")
	casos := []struct {
		nome string
		tab  ResourceTab
		it   itemLista
		quer string
	}{
		{"pod sem contêiner pronto", TabPods, pod(naoPronto), "Por que o pod payments/api-1 está com 0/1 contêineres prontos?"},
		{"pod que reiniciou", TabPods, pod(reiniciou), "Por que o pod payments/billing-1 reiniciou 3 vezes, a última há 5m?"},
		{"um reinício", TabPods, pod(umReinicio), "Por que o pod payments/worker-1 está em Error (1 reinício)?"},
		{"pod concluído", TabPods, pod(podDaPrancha("migrate-1", "Completed", 0, 0, time.Hour, 0, "", "")), "Explique o estado do pod payments/migrate-1."},
		{"statefulset", TabStatefulSets, itemLista{nome: "redis", namespace: "payments", obj: monitor.StatefulSet{Name: "redis", Replicas: 3, Ready: 2}},
			"Por que o statefulset payments/redis está com 2/3 réplicas prontas?"},
		{"job que falhou", TabJobs, itemLista{nome: "migrate", namespace: "payments", obj: monitor.Job{Name: "migrate", Failed: 2, Falhou: true}},
			"Por que o job payments/migrate falhou 2 vezes?"},
		{"job que tenta de novo", TabJobs, itemLista{nome: "migrate", namespace: "payments", obj: monitor.Job{Name: "migrate", Failed: 1, Active: 1, Completions: 1}},
			"Por que o job payments/migrate falhou 1 vez e tenta de novo?"},
		{"job aguardando", TabJobs, itemLista{nome: "migrate", namespace: "payments", obj: monitor.Job{Name: "migrate", Completions: 1}},
			"Por que o job payments/migrate está aguardando pods?"},
		{"nó fora", TabNodes, itemLista{nome: "k3d-0", obj: monitor.Node{Name: "k3d-0", Status: "NotReady"}}, "Por que o nó k3d-0 está em NotReady?"},
		{"evento de alerta", TabEvents, itemLista{nome: "worker-1", namespace: "payments", obj: monitor.Event{Name: "worker-1", Kind: "Pod", Type: "Warning", Reason: "BackOff", Message: "Back-off restarting\nfailed container"}},
			`O que significa o evento BackOff no pod payments/worker-1: "Back-off restarting failed container"?`},
		{"evento normal", TabEvents, itemLista{nome: "api", namespace: "payments", obj: monitor.Event{Name: "api", Kind: "Deployment", Type: "Normal", Reason: "ScalingReplicaSet"}},
			"Explique o evento ScalingReplicaSet no deployment payments/api."},
		{"service", TabServices, itemLista{nome: "web", namespace: "payments", obj: monitor.Service{Name: "web"}}, "Explique o estado do service payments/web."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := perguntaSobre(abas[c.tab], c.it, agora); got != c.quer {
				t.Fatalf("\n got %q\nquer %q", got, c.quer)
			}
		})
	}
	longa := itemLista{nome: "x", namespace: "ns", obj: monitor.Event{Name: "x", Kind: "Pod", Type: "Warning", Reason: "Failed", Message: strings.Repeat("a", 400)}}
	if got := perguntaSobre(abas[TabEvents], longa, agora); len([]rune(got)) > 250 || !strings.Contains(got, yui.G().Reticencias) {
		t.Fatalf("a mensagem longa do evento é cortada: %d runas", len([]rune(got)))
	}
}

func TestLocalizarYby(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == envBinarioDoCore {
				return v
			}
			return ""
		}
	}
	noPath := func(string) (string, error) { return "/usr/bin/yby", nil }
	semPath := func(string) (string, error) { return "", exec.ErrNotFound }
	existe := func(c string) bool { return c == binDoCore }

	if bin, err := localizarYby(env(binDoCore), noPath, existe); err != nil || bin != binDoCore {
		t.Errorf("o YBY_BIN do core vem primeiro: %q %v", bin, err)
	}
	if bin, err := localizarYby(env("/sumiu/yby"), noPath, existe); err != nil || bin != "/usr/bin/yby" {
		t.Errorf("YBY_BIN que não existe cai para o PATH: %q %v", bin, err)
	}
	if bin, err := localizarYby(env(""), noPath, existe); err != nil || bin != "/usr/bin/yby" {
		t.Errorf("sem YBY_BIN, o PATH: %q %v", bin, err)
	}
	if _, err := localizarYby(env(""), semPath, existe); !errors.Is(err, errSemYby) {
		t.Errorf("sem nenhum, errSemYby: %v", err)
	}
}

// A volta do Bard religa o mouse; a falha vira toast com a primeira linha do erro do yby.
func TestBard_Volta(t *testing.T) {
	semCor(t)
	m := telaDaPrancha(t, 140, 32)

	nm, cmd := atualizar(m, bardFechadoMsg{})
	if cmd == nil || nm.toast.Visivel() {
		t.Fatal("o Bard fechado sem erro só religa o mouse")
	}

	erros := &inicioDoTexto{}
	_, _ = erros.Write([]byte(yui.RenderError(yerrors.New(yerrors.ErrCodeConfig, "nenhum provider de IA disponível").WithHint("rode ollama serve"))))
	nm, _ = atualizar(m, bardFechadoMsg{err: saidaComCodigo(t, "1"), erros: erros, fim: &fimDoBard{rodou: true}})
	tela := ansi.Strip(nm.View())
	if !strings.Contains(tela, "o Bard fechou com erro") || !strings.Contains(tela, "nenhum provider de IA disponível") {
		t.Fatalf("o toast deveria mostrar a falha:\n%s", tela)
	}
	if strings.Contains(tela, yerrors.ErrCodeConfig) {
		t.Errorf("o código do erro fica fora da nota")
	}
}

// saidaComCodigo é o *exec.ExitError de um processo que saiu com o código dado.
func saidaComCodigo(t *testing.T, codigo string) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestAjudanteSaida$")
	cmd.Env = append(os.Environ(), "VIZ_AJUDANTE_SAIDA="+codigo)
	err := cmd.Run()
	var saida *exec.ExitError
	if !errors.As(err, &saida) {
		t.Fatalf("o ajudante deveria sair com %s: %v", codigo, err)
	}
	return err
}

// Cada falha na volta tem o seu aviso: o yby que nem começou não abriu o Bard; o erro de
// devolver o terminal depois de um Bard que fechou bem não vira toast.
func TestBard_VoltaSeparaAsFalhas(t *testing.T) {
	semCor(t)
	m := telaDaPrancha(t, 140, 32)

	naoRodou := &exec.Error{Name: binDoCore, Err: exec.ErrNotFound}
	nm, _ := atualizar(m, bardFechadoMsg{err: naoRodou, erros: &inicioDoTexto{}, fim: &fimDoBard{rodou: true, err: naoRodou}})
	if tela := ansi.Strip(nm.View()); !strings.Contains(tela, "o Bard não abriu") {
		t.Errorf("o yby que não começa não abriu o Bard:\n%s", tela)
	}

	semTerminal := errors.New("could not restore terminal")
	nm, cmd := atualizar(m, bardFechadoMsg{err: semTerminal, erros: &inicioDoTexto{}, fim: &fimDoBard{rodou: true}})
	if nm.toast.Visivel() || cmd == nil {
		t.Errorf("o Bard fechou bem: só o mouse volta, sem toast")
	}

	nm, _ = atualizar(m, bardFechadoMsg{err: errors.New("could not release terminal"), erros: &inicioDoTexto{}, fim: &fimDoBard{}})
	if tela := ansi.Strip(nm.View()); !strings.Contains(tela, "o Bard não abriu") || !strings.Contains(tela, "could not release terminal") {
		t.Errorf("sem soltar o terminal o Bard não abriu:\n%s", tela)
	}
}

// O execBard guarda o que o Run do yby devolveu, para a volta separar as falhas.
func TestExecBard_GuardaOFimDoRun(t *testing.T) {
	fim := &fimDoBard{}
	cmd := exec.Command(os.Args[0], "-test.run=^TestAjudanteSaida$")
	cmd.Env = append(os.Environ(), "VIZ_AJUDANTE_SAIDA=0")
	if err := (execBard{cmd: cmd, erros: &inicioDoTexto{}, fim: fim}).Run(); err != nil || !fim.rodou || fim.err != nil {
		t.Fatalf("o Run sem erro: err=%v fim=%+v", err, fim)
	}
	fim = &fimDoBard{}
	cmd = exec.Command(os.Args[0], "-test.run=^TestAjudanteSaida$")
	cmd.Env = append(os.Environ(), "VIZ_AJUDANTE_SAIDA=3")
	if err := (execBard{cmd: cmd, erros: &inicioDoTexto{}, fim: fim}).Run(); err == nil || !fim.rodou || fim.err == nil {
		t.Fatalf("o Run com erro: err=%v fim=%+v", err, fim)
	}
}

// Dois b seguidos, antes de o primeiro Bard abrir, abrem o Bard uma vez só; quando ele
// volta, o b abre de novo.
func TestBard_DoisBAbremUmaVez(t *testing.T) {
	semCor(t)
	m, f := ligarBardFalso(telaDaPrancha(t, 140, 32), binDoCore, nil)
	m, _ = teclas(t, m, "b")
	m, cmd := teclas(t, m, "b")
	if len(f.execs) != 1 || cmd != nil {
		t.Fatalf("o segundo b não abre outro Bard: %d execs", len(f.execs))
	}
	m, _ = atualizar(m, f.fim(nil).(bardFechadoMsg))
	teclas(t, m, "b")
	if len(f.execs) != 2 {
		t.Fatalf("depois da volta, o b abre de novo: %d execs", len(f.execs))
	}
}

// O b não abre o Bard quando é texto (a busca) nem com outra camada aberta: o seletor de
// namespace, os logs, a ajuda. No menu x, o b é a ação "perguntar ao Bard".
func TestBard_BComOutraCamadaAberta(t *testing.T) {
	semCor(t)
	for _, c := range []struct {
		nome   string
		passos []string
	}{
		{"busca", []string{"/"}},
		{"seletor de namespace", []string{"n"}},
		{"logs", []string{"l"}},
		{"yaml", []string{"y"}},
		{"ajuda", []string{"?"}},
	} {
		t.Run(c.nome, func(t *testing.T) {
			m, f := ligarBardFalso(telaDaPrancha(t, 140, 32), binDoCore, nil)
			m, _ = teclas(t, m, c.passos...)
			m, _ = teclas(t, m, "b")
			if len(f.execs) != 0 {
				t.Fatalf("com %s aberto, o b não abre o Bard: %d execs", c.nome, len(f.execs))
			}
			if c.nome == "busca" && m.filter.InputBuffer != "b" {
				t.Errorf("na busca o b é letra: busca %q", m.filter.InputBuffer)
			}
		})
	}
}

// No menu x, "b perguntar ao Bard" (folha VizConfirmar) abre o Bard sobre o alvo do menu,
// pela letra ou pelo ↵, com a nota do problema da linha.
func TestBard_PeloMenuDeAcoes(t *testing.T) {
	semCor(t)
	m, f := ligarBardFalso(telaDaPrancha(t, 140, 32), binDoCore, nil)
	m, _ = teclas(t, m, "2", "x")
	tela := ansi.Strip(m.View())
	if !strings.Contains(tela, "◇ b  perguntar ao Bard") || !strings.Contains(tela, "por que está 0/2") {
		t.Fatalf("o menu deveria ter o b com a nota do problema:\n%s", tela)
	}
	m, _ = teclas(t, m, "b")
	if m.acao.aberta() {
		t.Error("escolher o Bard fecha o menu")
	}
	if args := f.argsDoBard(t); args[2] != "Por que o deployment payments/worker está sem réplicas prontas (0/2)?" {
		t.Fatalf("pergunta: %q", args[2])
	}

	m, f = ligarBardFalso(telaDaPrancha(t, 140, 32), binDoCore, nil)
	teclas(t, m, "x", "enter") // nos pods, o Bard é a primeira ação
	if args := f.argsDoBard(t); args[2] != "Por que o pod payments/worker-5d8b6-q7tvn está em CrashLoopBackOff (14 reinícios)?" {
		t.Fatalf("pergunta pelo ↵: %q", args[2])
	}
}

func TestNotaBard(t *testing.T) {
	agora := horaDaPrancha
	pod := func(p monitor.Pod) itemLista { return itemLista{nome: p.Name, namespace: p.Namespace, obj: p} }
	for _, c := range []struct {
		it   itemLista
		quer string
	}{
		{pod(podDaPrancha("worker-1", "CrashLoopBackOff", 0, 14, time.Hour, time.Minute, "", "")), "por que está em CrashLoopBackOff"},
		{pod(podDaPrancha("api-1", "Running", 0, 0, time.Hour, 0, "", "")), "por que está 0/1"},
		{pod(podDaPrancha("redis-0", "Running", 1, 0, dia, 0, "", "")), "explicar o estado"},
		{itemLista{obj: monitor.Deployment{Name: "web", Replicas: 2, Ready: 1}}, "por que está 1/2"},
		{itemLista{obj: monitor.Deployment{Name: "api", Replicas: 2, Ready: 2}}, "explicar o estado"},
		{itemLista{obj: monitor.Job{Name: "migrate", Failed: 2, Falhou: true}}, "por que falhou"},
		{itemLista{obj: monitor.Node{Name: "k3d-0", Status: "NotReady"}}, "por que está em NotReady"},
		{itemLista{obj: monitor.Event{Type: "Warning", Reason: "BackOff"}}, "o que significa"},
		{itemLista{obj: monitor.Service{Name: "web"}}, "explicar o estado"},
	} {
		if got := notaBard(c.it, agora); got != c.quer {
			t.Errorf("%s: %q, quer %q", c.it.nome, got, c.quer)
		}
	}
}

// A nota é o título do erro, sem glifo e sem o código, perto ou longe dele.
func TestInicioDoTexto_PrimeiraLinha(t *testing.T) {
	semCor(t)
	for entrada, quer := range map[string]string{
		"\n✕ o --pergunta abre a conversa do Bard e precisa de um terminal  ERR_VALIDATION\n  faça      use -p\n": "o --pergunta abre a conversa do Bard e precisa de um terminal",
		"✕ falta o valor de --pergunta                     ERR_VALIDATION\n":                                      "falta o valor de --pergunta",
		"plugin bard não encontrado\n": "plugin bard não encontrado",
		"":                             "",
	} {
		e := &inicioDoTexto{}
		_, _ = e.Write([]byte(entrada))
		if got := e.primeiraLinha(); got != quer {
			t.Errorf("primeiraLinha(%q) = %q, quer %q", entrada, got, quer)
		}
	}
}

// O yby que saiu com 130 (o ctrl+c no Bard, que a TUI dele devolve; ou o do Windows) fecha
// o Viz também; outro código vira a nota. O esc do Bard volta com 0 (TestBard_Volta).
func TestBard_VoltaComCodigo(t *testing.T) {
	semCor(t)
	m := telaDaPrancha(t, 140, 32)
	for _, c := range []struct {
		codigo string
		sai    bool
	}{{"130", true}, {"3", false}} {
		err := saidaComCodigo(t, c.codigo)
		nm, mais := atualizar(m, bardFechadoMsg{err: err, erros: &inicioDoTexto{}, fim: &fimDoBard{rodou: true, err: err}})
		if nm.Interrompido() != c.sai {
			t.Errorf("código %s: interrompido = %v", c.codigo, nm.Interrompido())
		}
		if !c.sai {
			if tela := ansi.Strip(nm.View()); !strings.Contains(tela, "o yby saiu com o código 3") {
				t.Errorf("a nota diz o código:\n%s", tela)
			}
		} else if mais == nil {
			t.Error("o 130 devolve o tea.Quit")
		}
	}
}

// O código do ctrl+c: o 130 e o STATUS_CONTROL_C_EXIT do Windows, lido como int de 64 ou
// de 32 bits.
func TestSaiuPorCtrlC(t *testing.T) {
	for codigo, quer := range map[int]bool{130: true, 3221225786: true, -1073741510: true, 0: false, 1: false, 3: false} {
		if saiuPorCtrlC(codigo) != quer {
			t.Errorf("saiuPorCtrlC(%d) = %v", codigo, !quer)
		}
	}
}

// TestAjudanteSaida é o processo falso do TestBard_VoltaComCodigo.
func TestAjudanteSaida(t *testing.T) {
	codigo := os.Getenv("VIZ_AJUDANTE_SAIDA")
	if codigo == "" {
		return
	}
	n := 0
	for _, r := range codigo {
		n = n*10 + int(r-'0')
	}
	os.Exit(n)
}

// TestTeclaBNaLegenda: o b aparece no rodapé do painel ao lado (curto, "b Bard",
// quando "perguntar ao Bard" não cabe) e, sem o painel ao lado, na legenda da
// lista quando cabe inteiro, sem tirar o "? ajuda" a 80 colunas.
func TestTeclaBNaLegenda(t *testing.T) {
	descricoes := func(largura int) []string {
		var d []string
		for _, b := range teclasDoPainel("pod", largura) {
			d = append(d, b.Help().Key+" "+b.Help().Desc)
		}
		return d
	}
	if got := strings.Join(descricoes(60), "  "); got != "l logs  y yaml  e eventos  b perguntar ao Bard" {
		t.Errorf("com espaço, o b vai inteiro: %q", got)
	}
	if got := strings.Join(descricoes(44), "  "); got != "l logs  y yaml  e eventos  b Bard" {
		t.Errorf("sem espaço, o b fica curto: %q", got)
	}
	if got := teclasDoPainel("pod", 44)[3]; !got.Enabled() || strings.Join(got.Keys(), ",") != "b" {
		t.Errorf("a tecla curta continua sendo o b: %v", got.Keys())
	}

	for _, c := range []struct {
		largura int
		comB    bool
	}{{80, false}, {100, true}, {140, false}} {
		m := telaDaPrancha(t, c.largura, 30)
		linhas := strings.Split(ansi.Strip(m.View()), "\n")
		legenda := linhas[len(linhas)-1]
		if strings.Contains(legenda, "b perguntar ao Bard") != c.comB || !strings.Contains(legenda, "? ajuda") {
			t.Errorf("%d colunas: legenda da lista %q (b esperado: %v)", c.largura, legenda, c.comB)
		}
	}
}
