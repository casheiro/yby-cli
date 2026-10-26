package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// ---- ShouldIgnore ----

func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		ignores []string
		want    bool
	}{
		{
			name:    "diretório vendor é ignorado",
			path:    "vendor/pkg/foo.go",
			ignores: []string{"vendor"},
			want:    true,
		},
		{
			name:    "my-vendor-lib NÃO é ignorado",
			path:    "my-vendor-lib/main.go",
			ignores: []string{"vendor"},
			want:    false,
		},
		{
			name:    "node_modules é ignorado",
			path:    "a/b/node_modules/c.js",
			ignores: []string{"node_modules"},
			want:    true,
		},
		{
			name:    "my-node_modules-lib NÃO é ignorado",
			path:    "my-node_modules-lib/x.go",
			ignores: []string{"node_modules"},
			want:    false,
		},
		{
			name:    "sem ignores retorna false",
			path:    "vendor/foo.go",
			ignores: nil,
			want:    false,
		},
		{
			name:    "múltiplos ignores",
			path:    "src/dist/bundle.js",
			ignores: []string{"vendor", "node_modules", "dist"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIgnore(tt.path, tt.ignores)
			if got != tt.want {
				t.Errorf("ShouldIgnore(%q, %v) = %v, esperado %v", tt.path, tt.ignores, got, tt.want)
			}
		})
	}
}

// ---- Blueprint e Component ----

func TestBlueprint_Empty(t *testing.T) {
	bp := &Blueprint{
		Components: []Component{},
		Roots:      []string{"/project"},
	}
	if len(bp.Components) != 0 {
		t.Errorf("esperado nenhum componente, obtido %d", len(bp.Components))
	}
	if len(bp.Roots) != 1 || bp.Roots[0] != "/project" {
		t.Errorf("esperado roots [/project], obtido %v", bp.Roots)
	}
}

func TestComponent_Fields(t *testing.T) {
	c := Component{
		Name: "backend",
		Type: "app",
		Path: "/project/backend",
		Tags: []string{"golang", "api"},
		Metadata: map[string]string{
			"module": "github.com/example/backend",
		},
	}
	if c.Name != "backend" || c.Type != "app" {
		t.Errorf("campos do Component não correspondem: %+v", c)
	}
	if len(c.Tags) != 2 {
		t.Errorf("esperado 2 tags, obtido %d", len(c.Tags))
	}
	if c.Metadata["module"] != "github.com/example/backend" {
		t.Errorf("esperado metadata module, obtido %v", c.Metadata)
	}
}

// ---- Testes do Scan ----

func TestScan_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if bp == nil {
		t.Fatal("esperado Blueprint não-nulo")
	}
	if len(bp.Components) != 0 {
		t.Errorf("esperado nenhum componente em dir vazio, obtido %d", len(bp.Components))
	}
}

func TestScan_WithGoModule(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "myservice")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module myservice\n\ngo 1.26\n"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if len(bp.Components) == 0 {
		t.Error("esperado pelo menos 1 componente (módulo Go), obtido 0")
	}
}

func TestScan_WithPackageJson(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "frontend")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "package.json"), []byte(`{"name":"frontend"}`), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if len(bp.Components) == 0 {
		t.Error("esperado pelo menos 1 componente (app Node.js), obtido 0")
	}
}

func TestScan_WithDockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "myservice")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "Dockerfile"), []byte("FROM alpine\n"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if len(bp.Components) == 0 {
		t.Error("esperado pelo menos 1 componente (Dockerfile/infra), obtido 0")
	}
}

func TestScan_IgnoredDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	ignoredDir := filepath.Join(tmpDir, "node_modules", "some-pkg")
	os.MkdirAll(ignoredDir, 0755)
	os.WriteFile(filepath.Join(ignoredDir, "go.mod"), []byte("module ignored"), 0644)

	bp, err := Scan(tmpDir, []string{"node_modules"})
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if len(bp.Components) != 0 {
		t.Errorf("esperado 0 componentes (dir ignorado), obtido %d", len(bp.Components))
	}
}

func TestScan_NoDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "myapi")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module myapi"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if len(bp.Components) != 1 {
		t.Errorf("esperado 1 componente (sem duplicatas), obtido %d", len(bp.Components))
	}
}

func TestScan_RootsField(t *testing.T) {
	tmpDir := t.TempDir()
	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if len(bp.Roots) != 1 || bp.Roots[0] != "." {
		t.Errorf("esperado Roots=[.], obtido %v", bp.Roots)
	}
}

func TestScan_CaminhosRelativosComBarra(t *testing.T) {
	tmpDir := t.TempDir()
	api := filepath.Join(tmpDir, "services", "api")
	os.MkdirAll(api, 0755)
	os.WriteFile(filepath.Join(api, "go.mod"), []byte("module api"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if len(bp.Components) != 1 || bp.Components[0].Path != "services/api" {
		t.Errorf("esperado Path 'services/api', obtido %+v", bp.Components)
	}
}

func TestScanInfra_RootPathRelativo(t *testing.T) {
	bp, err := ScanInfra(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("ScanInfra falhou: %v", err)
	}
	if bp.RootPath != "." {
		t.Errorf("esperado RootPath '.', obtido %q", bp.RootPath)
	}
}

func TestScan_InvalidPath(t *testing.T) {
	_, err := Scan("/nonexistent/path/xyz123", nil)
	if err == nil {
		t.Error("esperado erro para caminho inexistente, obtido nil")
	}
}

// ---- Novos testes: Helm, Kustomize, Dockerfile variante, go.mod metadata, relações ----

func TestScan_WithChartYaml(t *testing.T) {
	tmpDir := t.TempDir()
	chartDir := filepath.Join(tmpDir, "meu-chart")
	os.MkdirAll(chartDir, 0755)
	os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: meu-chart\n"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, comp := range bp.Components {
		if comp.Type == "helm" && comp.Name == "meu-chart" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Error("esperado componente do tipo 'helm' com nome 'meu-chart'")
	}
}

func TestScan_WithKustomization(t *testing.T) {
	tmpDir := t.TempDir()
	kustomizeDir := filepath.Join(tmpDir, "overlays")
	os.MkdirAll(kustomizeDir, 0755)
	os.WriteFile(filepath.Join(kustomizeDir, "kustomization.yaml"), []byte("resources:\n- ../base\n"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, comp := range bp.Components {
		if comp.Type == "kustomize" && comp.Name == "overlays" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Error("esperado componente do tipo 'kustomize' com nome 'overlays'")
	}
}

func TestScan_WithDockerfileVariant(t *testing.T) {
	tmpDir := t.TempDir()
	serviceDir := filepath.Join(tmpDir, "api")
	os.MkdirAll(serviceDir, 0755)
	os.WriteFile(filepath.Join(serviceDir, "Dockerfile.prod"), []byte("FROM golang:1.26\n"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, comp := range bp.Components {
		if comp.Type == "infra" && comp.Name == "api" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Error("esperado componente do tipo 'infra' para Dockerfile.prod")
	}
}

func TestScan_GoModMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "meu-app")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module github.com/exemplo/meu-app\n\ngo 1.26\n"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, comp := range bp.Components {
		if comp.Type == "app" && comp.Name == "meu-app" {
			if comp.Metadata == nil {
				t.Error("esperado Metadata preenchido para componente Go")
				break
			}
			if comp.Metadata["module"] != "github.com/exemplo/meu-app" {
				t.Errorf("esperado module 'github.com/exemplo/meu-app', obtido %q", comp.Metadata["module"])
			}
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Error("esperado componente 'meu-app' com metadata de módulo Go")
	}
}

func TestScan_DetectsRelations(t *testing.T) {
	tmpDir := t.TempDir()

	// Criar componente app com go.mod que tem replace local
	appDir := filepath.Join(tmpDir, "app")
	libDir := filepath.Join(tmpDir, "lib")
	os.MkdirAll(appDir, 0755)
	os.MkdirAll(libDir, 0755)

	os.WriteFile(filepath.Join(libDir, "go.mod"), []byte("module github.com/exemplo/lib\n\ngo 1.26\n"), 0644)
	os.WriteFile(filepath.Join(appDir, "go.mod"), []byte(
		"module github.com/exemplo/app\n\ngo 1.26\n\nrequire github.com/exemplo/lib v0.0.0\n\nreplace github.com/exemplo/lib => ../lib\n",
	), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, rel := range bp.Relations {
		if rel.From == "app" && rel.To == "lib" && rel.Type == "imports" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Errorf("esperado relação 'imports' de app para lib, relações encontradas: %+v", bp.Relations)
	}
}

func TestScan_DetectsDockerfileRelations(t *testing.T) {
	tmpDir := t.TempDir()

	// Criar componente infra com Dockerfile que referencia outro diretório
	appDir := filepath.Join(tmpDir, "app")
	infraDir := filepath.Join(tmpDir, "infra")
	os.MkdirAll(appDir, 0755)
	os.MkdirAll(infraDir, 0755)

	os.WriteFile(filepath.Join(appDir, "go.mod"), []byte("module github.com/exemplo/app\n\ngo 1.26\n"), 0644)
	os.WriteFile(filepath.Join(infraDir, "Dockerfile"), []byte("FROM golang:1.26\nCOPY app/ /src/\n"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, rel := range bp.Relations {
		if rel.From == "infra" && rel.To == "app" && rel.Type == "builds" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Errorf("esperado relação 'builds' de infra para app, relações encontradas: %+v", bp.Relations)
	}
}

func TestScan_DetectsLanguageAndFramework(t *testing.T) {
	tmpDir := t.TempDir()

	// Go com Gin
	goDir := filepath.Join(tmpDir, "api-go")
	os.MkdirAll(goDir, 0755)
	os.WriteFile(filepath.Join(goDir, "go.mod"), []byte(
		"module myapi\n\ngo 1.26\n\nrequire github.com/gin-gonic/gin v1.9.1\n",
	), 0644)

	// Node.js com Express
	nodeDir := filepath.Join(tmpDir, "api-node")
	os.MkdirAll(nodeDir, 0755)
	os.WriteFile(filepath.Join(nodeDir, "package.json"), []byte(
		`{"name":"api-node","dependencies":{"express":"^4.18.0"}}`,
	), 0644)

	// Dockerfile (sem linguagem/framework)
	infraDir := filepath.Join(tmpDir, "infra")
	os.MkdirAll(infraDir, 0755)
	os.WriteFile(filepath.Join(infraDir, "Dockerfile"), []byte("FROM alpine\n"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	for _, comp := range bp.Components {
		switch comp.Name {
		case "api-go":
			if comp.Language != "go" {
				t.Errorf("api-go: Language = %q, esperado %q", comp.Language, "go")
			}
			if comp.Framework != "gin" {
				t.Errorf("api-go: Framework = %q, esperado %q", comp.Framework, "gin")
			}
		case "api-node":
			if comp.Language != "nodejs" {
				t.Errorf("api-node: Language = %q, esperado %q", comp.Language, "nodejs")
			}
			if comp.Framework != "express" {
				t.Errorf("api-node: Framework = %q, esperado %q", comp.Framework, "express")
			}
		case "infra":
			if comp.Language != "" {
				t.Errorf("infra: Language = %q, esperado vazio", comp.Language)
			}
			if comp.Framework != "" {
				t.Errorf("infra: Framework = %q, esperado vazio", comp.Framework)
			}
		}
	}
}

func TestScan_DetectsGoImportRelations(t *testing.T) {
	tmpDir := t.TempDir()

	// Criar módulo raiz
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module github.com/exemplo/monorepo\n\ngo 1.26\n"), 0644)

	// Criar lib com go.mod
	libDir := filepath.Join(tmpDir, "lib")
	os.MkdirAll(libDir, 0755)
	os.WriteFile(filepath.Join(libDir, "go.mod"), []byte("module github.com/exemplo/monorepo/lib\n\ngo 1.26\n"), 0644)

	// Criar app que importa lib
	appDir := filepath.Join(tmpDir, "app")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "go.mod"), []byte("module github.com/exemplo/monorepo/app\n\ngo 1.26\n"), 0644)
	os.WriteFile(filepath.Join(appDir, "main.go"), []byte(`package main

import (
	"fmt"
	"github.com/exemplo/monorepo/lib"
)

func main() {
	fmt.Println(lib.Hello())
}
`), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, rel := range bp.Relations {
		if rel.From == "app" && rel.To == "lib" && rel.Type == "imports" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Errorf("esperado relação 'imports' de app para lib via Go imports, relações encontradas: %+v", bp.Relations)
	}
}

func TestScan_DetectsDockerFromRelations(t *testing.T) {
	tmpDir := t.TempDir()

	// Criar componente app
	appDir := filepath.Join(tmpDir, "app")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "go.mod"), []byte("module github.com/exemplo/app\n\ngo 1.26\n"), 0644)

	// Criar componente infra com COPY --from referenciando componente
	infraDir := filepath.Join(tmpDir, "infra")
	os.MkdirAll(infraDir, 0755)
	os.WriteFile(filepath.Join(infraDir, "Dockerfile"), []byte(`FROM golang:1.26 AS builder
RUN go build -o /bin/app .
FROM alpine AS runtime
COPY --from=builder /bin/app /usr/local/bin/
COPY --from=app ./dist /app/
`), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, rel := range bp.Relations {
		if rel.From == "infra" && rel.To == "app" && rel.Type == "builds" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Errorf("esperado relação 'builds' de infra para app via COPY --from, relações encontradas: %+v", bp.Relations)
	}
}

func TestScan_DetectsHelmRemoteRelations(t *testing.T) {
	tmpDir := t.TempDir()

	chartDir := filepath.Join(tmpDir, "charts", "myapp")
	os.MkdirAll(chartDir, 0755)
	os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`apiVersion: v2
name: myapp
dependencies:
  - name: redis
    repository: "https://charts.bitnami.com/bitnami"
    version: "17.0.0"
`), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, rel := range bp.Relations {
		if rel.Type == "depends" && rel.To == "https://charts.bitnami.com/bitnami" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Errorf("esperado relação 'depends' para repositório remoto Helm, relações encontradas: %+v", bp.Relations)
	}
}

func TestScan_DetectsPackageJsonRelations(t *testing.T) {
	tmpDir := t.TempDir()

	// Criar lib local
	libDir := filepath.Join(tmpDir, "shared-lib")
	os.MkdirAll(libDir, 0755)
	os.WriteFile(filepath.Join(libDir, "package.json"), []byte(`{"name":"shared-lib"}`), 0644)

	// Criar app com dependência local
	appDir := filepath.Join(tmpDir, "webapp")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "package.json"), []byte(`{
		"name": "webapp",
		"dependencies": {
			"shared-lib": "file:../shared-lib",
			"express": "^4.18.0"
		}
	}`), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, rel := range bp.Relations {
		if rel.From == "webapp" && rel.To == "shared-lib" && rel.Type == "imports" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Errorf("esperado relação 'imports' de webapp para shared-lib via package.json, relações encontradas: %+v", bp.Relations)
	}
}

func TestScan_DetectsHelmRelations(t *testing.T) {
	tmpDir := t.TempDir()

	// Criar chart Helm com dependência local
	chartDir := filepath.Join(tmpDir, "charts", "main")
	depDir := filepath.Join(tmpDir, "charts", "dep")
	os.MkdirAll(chartDir, 0755)
	os.MkdirAll(depDir, 0755)

	os.WriteFile(filepath.Join(depDir, "Chart.yaml"), []byte("apiVersion: v2\nname: dep\n"), 0644)
	os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(
		"apiVersion: v2\nname: main\ndependencies:\n  - name: dep\n    repository: \"file://../dep\"\n",
	), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	encontrado := false
	for _, rel := range bp.Relations {
		if rel.Type == "deploys" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Errorf("esperado relação 'deploys' entre charts Helm, relações encontradas: %+v", bp.Relations)
	}
}

// escreverArquivos grava os arquivos (caminho relativo com '/') sob dir.
func escreverArquivos(t *testing.T, dir string, arquivos map[string]string) {
	t.Helper()
	for nome, conteudo := range arquivos {
		caminho := filepath.Join(dir, filepath.FromSlash(nome))
		if err := os.MkdirAll(filepath.Dir(caminho), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(conteudo), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// contarRelacoes devolve quantas relações batem com (from, to, tipo).
func contarRelacoes(rels []Relation, from, to, tipo string) int {
	n := 0
	for _, r := range rels {
		if r.From == from && r.To == to && r.Type == tipo {
			n++
		}
	}
	return n
}

func TestScan_DockerfileNaoCasaPontoNemPrefixo(t *testing.T) {
	tmpDir := t.TempDir()
	escreverArquivos(t, tmpDir, map[string]string{
		"go.mod":                   "module github.com/x/mono\n\ngo 1.26\n",
		"services/api/go.mod":      "module github.com/x/mono/services/api\n\ngo 1.26\n",
		"services/api/Dockerfile":  "FROM golang:1.26\nCOPY . .\n",
		"apigateway/go.mod":        "module github.com/x/mono/apigateway\n\ngo 1.26\n",
		"deploy/Dockerfile":        "FROM golang:1.26\nCOPY apigateway/ .\n",
		"api/go.mod":               "module github.com/x/mono/api\n\ngo 1.26\n",
		"services/go.mod":          "module github.com/x/mono/services\n\ngo 1.26\n",
		"services/api2/go.mod":     "module github.com/x/mono/services/api2\n\ngo 1.26\n",
		"services/api2/Dockerfile": "FROM golang:1.26\nCOPY . .\n",
		"tools/Dockerfile":         "FROM golang:1.26\nCOPY services/api2/x .\n",
	})

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	if contarRelacoes(bp.Relations, "services/api", ".", "builds") != 0 {
		t.Errorf("COPY . . não pode ligar o componente à raiz: %+v", bp.Relations)
	}
	if contarRelacoes(bp.Relations, "deploy", "api", "builds") != 0 {
		t.Errorf("'api' não pode casar com 'apigateway': %+v", bp.Relations)
	}
	if contarRelacoes(bp.Relations, "services/api2", "services", "builds") != 0 {
		t.Errorf("COPY . . não pode ligar o componente ao pai: %+v", bp.Relations)
	}
	if contarRelacoes(bp.Relations, "tools", "services", "builds") != 0 ||
		contarRelacoes(bp.Relations, "tools", "services/api2", "builds") != 1 {
		t.Errorf("a origem liga só ao componente mais específico: %+v", bp.Relations)
	}
	if contarRelacoes(bp.Relations, "deploy", "apigateway", "builds") != 1 {
		t.Errorf("esperada uma relação deploy -> apigateway: %+v", bp.Relations)
	}
}

func TestScan_ImportsGoNaoEntraEmPastasIgnoradas(t *testing.T) {
	tmpDir := t.TempDir()
	imp := "package p\n\nimport \"github.com/x/mono/lib\"\n"
	escreverArquivos(t, tmpDir, map[string]string{
		"go.mod":                "module github.com/x/mono\n\ngo 1.26\n",
		"lib/go.mod":            "module github.com/x/mono/lib\n\ngo 1.26\n",
		"node_modules/pkg/p.go": imp,
		"dist/d.go":             imp,
		"meu_ignorado/i.go":     imp,
	})

	ignores := append(append([]string(nil), IgnoradosPadrao...), "meu_ignorado")
	bp, err := Scan(tmpDir, ignores)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if n := contarRelacoes(bp.Relations, ".", "lib", "imports"); n != 0 {
		t.Errorf("imports de pastas ignoradas não geram relação: %+v", bp.Relations)
	}

	// Controle positivo: o mesmo import em pasta não ignorada gera a relação.
	escreverArquivos(t, tmpDir, map[string]string{"src/s.go": imp})
	bp, err = Scan(tmpDir, ignores)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if n := contarRelacoes(bp.Relations, ".", "lib", "imports"); n != 1 {
		t.Errorf("controle positivo: esperada 1 relação . -> lib, veio %d: %+v", n, bp.Relations)
	}
}

func TestScan_ImportsGoDeduplicadosEComFronteira(t *testing.T) {
	tmpDir := t.TempDir()
	escreverArquivos(t, tmpDir, map[string]string{
		"go.mod":      "module github.com/x/mono\n\ngo 1.26\n",
		"lib/go.mod":  "module github.com/x/mono/lib\n\ngo 1.26\n",
		"lib2/go.mod": "module github.com/x/mono/lib2\n\ngo 1.26\n",
		"app/go.mod":  "module github.com/x/mono/app\n\ngo 1.26\n",
		"app/main.go": "package main\n\nimport (\n\t\"github.com/x/mono/lib/a\"\n\t\"github.com/x/mono/lib/b\"\n)\n",
		// subpastas e import de linha única
		"app/cmd/serve/serve.go":  "package serve\n\nimport \"github.com/x/mono/lib2\"\n",
		"app/internal/dep/dep.go": "package dep\n\nimport l \"github.com/x/mono/lib/c\"\n",
	})

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	if n := contarRelacoes(bp.Relations, "app", "lib", "imports"); n != 1 {
		t.Errorf("esperada 1 relação app -> lib (deduplicada), veio %d: %+v", n, bp.Relations)
	}
	if n := contarRelacoes(bp.Relations, "app", "lib2", "imports"); n != 1 {
		t.Errorf("import de linha única em subpasta deveria gerar app -> lib2, veio %d: %+v", n, bp.Relations)
	}
	if n := contarRelacoes(bp.Relations, "app", ".", "imports"); n != 0 {
		t.Errorf("o módulo mais longo vence: não pode haver app -> '.': %+v", bp.Relations)
	}
}
