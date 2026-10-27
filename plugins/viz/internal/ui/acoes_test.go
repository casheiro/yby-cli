package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Menu de ações e confirmação (folha VizConfirmar), com o clientset falso.

// TestAcao_ComBuscaAtingeLinhaSelecionada: com a busca ativa, a ação atinge a
// linha selecionada na tela: x ▸ d, o nome digitado e ↵ apagam só o recurso da
// linha, mesmo que a lista seja atualizada entre abrir a confirmação e confirmar.
func TestAcao_ComBuscaAtingeLinhaSelecionada(t *testing.T) {
	cs := fake.NewClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "loja"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Namespace: "fila"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Namespace: "loja"}},
	)
	m := NewModel(&clientComAcesso{MockClient: &MockClient{}, cs: cs})
	m.pods = []monitor.Pod{
		{Name: "api-1", Namespace: "loja"},
		{Name: "worker-1", Namespace: "fila"},
		{Name: "worker-1", Namespace: "loja"},
	}
	m = buscar(t, m, "worker")
	m, _ = teclas(t, m, "j") // a segunda linha da tela: loja/worker-1

	m, _ = teclas(t, m, "x", "d")
	if m.acao.etapa != confirmando || m.acao.alvo.namespace != "loja" {
		t.Fatalf("x ▸ d deveria pedir o nome de loja/worker-1: etapa %d, alvo %+v", m.acao.etapa, m.acao.alvo)
	}
	// A lista muda (atualização do cluster) antes da confirmação.
	nm, _ := m.Update([]monitor.Pod{{Name: "worker-1", Namespace: "fila"}, {Name: "api-1", Namespace: "loja"}})
	m = nm.(Model)
	m, _ = teclas(t, m, strings.Split("worker-1", "")...)
	m, cmd := teclas(t, m, "enter")
	if cmd == nil || m.acao.aberta() {
		t.Fatal("confirmar a exclusão deveria fechar a confirmação e devolver um comando")
	}
	res, ok := cmd().(resultadoAcaoMsg)
	if !ok || res.falha || res.texto != "Pod worker-1 excluído" || !res.recarregar {
		t.Fatalf("resultado inesperado: %#v", res)
	}

	pods, _ := cs.CoreV1().Pods("").List(t.Context(), metav1.ListOptions{})
	var sobra []string
	for _, p := range pods.Items {
		sobra = append(sobra, p.Namespace+"/"+p.Name)
	}
	if strings.Join(sobra, " ") != "fila/worker-1 loja/api-1" {
		t.Fatalf("deveria sobrar fila/worker-1 e loja/api-1, sobrou %v", sobra)
	}

	// O resultado vai para o Toast e a lista é lida de novo.
	m, cmd = atualizar(m, res)
	if !m.toast.Visivel() || cmd == nil || !strings.Contains(m.toast.View(), "Pod worker-1 excluído") {
		t.Fatalf("toast: %q, comando %v", m.toast.View(), cmd != nil)
	}
}

// TestAcao_NomeErradoNaoConfirma: o ↵ só exclui com o nome exato.
func TestAcao_NomeErradoNaoConfirma(t *testing.T) {
	cs := fake.NewClientset(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "payments"}})
	feitas := chamadas(cs)
	base := NewModel(&clientComAcesso{MockClient: &MockClient{}, cs: cs})
	base.activeTab = TabDeployments
	base.deployments = []monitor.Deployment{{Name: "worker", Namespace: "payments"}}
	base, _ = teclas(t, base, "x", "d")
	for _, digitado := range []string{"", "work", "workers", "Worker", "worker "} {
		m, _ := teclas(t, base, strings.Split(digitado, "")...)
		m, cmd := teclas(t, m, "enter")
		if cmd != nil || m.acao.etapa != confirmando {
			t.Errorf("com %q o ↵ não deveria confirmar (etapa %d)", digitado, m.acao.etapa)
		}
	}
	if len(*feitas) != 0 {
		t.Fatalf("nada deveria chegar à API: %v", *feitas)
	}
}

// TestAcao_EscalarEReiniciarPeloMenu: escalar e reiniciar, pela letra e pelo
// ◆ com ↵, chamam a API do tipo da linha (StatefulSet ou Deployment).
func TestAcao_EscalarEReiniciarPeloMenu(t *testing.T) {
	casos := []struct {
		tab       ResourceTab
		recurso   string
		rotulo    string
		escalar   []string // teclas do menu até o campo de réplicas
		reiniciar []string // teclas do menu até a confirmação do reinício
	}{
		{TabStatefulSets, "statefulsets", "StatefulSet db", []string{"s"}, []string{"r"}},
		// O ◆ começa em reiniciar; ↓ leva ao escalar.
		{TabDeployments, "deployments", "Deployment db", []string{"down", "enter"}, []string{"enter"}},
	}
	for _, c := range casos {
		t.Run(c.recurso, func(t *testing.T) {
			cs := fake.NewClientset(
				&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "dados"}},
				&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "dados"}},
			)
			feitas := chamadas(cs)
			m := NewModel(&clientComAcesso{MockClient: &MockClient{}, cs: cs})
			m.activeTab = c.tab
			m.statefulsets = []monitor.StatefulSet{{Name: "db", Namespace: "dados", Replicas: 2}}
			m.deployments = []monitor.Deployment{{Name: "db", Namespace: "dados", Replicas: 2}}

			// Escalar: o campo começa com as réplicas de hoje e a primeira tecla as troca.
			e, _ := teclas(t, m, append([]string{"x"}, c.escalar...)...)
			if e.acao.etapa != escalando || e.acao.replicas != "2" {
				t.Fatalf("escalar: etapa %d, campo %q", e.acao.etapa, e.acao.replicas)
			}
			_, cmd := teclas(t, e, "3", "enter")
			if res := cmd().(resultadoAcaoMsg); res.falha || res.texto != c.rotulo+" escalado para 3 réplicas" {
				t.Fatalf("escalar: %#v", res)
			}
			// Reiniciar: o ↵ confirma.
			r, _ := teclas(t, m, append([]string{"x"}, c.reiniciar...)...)
			if r.acao.etapa != confirmando || r.acao.tipo != acaoReiniciar {
				t.Fatalf("reiniciar: etapa %d, ação %d", r.acao.etapa, r.acao.tipo)
			}
			_, cmd = teclas(t, r, "enter")
			if res := cmd().(resultadoAcaoMsg); res.falha || res.texto != c.rotulo+" reiniciado" {
				t.Fatalf("reiniciar: %#v", res)
			}

			// As 3 réplicas digitadas chegam à API.
			esperadas := []string{"get " + c.recurso + "/scale", "update " + c.recurso + "/scale=3", "patch " + c.recurso}
			if strings.Join(*feitas, ", ") != strings.Join(esperadas, ", ") {
				t.Fatalf("chamadas = %v, esperado %v", *feitas, esperadas)
			}
		})
	}
}

// TestAcao_EscalarValidaONumero: só um inteiro de 0 para cima escala; o
// número inválido avisa no quadro e não sai do campo.
func TestAcao_EscalarValidaONumero(t *testing.T) {
	m := comJanela(NewModel(&MockClient{}))
	m.activeTab = TabDeployments
	m.deployments = []monitor.Deployment{{Name: "api", Namespace: "loja", Replicas: 3}}
	m.carregadas = 1 << uint(TabDeployments)
	for _, digitado := range []string{"-1", "dois", "1.5", "99999999999", ""} {
		nm, _ := teclas(t, m, "x", "s", "backspace")
		nm, _ = teclas(t, nm, strings.Split(digitado, "")...)
		nm, cmd := teclas(t, nm, "enter")
		if cmd != nil || nm.acao.etapa != escalando || !nm.acao.invalido {
			t.Errorf("%q não deveria escalar (etapa %d, inválido %v)", digitado, nm.acao.etapa, nm.acao.invalido)
		}
		if v := ansi.Strip(nm.View()); !strings.Contains(v, "um número inteiro, 0 ou mais") {
			t.Errorf("%q: o quadro deveria avisar:\n%s", digitado, v)
		}
	}
	// Apagar o valor de hoje e escrever 0 vale; o ↵ sem mexer escala para o
	// mesmo valor. Os dois chegam à API com o número certo.
	cs := fake.NewClientset(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "loja"}})
	feitas := chamadas(cs)
	m.client = &clientComAcesso{MockClient: &MockClient{}, cs: cs}
	for _, c := range []struct {
		teclas []string
		quer   string
	}{
		{[]string{"x", "s", "backspace", "0", "enter"}, "Deployment api escalado para 0 réplicas"},
		{[]string{"x", "s", "enter"}, "Deployment api escalado para 3 réplicas"},
	} {
		_, cmd := teclas(t, m, c.teclas...)
		if cmd == nil {
			t.Fatalf("%v deveria escalar", c.teclas)
		}
		if res := cmd().(resultadoAcaoMsg); res.falha || res.texto != c.quer {
			t.Fatalf("%v: %#v", c.teclas, res)
		}
	}
	esperadas := "get deployments/scale, update deployments/scale=0, get deployments/scale, update deployments/scale=3"
	if got := strings.Join(*feitas, ", "); got != esperadas {
		t.Fatalf("chamadas = %s, esperado %s", got, esperadas)
	}
}

// TestAcao_MenuContextual: cada tipo tem as ações que valem para ele, e
// excluir apaga cada tipo que o menu oferece (o ingress também).
func TestAcao_MenuContextual(t *testing.T) {
	quer := map[ResourceTab]string{
		TabPods: "b c d", TabDeployments: "r s b c d", TabStatefulSets: "r s b c d", TabServices: "b c d", TabNodes: "b c",
		TabJobs: "b c d", TabIngresses: "b c d", TabConfigMaps: "b c d", TabEvents: "b c",
	}
	for tab, q := range quer {
		var letras []string
		for _, a := range acoesDoTipo(abas[tab].kind) {
			letras = append(letras, a.tecla)
		}
		if got := strings.Join(letras, " "); got != q {
			t.Errorf("%s: %s, esperado %s", abas[tab].titulo, got, q)
		}
	}

	cs := fake.NewClientset(
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "loja"}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "loja"}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "loja"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "loja"}},
	)
	feitas := chamadas(cs)
	m := NewModel(&clientComAcesso{MockClient: &MockClient{}, cs: cs})
	m.services = []monitor.Service{{Name: "web", Namespace: "loja"}}
	m.jobs = []monitor.Job{{Name: "web", Namespace: "loja"}}
	m.ingresses = []monitor.Ingress{{Name: "web", Namespace: "loja"}}
	m.configmaps = []monitor.ConfigMap{{Name: "web", Namespace: "loja"}}
	for _, tab := range []ResourceTab{TabServices, TabJobs, TabIngresses, TabConfigMaps} {
		m.activeTab = tab
		_, cmd := teclas(t, m, "x", "d", "w", "e", "b", "enter")
		if res := cmd().(resultadoAcaoMsg); res.falha {
			t.Errorf("%s: %s", abas[tab].titulo, res.texto)
		}
	}
	esperadas := "delete services, delete jobs, delete ingresses, delete configmaps"
	if got := strings.Join(*feitas, ", "); got != esperadas {
		t.Fatalf("chamadas = %s, esperado %s", got, esperadas)
	}
}

// TestAcao_CopiarNome: c manda o nome da linha para a área de transferência
// sem perguntar nada nem ler a lista de novo.
func TestAcao_CopiarNome(t *testing.T) {
	var copiado string
	m := NewModel(&MockClient{})
	m.activeTab = TabNodes
	m.nodes = []monitor.Node{{Name: "ip-10-0-1-5"}}
	m.copiar = func(s string) error { copiado = s; return nil }
	m, cmd := teclas(t, m, "x", "c")
	if m.acao.aberta() || cmd == nil {
		t.Fatalf("copiar fecha o menu e devolve um comando: etapa %d", m.acao.etapa)
	}
	res := cmd().(resultadoAcaoMsg)
	if copiado != "ip-10-0-1-5" || res.falha || res.recarregar || res.texto != "nome ip-10-0-1-5 enviado à área de transferência" {
		t.Fatalf("copiado %q, resultado %#v", copiado, res)
	}
	m.copiar = func(string) error { return fmt.Errorf("sem terminal") }
	_, cmd = teclas(t, m, "x", "j", "enter")
	if res := cmd().(resultadoAcaoMsg); !res.falha || !strings.Contains(res.texto, "sem terminal") {
		t.Fatalf("falha ao copiar: %#v", res)
	}
}

// TestAcao_SemAcessoDiretoAvisa: o client que só lista não executa a ação, e
// o Toast diz por quê.
func TestAcao_SemAcessoDiretoAvisa(t *testing.T) {
	m := NewModel(&MockClient{})
	m.pods = []monitor.Pod{{Name: "api-1", Namespace: "loja"}}
	m, _ = teclas(t, m, "x", "d")
	_, cmd := teclas(t, m, "a", "p", "i", "-", "1", "enter")
	if res := cmd().(resultadoAcaoMsg); !res.falha || res.texto != "não deu para excluir Pod api-1" || res.nota != errSemAcesso.Error() {
		t.Fatalf("resultado: %#v", res)
	}
}

// TestAcao_SemListaNaTelaCancela: quando a lista sai da tela (janela abaixo de
// 60x16 ou conexão caída), o menu e a confirmação fecham: as teclas seguintes
// não confirmam uma exclusão que a pessoa não vê. Sem conexão o aviso sai na
// hora; com a janela pequena, quando ela crescer (TestJanelaPequena_*).
func TestAcao_SemListaNaTelaCancela(t *testing.T) {
	sair := map[string]func(Model) Model{
		"janela pequena": func(m Model) Model {
			m, _ = atualizar(m, tea.WindowSizeMsg{Width: 50, Height: 12})
			return m
		},
		"sem conexão": func(m Model) Model {
			for range maxRetries {
				m, _ = atualizar(m, errors.New("conexão recusada"))
			}
			return m
		},
	}
	for nome, fn := range sair {
		t.Run(nome, func(t *testing.T) {
			cs := fake.NewClientset(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "payments"}})
			feitas := chamadas(cs)
			m := comJanela(NewModel(&clientComAcesso{MockClient: &MockClient{}, cs: cs}))
			m.activeTab = TabDeployments
			m.deployments = []monitor.Deployment{{Name: "worker", Namespace: "payments"}}
			for _, antes := range [][]string{{"x"}, append([]string{"x", "d"}, strings.Split("worker", "")...)} {
				a, _ := teclas(t, m, antes...)
				a = fn(a)
				// Com a janela pequena, o aviso espera a janela crescer (cancelouAcao).
				avisou := strings.Contains(ansi.Strip(a.actionFeedback), "ação cancelada") || a.cancelouAcao
				if a.acao.aberta() || !avisou {
					t.Fatalf("%v: a ação deveria fechar (etapa %d, aviso %q)", antes, a.acao.etapa, a.actionFeedback)
				}
				for _, k := range append([]string{"d"}, append(strings.Split("worker", ""), "enter")...) {
					if _, cmd := teclas(t, a, k); cmd != nil {
						if _, ok := cmd().(resultadoAcaoMsg); ok {
							t.Fatalf("%v: a tecla %q mandou uma ação sem a lista na tela", antes, k)
						}
					}
					a, _ = teclas(t, a, k)
				}
				if teclasNaLegenda := ansi.Strip(a.View()); strings.Contains(teclasNaLegenda, "excluir  esc cancelar") {
					t.Fatalf("%v: a legenda ainda oferece o excluir:\n%s", antes, teclasNaLegenda)
				}
			}
			if len(*feitas) != 0 {
				t.Fatalf("nada deveria chegar à API: %v", *feitas)
			}
		})
	}
}

// TestAcao_UmaPorVez: enquanto a ação não responde, a linha de estado diz que
// ela está em andamento e outra ação no cluster não sai (copiar o nome sai); a
// resposta libera a próxima.
func TestAcao_UmaPorVez(t *testing.T) {
	cs := fake.NewClientset(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "loja"}})
	m := comJanela(NewModel(&clientComAcesso{MockClient: &MockClient{}, cs: cs}))
	m.activeTab = TabDeployments
	m.deployments = []monitor.Deployment{{Name: "api", Namespace: "loja", Replicas: 2}}
	m.copiar = func(string) error { return nil }
	m, primeira := teclas(t, m, "x", "r", "enter")
	if primeira == nil || m.executando != "reiniciando Deployment api" {
		t.Fatalf("o ↵ deveria mandar o reinício: executando %q", m.executando)
	}
	if v := ansi.Strip(m.View()); !strings.Contains(v, "◈ reiniciando Deployment api…") {
		t.Fatalf("a linha de estado deveria mostrar a ação em andamento:\n%s", v)
	}
	for _, ks := range [][]string{{"x", "r"}, {"x", "s"}, {"x", "d"}} {
		s, cmd := teclas(t, m, ks...)
		if cmd != nil || s.acao.aberta() || !strings.Contains(ansi.Strip(s.actionFeedback), "uma ação por vez") {
			t.Fatalf("%v com uma ação em andamento: etapa %d, aviso %q", ks, s.acao.etapa, s.actionFeedback)
		}
	}
	if _, cmd := teclas(t, m, "x", "c"); cmd == nil {
		t.Fatal("copiar o nome não espera a ação")
	}
	m, _ = atualizar(m, primeira())
	if m.executando != "" || !strings.Contains(ansi.Strip(m.View()), "✓ Deployment api reiniciado") {
		t.Fatalf("a resposta deveria liberar e ir para o Toast: executando %q", m.executando)
	}
	if _, cmd := teclas(t, m, "x", "r", "enter"); cmd == nil {
		t.Fatal("depois da resposta, a próxima ação sai")
	}
}

// TestAcao_FalhaResumida: o Toast da falha é curto e em português, como na
// prancha ("✕ sem permissão para escalar · RBAC"); o texto da API fica no log.
func TestAcao_FalhaResumida(t *testing.T) {
	gr := schema.GroupResource{Group: "apps", Resource: "deployments"}
	casos := []struct {
		err         error
		texto, nota string
	}{
		{apierrors.NewForbidden(gr, "worker", errors.New(`User "dev" cannot patch`)), "sem permissão para escalar", "RBAC"},
		{fmt.Errorf("falha ao obter scale do deployment 'worker': %w", apierrors.NewNotFound(gr, "worker")), "Deployment worker não existe mais", ""},
		{apierrors.NewConflict(gr, "worker", errors.New("modificado")), "Deployment worker não foi escalado", "ele mudou no meio; tente de novo"},
		{fmt.Errorf("falha: %w", context.DeadlineExceeded), "Deployment worker não foi escalado", "o cluster não respondeu a tempo"},
		{&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, "Deployment worker não foi escalado", "sem conexão com o cluster"},
		{fmt.Errorf("falha ao escalar deployment 'worker': %w", errors.New("quota excedida")), "não deu para escalar Deployment worker", "quota excedida"},
	}
	for _, c := range casos {
		if texto, nota := fraseDaFalha(acaoEscalar, "Deployment worker", c.err); texto != c.texto || nota != c.nota {
			t.Errorf("%v: %q · %q, esperado %q · %q", c.err, texto, nota, c.texto, c.nota)
		}
	}

	// Pelo menu, com o clientset falso negando o scale.
	cs := fake.NewClientset(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "payments"}})
	cs.PrependReactor("*", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(gr, "worker", errors.New(`User "dev" cannot patch resource`))
	})
	m := comJanela(NewModel(&clientComAcesso{MockClient: &MockClient{}, cs: cs}))
	m.activeTab = TabDeployments
	m.deployments = []monitor.Deployment{{Name: "worker", Namespace: "payments", Replicas: 2}}
	m, cmd := teclas(t, m, "x", "s", "3", "enter")
	m, _ = atualizar(m, cmd())
	if v := ansi.Strip(m.View()); !strings.Contains(v, "✕ sem permissão para escalar · RBAC") || strings.Contains(v, "forbidden") {
		t.Fatalf("o Toast deveria resumir a falha:\n%s", v)
	}
}

// TestAcao_FundoSemPainelPorCima: com o painel por cima aberto (80 a 119
// colunas), o menu tira o painel e a lista atrás ocupa o corpo todo, sem as
// linhas em branco da altura de quando o painel está na tela.
func TestAcao_FundoSemPainelPorCima(t *testing.T) {
	semCor(t)
	m := NewModel(&MockClient{})
	m.activeTab = TabDeployments
	for i := range 30 {
		m.deployments = append(m.deployments, monitor.Deployment{Name: fmt.Sprintf("dep-%02d", i), Namespace: "loja", Replicas: 1, Ready: 1})
	}
	m.carregadas = 1 << uint(TabDeployments)
	m, _ = atualizar(m, tea.WindowSizeMsg{Width: 90, Height: 30})
	m, _ = teclas(t, m, "j", "j", "j", "j", "j", "enter")
	if !m.painelPorCima() {
		t.Fatal("o ↵ deveria abrir o painel por cima")
	}
	m, _ = teclas(t, m, "x")
	v := ansi.Strip(m.View())
	for _, q := range []string{"AÇÕES · Deployment dep-05", "dep-00", "dep-19", "deployments abaixo"} {
		if !strings.Contains(v, q) {
			t.Errorf("o fundo do menu deveria mostrar %q:\n%s", q, v)
		}
	}
	if strings.Contains(v, "DEPLOYMENT · dep-05") {
		t.Errorf("o painel por cima deveria sair com o menu:\n%s", v)
	}
	// Fechando o menu, o painel e a geometria dele voltam.
	if f, _ := teclas(t, m, "esc"); !f.painelPorCima() || !strings.Contains(ansi.Strip(f.View()), "DEPLOYMENT · dep-05") {
		t.Fatal("o esc deveria voltar ao painel por cima")
	}
}

// telaDeAcoes é a tela da prancha na aba Deployments, com os seletores dos
// pods (o resumo da exclusão conta os pods do worker).
func telaDeAcoes(t *testing.T, largura, altura int) Model {
	t.Helper()
	m := telaDaPrancha(t, largura, altura)
	// Uma lista nova, como as que chegam do cluster: o Model não muda listas no lugar.
	deployments := append([]monitor.Deployment(nil), m.deployments...)
	for i := range deployments {
		deployments[i].Seletor = "app=" + deployments[i].Name
	}
	m.deployments = deployments
	m, _ = teclas(t, m, "2")
	return m
}

// TestTelaAcoes reproduz a folha "ações e confirmação": o menu x aberto no
// Deployment worker, por cima da lista apagada, e a exclusão com o nome
// digitado pela metade (o ↵ ainda apagado, "faltam 2 letras"), com o Toast da
// ação anterior na linha de estado.
func TestTelaAcoes(t *testing.T) {
	escuro := modo{"escuro", yui.TemaEscuro, false, termenv.TrueColor}
	ascii := modo{"ascii", yui.TemaEscuro, true, termenv.TrueColor}
	menu := []string{"AÇÕES · Deployment worker", "◆ r  reiniciar", "rollout restart, sem downtime", "hoje 2 réplicas",
		"copiar nome", "escalar…", "excluir…", "pede confirmação", "↵ escolher"}
	excluir := []string{"EXCLUIR DEPLOYMENT", "worker em payments · 2 réplicas · 1 pod", "Os pods param e o Deployment some",
		"Digite worker para confirmar", "faltam 2 letras", "Deployment api reiniciado", "excluir  esc cancelar"}
	casos := []struct {
		nome            string
		md              modo
		largura, altura int
		teclas          []string
		quer            []string
	}{
		{"menu", escuro, 100, 30, []string{"x"}, menu},
		{"menu", escuro, 140, 32, []string{"x"}, menu}, // o painel ao lado fica apagado
		{"menu", escuro, 72, 24, []string{"x"}, menu},
		// No ASCII, as marcas e as reticências dos rótulos seguem o modo.
		{"menu", ascii, 100, 30, []string{"x"}, []string{"AÇÕES · Deployment worker", "* r  reiniciar", "o s  escalar~", "o d  excluir~"}},
		{"excluir", escuro, 100, 30, []string{"x", "d", "w", "o", "r", "k"}, excluir},
		{"excluir", ascii, 100, 30, []string{"x", "d", "w", "o", "r", "k"}, []string{"EXCLUIR DEPLOYMENT", "[ enter excluir ]"}},
		// Sem altura para tudo, a explicação sai.
		{"excluir", escuro, 60, 16, []string{"x", "d", "w", "o", "r", "k"}, []string{"EXCLUIR DEPLOYMENT", "faltam 2 letras"}},
		{"reiniciar", escuro, 100, 30, []string{"x", "r"}, []string{"REINICIAR DEPLOYMENT", "sem downtime", "↵ reiniciar"}},
		{"escalar", escuro, 100, 30, []string{"x", "s", "5"}, []string{"ESCALAR DEPLOYMENT", "Quantas réplicas?", "› 5", "↵ escalar"}},
	}
	for _, c := range casos {
		t.Run(fmt.Sprintf("%s_%s_%dx%d", c.nome, c.md.nome, c.largura, c.altura), func(t *testing.T) {
			usarModo(t, c.md)
			m := telaDeAcoes(t, c.largura, c.altura)
			if c.nome == "excluir" {
				m, _ = atualizar(m, resultadoAcaoMsg{texto: "Deployment api reiniciado"})
			}
			m, _ = teclas(t, m, c.teclas...)
			tela := ansi.Strip(conferirTela(t, m, c.largura, c.altura))
			for _, q := range c.quer {
				if !strings.Contains(tela, q) {
					t.Errorf("a tela deveria mostrar %q", q)
				}
			}
		})
	}
}

// TestAcao_ExcluirMandaOUIDDaLinha: o UID do objeto da linha chega à API como
// precondição da exclusão; o homônimo recriado (UID outro) não é apagado.
func TestAcao_ExcluirMandaOUIDDaLinha(t *testing.T) {
	cs := fake.NewClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-0", Namespace: "loja"}})
	var uid string
	cs.PrependReactor("delete", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if p := a.(k8stesting.DeleteAction).GetDeleteOptions().Preconditions; p != nil && p.UID != nil {
			uid = string(*p.UID)
		}
		return true, nil, nil
	})
	m := NewModel(&clientComAcesso{MockClient: &MockClient{}, cs: cs})
	alvo := m.alvoDaLinha("pod", itemLista{nome: "api-0", namespace: "loja", obj: monitor.Pod{Name: "api-0", Namespace: "loja", UID: "uid-do-pod"}})
	if alvo.uid != "uid-do-pod" {
		t.Fatalf("alvo.uid = %q", alvo.uid)
	}
	if r := fazer(m.client, acaoExcluir, alvo, 0); r.falha {
		t.Fatalf("falhou: %+v", r)
	}
	if uid != "uid-do-pod" {
		t.Errorf("a API recebeu o UID %q", uid)
	}
}
