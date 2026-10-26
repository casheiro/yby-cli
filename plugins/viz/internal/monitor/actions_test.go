package monitor

import (
	"testing"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// TestEscalar_MandaAsReplicasPedidas: ScaleDeployment e ScaleStatefulSet lêem o
// scale do recurso e gravam o número pedido (o zero também) no recurso certo.
func TestEscalar_MandaAsReplicasPedidas(t *testing.T) {
	casos := []struct {
		recurso string
		escalar func(*fake.Clientset, int32) error
	}{
		{"deployments", func(cs *fake.Clientset, n int32) error { return ScaleDeployment(cs, "api", "loja", n) }},
		{"statefulsets", func(cs *fake.Clientset, n int32) error { return ScaleStatefulSet(cs, "api", "loja", n) }},
	}
	for _, c := range casos {
		for _, n := range []int32{0, 3} {
			cs := fake.NewClientset()
			var gravado *autoscalingv1.Scale
			cs.PrependReactor("*", c.recurso, func(a k8stesting.Action) (bool, runtime.Object, error) {
				if a.GetSubresource() != "scale" {
					return false, nil, nil
				}
				if u, ok := a.(k8stesting.UpdateAction); ok {
					gravado = u.GetObject().(*autoscalingv1.Scale)
					return true, gravado, nil
				}
				atual := &autoscalingv1.Scale{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "loja"}}
				atual.Spec.Replicas = 2
				return true, atual, nil
			})
			if err := c.escalar(cs, n); err != nil {
				t.Fatalf("%s para %d: %v", c.recurso, n, err)
			}
			if gravado == nil || gravado.Spec.Replicas != n || gravado.Name != "api" || gravado.Namespace != "loja" {
				t.Fatalf("%s para %d: gravado %+v", c.recurso, n, gravado)
			}
		}
	}
}
