package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	cmd.Env = append(os.Environ(), fmt.Sprintf("YBY_PLUGIN_REQUEST=%s", string(reqJSON)))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("plugin '%s' falhou: %w\n%s", binaryPath, err, string(output))
	}

	// Extrair JSON do output (plugins podem imprimir texto antes do JSON)
	jsonData := extractJSON(output)
	if jsonData == nil {
		return nil, fmt.Errorf("plugin nao retornou JSON valido: %s", string(output))
	}

	var resp plugin.PluginResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return nil, fmt.Errorf("resposta inválida do plugin: %w\n%s", err, string(output))
	}

	if resp.Error != "" {
		return &resp, fmt.Errorf("plugin retornou erro: %s", resp.Error)
	}

	return &resp, nil
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
