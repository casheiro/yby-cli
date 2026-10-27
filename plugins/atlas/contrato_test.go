package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

// Contrato com a ferramenta atlas_blueprint do Bard (plugins/bard/tools): o Bard manda o
// pedido de testdata/contrato-blueprint-pedido.json e lê a resposta gravada em
// testdata/contrato-blueprint.json. Este lado grava o golden (com -update) e confere que
// ele está atual; o Bard o passa pelo formatador dele. Mudar a resposta do hook context
// sem regravar o golden quebra este teste; regravar sem ajustar o Bard quebra o dele.

// projetoDoContrato monta, numa pasta temporária que vira a atual, um projeto com dois
// módulos Go (um importa o outro por replace), um Dockerfile, um chart com dependência
// remota e manifests com um Deployment e o Service que o seleciona.
func projetoDoContrato(t *testing.T) {
	t.Helper()
	raiz := t.TempDir()
	arquivos := map[string]string{
		"meu-servico/go.mod":                   "module example.com/meu-servico\n\ngo 1.26\n\nrequire (\n\texample.com/lib v0.0.0\n\tgithub.com/gin-gonic/gin v1.10.0\n)\n\nreplace example.com/lib => ../lib\n",
		"meu-servico/main.go":                  "package main\n\nimport (\n\t\"example.com/lib\"\n\t\"github.com/gin-gonic/gin\"\n)\n\nfunc main() { lib.Ola(); gin.New() }\n",
		"meu-servico/Dockerfile":               "FROM golang:1.26\nCOPY . .\n",
		"lib/go.mod":                           "module example.com/lib\n\ngo 1.26\n",
		"lib/lib.go":                           "package lib\n\nfunc Ola() {}\n",
		"charts/api/Chart.yaml":                "apiVersion: v2\nname: api\nversion: 1.0.0\ndependencies:\n  - name: redis\n    version: 17.0.0\n    repository: https://charts.bitnami.com/bitnami\n",
		"charts/api/templates/deployment.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api-server\n",
		"deploy/web.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  namespace: loja\nspec:\n  selector:\n    matchLabels:\n      app: web\n  template:\n    metadata:\n      labels:\n        app: web\n    spec:\n      containers:\n        - name: web\n          image: loja/web:1.2.3\n" +
			"---\napiVersion: v1\nkind: Service\nmetadata:\n  name: web\n  namespace: loja\nspec:\n  selector:\n    app: web\n  ports:\n    - port: 80\n",
	}
	for nome, conteudo := range arquivos {
		caminho := filepath.Join(raiz, filepath.FromSlash(nome))
		if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(raiz)
}

// hookContext roda o executar com o pedido dado na variável do ambiente, como o core e o
// Bard rodam, e devolve o stdout.
func hookContext(t *testing.T, pedido string) []byte {
	t.Helper()
	t.Setenv(plugin.EnvPedido, pedido)
	t.Setenv(plugin.EnvPedidoArquivo, "")
	var saida, erros bytes.Buffer
	if codigo := executar(&saida, &erros); codigo != 0 {
		t.Fatalf("o hook context saiu com %d\nstdout: %s\nstderr: %s", codigo, saida.String(), erros.String())
	}
	return saida.Bytes()
}

func TestContrato_BlueprintDoHookContext(t *testing.T) {
	pedido, err := os.ReadFile(filepath.Join(dirTestdata, "contrato-blueprint-pedido.json"))
	if err != nil {
		t.Fatal(err)
	}
	projetoDoContrato(t)
	saida := hookContext(t, strings.TrimSpace(string(pedido)))

	// O golden é a resposta indentada, para o diff ser legível; o conteúdo é o mesmo.
	var formatado bytes.Buffer
	if err := json.Indent(&formatado, saida, "", "  "); err != nil {
		t.Fatalf("a resposta não é JSON: %v\n%s", err, saida)
	}
	conferirGolden(t, "contrato-blueprint.json", formatado.String())
}

// Sem o argumento infra (o pedido do yby init), o hook não varre a infraestrutura.
func TestHookContext_SemInfraSoOBlueprint(t *testing.T) {
	projetoDoContrato(t)
	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(hookContext(t, `{"hook":"context"}`), &resp); err != nil {
		t.Fatal(err)
	}
	if _, ok := resp.Data["blueprint"]; !ok {
		t.Errorf("a resposta deveria ter o blueprint: %v", resp.Data)
	}
	if _, ok := resp.Data["infra"]; ok {
		t.Errorf("sem o argumento infra, a resposta não deveria ter a infraestrutura")
	}
}
