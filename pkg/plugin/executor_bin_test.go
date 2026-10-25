package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// O plugin interativo recebe o caminho do core em YBY_BIN, para abrir outro plugin pelo
// mesmo core (o b do Viz abre o Bard), e o pedido em YBY_PLUGIN_REQUEST.
func TestAmbienteInterativo_PassaOCaminhoDoCore(t *testing.T) {
	original := executavelDoCore
	defer func() { executavelDoCore = original }()
	executavelDoCore = func() (string, error) { return `C:\yby\yby.exe`, nil }

	env := ambienteInterativo([]string{"PATH=/bin", "KUBECONFIG=/k"}, []byte(`{"hook":"command"}`))

	assert.Equal(t, []string{"PATH=/bin", "KUBECONFIG=/k", `YBY_PLUGIN_REQUEST={"hook":"command"}`, `YBY_BIN=C:\yby\yby.exe`}, env)
}

// O core aberto por um plugin (o yby bard que o Viz chama) herda o pedido e o YBY_BIN do
// plugin: valem os deste core, uma vez só.
func TestAmbienteInterativo_TrocaOsHerdados(t *testing.T) {
	original := executavelDoCore
	defer func() { executavelDoCore = original }()
	executavelDoCore = func() (string, error) { return "/novo/yby", nil }

	env := ambienteInterativo([]string{"YBY_BIN=/velho/yby", `YBY_PLUGIN_REQUEST={"hook":"velho"}`, "HOME=/h"}, []byte(`{}`))

	assert.Equal(t, []string{"HOME=/h", "YBY_PLUGIN_REQUEST={}", "YBY_BIN=/novo/yby"}, env)
}

// Sem o caminho do executável, o YBY_BIN fica de fora: o plugin cai para o yby do PATH.
func TestAmbienteInterativo_SemCaminhoDoCore(t *testing.T) {
	original := executavelDoCore
	defer func() { executavelDoCore = original }()
	executavelDoCore = func() (string, error) { return "", errors.New("sem /proc") }

	env := ambienteInterativo([]string{"YBY_BIN=/velho/yby"}, []byte(`{}`))

	assert.Equal(t, []string{"YBY_PLUGIN_REQUEST={}"}, env)
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
