package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "k3d":
		if len(args) > 0 && args[0] == "cluster" && args[1] == "start" {
			if args[2] == "fail-cluster" {
				fmt.Fprintln(os.Stderr, "k3d failed to start")
				os.Exit(1)
			}
		}
	case "helm":
		if len(args) > 0 && args[0] == "repo" && args[1] == "add" {
			if args[3] == "https://charts.fail.com" { // name url
				fmt.Fprintln(os.Stderr, "helm repo add failed")
				os.Exit(1)
			}
		}
	case "kubectl":
		if len(args) >= 3 && args[0] == "create" && args[1] == "namespace" {
			if args[2] == "exists-ns" {
				fmt.Fprintln(os.Stdout, "Error from server (AlreadyExists): namespaces \"exists-ns\" already exists")
				os.Exit(1)
			}
			if args[2] == "fail-ns" {
				fmt.Fprintln(os.Stdout, "Error from server (Forbidden): forbidden")
				os.Exit(1)
			}
		}
		// Simular falha ao criar secret genérico quando nome é "fail-secret"
		if len(args) >= 4 && args[0] == "create" && args[1] == "secret" && args[2] == "generic" {
			if args[3] == "fail-secret" {
				fmt.Fprintln(os.Stderr, "erro ao criar secret")
				os.Exit(1)
			}
			// Sucesso: emitir YAML mock para o secret
			fmt.Fprintln(os.Stdout, "apiVersion: v1\nkind: Secret\nmetadata:\n  name: test-secret")
			return
		}
		// Mock success for kubectl by default
		return
	case "kubeseal":
		// Se variável de ambiente indicar falha, simular erro
		if os.Getenv("GO_KUBESEAL_FAIL") == "1" {
			fmt.Fprintln(os.Stderr, "erro ao selar secret")
			os.Exit(1)
		}
		// Sucesso: emitir YAML mock de sealed secret
		fmt.Fprintln(os.Stdout, "apiVersion: bitnami.com/v1alpha1\nkind: SealedSecret\nmetadata:\n  name: sealed-test")
		return
	}
}

// mockExecCommand sets up the execCommand variable to use the helper process.
// It returns a teardown function that restores the original execCommand.
func mockExecCommand() func() {
	originalExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// Mock LookPath to always succeed for tools we expect, or fail for "fail-tool"
	originalLookPath := lookPath
	lookPath = func(file string) (string, error) {
		if file == "fail-tool" || file == "missing-tool" {
			return "", fmt.Errorf("tool not found: %s", file)
		}
		return "/usr/bin/" + file, nil
	}

	return func() {
		execCommand = originalExecCommand
		lookPath = originalLookPath
	}
}
