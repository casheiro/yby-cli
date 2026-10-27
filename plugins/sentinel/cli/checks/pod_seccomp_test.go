//go:build k8s

package checks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSeccompCheck_SemPerfil(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-noseccomp", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}},
		},
	})

	check := &SeccompCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para pod sem perfil seccomp")
	}
}

func TestSeccompCheck_ComPerfil(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-seccomp", Namespace: "default"},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{{Name: "app"}},
		},
	})

	check := &SeccompCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings, obteve %d", len(findings))
	}
}

func TestSeccompCheck_PerfilNoContainer(t *testing.T) {
	perfil := &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	client := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "default"}, Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "a", SecurityContext: &corev1.SecurityContext{SeccompProfile: perfil}}},
		}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "falta", Namespace: "default"}, Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "a", SecurityContext: &corev1.SecurityContext{SeccompProfile: perfil}}, {Name: "b"}},
		}},
	)
	findings, err := (&SeccompCheck{}).Run(context.Background(), client, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Pod != "falta" {
		t.Errorf("só o pod com contêiner sem perfil deve ser achado: %+v", findings)
	}
}
