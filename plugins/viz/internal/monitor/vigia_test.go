package monitor

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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

// ultimaAplicada é a annotation que o kubectl apply grava com o manifesto inteiro.
const ultimaAplicada = "kubectl.kubernetes.io/last-applied-configuration"

// TestParaOCache_ConfigMap: o ConfigMap entra no cache sem os valores, e a aba
// mostra as mesmas chaves e o mesmo tamanho de antes. O transform pode rodar de
// novo sobre o objeto já enxuto sem zerar o tamanho.
func TestParaOCache_ConfigMap(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "payments", Annotations: map[string]string{
			ultimaAplicada: strings.Repeat("{}", 512), "dono": "time-a",
		}},
		Data:       map[string]string{"a": strings.Repeat("x", 2048)},
		BinaryData: map[string][]byte{"b": []byte("0123456789")},
	}
	esperado := configMapDeK8s(cm.DeepCopy())
	if esperado.Keys != 2 || esperado.DataSize != "2.0Ki" {
		t.Fatalf("conversão do objeto inteiro: %+v", esperado)
	}
	for vez := 1; vez <= 2; vez++ {
		o, err := paraOCache(cm)
		if err != nil {
			t.Fatal(err)
		}
		cm = o.(*corev1.ConfigMap)
		if cm.Data["a"] != "" || cm.BinaryData["b"] != nil {
			t.Fatalf("vez %d: o cache guardou os valores: %q %q", vez, cm.Data["a"], cm.BinaryData["b"])
		}
		if _, ok := cm.Annotations[ultimaAplicada]; ok || cm.Annotations["dono"] != "time-a" {
			t.Fatalf("vez %d: annotations: %v", vez, cm.Annotations)
		}
		if got := configMapDeK8s(cm); !reflect.DeepEqual(got, esperado) {
			t.Fatalf("vez %d: a aba mudou:\n%+v\n%+v", vez, got, esperado)
		}
	}
}

// TestParaOCache_Pod: o Pod entra no cache sem env, volumes, args e command
// (nem nos de init), e a conversão dá o mesmo pod de antes.
func TestParaOCache_Pod(t *testing.T) {
	conteiner := func(nome string) corev1.Container {
		return corev1.Container{
			Name: nome, Image: "registry.local/" + nome + ":1",
			Command: []string{"/bin/app"}, Args: []string{"--porta", "8080"},
			Env:          []corev1.EnvVar{{Name: "SENHA", Value: "x"}},
			EnvFrom:      []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "app"}}}},
			VolumeMounts: []corev1.VolumeMount{{Name: "dados", MountPath: "/dados"}},
			Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("64Mi"),
			}},
		}
	}
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-1", Namespace: "payments", Labels: map[string]string{"pod-template-hash": "abc"},
			Annotations:     map[string]string{ultimaAplicada: "{}", "kubectl.kubernetes.io/default-container": "app"},
			ManagedFields:   []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "web-abc", Controller: new(true)}},
		},
		Spec: corev1.PodSpec{
			NodeName:       "no-1",
			InitContainers: []corev1.Container{conteiner("migra")},
			Containers:     []corev1.Container{conteiner("app"), conteiner("istio-proxy")},
			Volumes:        []corev1.Volume{{Name: "dados", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.1", ContainerStatuses: []corev1.ContainerStatus{
			{Name: "app", Ready: true, RestartCount: 2, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
		}},
	}
	esperado := podDeK8s(p.DeepCopy())
	o, err := paraOCache(p)
	if err != nil {
		t.Fatal(err)
	}
	p = o.(*corev1.Pod)
	if p.Spec.Volumes != nil || p.ManagedFields != nil {
		t.Fatalf("volumes ou managedFields no cache: %+v", p)
	}
	if _, ok := p.Annotations[ultimaAplicada]; ok {
		t.Fatal("a last-applied ficou no cache")
	}
	for _, c := range append(append([]corev1.Container(nil), p.Spec.InitContainers...), p.Spec.Containers...) {
		if c.Env != nil || c.EnvFrom != nil || c.Command != nil || c.Args != nil || c.VolumeMounts != nil {
			t.Fatalf("o contêiner %s ficou inteiro no cache: %+v", c.Name, c)
		}
	}
	if got := podDeK8s(p); !reflect.DeepEqual(got, esperado) {
		t.Fatalf("a conversão mudou:\n%+v\n%+v", got, esperado)
	}
}

// TestParaOCache_SoALastApplied: nos outros tipos sai a last-applied (e as
// annotations, se só havia ela) e os managedFields; o resto fica.
func TestParaOCache_SoALastApplied(t *testing.T) {
	d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: "api", Namespace: "payments", Annotations: map[string]string{ultimaAplicada: "{}"},
		ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}}, Labels: map[string]string{"app": "api"},
	}}
	o, err := paraOCache(d)
	if err != nil {
		t.Fatal(err)
	}
	if d := o.(*appsv1.Deployment); d.Annotations != nil || d.ManagedFields != nil || d.Labels["app"] != "api" {
		t.Fatalf("deployment no cache: %+v", d.ObjectMeta)
	}
}

// TestVigia_ConfigMapPeloCache: pelo watch, a aba ConfigMaps mostra as chaves
// e o tamanho de quando o objeto chegou inteiro.
func TestVigia_ConfigMapPeloCache(t *testing.T) {
	c, _, _ := vigiado(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "payments"},
		Data:       map[string]string{"b": strings.Repeat("x", 2048), "a": "1"},
	})
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	vistos := ultimos(t, c.Observar(ctx, "payments"), map[string]Retrato{}, todosOsTipos)
	cms := vistos["configmap"].Lista.([]ConfigMap)
	if len(cms) != 1 || cms[0].Keys != 2 || cms[0].DataSize != "2.0Ki" || !reflect.DeepEqual(cms[0].Chaves, []string{"a", "b"}) {
		t.Fatalf("configmaps pelo watch: %+v", cms)
	}
}
