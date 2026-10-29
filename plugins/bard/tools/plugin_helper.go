package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/plugin"
)

var (
	// errPluginAusente é o plugin que não está em nenhuma das pastas de plugins.
	errPluginAusente = errors.New("plugin não instalado")
	// errNaoConfiavel é o plugin achado que não está registrado como confiável, ou cujo
	// binário mudou depois do registro.
	errNaoConfiavel = errors.New("plugin não registrado como confiável")
)

// nomeDoBinario é o arquivo do plugin: yby-plugin-<nome>, com .exe no Windows.
func nomeDoBinario(nome, goos string) string {
	if goos == "windows" {
		return "yby-plugin-" + nome + ".exe"
	}
	return "yby-plugin-" + nome
}

// discoverPluginBinary acha o binário de um plugin como o core: ~/.yby/plugins e depois
// ./.yby/plugins, e vale o primeiro achado. Só devolve o binário registrado como confiável
// (yby plugin trust, ou o install), com o mesmo registro e a mesma verificação do core: um
// repositório clonado pode trazer um .yby/plugins.
func discoverPluginBinary(name string) (string, error) {
	arquivo := nomeDoBinario(name, runtime.GOOS)
	var pastas []string
	if home, err := os.UserHomeDir(); err == nil {
		pastas = append(pastas, filepath.Join(home, ".yby", "plugins"))
	}
	if cwd, err := os.Getwd(); err == nil {
		pastas = append(pastas, filepath.Join(cwd, ".yby", "plugins"))
	}

	for _, pasta := range pastas {
		caminho := filepath.Join(pasta, arquivo)
		if info, err := os.Stat(caminho); err != nil || info.IsDir() {
			continue
		}
		ok, err := plugin.IsTrusted(caminho)
		if err != nil {
			return "", fmt.Errorf("falha ao conferir a confiança de %s: %w", caminho, err)
		}
		if !ok {
			return "", fmt.Errorf("%w: %s (rode yby plugin trust %s)", errNaoConfiavel, caminho, name)
		}
		return caminho, nil
	}
	return "", fmt.Errorf("%w: %s não está em ~/.yby/plugins nem em .yby/plugins (rode yby plugin install %s)", errPluginAusente, arquivo, name)
}

// invokePlugin executa um plugin como subprocesso e retorna a resposta. O cancelamento do
// ctx (esc na TUI) mata o plugin.
func invokePlugin(ctx context.Context, binaryPath string, req plugin.PluginRequest) (*plugin.PluginResponse, error) {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar request: %w", err)
	}

	cmd := exec.CommandContext(ctx, binaryPath)
	// Sem filhos presos ao stdout, o Wait volta logo depois do kill; o prazo cobre o resto.
	cmd.WaitDelay = 2 * time.Second
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
