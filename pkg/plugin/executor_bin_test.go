package plugin

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// O plugin interativo recebe o caminho do core em YBY_BIN, para abrir outro plugin pelo
// mesmo core (o b do Viz abre o Bard), e o pedido em YBY_PLUGIN_REQUEST.
func TestAmbienteInterativo_PassaOCaminhoDoCore(t *testing.T) {
	original := executavelDoCore
	defer func() { executavelDoCore = original }()
	executavelDoCore = func() (string, error) { return `C:\yby\yby.exe`, nil }

	env, _, err := ambienteInterativo([]string{"PATH=/bin", "KUBECONFIG=/k"}, []byte(`{"hook":"command"}`))
	assert.NoError(t, err)

	assert.Equal(t, []string{"PATH=/bin", "KUBECONFIG=/k", `YBY_PLUGIN_REQUEST={"hook":"command"}`, `YBY_BIN=C:\yby\yby.exe`}, env)
}

// O core aberto por um plugin (o yby bard que o Viz chama) herda o pedido e o YBY_BIN do
// plugin: valem os deste core, uma vez só.
func TestAmbienteInterativo_TrocaOsHerdados(t *testing.T) {
	original := executavelDoCore
	defer func() { executavelDoCore = original }()
	executavelDoCore = func() (string, error) { return "/novo/yby", nil }

	env, _, err := ambienteInterativo([]string{"YBY_BIN=/velho/yby", `YBY_PLUGIN_REQUEST={"hook":"velho"}`,
		"YBY_PLUGIN_REQUEST_FILE=/tmp/velho.json", "HOME=/h"}, []byte(`{}`))
	assert.NoError(t, err)

	assert.Equal(t, []string{"HOME=/h", "YBY_PLUGIN_REQUEST={}", "YBY_BIN=/novo/yby"}, env)
}

// Sem o caminho do executável, o YBY_BIN fica de fora: o plugin cai para o yby do PATH.
func TestAmbienteInterativo_SemCaminhoDoCore(t *testing.T) {
	original := executavelDoCore
	defer func() { executavelDoCore = original }()
	executavelDoCore = func() (string, error) { return "", errors.New("sem /proc") }

	env, _, err := ambienteInterativo([]string{"YBY_BIN=/velho/yby"}, []byte(`{}`))
	assert.NoError(t, err)

	assert.Equal(t, []string{"YBY_PLUGIN_REQUEST={}"}, env)
}

// Um pedido grande (o contexto de um projeto grande) passa do limite que o Linux aceita
// numa variável: vai por arquivo, que o plugin lê pelo PedidoDoAmbiente e que some depois.
func TestAmbienteInterativo_PedidoGrandeVaiPorArquivo(t *testing.T) {
	original := executavelDoCore
	defer func() { executavelDoCore = original }()
	executavelDoCore = func() (string, error) { return "", errors.New("sem caminho") }

	req := []byte(`{"hook":"command","context":{"data":"` + strings.Repeat("x", 200*1024) + `"}}`)
	env, limpar, err := ambienteInterativo([]string{"HOME=/h"}, req)
	assert.NoError(t, err)

	assert.Len(t, env, 2)
	assert.True(t, strings.HasPrefix(env[1], EnvPedidoArquivo+"="), env[1][:40])
	arquivo := strings.TrimPrefix(env[1], EnvPedidoArquivo+"=")
	getenv := func(k string) string {
		if k == EnvPedidoArquivo {
			return arquivo
		}
		return ""
	}
	lido, err := PedidoDoAmbiente(getenv)
	assert.NoError(t, err)
	assert.Equal(t, req, lido)

	limpar()
	_, err = os.Stat(arquivo)
	assert.True(t, os.IsNotExist(err), "o arquivo do pedido some depois do plugin")
}

// O processo do plugin abre com um pedido de mais de 200 KB e recebe o pedido inteiro.
// Pela variável, o Linux recusaria a execução ("argument list too long").
func TestRunInteractive_PedidoGrandeChegaInteiro(t *testing.T) {
	original := execCommandContext
	execCommandContext = mockExecCommandContext
	defer func() { execCommandContext = original }()

	req := PluginRequest{Hook: "command", Args: []string{strings.Repeat("x", 220*1024)}}
	e := &Executor{SkipTrustCheck: true}

	assert.NoError(t, e.RunInteractive(context.Background(), "/path/to/pedido-grande-plugin", req))
}

// Um pedido pequeno continua na variável, como os plugins antigos esperam.
func TestPedidoDoAmbiente_VariavelTemPrioridade(t *testing.T) {
	getenv := func(k string) string {
		return map[string]string{EnvPedido: `{"hook":"command"}`, EnvPedidoArquivo: "/nao/existe.json"}[k]
	}
	lido, err := PedidoDoAmbiente(getenv)
	assert.NoError(t, err)
	assert.Equal(t, `{"hook":"command"}`, string(lido))

	lido, err = PedidoDoAmbiente(func(string) string { return "" })
	assert.NoError(t, err)
	assert.Nil(t, lido)

	_, err = PedidoDoAmbiente(func(k string) string {
		if k == EnvPedidoArquivo {
			return "/nao/existe.json"
		}
		return ""
	})
	assert.ErrorContains(t, err, "pedido do core")
}

// O ctrl+c do Windows (STATUS_CONTROL_C_EXIT) sai do yby com 130, como no Unix; os outros
// códigos seguem.
func TestCodigoDoPlugin_CtrlCDoWindowsVira130(t *testing.T) {
	assert.Equal(t, 130, codigoDoPlugin(3221225786))
	assert.Equal(t, 130, codigoDoPlugin(-1073741510), "o mesmo código lido como int32")
	assert.Equal(t, 130, codigoDoPlugin(130))
	assert.Equal(t, 3, codigoDoPlugin(3))
	assert.Equal(t, -1, codigoDoPlugin(-1))
}
