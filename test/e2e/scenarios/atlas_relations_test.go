//go:build e2e

package scenarios

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compileAtlas compila o binário do plugin Atlas em um diretório temporário.
func compileAtlas(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binary := filepath.Join(tmpDir, "atlas")
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err, "falha ao resolver raiz do projeto")

	cmd := exec.Command("go", "build", "-o", binary, "./plugins/atlas")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "falha ao compilar atlas: %s", string(out))
	return binary
}

// runAtlasContext executa o atlas com hook "context" no workDir e retorna o blueprint.
func runAtlasContext(t *testing.T, binary, workDir string) map[string]interface{} {
	t.Helper()
	req := plugin.PluginRequest{Hook: "context"}
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	cmd := exec.Command(binary)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "YBY_PLUGIN_REQUEST="+string(reqJSON))
	output, err := cmd.Output()
	require.NoError(t, err, "falha ao executar atlas: %s", string(output))

	var resp plugin.PluginResponse
	require.NoError(t, json.Unmarshal(output, &resp), "falha ao decodificar resposta atlas")
	require.Empty(t, resp.Error, "atlas retornou erro: %s", resp.Error)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Data deveria ser map[string]interface{}")
	return data
}

// writeFile cria um arquivo com conteúdo no diretório especificado.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
}

// findRelation procura uma relação com os campos esperados na lista de relações.
func findRelation(relations []interface{}, from, to, relType string) bool {
	for _, r := range relations {
		rel, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		if rel["from"] == from && rel["to"] == to && rel["type"] == relType {
			return true
		}
	}
	return false
}

func TestAtlas_GoImportRelations(t *testing.T) {
	binary := compileAtlas(t)
	workDir := t.TempDir()

	// Criar monorepo com dois módulos Go
	writeFile(t, workDir, "go.mod", "module github.com/test/monorepo\n\ngo 1.26\n")
	writeFile(t, workDir, "app/go.mod", "module github.com/test/monorepo/app\n\ngo 1.26\n")
	writeFile(t, workDir, "app/main.go", `package main

import (
	"fmt"

	"github.com/test/monorepo/lib"
)

func main() {
	fmt.Println(lib.Hello())
}
`)
	writeFile(t, workDir, "lib/go.mod", "module github.com/test/monorepo/lib\n\ngo 1.26\n")
	writeFile(t, workDir, "lib/lib.go", "package lib\n\nfunc Hello() string { return \"hello\" }\n")

	data := runAtlasContext(t, binary, workDir)

	bp, ok := data["blueprint"].(map[string]interface{})
	require.True(t, ok, "blueprint deveria ser um mapa")

	relations, ok := bp["relations"].([]interface{})
	require.True(t, ok, "relations deveria ser um slice")

	assert.True(t, findRelation(relations, "app", "lib", "imports"),
		"deveria detectar relação de import Go: app -> lib, relações encontradas: %v", relations)
}

func TestAtlas_DockerFromRelations(t *testing.T) {
	binary := compileAtlas(t)
	workDir := t.TempDir()

	// Criar componente com Dockerfile multi-stage
	writeFile(t, workDir, "app/go.mod", "module github.com/test/app\n\ngo 1.26\n")
	writeFile(t, workDir, "app/Dockerfile", `FROM golang:1.26 AS builder
WORKDIR /app
COPY . .
RUN go build -o main .

FROM alpine:3.20
COPY --from=builder /app/main /usr/local/bin/main
CMD ["main"]
`)

	data := runAtlasContext(t, binary, workDir)

	bp, ok := data["blueprint"].(map[string]interface{})
	require.True(t, ok, "blueprint deveria ser um mapa")

	// Verificar que componentes foram detectados (app como "app" via go.mod, app como "infra" via Dockerfile)
	components, ok := bp["components"].([]interface{})
	require.True(t, ok, "components deveria ser um slice")
	require.NotEmpty(t, components, "deveria detectar ao menos um componente")

	// Verificar que stages Docker internos (builder) NÃO geram relações externas falsas
	// O COPY --from=builder é uma referência interna ao stage, não a outro componente
	relations, _ := bp["relations"].([]interface{})
	for _, r := range relations {
		rel, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		// Não deveria ter relação "builds" referenciando "builder" como componente externo
		assert.NotEqual(t, "builder", rel["to"],
			"stage Docker interno 'builder' não deveria aparecer como destino de relação")
	}
}

func TestAtlas_PackageJsonRelations(t *testing.T) {
	binary := compileAtlas(t)
	workDir := t.TempDir()

	// Criar projeto Node.js com dependência local
	writeFile(t, workDir, "frontend/package.json", `{
  "name": "frontend",
  "version": "1.0.0",
  "dependencies": {
    "shared": "file:../shared"
  }
}`)
	writeFile(t, workDir, "shared/package.json", `{
  "name": "shared",
  "version": "1.0.0"
}`)

	data := runAtlasContext(t, binary, workDir)

	bp, ok := data["blueprint"].(map[string]interface{})
	require.True(t, ok, "blueprint deveria ser um mapa")

	relations, ok := bp["relations"].([]interface{})
	require.True(t, ok, "relations deveria ser um slice")

	assert.True(t, findRelation(relations, "frontend", "shared", "imports"),
		"deveria detectar relação package.json: frontend -> shared, relações: %v", relations)
}

func TestAtlas_ShouldIgnore_NoFalsePositives(t *testing.T) {
	binary := compileAtlas(t)
	workDir := t.TempDir()

	// "my-vendor-lib" NÃO deve ser ignorado (contém "vendor" no nome mas não é um segmento exato)
	writeFile(t, workDir, "my-vendor-lib/go.mod", "module github.com/test/my-vendor-lib\n\ngo 1.26\n")
	// "vendor" DEVE ser ignorado (é um segmento exato do caminho)
	writeFile(t, workDir, "vendor/lib/go.mod", "module github.com/test/vendor-lib\n\ngo 1.26\n")

	data := runAtlasContext(t, binary, workDir)

	bp, ok := data["blueprint"].(map[string]interface{})
	require.True(t, ok, "blueprint deveria ser um mapa")

	components, ok := bp["components"].([]interface{})
	require.True(t, ok, "components deveria ser um slice")

	// Verificar que my-vendor-lib foi detectado
	foundMyVendor := false
	foundVendor := false
	for _, c := range components {
		comp, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := comp["name"].(string)
		if name == "my-vendor-lib" {
			foundMyVendor = true
		}
		if name == "lib" {
			// Verificar se o path contém "vendor/"
			path, _ := comp["path"].(string)
			if filepath.Base(filepath.Dir(path)) == "vendor" || filepath.Base(path) == "vendor" {
				foundVendor = true
			}
		}
	}

	assert.True(t, foundMyVendor, "my-vendor-lib deveria ser detectado como componente")
	assert.False(t, foundVendor, "vendor/lib NÃO deveria ser detectado (diretório vendor é ignorado)")
}

func TestAtlas_HelmRemoteRelations(t *testing.T) {
	binary := compileAtlas(t)
	workDir := t.TempDir()

	// Criar Chart.yaml com dependência remota
	writeFile(t, workDir, "infra/Chart.yaml", `apiVersion: v2
name: my-chart
version: 0.1.0
dependencies:
  - name: redis
    version: "18.x.x"
    repository: https://charts.bitnami.com/bitnami
`)

	data := runAtlasContext(t, binary, workDir)

	bp, ok := data["blueprint"].(map[string]interface{})
	require.True(t, ok, "blueprint deveria ser um mapa")

	relations, ok := bp["relations"].([]interface{})
	require.True(t, ok, "relations deveria ser um slice")

	assert.True(t, findRelation(relations, "infra", "https://charts.bitnami.com/bitnami", "depends"),
		"deveria detectar relação Helm remota: infra -> bitnami, relações: %v", relations)
}
