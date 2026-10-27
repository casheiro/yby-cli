//go:build k8s

package checks

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type chaveCachePods struct{}

// cachePods guarda a lista de pods de cada namespace durante um scan.
type cachePods struct {
	mu    sync.Mutex
	lista map[string]*corev1.PodList
}

// ComCachePods devolve um contexto em que listarPods consulta o cluster uma vez só por
// namespace: os checks internos, que são vários, dividem a mesma lista em vez de cada um
// listar os pods de novo.
func ComCachePods(ctx context.Context) context.Context {
	return context.WithValue(ctx, chaveCachePods{}, &cachePods{lista: map[string]*corev1.PodList{}})
}

// listarPods lista os pods do namespace; com o cache de ComCachePods no contexto, só a
// primeira chamada vai ao cluster (erros não ficam no cache). Quem chama não deve alterar
// a lista devolvida.
func listarPods(ctx context.Context, client kubernetes.Interface, namespace string) (*corev1.PodList, error) {
	c, _ := ctx.Value(chaveCachePods{}).(*cachePods)
	if c == nil {
		return client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if l, ok := c.lista[namespace]; ok {
		return l, nil
	}
	l, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	c.lista[namespace] = l
	return l, nil
}
