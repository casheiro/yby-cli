//go:build k8s

package checks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestChecksListamPodsUmaVezSo(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx"}}},
	})
	listagens := 0
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		listagens++
		return false, nil, nil
	})
	ctx := ComCachePods(context.Background())
	for _, c := range GetAll() {
		_, _ = c.Run(ctx, client, "default")
	}
	if listagens != 1 {
		t.Errorf("listou os pods %d vezes, esperava 1", listagens)
	}

	listagens = 0
	_, _ = listarPods(context.Background(), client, "default")
	_, _ = listarPods(context.Background(), client, "default")
	if listagens != 2 {
		t.Errorf("sem o cache cada chamada lista: %d", listagens)
	}
}
