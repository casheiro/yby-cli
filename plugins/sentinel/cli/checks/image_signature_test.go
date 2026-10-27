//go:build k8s

package checks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

	check := &ImageSignatureCheck{ExecCommand: fakeCosignNotFound, Chave: "cosign.pub"}
	findings, err := check.Run(context.Background(), client, "default")
	var naoVerif *ErrNaoVerificado
	if !errors.As(err, &naoVerif) || len(findings) != 0 {
		t.Fatalf("esperava ErrNaoVerificado e 0 achados: %v %+v", err, findings)
	}
}

func TestImageSignatureCheck_SemAssinatura(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-unsigned", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "nginx:1.25"}},
		},
	})

	check := &ImageSignatureCheck{ExecCommand: fakeCosignVerifyFail, Chave: "cosign.pub"}
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

func TestImageSignatureCheck_SemConfiguracaoNaoVerifica(t *testing.T) {
	t.Setenv(envCosignChave, "")
	t.Setenv(envCosignIdentidade, "")
	t.Setenv(envCosignEmissor, "")
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-img", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:1.25"}}},
	})
	chamadas := 0
	check := &ImageSignatureCheck{ExecCommand: func(name string, args ...string) *exec.Cmd {
		chamadas++
		return fakeCosignVerifyFail(name, args...)
	}}
	findings, err := check.Run(context.Background(), client, "default")
	if chamadas != 0 {
		t.Errorf("sem chave nem identidade o cosign não deve rodar, rodou %d vezes", chamadas)
	}
	var naoVerif *ErrNaoVerificado
	if !errors.As(err, &naoVerif) || len(findings) != 0 {
		t.Errorf("esperava ErrNaoVerificado e 0 achados: %v %+v", err, findings)
	}
}

func TestImageSignatureCheck_ArgsDeConfianca(t *testing.T) {
	t.Setenv(envCosignChave, "")
	t.Setenv(envCosignIdentidade, "")
	t.Setenv(envCosignEmissor, "")
	casos := []struct {
		nome string
		c    ImageSignatureCheck
		want string
	}{
		{"nada", ImageSignatureCheck{}, ""},
		{"chave", ImageSignatureCheck{Chave: "k.pub"}, "--key k.pub"},
		{"identidade com emissor", ImageSignatureCheck{Identidade: "a@b", Emissor: "https://i"}, "--certificate-identity a@b --certificate-oidc-issuer https://i"},
		{"identidade sem emissor", ImageSignatureCheck{Identidade: "a@b"}, ""},
	}
	for _, tc := range casos {
		if got := strings.Join(tc.c.argsDeConfianca(), " "); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.nome, got, tc.want)
		}
	}
	t.Setenv(envCosignChave, "env.pub")
	if got := strings.Join((&ImageSignatureCheck{}).argsDeConfianca(), " "); got != "--key env.pub" {
		t.Errorf("a chave do ambiente deve valer: %q", got)
	}
}
