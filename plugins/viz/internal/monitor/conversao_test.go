package monitor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var criado = time.Date(2026, 10, 25, 11, 12, 0, 0, time.UTC)

var verdade = true

// esperando é o estado de um contêiner parado com o motivo dado.
func esperando(motivo string) corev1.ContainerState {
	return corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: motivo}}
}

// TestPodDeK8s confere a conversão do pod: estado do contêiner, prontos,
// reinícios com a hora do último, nó, IP e criação.
func TestPodDeK8s(t *testing.T) {
	ultimo := metav1.NewTime(criado.Add(50 * time.Minute))
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-5d8b6-q7tvn", Namespace: "payments", CreationTimestamp: metav1.NewTime(criado),
			Labels:          map[string]string{"app": "worker", "pod-template-hash": "5d8b6"},
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "worker-5d8b6", Controller: &verdade}}},
		Spec: corev1.PodSpec{NodeName: "ip-10-0-2-17", Containers: []corev1.Container{
			{Name: "worker", Image: "worker:1.8.2", Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("200Mi")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			}},
			{Name: "proxy", Image: "envoy:1.30", Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("20m"), corev1.ResourceMemory: resource.MustParse("10Mi")},
			}},
		}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.42.1.23",
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "worker", RestartCount: 14, State: esperando("CrashLoopBackOff"),
					LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 1, FinishedAt: ultimo}},
				},
				{Name: "proxy", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			},
		},
	}
	got := podDeK8s(p, map[string][2]string{"payments/worker-5d8b6-q7tvn": {"120m", "210Mi"}})
	want := Pod{
		Name: "worker-5d8b6-q7tvn", Namespace: "payments", Status: "CrashLoopBackOff", CPU: "120m", Memory: "210Mi",
		Prontos: 1, Containers: 2, Reinicios: 14, UltimoReinicio: ultimo.Time,
		No: "ip-10-0-2-17", IP: "10.42.1.23", CriadoEm: criado, Labels: map[string]string{"app": "worker", "pod-template-hash": "5d8b6"},
		Motivo: "Error · código de saída 1", Imagens: []string{"worker:1.8.2", "envoy:1.30"}, Dono: "Deployment worker",
		Recursos: Recursos{CPUPedido: "120m", CPULimite: "500m", MemPedido: "210Mi"}, Conteiner: "worker",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pod convertido:\n%+v\nesperado:\n%+v", got, want)
	}

	// Sem métricas e sem reinícios: N/A e hora zero.
	p.Status.ContainerStatuses[0].RestartCount = 0
	got = podDeK8s(p, nil)
	if got.CPU != "N/A" || got.Memory != "N/A" || !got.UltimoReinicio.IsZero() {
		t.Fatalf("sem métricas nem reinícios: %+v", got)
	}
}

// TestStatusDoPod cobre os estados que a coluna STATUS mostra, como no kubectl.
func TestStatusDoPod(t *testing.T) {
	rodando := corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	terminou := func(motivo string, codigo int32) corev1.ContainerState {
		return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: motivo, ExitCode: codigo}}
	}
	agora := metav1.NewTime(criado)
	casos := []struct {
		nome string
		pod  corev1.Pod
		quer string
	}{
		{"rodando", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Ready: true, State: rodando}}}}, "Running"},
		{"pendente sem contêiner", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}}, "Pending"},
		{"criando contêiner", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{{State: esperando("ContainerCreating")}}}}, "ContainerCreating"},
		{"crash loop", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{State: esperando("CrashLoopBackOff")}}}}, "CrashLoopBackOff"},
		{"concluído", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{{State: terminou("Completed", 0)}}}}, "Completed"},
		{"erro sem motivo", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{{State: terminou("", 2)}}}}, "ExitCode:2"},
		{"despejado", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed, Reason: "Evicted"}}, "Evicted"},
		{"saindo", corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &agora},
			Status: corev1.PodStatus{Phase: corev1.PodRunning}}, "Terminating"},
		{"init pendente", corev1.Pod{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{}}},
			Status: corev1.PodStatus{Phase: corev1.PodPending, InitContainerStatuses: []corev1.ContainerStatus{{State: esperando("PodInitializing")}}}}, "Init:0/1"},
		{"init com erro", corev1.Pod{Spec: corev1.PodSpec{InitContainers: []corev1.Container{{}}},
			Status: corev1.PodStatus{Phase: corev1.PodPending, InitContainerStatuses: []corev1.ContainerStatus{{State: terminou("Error", 1)}}}}, "Init:Error"},
		{"sidecar concluído ao lado de um rodando", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Ready: true, State: rodando}, {State: terminou("Completed", 0)}}}}, "Running"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := statusDoPod(&c.pod); got != c.quer {
				t.Fatalf("status = %q, esperado %q", got, c.quer)
			}
		})
	}
}

// TestK8sClient_ListasComClientsetFalso confere as listas com a API falsa:
// pods (sem metrics-server) e as réplicas atualizadas e a criação dos
// Deployments e StatefulSets.
func TestK8sClient_ListasComClientsetFalso(t *testing.T) {
	tres := int32(3)
	meta := metav1.ObjectMeta{Name: "api", Namespace: "payments", CreationTimestamp: metav1.NewTime(criado)}
	c := &K8sClient{clientset: fake.NewClientset(
		&corev1.Pod{ObjectMeta: meta, Spec: corev1.PodSpec{Containers: []corev1.Container{{}}}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
		&appsv1.Deployment{ObjectMeta: meta, Spec: appsv1.DeploymentSpec{Replicas: &tres},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 2, AvailableReplicas: 2, UpdatedReplicas: 3}},
		&appsv1.StatefulSet{ObjectMeta: meta, Spec: appsv1.StatefulSetSpec{Replicas: &tres},
			Status: appsv1.StatefulSetStatus{ReadyReplicas: 1, UpdatedReplicas: 2}},
	)}

	pods, err := c.GetPods(ListFilter{Namespace: "payments"})
	if err != nil || len(pods) != 1 {
		t.Fatalf("pods: %v, %v", pods, err)
	}
	if p := pods[0]; p.Status != "Pending" || p.Containers != 1 || p.CPU != "N/A" || !p.CriadoEm.Equal(criado) {
		t.Fatalf("pod: %+v", p)
	}

	deps, err := c.GetDeployments(ListFilter{})
	if err != nil || len(deps) != 1 {
		t.Fatalf("deployments: %v, %v", deps, err)
	}
	if d := deps[0]; d.Replicas != 3 || d.Ready != 2 || d.Atualizados != 3 || !d.CriadoEm.Equal(criado) {
		t.Fatalf("deployment: %+v", d)
	}

	sets, err := c.GetStatefulSets(ListFilter{})
	if err != nil || len(sets) != 1 {
		t.Fatalf("statefulsets: %v, %v", sets, err)
	}
	if s := sets[0]; s.Ready != 1 || s.Atualizados != 2 || !s.CriadoEm.Equal(criado) {
		t.Fatalf("statefulset: %+v", s)
	}
}

// TestK8sClient_JobsEventosELabels: as condições Complete e Failed do Job, o
// nome do objeto Event (dois eventos do mesmo pod) e as labels dos objetos.
func TestK8sClient_JobsEventosELabels(t *testing.T) {
	labels := map[string]string{"app": "api"}
	c := &K8sClient{clientset: fake.NewClientset(
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "fila", Namespace: "payments", Labels: labels},
			Status: batchv1.JobStatus{Succeeded: 1, Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
				{Type: batchv1.JobFailed, Status: corev1.ConditionFalse},
			}}},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "worker.1", Namespace: "payments", Labels: labels},
			InvolvedObject: corev1.ObjectReference{Name: "worker"}, Type: "Warning"},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "worker.2", Namespace: "payments"},
			InvolvedObject: corev1.ObjectReference{Name: "worker"}, Type: "Normal"},
	)}

	jobs, err := c.GetJobs(ListFilter{})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs: %v, %v", jobs, err)
	}
	if j := jobs[0]; !j.Concluido || j.Falhou || j.Completions != 0 || j.Labels["app"] != "api" {
		t.Fatalf("job: %+v", j)
	}

	eventos, err := c.GetEvents(ListFilter{})
	if err != nil || len(eventos) != 2 {
		t.Fatalf("eventos: %v, %v", eventos, err)
	}
	ids := map[string]bool{}
	for _, e := range eventos {
		if e.Name != "worker" {
			t.Errorf("objeto envolvido = %q", e.Name)
		}
		ids[e.ID] = true
	}
	if !ids["worker.1"] || !ids["worker.2"] {
		t.Fatalf("IDs dos eventos: %v", ids)
	}
}

// TestIdade formata como o kubectl e devolve vazio sem data.
func TestIdade(t *testing.T) {
	agora := criado.Add(52 * time.Minute)
	casos := map[time.Duration]string{30 * time.Second: "30s", 52 * time.Minute: "52m", 3 * time.Hour: "3h", 6 * 24 * time.Hour: "6d"}
	for d, quer := range casos {
		if got := Idade(agora.Add(-d), agora); got != quer {
			t.Errorf("Idade(%v) = %q, esperado %q", d, got, quer)
		}
	}
	if got := Idade(time.Time{}, agora); got != "" {
		t.Errorf("sem data: %q", got)
	}
	if got := Idade(agora.Add(time.Minute), agora); got != "0s" {
		t.Errorf("relógio adiantado: %q", got)
	}
}

// TestNewK8sClient_Contexto lê o contexto atual do kubeconfig, e o RetryClient o repassa.
func TestNewK8sClient_Contexto(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "config")
	conteudo := `apiVersion: v1
kind: Config
current-context: staging
clusters:
- name: staging
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: staging
  context:
    cluster: staging
    user: eu
    namespace: payments
users:
- name: eu
  user:
    token: falso
`
	if err := os.WriteFile(kubeconfig, []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig)

	c, err := NewK8sClient()
	if err != nil {
		t.Fatalf("NewK8sClient: %v", err)
	}
	if c.Contexto() != "staging" {
		t.Fatalf("contexto = %q, esperado staging", c.Contexto())
	}
	if r := NewRetryClient(c); r.Contexto() != "staging" {
		t.Fatalf("RetryClient.Contexto() = %q", r.Contexto())
	}
	if r := NewRetryClient(&FakeClient{}); r.Contexto() != "" {
		t.Fatalf("client sem contexto: %q", r.Contexto())
	}
	if r := NewRetryClient(c); r.NamespaceInicial() != "payments" {
		t.Fatalf("namespace do contexto = %q, esperado payments", r.NamespaceInicial())
	}
}
