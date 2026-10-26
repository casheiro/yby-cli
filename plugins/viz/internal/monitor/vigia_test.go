package monitor

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// prazoDoWatch é quanto um teste espera um lote antes de desistir.
const prazoDoWatch = 5 * time.Second

func podEm(ns, nome string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: nome, Namespace: ns}}
}

// vigiado é o client sobre o fake, com lotes a cada 10 ms, e o número de
// watches abertos no fake (o objeto criado antes do watch abrir não gera evento).
func vigiado(objs ...k8sruntime.Object) (*K8sClient, *fake.Clientset, *atomic.Int32) {
	cs := fake.NewClientset(objs...)
	abertos := &atomic.Int32{}
	cs.PrependWatchReactor("*", func(k8stesting.Action) (bool, watch.Interface, error) {
		abertos.Add(1)
		return false, nil, nil
	})
	return &K8sClient{clientset: cs, intervaloLote: 10 * time.Millisecond}, cs, abertos
}

// ultimos junta os lotes do canal, guardando o retrato mais novo de cada
// tipo, até pronto aceitar ou o prazo acabar.
func ultimos(t *testing.T, ch <-chan Lote, vistos map[string]Retrato, pronto func(map[string]Retrato) bool) map[string]Retrato {
	t.Helper()
	prazo := time.After(prazoDoWatch)
	for !pronto(vistos) {
		select {
		case l, ok := <-ch:
			if !ok {
				t.Fatal("o canal fechou antes da hora")
			}
			for _, r := range l.Retratos {
				vistos[r.Kind] = r
			}
		case <-prazo:
			t.Fatalf("o watch não chegou lá: %v", vistos)
		}
	}
	return vistos
}

// todosOsTipos aceita quando cada tipo já mandou o primeiro retrato.
func todosOsTipos(v map[string]Retrato) bool { return len(v) == len(TiposObservados) }

// quantosPods aceita quando a lista de pods tem n itens.
func quantosPods(n int) func(map[string]Retrato) bool {
	return func(v map[string]Retrato) bool {
		pods, ok := v["pod"].Lista.([]Pod)
		return ok && len(pods) == n
	}
}

// TestVigia_ContagemSegueOCluster: o primeiro lote traz todos os tipos do
// namespace (os outros namespaces não entram); criar e apagar um pod no
// cluster chega num lote, sem leitura nenhuma de quem observa.
func TestVigia_ContagemSegueOCluster(t *testing.T) {
	c, cs, abertos := vigiado(podEm("payments", "web-2"), podEm("payments", "web-1"), podEm("outro", "x"),
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "payments"}})
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	ch := c.Observar(ctx, "payments")
	vistos := ultimos(t, ch, map[string]Retrato{}, todosOsTipos)
	pods := vistos["pod"].Lista.([]Pod)
	if len(pods) != 2 || pods[0].Name != "web-1" || pods[1].Name != "web-2" {
		t.Fatalf("pods de payments, na ordem da API: %+v", pods)
	}
	if deps := vistos["deployment"].Lista.([]Deployment); len(deps) != 1 || deps[0].Name != "api" {
		t.Fatalf("deployments: %+v", deps)
	}
	for _, kind := range TiposObservados {
		if vistos[kind].Err != nil {
			t.Fatalf("%s: %v", kind, vistos[kind].Err)
		}
	}
	for abertos.Load() < int32(len(TiposObservados)) {
		time.Sleep(time.Millisecond)
	}

	if _, err := cs.CoreV1().Pods("payments").Create(ctx, podEm("payments", "web-3"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	ultimos(t, ch, vistos, quantosPods(3))
	if err := cs.CoreV1().Pods("payments").Delete(ctx, "web-1", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	ultimos(t, ch, vistos, quantosPods(2))
	// Um pod em outro namespace não muda a lista de payments: o lote seguinte
	// (do deployment criado depois) chega com os pods como estavam.
	if _, err := cs.CoreV1().Pods("outro").Create(ctx, podEm("outro", "y"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.AppsV1().Deployments("payments").Create(ctx,
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "payments"}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	ultimos(t, ch, vistos, func(v map[string]Retrato) bool { return len(v["deployment"].Lista.([]Deployment)) == 2 })
	if pods := vistos["pod"].Lista.([]Pod); len(pods) != 2 {
		t.Fatalf("o pod de outro namespace entrou em payments: %+v", pods)
	}
}

// TestVigia_TodosOsNamespaces: o namespace vazio acompanha o cluster inteiro.
func TestVigia_TodosOsNamespaces(t *testing.T) {
	c, _, _ := vigiado(podEm("payments", "web-1"), podEm("outro", "x"))
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	vistos := ultimos(t, c.Observar(ctx, ""), map[string]Retrato{}, todosOsTipos)
	if pods := vistos["pod"].Lista.([]Pod); len(pods) != 2 || pods[0].Namespace != "outro" {
		t.Fatalf("pods do cluster, por namespace: %+v", pods)
	}
}

// TestVigia_SemPermissao: o tipo que o RBAC não deixa listar chega como
// negado, uma vez, e o watch dele para; os outros tipos seguem.
func TestVigia_SemPermissao(t *testing.T) {
	c, cs, _ := vigiado(podEm("payments", "web-1"))
	listas := &atomic.Int32{}
	cs.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, k8sruntime.Object, error) {
		listas.Add(1)
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "nodes"}, "", nil)
	})
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	ch := c.Observar(ctx, "payments")
	vistos := ultimos(t, ch, map[string]Retrato{}, todosOsTipos)
	if r := vistos["node"]; !r.Negado || r.Err == nil || r.Lista != nil {
		t.Fatalf("nós sem permissão: %+v", r)
	}
	if pods, ok := vistos["pod"].Lista.([]Pod); !ok || len(pods) != 1 {
		t.Fatalf("os pods seguem: %+v", vistos["pod"])
	}
	// O informer dos nós parou: passado o primeiro recuo do reflector, ele não lista de novo.
	time.Sleep(1500 * time.Millisecond)
	if n := listas.Load(); n != 1 {
		t.Fatalf("o watch negado listou %d vezes", n)
	}
}

// TestVigia_ParadaLimpa: cancelar o contexto para os informers e fecha o
// canal, sem goroutine sobrando.
func TestVigia_ParadaLimpa(t *testing.T) {
	antes := runtime.NumGoroutine()
	c, _, _ := vigiado(podEm("payments", "web-1"))
	ctx, cancelar := context.WithCancel(context.Background())
	ch := c.Observar(ctx, "payments")
	ultimos(t, ch, map[string]Retrato{}, todosOsTipos)
	cancelar()
	prazo := time.After(prazoDoWatch)
	for aberto := true; aberto; {
		select {
		case _, aberto = <-ch:
		case <-prazo:
			t.Fatal("o canal não fechou depois do cancelamento")
		}
	}
	for fim := time.Now().Add(prazoDoWatch); runtime.NumGoroutine() > antes; {
		if time.Now().After(fim) {
			t.Fatalf("goroutines: %d antes, %d depois", antes, runtime.NumGoroutine())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestVigia_RetryRepassa: o RetryClient observa pelo client de dentro; sem
// watch no de dentro, não há canal.
func TestVigia_RetryRepassa(t *testing.T) {
	c, _, _ := vigiado()
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	if ch := NewRetryClient(c).Observar(ctx, "payments"); ch == nil {
		t.Fatal("o RetryClient deveria repassar o watch")
	}
	if ch := NewRetryClient(&FakeClient{}).Observar(ctx, "payments"); ch != nil {
		t.Fatal("sem watch no client de dentro, o canal é nil")
	}
}
