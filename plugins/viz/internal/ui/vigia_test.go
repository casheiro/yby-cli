package ui

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// As contagens das abas pelo watch, com o clientset falso do client-go: o
// K8sClient de verdade (monitor.NewK8sClientDe) e nenhum relógio.

const prazoDoLote = 5 * time.Second

func podDe(ns, nome string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: nome, Namespace: ns}}
}

func deploymentDe(ns, nome string) *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: nome, Namespace: ns}}
}

// clusterFalso é o clientset falso e quantos watches já abriram nele (o
// objeto criado antes do watch abrir não gera evento no falso).
type clusterFalso struct {
	cs      *fake.Clientset
	abertos *atomic.Int32
}

func novoCluster(objs ...runtime.Object) clusterFalso {
	c := clusterFalso{cs: fake.NewClientset(objs...), abertos: &atomic.Int32{}}
	c.cs.PrependWatchReactor("*", func(k8stesting.Action) (bool, watch.Interface, error) {
		c.abertos.Add(1)
		return false, nil, nil
	})
	return c
}

// esperarWatches espera n watches abertos no falso.
func (c clusterFalso) esperarWatches(t *testing.T, n int) {
	t.Helper()
	for fim := time.Now().Add(prazoDoLote); c.abertos.Load() < int32(n); time.Sleep(time.Millisecond) {
		if time.Now().After(fim) {
			t.Fatalf("só %d de %d watches abriram", c.abertos.Load(), n)
		}
	}
}

// vizVigiado é o Viz sobre o K8sClient do falso, sem relógio, no namespace dado.
func vizVigiado(c clusterFalso, ns string) Model {
	m := comJanela(NewModel(monitor.NewK8sClientDe(c.cs)))
	m.filter.Namespace = ns
	m.tique = nil
	return m
}

// bomba roda os comandos do modelo como o Bubble Tea: cada um numa goroutine,
// as mensagens voltam para o Update (BatchMsg abre os comandos de dentro).
type bomba struct {
	msgs chan tea.Msg
}

func novaBomba() *bomba { return &bomba{msgs: make(chan tea.Msg, 256)} }

func (b *bomba) rodar(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() { b.msgs <- cmd() }()
}

// ate entrega as mensagens ao modelo até pronto aceitar ou o prazo acabar.
func (b *bomba) ate(t *testing.T, m Model, pronto func(Model) bool) Model {
	t.Helper()
	prazo := time.After(prazoDoLote)
	for !pronto(m) {
		select {
		case msg := <-b.msgs:
			if lote, ok := msg.(tea.BatchMsg); ok {
				for _, c := range lote {
					b.rodar(c)
				}
				continue
			}
			var cmd tea.Cmd
			m, cmd = atualizar(m, msg)
			b.rodar(cmd)
		case <-prazo:
			normais, _ := m.linhaAbas()
			t.Fatalf("o modelo não chegou lá; abas: %s", ansi.Strip(normais))
		}
	}
	return m
}

// abrir manda o vigiarMsg do Init e devolve a bomba com o watch aberto.
func abrir(t *testing.T, m Model) (Model, *bomba) {
	t.Helper()
	b := novaBomba()
	m, cmd := atualizar(m, vigiarMsg{})
	if cmd == nil || m.vigia.canal == nil {
		t.Fatal("o vigiarMsg deveria abrir o watch")
	}
	b.rodar(cmd)
	return m, b
}

// contagem é o número da aba na tela (-1 sem número), como linhaAbas o calcula.
func contagem(m Model, tab ResourceTab) int {
	if !m.carregada(tab) {
		return -1
	}
	return len(m.itensDaAba(tab))
}

// contagens aceita quando as abas têm os números dados.
func contagens(quer map[ResourceTab]int) func(Model) bool {
	return func(m Model) bool {
		for tab, n := range quer {
			if contagem(m, tab) != n {
				return false
			}
		}
		return true
	}
}

// fechou espera o canal do watch fechar (o watch parou por inteiro).
func fechou(t *testing.T, canal <-chan monitor.Lote) {
	t.Helper()
	prazo := time.After(prazoDoLote)
	for {
		select {
		case _, aberto := <-canal:
			if !aberto {
				return
			}
		case <-prazo:
			t.Fatal("o watch não parou")
		}
	}
}

// TestVigia_ContagemSemRelogio: o Init abre o watch e as nove abas ganham
// número sem leitura nenhuma; criar e apagar no cluster muda o número da aba
// de fundo na hora, sem relógio, e a contagem segue os filtros da tela.
func TestVigia_ContagemSemRelogio(t *testing.T) {
	c := novoCluster(podDe("payments", "web-1"), podDe("payments", "api-1"), podDe("outro", "x"),
		deploymentDe("payments", "api"))
	m := vizVigiado(c, "payments")
	if _, ok := m.Init()().(tea.BatchMsg); !ok {
		t.Fatal("com um client que observa, o Init lê e abre o watch")
	}
	m, b := abrir(t, m)
	defer m.Parar()
	m = b.ate(t, m, contagens(map[ResourceTab]int{TabPods: 2, TabDeployments: 1, TabServices: 0, TabNodes: 0, TabEvents: 0}))
	for tab := ResourceTab(0); tab < tabCount; tab++ {
		if !m.carregada(tab) || !m.vigiada(tab) {
			t.Fatalf("a aba %s deveria vir do watch", tab)
		}
	}
	c.esperarWatches(t, len(monitor.TiposObservados))

	ctx := t.Context()
	if _, err := c.cs.AppsV1().Deployments("payments").Create(ctx, deploymentDe("payments", "worker"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	m = b.ate(t, m, contagens(map[ResourceTab]int{TabDeployments: 2}))
	if err := c.cs.CoreV1().Pods("payments").Delete(ctx, "web-1", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	m = b.ate(t, m, contagens(map[ResourceTab]int{TabPods: 1}))
	usarModo(t, modo{"semcor", yui.TemaEscuro, false, termenv.Ascii})
	if normais, _ := m.linhaAbas(); !strings.Contains(ansi.Strip(normais), "2 Deployments 2") {
		t.Fatalf("a aba de fundo deveria mostrar 2: %s", ansi.Strip(normais))
	}

	// A contagem é a da lista que a aba mostraria: a busca vale em todas as abas.
	m = buscar(t, m, "wor")
	if contagem(m, TabDeployments) != 1 || contagem(m, TabPods) != 0 {
		t.Fatalf("com a busca: deployments %d, pods %d", contagem(m, TabDeployments), contagem(m, TabPods))
	}
}

// TestVigia_LeituraDoRelogioNaoTrocaALista: com a aba no watch, a leitura do
// relógio só confirma a conexão; sem o watch na aba, ela volta a ser a lista.
func TestVigia_LeituraDoRelogioNaoTrocaALista(t *testing.T) {
	m := comJanela(NewModel(&MockClient{}))
	m, _ = atualizar(m, muitosPods(3))
	m.vigiadas = 1 << uint(TabPods)
	m.reconnecting, m.retryCount = true, 1
	m, _ = atualizar(m, leituraMsg{abas: []ResourceTab{TabPods}, respostas: []tea.Msg{muitosPods(5)}})
	if len(m.pods) != 3 || m.reconnecting || m.retryCount != 0 {
		t.Fatalf("a leitura trocou a lista do watch: %d pods, reconectando %v", len(m.pods), m.reconnecting)
	}
	m.vigiadas = 0
	if m, _ = atualizar(m, leituraMsg{abas: []ResourceTab{TabPods}, respostas: []tea.Msg{muitosPods(5)}}); len(m.pods) != 5 {
		t.Fatalf("sem watch na aba, a leitura é a lista: %d pods", len(m.pods))
	}
}

// TestVigia_RelogioNaoListaAbaVigiada: com a aba no watch, o tique não faz
// LIST de pods no servidor (só a sonda de conexão); sem o watch, lê a lista.
func TestVigia_RelogioNaoListaAbaVigiada(t *testing.T) {
	c := novoCluster(podDe("payments", "web-1"))
	m, b := abrir(t, vizVigiado(c, "payments"))
	defer m.Parar()
	m = b.ate(t, m, contagens(map[ResourceTab]int{TabPods: 1}))
	c.esperarWatches(t, len(monitor.TiposObservados))

	var listas atomic.Int32
	c.cs.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		listas.Add(1)
		return false, nil, nil
	})
	tique := func(m Model) Model {
		t.Helper()
		m, cmd := atualizar(m, tickMsg(time.Now()))
		// O Batch com um comando só devolve o próprio comando.
		msgs := []tea.Msg{}
		if cmd != nil {
			msgs = append(msgs, cmd())
		}
		if lote, ok := msgs[0].(tea.BatchMsg); ok {
			msgs = msgs[:0]
			for _, sub := range lote {
				if sub != nil {
					msgs = append(msgs, sub())
				}
			}
		}
		for _, msg := range msgs {
			if msg, ok := msg.(leituraMsg); ok {
				m, _ = atualizar(m, msg)
			}
		}
		return m
	}

	m.reconnecting, m.retryCount = true, 1
	m = tique(m)
	if n := listas.Load(); n != 0 {
		t.Fatalf("o tique listou pods %d vez(es) com a aba no watch", n)
	}
	if m.reconnecting || len(m.pods) != 1 {
		t.Fatalf("a sonda deveria confirmar a conexão sem trocar a lista: reconectando %v, %d pods", m.reconnecting, len(m.pods))
	}

	m.vigiadas = 0
	tique(m)
	if n := listas.Load(); n != 1 {
		t.Fatalf("sem watch na aba, o tique deveria listar pods uma vez; foram %d", n)
	}
}

// clienteQueCaiu é um client que observa e sonda, e cuja sonda falha.
type clienteQueCaiu struct{ *MockClient }

func (clienteQueCaiu) Sondar() error { return fmt.Errorf("sem rota para o cluster") }

// TestVigia_SondaQueFalhaReconecta: a queda do cluster com o watch parado
// ainda aparece, pela sonda do relógio.
func TestVigia_SondaQueFalhaReconecta(t *testing.T) {
	m := comJanela(NewModel(clienteQueCaiu{&MockClient{}}))
	m, _ = atualizar(m, muitosPods(3))
	m.vigiadas = 1 << uint(TabPods)
	msg, ok := m.ler(true, TabPods)().(leituraMsg)
	if !ok || len(msg.respostas) != 1 {
		t.Fatalf("leitura inesperada: %#v", msg)
	}
	if _, ok := msg.respostas[0].(error); !ok {
		t.Fatalf("a sonda que falha deveria virar erro: %#v", msg.respostas[0])
	}
	m, _ = atualizar(m, msg)
	if !m.reconnecting || len(m.pods) != 3 {
		t.Fatalf("deveria reconectar sem perder a lista: reconectando %v, %d pods", m.reconnecting, len(m.pods))
	}
}

// TestVigia_TrocaDeNamespace: o n para o watch do namespace anterior e abre
// outro no novo; um lote atrasado do anterior não vale.
func TestVigia_TrocaDeNamespace(t *testing.T) {
	c := novoCluster(podDe("payments", "web-1"), podDe("payments", "web-2"), podDe("outro", "x"))
	m, b := abrir(t, vizVigiado(c, "payments"))
	defer func() { m.Parar() }()
	m = b.ate(t, m, contagens(map[ResourceTab]int{TabPods: 2}))
	antigo, idAntigo := m.vigia.canal, m.vigia.id

	m, cmd := teclas(t, m, "n", "o", "u", "t", "r", "o", "enter")
	if m.vigia.id == idAntigo || m.vigia.canal == antigo {
		t.Fatal("o namespace novo deveria abrir outro watch")
	}
	fechou(t, antigo)
	// Um lote do watch anterior que chegasse agora seria descartado.
	velho := loteMsg{id: idAntigo, aberto: true, lote: monitor.Lote{Namespace: "payments",
		Retratos: []monitor.Retrato{{Kind: "pod", Lista: muitosPods(9)}}}}
	if m2, c2 := atualizar(m, velho); m2.carregada(TabPods) || c2 != nil {
		t.Fatal("o lote do watch anterior não vale para o namespace novo")
	}
	b.rodar(cmd)
	m = b.ate(t, m, contagens(map[ResourceTab]int{TabPods: 1, TabDeployments: 0}))
	if m.pods[0].Name != "x" || m.filter.Namespace != "outro" {
		t.Fatalf("pods de outro: %+v", m.pods)
	}
}

// TestVigia_SairParaOWatch: o q (e o ctrl+c) para o watch antes de sair.
func TestVigia_SairParaOWatch(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		c := novoCluster(podDe("payments", "web-1"))
		m, b := abrir(t, vizVigiado(c, "payments"))
		m = b.ate(t, m, contagens(map[ResourceTab]int{TabPods: 1}))
		_, cmd := teclas(t, m, k)
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%s deveria sair", k)
		}
		fechou(t, m.vigia.canal)
	}
}

// TestVigia_SemPermissao: o tipo que o RBAC não deixa listar deixa a aba sem
// número, sem derrubar a tela; as outras abas seguem com número.
func TestVigia_SemPermissao(t *testing.T) {
	c := novoCluster(podDe("payments", "web-1"))
	c.cs.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "nodes"}, "", fmt.Errorf("RBAC"))
	})
	m, b := abrir(t, vizVigiado(c, "payments"))
	defer m.Parar()
	m = b.ate(t, m, func(m Model) bool { return contagem(m, TabPods) == 1 && contagem(m, TabEvents) == 0 })
	if m.carregada(TabNodes) || m.vigiada(TabNodes) || m.err != nil || m.reconnecting {
		t.Fatalf("nós negados: carregada %v, vigiada %v, err %v", m.carregada(TabNodes), m.vigiada(TabNodes), m.err)
	}
	usarModo(t, modo{"semcor", yui.TemaEscuro, false, termenv.Ascii})
	if normais, _ := m.linhaAbas(); !strings.Contains(ansi.Strip(normais), "5 Nodes   ") || !strings.Contains(ansi.Strip(normais), "1 Pods 1") {
		t.Fatalf("a aba Nodes fica sem número: %s", ansi.Strip(normais))
	}

	// Um negado que chega para uma aba que já tinha lista apaga o número dela.
	m.vigiadas |= 1 << uint(TabJobs)
	negado := loteMsg{id: m.vigia.id, aberto: true, lote: monitor.Lote{Namespace: "payments",
		Retratos: []monitor.Retrato{{Kind: "job", Err: fmt.Errorf("forbidden"), Negado: true}}}}
	if m, _ = atualizar(m, negado); m.carregada(TabJobs) || m.vigiada(TabJobs) {
		t.Fatal("a aba negada deveria perder a lista")
	}
}
