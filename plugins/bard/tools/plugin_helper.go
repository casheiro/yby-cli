package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

// discoverPluginBinary busca o binário de um plugin nos paths padrão.
// Retorna o caminho absoluto ou erro se não encontrado.
func discoverPluginBinary(name string) (string, error) {
	// Nomes possíveis: "yby-plugin-{name}" (padrão) e "{name}" (legado)
	binaryName := "yby-plugin-" + name
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(".yby", "plugins", binaryName),
		filepath.Join(home, ".yby", "plugins", binaryName),
		filepath.Join(".yby", "plugins", name),
		filepath.Join(home, ".yby", "plugins", name),
	}

	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}

	return "", fmt.Errorf("binário do plugin '%s' não encontrado nos paths: %v", name, paths)
}

// invokePlugin executa um plugin como subprocesso e retorna a resposta.
func invokePlugin(binaryPath string, req plugin.PluginRequest) (*plugin.PluginResponse, error) {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar request: %w", err)
	}

	cmd := exec.Command(binaryPath)
	// O pedido do core ao Bard (variável ou arquivo) fica de fora: vale só o desta chamada.
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, plugin.EnvPedido+"=") && !strings.HasPrefix(kv, plugin.EnvPedidoArquivo+"=") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, plugin.EnvPedido+"="+string(reqJSON))
	// As etapas e os erros do plugin vão para o stderr; o resultado, para o stdout.
	var erros bytes.Buffer
	cmd.Stderr = &erros

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("plugin '%s' falhou: %w\n%s%s", binaryPath, err, string(output), erros.String())
	}
	return respostaDoPlugin(output)
}

// respostaDoPlugin lê o JSON que o plugin escreveu no stdout. O hook command do Sentinel
// escreve o relatório direto (o mesmo do -o json), sem o envelope {"data": …} do
// PluginResponse: um objeto com outras chaves vira o Data da resposta.
func respostaDoPlugin(output []byte) (*plugin.PluginResponse, error) {
	// Extrair JSON do output (plugins podem imprimir texto antes do JSON)
	jsonData := extractJSON(output)
	if jsonData == nil {
		return nil, fmt.Errorf("plugin nao retornou JSON valido: %s", string(output))
	}

	var resp plugin.PluginResponse
	var err error
	if envelope(jsonData) {
		err = json.Unmarshal(jsonData, &resp)
	} else {
		err = json.Unmarshal(jsonData, &resp.Data)
	}
	if err != nil {
		return nil, fmt.Errorf("resposta inválida do plugin: %w\n%s", err, string(output))
	}

	if resp.Error != "" {
		return &resp, fmt.Errorf("plugin retornou erro: %s", resp.Error)
	}

	return &resp, nil
}

// envelope informa se o JSON é um PluginResponse: um objeto só com as chaves data e error.
func envelope(jsonData []byte) bool {
	var chaves map[string]json.RawMessage
	if err := json.Unmarshal(jsonData, &chaves); err != nil {
		return false
	}
	for k := range chaves {
		if k != "data" && k != "error" {
			return false
		}
	}
	return true
}

// extractJSON encontra o primeiro objeto JSON válido no output.
// Plugins podem imprimir texto de progresso antes do JSON.
func extractJSON(data []byte) []byte {
	// Procurar primeira ocorrência de '{' que inicia um JSON válido
	for i := range data {
		if data[i] == '{' {
			// Tentar parsear do '{' até o final
			var js json.RawMessage
			if err := json.Unmarshal(data[i:], &js); err == nil {
				return data[i:]
			}
		}
	}
	return nil
}
