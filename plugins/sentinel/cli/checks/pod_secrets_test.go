//go:build k8s

package checks

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestExposedSecretsCheck_EnvHardcoded(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-secrets", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "app",
				Env: []corev1.EnvVar{
					{Name: "DB_PASSWORD", Value: "senha-hardcoded"},
				},
			}},
		},
	})

	check := &ExposedSecretsCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para env var sensível com valor hardcoded")
	}
	if findings[0].Severity != SeverityCritical || findings[0].CheckID != "POD_EXPOSED_SECRETS" {
		t.Errorf("esperava critical/POD_EXPOSED_SECRETS, obteve %s/%s", findings[0].Severity, findings[0].CheckID)
	}
	if !strings.Contains(findings[0].Message, "DB_PASSWORD") {
		t.Errorf("mensagem deveria mencionar DB_PASSWORD, obteve: %s", findings[0].Message)
	}
}

func TestExposedSecretsCheck_EnvViaSecretKeyRef(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-secretref", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "app",
				Env: []corev1.EnvVar{{
					Name: "DB_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "db-secret"},
							Key:                  "password",
						},
					},
				}},
			}},
		},
	})

	check := &ExposedSecretsCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings para env via secretKeyRef, obteve %d", len(findings))
	}
}
