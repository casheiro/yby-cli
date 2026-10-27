package discovery

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// copiarFixture copia testdata/<nome> para um diretório temporário e devolve a
// raiz da cópia, para a varredura não depender do git do repositório.
func copiarFixture(t *testing.T, nome string) string {
	t.Helper()
	origem := filepath.Join("testdata", nome)
	raiz := t.TempDir()
	err := filepath.WalkDir(origem, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(origem, p)
		if err != nil {
			return err
		}
		dados, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		escrever(t, raiz, filepath.ToSlash(rel), string(dados))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return raiz
}

// Base em apps/api/base usada por dois overlays com namespaces diferentes: os
// recursos da base entram uma vez em cada namespace, os patches não viram
// recursos e as kustomizations se ligam pelo caminho e aos manifests que listam.
func TestScanInfra_KustomizeBaseEOverlays(t *testing.T) {
	bp, err := ScanInfra(copiarFixture(t, "kustomize"), nil)
	if err != nil {
		t.Fatal(err)
	}

	var recursos []string
	for _, r := range bp.Resources {
		recursos = append(recursos, r.ID())
	}
	sort.Strings(recursos)
	esperados := []string{
		"Deployment/prod/api",
		"Deployment/staging/api",
		"Ingress/prod/web",
		"Kustomization/apps/api/base",
		"Kustomization/apps/api/overlays/prod",
		"Kustomization/apps/api/overlays/staging",
		"Service/prod/api",
		"Service/staging/api",
	}
	if strings.Join(recursos, "\n") != strings.Join(esperados, "\n") {
		t.Errorf("recursos:\n%s\nesperados:\n%s", strings.Join(recursos, "\n"), strings.Join(esperados, "\n"))
	}

	var relacoes []string
	for _, r := range bp.Relations {
		relacoes = append(relacoes, r.From+" -"+r.Type+"-> "+r.To)
	}
	sort.Strings(relacoes)
	esperadas := []string{
		"Ingress/prod/web -routes-> Service/prod/api",
		"Kustomization/apps/api/base -includes-> Deployment/prod/api",
		"Kustomization/apps/api/base -includes-> Deployment/staging/api",
		"Kustomization/apps/api/base -includes-> Service/prod/api",
		"Kustomization/apps/api/base -includes-> Service/staging/api",
		"Kustomization/apps/api/overlays/prod -includes-> Ingress/prod/web",
		"Kustomization/apps/api/overlays/prod -includes-> Kustomization/apps/api/base",
		"Kustomization/apps/api/overlays/staging -includes-> Kustomization/apps/api/base",
		"Service/prod/api -selects-> Deployment/prod/api",
		"Service/staging/api -selects-> Deployment/staging/api",
	}
	if strings.Join(relacoes, "\n") != strings.Join(esperadas, "\n") {
		t.Errorf("relações:\n%s\nesperadas:\n%s", strings.Join(relacoes, "\n"), strings.Join(esperadas, "\n"))
	}
}

// Dois ambientes com recursos de mesmo nome e o mesmo módulo local: cada
// recurso leva o diretório no ID, a chamada liga ao diretório resolvido do
// módulo, que liga aos recursos dele, e as referências entre recursos viram
// relações, sem contar comentários, texto de strings e heredocs.
func TestScanInfra_TerraformAmbientesEModulo(t *testing.T) {
	bp, err := ScanInfra(copiarFixture(t, "terraform"), nil)
	if err != nil {
		t.Fatal(err)
	}

	var recursos []string
	for _, r := range bp.Resources {
		recursos = append(recursos, r.ID())
	}
	sort.Strings(recursos)
	esperados := []string{
		"TerraformData/modules/net/aws_availability_zones.disponiveis",
		"TerraformModule/envs/dev/module.net",
		"TerraformModule/envs/prod/module.net",
		"TerraformModule/modules/net",
		"TerraformResource/envs/dev/aws_instance.app",
		"TerraformResource/envs/dev/aws_s3_bucket.logs",
		"TerraformResource/envs/dev/aws_s3_bucket_policy.logs",
		"TerraformResource/envs/prod/aws_s3_bucket.logs",
		"TerraformResource/modules/net/aws_subnet.a",
		"TerraformResource/modules/net/aws_vpc.this",
	}
	if strings.Join(recursos, "\n") != strings.Join(esperados, "\n") {
		t.Errorf("recursos:\n%s\nesperados:\n%s", strings.Join(recursos, "\n"), strings.Join(esperados, "\n"))
	}

	var relacoes []string
	for _, r := range bp.Relations {
		relacoes = append(relacoes, r.From+" -"+r.Type+"-> "+r.To)
	}
	sort.Strings(relacoes)
	esperadas := []string{
		"TerraformModule/envs/dev/module.net -includes-> TerraformModule/modules/net",
		"TerraformModule/envs/prod/module.net -includes-> TerraformModule/modules/net",
		"TerraformModule/modules/net -includes-> TerraformData/modules/net/aws_availability_zones.disponiveis",
		"TerraformModule/modules/net -includes-> TerraformResource/modules/net/aws_subnet.a",
		"TerraformModule/modules/net -includes-> TerraformResource/modules/net/aws_vpc.this",
		"TerraformResource/envs/dev/aws_instance.app -references-> TerraformModule/envs/dev/module.net",
		"TerraformResource/envs/dev/aws_instance.app -references-> TerraformResource/envs/dev/aws_s3_bucket.logs",
		"TerraformResource/envs/dev/aws_s3_bucket_policy.logs -references-> TerraformResource/envs/dev/aws_s3_bucket.logs",
		"TerraformResource/modules/net/aws_subnet.a -references-> TerraformData/modules/net/aws_availability_zones.disponiveis",
		"TerraformResource/modules/net/aws_subnet.a -references-> TerraformResource/modules/net/aws_vpc.this",
	}
	if strings.Join(relacoes, "\n") != strings.Join(esperadas, "\n") {
		t.Errorf("relações:\n%s\nesperadas:\n%s", strings.Join(relacoes, "\n"), strings.Join(esperadas, "\n"))
	}
}
