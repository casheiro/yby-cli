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
	// Paths de busca em ordem de prioridade
	home, _ := os.UserHomeDir()
	paths := []string{
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

	var resp plugin.PluginResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("resposta inv��lida do plugin: %w\n%s", err, string(output))
	}

	if resp.Error != "" {
		return &resp, fmt.Errorf("plugin retornou erro: %s", resp.Error)
	}

	return &resp, nil
}
