package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func executeHelmRepoAdd(name, url string) {
	if err := execCommand("helm", "repo", "add", name, url).Run(); err != nil {
		fmt.Printf("%s Falha ao adicionar repo Helm '%s': %v\n", crossStyle.String(), name, err)
		osExit(1)
	}
	if err := execCommand("helm", "repo", "update", name).Run(); err != nil {
		fmt.Printf("%s Falha ao atualizar repo Helm '%s': %v\n", crossStyle.String(), name, err)
		osExit(1)
	}
}

func createNamespace(ns string) {
	out, err := execCommand("kubectl", "create", "namespace", ns).CombinedOutput()
	if err != nil {
		// Ignore if already exists
		if strings.Contains(string(out), "already exists") {
			return
		}
		fmt.Printf("%s Falha ao criar namespace '%s': %s\n", crossStyle.String(), ns, string(out))
		osExit(1)
	}
}

func runCommand(name string, args ...string) {
	cmd := execCommand(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("%s Executando: %s %s\n", grayStyle.Render("Exec >"), name, strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s Erro ao executar %s\n", crossStyle.String(), name)
		osExit(1)
	}
}

func waitPodReady(label, ns string) {
	cmd := exec.Command("kubectl", "wait", "--for=condition=Ready", "pod", "-l", label, "-n", ns, "--timeout=300s")
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s Timeout aguardando pod %s no namespace %s\n", warningStyle.String(), label, ns)
	}
}

func waitCRD(crdName string) {
	cmd := exec.Command("kubectl", "wait", "--for", "condition=established", "--timeout=60s", "crd/"+crdName)
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s Timeout aguardando CRD %s\n", warningStyle.String(), crdName)
	}
}

func ensureToolsInstalled() {
	if _, err := lookPath("kubectl"); err != nil {
		fmt.Println(crossStyle.Render("kubectl não encontrado."))
		osExit(1)
	}
	if _, err := lookPath("helm"); err != nil {
		fmt.Println(crossStyle.Render("helm não encontrado."))
		osExit(1)
	}
}
