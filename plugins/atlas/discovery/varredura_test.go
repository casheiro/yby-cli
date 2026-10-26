package discovery

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func escrever(t *testing.T, raiz, rel, conteudo string) {
	t.Helper()
	p := filepath.Join(raiz, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

const deploymentYAML = "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n"

func nomesDe(bp *InfraBlueprint) []string {
	var nomes []string
	for _, r := range bp.Resources {
		nomes = append(nomes, r.Kind+"/"+r.Name)
	}
	return nomes
}

func TestScanInfra_RaizDentroDePastaIgnorada(t *testing.T) {
	raiz := filepath.Join(t.TempDir(), "vendor", "proj")
	escrever(t, raiz, "k8s/deploy.yaml", deploymentYAML)

	bp, err := ScanInfra(raiz, IgnoradosPadrao)
	if err != nil {
		t.Fatal(err)
	}
	if len(bp.Resources) != 1 {
		t.Fatalf("esperava 1 recurso numa raiz dentro de vendor/, veio %v", nomesDe(bp))
	}

	bp2, err := ScanWithRules(raiz, IgnoradosPadrao, DefaultRules)
	if err != nil {
		t.Fatal(err)
	}
	escrever(t, raiz, "Dockerfile", "FROM alpine\n")
	bp2, err = ScanWithRules(raiz, IgnoradosPadrao, DefaultRules)
	if err != nil {
		t.Fatal(err)
	}
	if len(bp2.Components) == 0 {
		t.Fatal("ScanWithRules não achou componente numa raiz dentro de vendor/")
	}
}

func TestScanInfra_PastasDeBuildSaemDaVarredura(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, raiz, "k8s/deploy.yaml", deploymentYAML)
	escrever(t, raiz, "dist/x.yaml", deploymentYAML)
	escrever(t, raiz, "build/y.yaml", deploymentYAML)
	escrever(t, raiz, ".terraform/modules/a.tf", "resource \"aws_s3_bucket\" \"b\" {}\n")

	bp, err := ScanInfra(raiz, IgnoradosPadrao)
	if err != nil {
		t.Fatal(err)
	}
	if len(bp.Resources) != 1 || bp.Resources[0].Path != "k8s/deploy.yaml" {
		t.Fatalf("esperava só k8s/deploy.yaml, veio %v", nomesDe(bp))
	}
}

func TestScanInfra_RespeitaGitignore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não instalado")
	}
	raiz := t.TempDir()
	if err := exec.Command("git", "-C", raiz, "init", "-q").Run(); err != nil {
		t.Skipf("git init falhou: %v", err)
	}
	escrever(t, raiz, ".gitignore", "gerado/\nlocal.yaml\n")
	escrever(t, raiz, "k8s/deploy.yaml", deploymentYAML)
	escrever(t, raiz, "gerado/a.yaml", deploymentYAML)
	escrever(t, raiz, "local.yaml", deploymentYAML)

	bp, err := ScanInfra(raiz, IgnoradosPadrao)
	if err != nil {
		t.Fatal(err)
	}
	// k8s/deploy.yaml é novo e não rastreado: tem de entrar mesmo sem commit.
	if len(bp.Resources) != 1 || bp.Resources[0].Path != "k8s/deploy.yaml" {
		t.Fatalf("esperava só k8s/deploy.yaml, veio %v", nomesDe(bp))
	}
}

func TestScanInfra_EntraNoRepositorioAninhado(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não instalado")
	}
	raiz := t.TempDir()
	if err := exec.Command("git", "-C", raiz, "init", "-q").Run(); err != nil {
		t.Skipf("git init falhou: %v", err)
	}
	escrever(t, raiz, "k8s/deploy.yaml", deploymentYAML)
	// Repositório próprio dentro do projeto (como um submódulo): o pai só o
	// conhece como entrada única, mas o conteúdo tem de ser varrido.
	aninhado := filepath.Join(raiz, "charts", "sub")
	escrever(t, raiz, "charts/sub/deploy.yaml", deploymentYAML)
	if err := exec.Command("git", "-C", aninhado, "init", "-q").Run(); err != nil {
		t.Skipf("git init falhou: %v", err)
	}

	bp, err := ScanInfra(raiz, IgnoradosPadrao)
	if err != nil {
		t.Fatal(err)
	}
	if len(bp.Resources) != 2 {
		t.Fatalf("esperava k8s/deploy.yaml e charts/sub/deploy.yaml, veio %v", nomesDe(bp))
	}
}

func TestScanInfra_SopsComCRLFNaoVaiParaK8s(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, raiz, "segredo.yaml",
		"apiVersion: v1\r\nkind: Secret\r\nmetadata:\r\n  name: s\r\nsops:\r\n  version: 3.8.1\r\n")
	escrever(t, raiz, "k8s/deploy.yaml", deploymentYAML)

	bp, err := ScanInfra(raiz, IgnoradosPadrao)
	if err != nil {
		t.Fatal(err)
	}
	if len(bp.Resources) != 1 || bp.Resources[0].Kind != "Deployment" {
		t.Fatalf("o arquivo SOPS com CRLF não devia entrar: %v", nomesDe(bp))
	}
}
