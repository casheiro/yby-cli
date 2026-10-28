//go:build k8s

package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/muesli/termenv"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/casheiro/yby-cli/pkg/ui"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func podReiniciado(terminouHa time.Duration) *corev1.Pod {
	return &corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app", Ready: true, RestartCount: 3,
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					FinishedAt: metav1.NewTime(time.Now().Add(-terminouHa)),
				}},
			}},
		},
	}
}

func TestIsPodHealthy_SucceededESaudavel(t *testing.T) {
	pod := &corev1.Pod{Status: corev1.PodStatus{
		Phase:             corev1.PodSucceeded,
		ContainerStatuses: []corev1.ContainerStatus{{Name: "job", Ready: false}},
	}}
	if !isPodHealthy(pod, nil) {
		t.Error("pod Succeeded é saudável")
	}
	warning := &corev1.EventList{Items: []corev1.Event{{Type: "Warning"}}}
	if isPodHealthy(pod, warning) {
		t.Error("Warning continua sendo sinal de problema")
	}
	if isPodHealthy(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed}}, nil) {
		t.Error("pod Failed não é saudável")
	}
}

func TestIsPodHealthy_ReinicioAntigoNaoConta(t *testing.T) {
	if !isPodHealthy(podReiniciado(3*time.Hour), nil) {
		t.Error("reinício de 3 h atrás não deixa o pod doente")
	}
	if isPodHealthy(podReiniciado(10*time.Minute), nil) {
		t.Error("reinício de 10 min atrás é sinal de problema")
	}
}

func TestCacheKey_DistingueClusterUIDEReinicios(t *testing.T) {
	base := idPod{Contexto: "a", Namespace: "ns", Pod: "p", UID: "u1", Reinicios: 1}
	k := cacheKey(base, "logs")
	for nome, outro := range map[string]idPod{
		"contexto":  {Contexto: "b", Namespace: "ns", Pod: "p", UID: "u1", Reinicios: 1},
		"uid":       {Contexto: "a", Namespace: "ns", Pod: "p", UID: "u2", Reinicios: 1},
		"reinícios": {Contexto: "a", Namespace: "ns", Pod: "p", UID: "u1", Reinicios: 2},
	} {
		if cacheKey(outro, "logs") == k {
			t.Errorf("a chave deveria mudar com %s", nome)
		}
	}
}

func TestOpcoesDeLogs_PorConteinerComPrevious(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}, {Name: "sidecar"}}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
			{Name: "app", RestartCount: 0}, {Name: "sidecar", RestartCount: 2},
		}},
	}
	op := opcoesDeLogs(pod, 50)
	if len(op) != 3 {
		t.Fatalf("esperava 3 pedidos (app, sidecar, sidecar anterior), obtido %d", len(op))
	}
	if op[0].Container != "app" || op[0].Previous {
		t.Errorf("pedido 0: %+v", op[0])
	}
	if op[1].Container != "sidecar" || op[1].Previous {
		t.Errorf("pedido 1: %+v", op[1])
	}
	if op[2].Container != "sidecar" || !op[2].Previous {
		t.Errorf("pedido 2: %+v", op[2])
	}
	for _, o := range op {
		if o.TailLines == nil || *o.TailLines != 50 {
			t.Errorf("faltou o tail: %+v", o)
		}
	}
	if sem := opcoesDeLogs(&corev1.Pod{}, 50); len(sem) != 1 || sem[0].Container != "" {
		t.Errorf("sem spec, um pedido sem nome: %+v", sem)
	}
}

func TestResumirEventos_SemManagedFields(t *testing.T) {
	ev := corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "x",
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubelet"}},
		},
		Type: "Warning", Reason: "BackOff", Message: "Back-off", Count: 4,
		LastTimestamp: metav1.NewTime(time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)),
	}
	b, err := json.Marshal(resumirEventos([]corev1.Event{ev}))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, quer := range []string{`"reason":"BackOff"`, `"message":"Back-off"`, `"count":4`, `"lastTimestamp":"2026-10-26T10:00:00Z"`} {
		if !strings.Contains(s, quer) {
			t.Errorf("faltou %s em %s", quer, s)
		}
	}
	if strings.Contains(s, "managedFields") || strings.Contains(s, "kubelet") {
		t.Errorf("managedFields não vai para a IA: %s", s)
	}
}

func TestInvestigate_CacheNaoVenceSaude(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	podRuim, ev := podComProblema()
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, fake.NewSimpleClientset(podRuim, ev), nil, nil, prov)
	if _, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments"); err != nil {
		t.Fatal(err)
	}

	// o mesmo pod, agora saudável, com os mesmos logs e os mesmos 12 reinícios (sem reinício
	// recente): a chave do cache não muda, então só checar a saúde antes do cache passa aqui
	curado := podRuim.DeepCopy()
	curado.Status = corev1.PodStatus{
		Phase:             corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{Name: "worker", Ready: true, RestartCount: 12}},
	}
	usarDependencias(t, fake.NewSimpleClientset(curado), nil, nil, prov)
	saida, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "pod saudável") || strings.Contains(saida, "análise guardada") {
		t.Errorf("esperava pod saudável, sem a análise antiga:\n%s", saida)
	}
}
