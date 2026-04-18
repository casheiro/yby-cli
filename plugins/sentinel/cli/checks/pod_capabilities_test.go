//go:build k8s

package checks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCapabilitiesCheck_SemDropAll(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-nocap", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}},
		},
	})

	check := &CapabilitiesCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	found := false
	for _, f := range findings {
		if f.Severity == SeverityMedium && f.CheckID == "POD_CAPABILITIES" {
			found = true
			break
		}
	}
	if !found {
		t.Error("esperava finding medium para container sem drop ALL")
	}
}

func TestCapabilitiesCheck_ComSysAdmin(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-sysadmin", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "app",
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{"SYS_ADMIN"},
					},
				},
			}},
		},
	})

	check := &CapabilitiesCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	found := false
	for _, f := range findings {
		if f.Severity == SeverityCritical && f.CheckID == "POD_CAPABILITIES" {
			found = true
			break
		}
	}
	if !found {
		t.Error("esperava finding critical para SYS_ADMIN")
	}
}

func TestCapabilitiesCheck_ComDropAll(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-dropall", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "app",
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
				},
			}},
		},
	})

	check := &CapabilitiesCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings para container com drop ALL, obteve %d", len(findings))
	}
}
