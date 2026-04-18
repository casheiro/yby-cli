//go:build k8s

package checks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRootContainerCheck_RunAsRoot(t *testing.T) {
	var uid int64 = 0
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-root", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "app",
				SecurityContext: &corev1.SecurityContext{RunAsUser: &uid},
			}},
		},
	})

	check := &RootContainerCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava pelo menos um finding para container rodando como root")
	}

	found := false
	for _, f := range findings {
		if f.Severity == SeverityCritical && f.CheckID == "POD_ROOT_CONTAINER" {
			found = true
			break
		}
	}
	if !found {
		t.Error("esperava finding crítico de POD_ROOT_CONTAINER para RunAsUser=0")
	}
}

func TestRootContainerCheck_SemSecurityContext(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-nosec", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "app",
			}},
		},
	})

	check := &RootContainerCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para container sem SecurityContext")
	}

	found := false
	for _, f := range findings {
		if f.Type == "warning" && f.CheckID == "POD_ROOT_CONTAINER" {
			found = true
			break
		}
	}
	if !found {
		t.Error("esperava finding de aviso quando SecurityContext é nil")
	}
}

func TestRootContainerCheck_RunAsNonRootTrue(t *testing.T) {
	nonRoot := true
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-nonroot", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "app",
				SecurityContext: &corev1.SecurityContext{RunAsNonRoot: &nonRoot},
			}},
		},
	})

	check := &RootContainerCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings para RunAsNonRoot=true, obteve %d", len(findings))
	}
}
