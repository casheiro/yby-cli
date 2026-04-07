//go:build k8s

package remediation

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestApplyPatches_PodPatch(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-test", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}},
		},
	})

	patches := []RemediationPatch{{
		ResourceKind: "Pod",
		ResourceName: "pod-test",
		Namespace:    "default",
		PatchType:    "strategic-merge",
		Patch:        `{"spec":{"containers":[{"name":"app","securityContext":{"runAsNonRoot":true}}]}}`,
		Description:  "Define runAsNonRoot",
	}}

	errs := ApplyPatches(context.Background(), client, patches)
	if len(errs) != 0 {
		t.Errorf("esperava zero erros, obteve: %v", errs)
	}
}

func TestApplyPatches_RecursoInexistente(t *testing.T) {
	client := fake.NewSimpleClientset()

	patches := []RemediationPatch{{
		ResourceKind: "Pod",
		ResourceName: "pod-inexistente",
		Namespace:    "default",
		PatchType:    "strategic-merge",
		Patch:        `{"spec":{}}`,
		Description:  "Patch em pod inexistente",
	}}

	errs := ApplyPatches(context.Background(), client, patches)
	if len(errs) == 0 {
		t.Error("esperava erro para pod inexistente")
	}
}

func TestApplyPatches_TipoNaoSuportado(t *testing.T) {
	client := fake.NewSimpleClientset()

	patches := []RemediationPatch{{
		ResourceKind: "ConfigMap",
		ResourceName: "cm-test",
		Namespace:    "default",
		PatchType:    "strategic-merge",
		Patch:        `{}`,
		Description:  "Tipo não suportado",
	}}

	errs := ApplyPatches(context.Background(), client, patches)
	if len(errs) == 0 {
		t.Error("esperava erro para tipo não suportado")
	}
}
