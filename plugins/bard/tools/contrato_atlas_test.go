package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/plugin"
)

// Contrato com o Atlas: os dois arquivos ficam no testdata do Atlas, que é quem escreve a
// resposta. O pedido é o que a ferramenta manda; a resposta é o golden que o teste
// TestContrato_BlueprintDoHookContext do Atlas grava rodando o hook context de verdade num
// projeto de exemplo (rode go test ./plugins/atlas/ -run TestContrato -update ao mudar o
// Atlas, e este teste diz se o Bard ainda entende a resposta).
var dirContratoAtlas = filepath.Join("..", "..", "atlas", "testdata")

// A ferramenta manda ao Atlas o mesmo pedido com que o golden foi gravado.
func TestContrato_AtlasPedido(t *testing.T) {
	dados, err := os.ReadFile(filepath.Join(dirContratoAtlas, "contrato-blueprint-pedido.json"))
	if err != nil {
		t.Fatal(err)
	}
	var esperado plugin.PluginRequest
	if err := json.Unmarshal(dados, &esperado); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pedidoDoBlueprint, esperado) {
		t.Errorf("o pedido ao Atlas mudou: %#v, e o golden foi gravado com %#v", pedidoDoBlueprint, esperado)
	}
}

// A resposta gravada pelo Atlas passa pelo mesmo caminho da ferramenta (respostaDoPlugin e
// formatBlueprintResults) e chega à IA com os nomes, não só o título.
func TestContrato_AtlasBlueprint(t *testing.T) {
	saida, err := os.ReadFile(filepath.Join(dirContratoAtlas, "contrato-blueprint.json"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := respostaDoPlugin(saida)
	if err != nil {
		t.Fatal(err)
	}
	texto := formatBlueprintResults(resp.Data)
	for _, trecho := range []string{
		"Componentes (3)",
		"meu-servico (app, go, gin) em meu-servico",
		"lib (app, go) em lib",
		"Charts Helm (1)",
		"api em charts/api",
		"Workloads (3)",
		"Deployment api-server",
		"Deployment loja/web",
		"ComposeService worker em meu-servico/docker-compose.yml · imagem loja/worker:2.0.1",
		"Service: loja/web",
		"HelmRepository: redis",
		"Relações entre componentes (2)",
		"meu-servico → lib (imports)",
		"api → https://charts.bitnami.com/bitnami (depends)",
		"Relações da infraestrutura (3)",
		"Service/loja/web → Deployment/loja/web (selects)",
	} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("o resumo deveria ter %q:\n%s", trecho, texto)
		}
	}
	// O chart do blueprint não se repete entre os outros recursos da infra.
	if strings.Contains(texto, "HelmChart:") {
		t.Errorf("o chart já listado em Charts Helm não deveria se repetir em outros recursos:\n%s", texto)
	}
	// Os caminhos chegam relativos: nada da máquina que gravou o golden.
	if strings.Contains(texto, `\`) || strings.Contains(texto, "Temp") {
		t.Errorf("o resumo não deveria ter caminho absoluto:\n%s", texto)
	}
}
