package monitor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

// TestMotivoDoPod: o motivo explica o estado, do mais grave ao mais brando.
func TestMotivoDoPod(t *testing.T) {
	casos := []struct {
		nome string
		pod  corev1.Pod
		quer string
	}{
		{"terminou com erro", corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
			{State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}}}}}},
			"OOMKilled · código de saída 137"},
		{"esperando para reiniciar", corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
			{State: esperando("CrashLoopBackOff"), LastTerminationState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{ExitCode: 2}}}}}},
			"terminou · código de saída 2"},
		{"imagem que não baixa", corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
			{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "Back-off pulling image \"x\""}}}}}},
			"Back-off pulling image \"x\""},
		{"sem nó", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{
			{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable", Message: "0/3 nodes are available"}}}},
			"0/3 nodes are available"},
		{"despejado", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed, Reason: "Evicted", Message: "The node was low on memory."}},
			"The node was low on memory."},
		{"concluído sem erro", corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
			{State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed"}}}}}}, ""},
		{"rodando", corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Ready: true,
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := motivoDoPod(&c.pod); got != c.quer {
				t.Fatalf("motivo = %q, esperado %q", got, c.quer)
			}
		})
	}
}

// TestDonoDoPod: o controlador vale mais que outro dono; o ReplicaSet de um
// Deployment vira o Deployment só com o pod-template-hash no fim do nome.
func TestDonoDoPod(t *testing.T) {
	falso := false
	pod := func(hash string, donos ...metav1.OwnerReference) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"pod-template-hash": hash}, OwnerReferences: donos}}
	}
	casos := map[string]struct {
		pod  *corev1.Pod
		quer string
	}{
		"sem dono":         {pod(""), ""},
		"deployment":       {pod("5d8b6", metav1.OwnerReference{Kind: "ReplicaSet", Name: "worker-5d8b6", Controller: &verdade}), "Deployment worker"},
		"replicaset solto": {pod("", metav1.OwnerReference{Kind: "ReplicaSet", Name: "avulso"}), "ReplicaSet avulso"},
		"controlador primeiro": {pod("", metav1.OwnerReference{Kind: "ConfigMap", Name: "x", Controller: &falso},
			metav1.OwnerReference{Kind: "Job", Name: "migrate-28311", Controller: &verdade}), "Job migrate-28311"},
	}
	for nome, c := range casos {
		if got := donoDoPod(c.pod); got != c.quer {
			t.Errorf("%s: dono = %q, esperado %q", nome, got, c.quer)
		}
	}
}

// TestK8sClient_CamposDoDetalhe: o que o painel mostra de cada tipo vem da
// mesma leitura da lista, sem chamada a mais.
func TestK8sClient_CamposDoDetalhe(t *testing.T) {
	ns := "payments"
	surge, indisponivel := intstr.FromString("25%"), intstr.FromInt32(0)
	um := int32(1)
	template := corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "worker:1.8.2"}}}}
	seletor := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "worker"}}
	c := &K8sClient{clientset: fake.NewClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: ns}, Spec: appsv1.DeploymentSpec{
			Replicas: &um, Selector: seletor, Template: template,
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{MaxSurge: &surge, MaxUnavailable: &indisponivel}}}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: ns}, Spec: appsv1.StatefulSetSpec{
			ServiceName: "redis-headless", Selector: seletor, Template: template}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: ns}, Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer, Selector: map[string]string{"tier": "front", "app": "web"}, ExternalIPs: []string{"1.2.3.4"}},
			Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: []corev1.LoadBalancerIngress{{Hostname: "lb.exemplo"}}}}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "ip-10-0-1-5", Labels: map[string]string{
			"node-role.kubernetes.io/control-plane": "", "node-role.kubernetes.io/etcd": "", "kubernetes.io/os": "linux"}},
			Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeHostName, Address: "h"}, {Type: corev1.NodeInternalIP, Address: "10.0.1.5"}}}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: ns}, Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "10.43.0.9"}}}}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: ns},
			Data: map[string]string{"z.yaml": "a", "LOG_LEVEL": "info"}, BinaryData: map[string][]byte{"cert": {1}}},
	)}

	deps, _ := c.GetDeployments(ListFilter{})
	if d := deps[0]; d.Estrategia != "RollingUpdate · surge 25% · indisponível 0" || d.Seletor != "app=worker" ||
		!reflect.DeepEqual(d.Imagens, []string{"worker:1.8.2"}) {
		t.Errorf("deployment: %+v", d)
	}
	sets, _ := c.GetStatefulSets(ListFilter{})
	if s := sets[0]; s.Servico != "redis-headless" || s.Seletor != "app=worker" || len(s.Imagens) != 1 {
		t.Errorf("statefulset: %+v", s)
	}
	svcs, _ := c.GetServices(ListFilter{})
	if s := svcs[0]; s.Seletor != "app=web,tier=front" || s.Externos != "1.2.3.4, lb.exemplo" {
		t.Errorf("service: %+v", s)
	}
	nodes, _ := c.GetNodes(ListFilter{})
	if n := nodes[0]; n.Papeis != "control-plane, etcd" || n.IPInterno != "10.0.1.5" {
		t.Errorf("nó: %+v", n)
	}
	ings, _ := c.GetIngresses(ListFilter{})
	if i := ings[0]; i.Endereco != "10.43.0.9" {
		t.Errorf("ingress: %+v", i)
	}
	cms, _ := c.GetConfigMaps(ListFilter{})
	if cm := cms[0]; !reflect.DeepEqual(cm.Chaves, []string{"LOG_LEVEL", "cert", "z.yaml"}) || cm.Keys != 3 {
		t.Errorf("configmap: %+v", cm)
	}
}

// evento monta um evento do objeto kind/nome com a última ocorrência dada.
func evento(id, kind, nome, tipo, razao string, quando time.Time, vezes int32) *corev1.Event {
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: id, Namespace: "payments"},
		InvolvedObject: corev1.ObjectReference{Kind: kind, Name: nome, Namespace: "payments"},
		Type:           tipo, Reason: razao, Message: razao + " de " + nome,
		LastTimestamp: metav1.NewTime(quando), Count: vezes,
	}
}

// TestK8sClient_EventosDoObjeto: só os eventos do objeto (nome e tipo), dos
// mais recentes para os mais antigos, até o limite, com tipo, quantas vezes e
// quando; o filtro vale mesmo com o servidor ignorando o field selector.
func TestK8sClient_EventosDoObjeto(t *testing.T) {
	c := &K8sClient{clientset: fake.NewClientset(
		evento("w.1", "Pod", "worker", "Normal", "Pulled", criado, 1),
		evento("w.2", "Pod", "worker", "Warning", "BackOff", criado.Add(50*time.Minute), 14),
		evento("w.3", "Pod", "worker", "Normal", "Scheduled", criado.Add(-time.Minute), 1),
		evento("d.1", "Deployment", "worker", "Normal", "ScalingReplicaSet", criado.Add(time.Hour), 1),
		evento("o.1", "Pod", "outro", "Warning", "Failed", criado.Add(time.Hour), 1),
	)}

	eventos, err := c.EventosDoObjeto("Pod", "worker", "payments", 2)
	if err != nil {
		t.Fatal(err)
	}
	var razoes []string
	for _, e := range eventos {
		razoes = append(razoes, e.Reason)
	}
	if strings.Join(razoes, " ") != "BackOff Pulled" {
		t.Fatalf("eventos do pod worker, até 2: %v", razoes)
	}
	if e := eventos[0]; e.Kind != "Pod" || e.Quantas != 14 || !e.Quando.Equal(criado.Add(50*time.Minute)) || e.ID != "w.2" {
		t.Errorf("evento convertido: %+v", e)
	}

	// Sem tipo, vale qualquer objeto com o nome; sem limite, todos.
	todos, _ := c.EventosDoObjeto("", "worker", "", 0)
	if len(todos) != 4 || todos[0].Reason != "ScalingReplicaSet" {
		t.Errorf("eventos de qualquer worker: %+v", todos)
	}
}

// TestK8sClient_LogsDoPod: o log vem do contêiner pedido; o clientset falso
// responde "fake logs" a qualquer pedido, mas anota as opções.
func TestK8sClient_LogsDoPod(t *testing.T) {
	cs := fake.NewClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "payments"}})
	c := &K8sClient{clientset: cs}
	logs, err := c.LogsDoPod("worker", "payments", "app", 5)
	if err != nil || logs != "fake logs" {
		t.Fatalf("logs = %q, %v", logs, err)
	}
	var pedidos []string
	for _, a := range cs.Actions() {
		if g, ok := a.(k8stesting.GenericActionImpl); ok && a.GetSubresource() == "log" {
			if o, ok := g.Value.(*corev1.PodLogOptions); ok {
				pedidos = append(pedidos, o.Container)
			}
		}
	}
	if strings.Join(pedidos, ",") != "app" {
		t.Errorf("o log deveria ser pedido uma vez, do contêiner app: %v", pedidos)
	}
	if _, err := c.YAMLDoRecurso("pod", "worker", "payments"); err == nil {
		t.Error("sem configuração do cluster, o YAML deveria dar erro")
	}
}

// TestK8sClient_LogsDoPod_ExecucaoAnterior: com o log atual vazio (o contêiner
// espera para reiniciar), vem o da execução anterior; com os dois falhando,
// o erro do atual.
func TestK8sClient_LogsDoPod_ExecucaoAnterior(t *testing.T) {
	type pedido struct {
		conteiner string
		anterior  bool
	}
	var pedidos []pedido
	// O log de cada leitura: [0] a atual, [1] a anterior.
	var logs [2]string
	var errs [2]error
	c := &K8sClient{lerLog: func(ctx context.Context, _, _, conteiner string, _ int64, anterior bool) (string, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("a leitura do log deveria ter prazo")
		}
		pedidos = append(pedidos, pedido{conteiner, anterior})
		i := 0
		if anterior {
			i = 1
		}
		return logs[i], errs[i]
	}}

	logs = [2]string{"\n", "panic: connection refused\n"}
	if got, err := c.LogsDoPod("worker", "payments", "worker", 5); err != nil || got != "panic: connection refused\n" {
		t.Fatalf("log vazio deveria trazer o anterior: %q, %v", got, err)
	}
	if !reflect.DeepEqual(pedidos, []pedido{{"worker", false}, {"worker", true}}) {
		t.Errorf("pedidos: %+v", pedidos)
	}

	atual := errors.New("container \"worker\" is waiting to start")
	logs, errs = [2]string{}, [2]error{atual, errors.New("previous terminated container not found")}
	if _, err := c.LogsDoPod("worker", "payments", "worker", 5); !errors.Is(err, atual) {
		t.Errorf("com as duas leituras falhando, o erro deveria ser o do log atual: %v", err)
	}
}

// TestConteinerDoLog: o contêiner com problema vale mais que o padrão do pod,
// e o padrão vale mais que o primeiro; sidecar só se não houver outro.
func TestConteinerDoLog(t *testing.T) {
	cs := func(nomes ...string) []corev1.Container {
		var r []corev1.Container
		for _, n := range nomes {
			r = append(r, corev1.Container{Name: n})
		}
		return r
	}
	anterior := corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}
	casos := []struct {
		nome string
		pod  corev1.Pod
		quer string
	}{
		{"um contêiner", corev1.Pod{Spec: corev1.PodSpec{Containers: cs("app")}}, "app"},
		{"sidecar primeiro", corev1.Pod{Spec: corev1.PodSpec{Containers: cs("istio-proxy", "app")}}, "app"},
		{"anotação do kubectl", corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
			"kubectl.kubernetes.io/default-container": "web"}}, Spec: corev1.PodSpec{Containers: cs("app", "web")}}, "web"},
		{"esperando para reiniciar", corev1.Pod{Spec: corev1.PodSpec{Containers: cs("app", "worker")},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
				{Name: "worker", State: esperando("CrashLoopBackOff"), LastTerminationState: anterior}}}}, "worker"},
		{"terminou com erro", corev1.Pod{Spec: corev1.PodSpec{Containers: cs("app", "job")},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "job", State: anterior}}}}, "job"},
		{"init rodando", corev1.Pod{Spec: corev1.PodSpec{Containers: cs("app")},
			Status: corev1.PodStatus{InitContainerStatuses: []corev1.ContainerStatus{
				{Name: "migrate", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}, "migrate"},
		{"init concluído", corev1.Pod{Spec: corev1.PodSpec{Containers: cs("app")},
			Status: corev1.PodStatus{InitContainerStatuses: []corev1.ContainerStatus{
				{Name: "migrate", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed"}}}}}}, "app"},
		{"só sidecar", corev1.Pod{Spec: corev1.PodSpec{Containers: cs("linkerd-proxy")}}, "linkerd-proxy"},
		{"sem contêiner", corev1.Pod{}, ""},
	}
	for _, c := range casos {
		if got := conteinerDoLog(&c.pod); got != c.quer {
			t.Errorf("%s: contêiner = %q, esperado %q", c.nome, got, c.quer)
		}
	}

	// A lista traz o contêiner, para o detalhe pedir o log certo.
	k := &K8sClient{clientset: fake.NewClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "payments"},
		Spec: corev1.PodSpec{Containers: cs("istio-proxy", "web")}})}
	if pods, _ := k.GetPods(ListFilter{}); len(pods) != 1 || pods[0].Conteiner != "web" {
		t.Errorf("pod da lista: %+v", pods)
	}
}

// TestGetResourceYAML_Evento: o y na aba Eventos lê o YAML do Event.
func TestGetResourceYAML_Evento(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/payments/events/worker.BackOff" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Event","metadata":{"name":"worker.BackOff","namespace":"payments"},"reason":"BackOff"}`))
	}))
	defer srv.Close()
	yaml, err := GetResourceYAML(&rest.Config{Host: srv.URL}, "event", "worker.BackOff", "payments")
	if err != nil || !strings.Contains(yaml, "reason: BackOff") {
		t.Fatalf("YAML do evento: %q, %v", yaml, err)
	}
}

// TestRetryClient_LeitorDeDetalhe: o RetryClient repassa ao K8sClient e avisa
// quando o client de dentro não lê o detalhe.
func TestRetryClient_LeitorDeDetalhe(t *testing.T) {
	k := &K8sClient{clientset: fake.NewClientset(evento("w.1", "Pod", "worker", "Normal", "Pulled", criado, 1))}
	r := NewRetryClient(k)
	if eventos, err := r.EventosDoObjeto("Pod", "worker", "payments", 0); err != nil || len(eventos) != 1 {
		t.Errorf("eventos pelo RetryClient: %v, %v", eventos, err)
	}
	if logs, err := r.LogsDoPod("worker", "payments", "", 1); err != nil || logs != "fake logs" {
		t.Errorf("logs pelo RetryClient: %q, %v", logs, err)
	}

	sem := NewRetryClient(&FakeClient{})
	if _, err := sem.EventosDoObjeto("Pod", "worker", "payments", 0); err == nil {
		t.Error("sem leitor, os eventos deveriam dar erro")
	}
	if _, err := sem.LogsDoPod("worker", "payments", "", 1); err == nil {
		t.Error("sem leitor, o log deveria dar erro")
	}
	if _, err := sem.YAMLDoRecurso("pod", "worker", "payments"); err == nil {
		t.Error("sem leitor, o YAML deveria dar erro")
	}
}
