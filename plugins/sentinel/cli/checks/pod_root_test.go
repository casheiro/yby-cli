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

func TestRootContainerCheck_HerdaDoPod(t *testing.T) {
	sim, nao := true, false
	var zero, mil int64 = 0, 1000
	casos := []struct {
		nome string
		spec corev1.PodSpec
		want int
	}{
		{"runAsNonRoot no pod", corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: &sim},
			Containers:      []corev1.Container{{Name: "app"}},
		}, 0},
		{"runAsUser no pod", corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{RunAsUser: &mil},
			Containers:      []corev1.Container{{Name: "app"}},
		}, 0},
		{"uid 0 no pod", corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{RunAsUser: &zero},
			Containers:      []corev1.Container{{Name: "app"}},
		}, 1},
		{"o contêiner sobrepõe o pod", corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: &sim},
			Containers:      []corev1.Container{{Name: "app", SecurityContext: &corev1.SecurityContext{RunAsNonRoot: &nao}}},
		}, 1},
		{"nada definido", corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}, 1},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			client := fake.NewSimpleClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"}, Spec: tc.spec})
			findings, err := (&RootContainerCheck{}).Run(context.Background(), client, "default")
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != tc.want {
				t.Errorf("achados = %d, esperava %d: %+v", len(findings), tc.want, findings)
			}
		})
	}
}
