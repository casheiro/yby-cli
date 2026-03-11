//go:build !k8s

package sdk

import (
	"os"
	"testing"

	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/stretchr/testify/assert"
)

func TestGetKubeClient_SemBuildTag(t *testing.T) {
	client, err := GetKubeClient()
	assert.Nil(t, client)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubernetes client not available")
}

func TestGetValues_ComContexto(t *testing.T) {
	// Configura contexto global para testar GetValues com contexto não-nil
	oldCtx := currentContext
	defer func() { currentContext = oldCtx }()

	currentContext = &plugin.PluginFullContext{
		Values: map[string]interface{}{
			"chave": "valor",
		},
	}

	vals := GetValues()
	assert.NotNil(t, vals)
	assert.Equal(t, "valor", vals["chave"])
}

func TestInit_StdinJSON_Invalido(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	// JSON inválido
	w.Write([]byte(`{"invalid json`))
	w.Close()

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"plugin"}

	err = Init()
	assert.Error(t, err)
}
