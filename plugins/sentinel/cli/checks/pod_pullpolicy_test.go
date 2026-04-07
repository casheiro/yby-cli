//go:build k8s

package checks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestImagePullPolicyCheck_NaoAlways(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-pullpolicy", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "app",
				ImagePullPolicy: corev1.PullIfNotPresent,
			}},
		},
	})

	check := &ImagePullPolicyCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para ImagePullPolicy != Always")
	}
	if findings[0].Type != "warning" || findings[0].CheckID != "POD_IMAGE_PULL_POLICY" {
		t.Errorf("esperava warning/POD_IMAGE_PULL_POLICY, obteve %s/%s", findings[0].Type, findings[0].CheckID)
	}
}

func TestImagePullPolicyCheck_Always(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-always", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "app",
				ImagePullPolicy: corev1.PullAlways,
			}},
		},
	})

	check := &ImagePullPolicyCheck{}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("esperava zero findings para PullAlways, obteve %d", len(findings))
	}
}
