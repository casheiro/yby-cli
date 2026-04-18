//go:build k8s

package checks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// fakeCosignNotFound simula cosign não encontrado.
func fakeCosignNotFound(name string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperCosignNotFound", "--", name}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// fakeCosignVerifyFail simula falha na verificação.
func fakeCosignVerifyFail(name string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperCosignVerifyFail", "--", name}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperCosignNotFound(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Fprintln(os.Stderr, "cosign: command not found")
	os.Exit(1)
}

func TestHelperCosignVerifyFail(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) > 0 && args[0] == "cosign" && len(args) > 1 && args[1] == "version" {
		fmt.Println("cosign v2.0.0")
		os.Exit(0)
	}
	// verify falha
	fmt.Fprintln(os.Stderr, "Error: no matching signatures")
	os.Exit(1)
}

func TestImageSignatureCheck_CosignNaoEncontrado(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-img", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "nginx:1.25"}},
		},
	})

	check := &ImageSignatureCheck{ExecCommand: fakeCosignNotFound}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("esperava 1 finding info, obteve %d", len(findings))
	}
	if findings[0].Severity != SeverityInfo {
		t.Errorf("esperava severity info, obteve %s", findings[0].Severity)
	}
}

func TestImageSignatureCheck_SemAssinatura(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-unsigned", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "nginx:1.25"}},
		},
	})

	check := &ImageSignatureCheck{ExecCommand: fakeCosignVerifyFail}
	findings, err := check.Run(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("esperava finding para imagem sem assinatura")
	}
	if findings[0].Severity != SeverityHigh {
		t.Errorf("esperava severity high, obteve %s", findings[0].Severity)
	}
}
