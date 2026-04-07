//go:build k8s

package checks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNetpolCoverageCheck_PodSemCobertura(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-exposed",
				Namespace: "default",
				Labels:    map[string]string{"app": "web"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "app"}},
			},
		},
	)

	check := &NetpolCoverageCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para pod sem NetworkPolicy")
	}
}

func TestNetpolCoverageCheck_PodComCobertura(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-covered",
				Namespace: "default",
				Labels:    map[string]string{"app": "web"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "app"}},
			},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "allow-web", Namespace: "default"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "web"},
				},
			},
		},
	)

	check := &NetpolCoverageCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings para pod coberto por NetworkPolicy, obteve %d", len(findings))
	}
}

func TestNetpolCoverageCheck_SelectorVazioCobreTodos(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-any",
				Namespace: "default",
				Labels:    map[string]string{"app": "api"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "app"}},
			},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "default-deny", Namespace: "default"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{}, // seleciona todos
			},
		},
	)

	check := &NetpolCoverageCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings para pod coberto por selector vazio, obteve %d", len(findings))
	}
}
