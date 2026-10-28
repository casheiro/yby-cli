package cmd

import (
	stdErrors "errors"
	"slices"
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArgsDoPlugin(t *testing.T) {
	casos := []struct {
		caso     string
		linha    []string // a linha do yby
		nome     string   // o plugin
		cobra    []string // o que o cobra entrega ao comando do plugin: a linha sem o nome
		ybyEnv   string
		ambiente string
		plugin   []string
	}{
		{"sem ambiente", []string{"sentinel", "scan"}, "sentinel", []string{"scan"}, "", "", []string{"scan"}},
		{"-c antes do nome", []string{"-c", "staging", "sentinel", "scan"}, "sentinel", []string{"-c", "staging", "scan"}, "", "staging", []string{"scan"}},
		{"--context= antes do nome", []string{"--context=staging", "viz"}, "viz", []string{"--context=staging"}, "", "staging", []string{}},
		{"-c= antes do nome", []string{"-c=staging", "sentinel", "scan"}, "sentinel", []string{"-c=staging", "scan"}, "", "staging", []string{"scan"}},
		{"valor de flag do core igual ao nome", []string{"--log-level", "viz", "viz"}, "viz", []string{"--log-level", "viz"}, "", "", []string{}},
		{"o último -c vale", []string{"-c", "dev", "--context", "prod", "viz"}, "viz", []string{"-c", "dev", "--context", "prod"}, "", "prod", []string{}},
		{"YBY_ENV sem -c", []string{"viz"}, "viz", []string{}, "staging", "staging", []string{}},
		{"-c vence o YBY_ENV", []string{"-c", "prod", "viz"}, "viz", []string{"-c", "prod"}, "staging", "prod", []string{}},
		// depois do nome, o --context é do plugin (no Bard e no Viz, o contexto do kubeconfig)
		{"--context depois do nome é do plugin", []string{"bard", "--context", "k3d-local"}, "bard", []string{"--context", "k3d-local"}, "", "", []string{"--context", "k3d-local"}},
		// testes que rodam o rootCmd com SetArgs: os.Args é o do binário de teste
		{"linha que não bate fica como veio", []string{"-test.run=X"}, "viz", []string{"-c", "x"}, "", "", []string{"-c", "x"}},
		// o pflag aceita o valor colado ao -c; descartado, o plugin rodaria no current-context
		{"-c com o valor colado", []string{"-cprod", "sentinel", "scan"}, "sentinel", []string{"-cprod", "scan"}, "", "prod", []string{"scan"}},
		{"-c colado vence o YBY_ENV", []string{"-cprod", "sentinel", "scan", "--fix"}, "sentinel", []string{"-cprod", "scan", "--fix"}, "staging", "prod", []string{"scan", "--fix"}},
		{"flags de log do core", []string{"--log-level", "debug", "--log-format=json", "-c", "prod", "viz"}, "viz", []string{"--log-level", "debug", "--log-format=json", "-c", "prod"}, "", "prod", []string{}},
	}
	for _, c := range casos {
		t.Run(c.caso, func(t *testing.T) {
			t.Setenv("YBY_ENV", c.ybyEnv)

			ambiente, plugin, err := argsDoPlugin(c.linha, c.nome, c.cobra)

			require.NoError(t, err)
			assert.Equal(t, c.ambiente, ambiente)
			assert.Equal(t, c.plugin, plugin)
		})
	}
}

// Flag antes do nome que o core não reconhece é erro: se sumisse, uma forma do -c que
// escapasse da leitura faria o plugin rodar no current-context, fora do ambiente pedido.
func TestArgsDoPlugin_FlagDesconhecidaAntesDoNomeEErro(t *testing.T) {
	casos := []struct {
		caso  string
		linha []string
		flag  string
	}{
		{"flag longa", []string{"--contexto", "sentinel", "scan", "--fix"}, "--contexto"},
		{"flag curta", []string{"-x", "sentinel", "scan"}, "-x"},
		{"depois de um -c válido", []string{"-c", "prod", "--fix", "sentinel", "scan"}, "--fix"},
	}
	for _, c := range casos {
		t.Run(c.caso, func(t *testing.T) {
			t.Setenv("YBY_ENV", "staging")
			cobra := slices.DeleteFunc(slices.Clone(c.linha), func(a string) bool { return a == "sentinel" })

			ambiente, plugin, err := argsDoPlugin(c.linha, "sentinel", cobra)

			require.Error(t, err)
			var yerr *errors.YbyError
			require.True(t, stdErrors.As(err, &yerr))
			assert.Equal(t, errors.ErrCodeValidation, yerr.Code)
			assert.Contains(t, yerr.Message, c.flag)
			assert.Empty(t, ambiente)
			assert.Nil(t, plugin)
		})
	}
}
