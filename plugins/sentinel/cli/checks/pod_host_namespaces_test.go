//go:build k8s

package checks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestHostNamespacesCheck_HostPID(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-hostpid", Namespace: "default"},
		Spec: corev1.PodSpec{
			HostPID:    true,
			Containers: []corev1.Container{{Name: "app"}},
		},
	})

	check := &HostNamespacesCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	found := false
	for _, f := range findings {
		if f.CheckID == "POD_HOST_NAMESPACES" && f.Severity == SeverityHigh {
			found = true
			break
		}
	}
	if !found {
		t.Error("esperava finding high para hostPID")
	}
}

func TestHostNamespacesCheck_HostNetwork(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-hostnet", Namespace: "default"},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers:  []corev1.Container{{Name: "app"}},
		},
	})

	check := &HostNamespacesCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para hostNetwork")
	}
}

func TestHostNamespacesCheck_SemHostNamespaces(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-safe", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}},
		},
	})

	check := &HostNamespacesCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings, obteve %d", len(findings))
	}
}
