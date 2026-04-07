//go:build k8s

package checks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLatestTagCheck_ComLatest(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-latest", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: "nginx:latest",
			}},
		},
	})

	check := &LatestTagCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para imagem com :latest")
	}
}

func TestLatestTagCheck_SemTag(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-notag", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: "nginx",
			}},
		},
	})

	check := &LatestTagCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para imagem sem tag")
	}
}

func TestLatestTagCheck_ComTagFixa(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-tagged", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: "nginx:1.25.3",
			}},
		},
	})

	check := &LatestTagCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings para imagem com tag fixa, obteve %d", len(findings))
	}
}

func TestLatestTagCheck_ComSHA256(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-sha", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: "nginx@sha256:abc123",
			}},
		},
	})

	check := &LatestTagCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings para imagem com sha256, obteve %d", len(findings))
	}
}

func TestIsLatestOrNoTag(t *testing.T) {
	tests := []struct {
		image    string
		expected bool
	}{
		{"nginx", true},
		{"nginx:latest", true},
		{"nginx:1.25", false},
		{"myregistry.io/app:v2", false},
		{"myregistry.io/app@sha256:abc123", false},
		{"myregistry.io/app", true},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			result := isLatestOrNoTag(tt.image)
			if result != tt.expected {
				t.Errorf("isLatestOrNoTag(%q) = %v, esperava %v", tt.image, result, tt.expected)
			}
		})
	}
}
