//go:build k8s

package checks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPrivilegeEscalationCheck_Permitido(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-esc", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "app",
			}},
		},
	})

	check := &PrivilegeEscalationCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para container sem allowPrivilegeEscalation definido")
	}
}

func TestPrivilegeEscalationCheck_Bloqueado(t *testing.T) {
	f := false
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-noesc", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "app",
				SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: &f},
			}},
		},
	})

	check := &PrivilegeEscalationCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings, obteve %d", len(findings))
	}
}

func TestPrivilegeEscalationCheck_InitContainer(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-init", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers:     []corev1.Container{},
			InitContainers: []corev1.Container{{Name: "init-app"}},
		},
	})

	check := &PrivilegeEscalationCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para init container sem allowPrivilegeEscalation")
	}
}
