package monitor

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Medidas da conversão do cache do watch num cluster grande: 10 mil pods e
// 20 mil eventos, na ordem embaralhada em que o store do informer os devolve.
// Rodar com: cd plugins/viz && go test -run '^$' -bench . -benchmem ./internal/monitor/

// podsDoCache são n pods como o informer os guarda (sem managedFields).
func podsDoCache(n int) []any {
	objs := make([]any, n)
	criado := metav1.NewTime(time.Now().Add(-time.Hour))
	for i := range objs {
		objs[i] = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("app-%05d-x2k2p", i), Namespace: fmt.Sprintf("ns-%02d", i%50), UID: types.UID(fmt.Sprint(i)),
				CreationTimestamp: criado, Labels: map[string]string{"app": fmt.Sprintf("app-%d", i%100)},
				OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: fmt.Sprintf("app-%05d", i), Controller: new(true)}},
			},
			Spec: corev1.PodSpec{
				NodeName: fmt.Sprintf("ip-10-0-%d-5", i%30),
				Containers: []corev1.Container{{
					Name: "app", Image: "registry.local/app:1.8.2",
					Env: []corev1.EnvVar{{Name: "LOG_LEVEL", Value: "info"}, {Name: "PORT", Value: "8080"}},
				}},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning, PodIP: fmt.Sprintf("10.42.%d.%d", i/250, i%250),
				ContainerStatuses: []corev1.ContainerStatus{{Name: "app", Ready: true, State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{StartedAt: criado},
				}}},
			},
		}
	}
	return embaralhado(objs)
}

// eventosDoCache são n eventos como o informer os guarda.
func eventosDoCache(n int) []any {
	objs := make([]any, n)
	agora := time.Now()
	for i := range objs {
		quando := metav1.NewTime(agora.Add(-time.Duration(i) * time.Second))
		objs[i] = &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("app-%05d.17a3%04d", i, i%10000), Namespace: fmt.Sprintf("ns-%02d", i%50)},
			InvolvedObject: corev1.ObjectReference{
				Kind: "Pod", Name: fmt.Sprintf("app-%05d-x2k2p", i%10000), Namespace: fmt.Sprintf("ns-%02d", i%50),
			},
			Type: "Normal", Reason: "Pulled", Message: "Container image \"registry.local/app:1.8.2\" already present on machine",
			Source: corev1.EventSource{Component: "kubelet", Host: "ip-10-0-1-5"}, Count: 1,
			FirstTimestamp: quando, LastTimestamp: quando,
		}
	}
	return embaralhado(objs)
}

// embaralhado devolve os objetos fora de ordem, como o store (um mapa) os lista.
func embaralhado(objs []any) []any {
	r := rand.New(rand.NewPCG(1, 2))
	r.Shuffle(len(objs), func(i, j int) { objs[i], objs[j] = objs[j], objs[i] })
	return objs
}

// BenchmarkListaDoCache mede a conversão de um lote: o retrato inteiro do tipo.
func BenchmarkListaDoCache(b *testing.B) {
	for _, c := range []struct {
		kind string
		objs []any
	}{{"pod", podsDoCache(10_000)}, {"event", eventosDoCache(20_000)}} {
		b.Run(c.kind, func(b *testing.B) {
			copia := make([]any, len(c.objs))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				copy(copia, c.objs) // o store devolve uma fatia nova, fora de ordem, a cada List
				_ = listaDoCache(c.kind, copia)
			}
		})
	}
}
