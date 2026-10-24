package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/logger"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================================
// handleExecutionError — testes do handler de erros
// ========================================================

// capturarErro roda handleExecutionError no formato e nível indicados e devolve a saída
// renderizada e os registros slog (JSON, sem o campo time) emitidos durante a chamada.
func capturarErro(t *testing.T, formato, nivel string, err error) (saida string, registros []map[string]any) {
	t.Helper()
	origFormato, origNivel, origSaida, origUI := logFormatEfetivo, logLevelFlag, saidaErro, ui.Atual()
	origLogger := slog.Default()
	t.Cleanup(func() {
		logFormatEfetivo, logLevelFlag, saidaErro = origFormato, origNivel, origSaida
		ui.Configurar(origUI)
		slog.SetDefault(origLogger)
	})

	var out, logs bytes.Buffer
	logFormatEfetivo, logLevelFlag, saidaErro = formato, nivel, &out
	ui.Configurar(ui.Opcoes{})
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: logger.ParseLevel(nivel)})))

	handleExecutionError(err)

	for _, linha := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if linha == "" {
			continue
		}
		var r map[string]any
		require.NoError(t, json.Unmarshal([]byte(linha), &r))
		delete(r, "time")
		registros = append(registros, r)
	}
	return out.String(), registros
}

func TestHandleExecutionError_Texto_ComHint(t *testing.T) {
	yerr := errors.New(errors.ErrCodeClusterOffline, "Não deu para falar com o cluster staging").
		WithHint("yby cloud refresh --provider aws")

	saida, registros := capturarErro(t, "text", "info", yerr)

	require.True(t, strings.HasPrefix(saida, "\n"), "o erro começa com uma linha em branco")
	linhas := strings.Split(strings.Trim(saida, "\n"), "\n")
	require.Len(t, linhas, 2, "sem causa, não há o que o debug detalhar")
	assert.True(t, strings.HasPrefix(linhas[0], "✕ Não deu para falar com o cluster staging"))
	assert.True(t, strings.HasSuffix(linhas[0], "ERR_CLUSTER_OFFLINE"))
	assert.Equal(t, "  faça      yby cloud refresh --provider aws", linhas[1])
	assert.Empty(t, registros, "no formato text o erro não passa pelo slog")
}

func TestHandleExecutionError_Texto_HintDoRegistry(t *testing.T) {
	yerr := errors.Wrap(fmt.Errorf("conexão recusada"), errors.ErrCodeNetworkTimeout, "timeout na conexão SSH").
		WithContext("host", "10.0.0.1")

	saida, _ := capturarErro(t, "text", "info", yerr)

	assert.Contains(t, saida, "  causa     conexão recusada\n")
	assert.Contains(t, saida, "  faça      "+errors.GetDefaultHint(errors.ErrCodeNetworkTimeout)+"\n")
	assert.Contains(t, saida, "  detalhes  host=10.0.0.1\n")
}

func TestHandleExecutionError_Texto_ErroGenerico(t *testing.T) {
	saida, _ := capturarErro(t, "text", "info", fmt.Errorf("erro genérico inesperado"))

	assert.Equal(t, "\n✕ Falha inesperada\n"+
		"  causa     erro genérico inesperado\n"+
		"  faça      "+errors.GenericHint+"\n", saida)
}

func TestHandleExecutionError_Texto_Debug(t *testing.T) {
	yerr := errors.New(errors.ErrCodeIO, "falha ao ler arquivo").WithContext("path", "/tmp/test.yaml")

	saida, registros := capturarErro(t, "text", "debug", yerr)

	assert.Contains(t, saida, "falha ao ler arquivo")
	require.Len(t, registros, 1)
	assert.Equal(t, "DEBUG", registros[0]["level"])
	assert.Equal(t, errors.ErrCodeIO, registros[0]["code"])
	assert.Equal(t, yerr.Error(), registros[0]["details"])
	assert.Contains(t, saida, "  detalhes  path=/tmp/test.yaml\n")
}

func TestHandleExecutionError_JSON_ComHint(t *testing.T) {
	yerr := errors.New(errors.ErrCodeValidation, "campo obrigatório ausente").WithHint("informe --nome")

	saida, registros := capturarErro(t, "json", "info", yerr)

	assert.Empty(t, saida, "no formato json nada vai para a saída renderizada")
	assert.Equal(t, []map[string]any{
		{"level": "ERROR", "msg": "Falha na execução", "code": errors.ErrCodeValidation, "message": "campo obrigatório ausente"},
		{"level": "INFO", "msg": "Dica", "sugestão": "informe --nome"},
	}, registros)
}

func TestHandleExecutionError_JSON_SemHint(t *testing.T) {
	yerr := errors.New(errors.ErrCodeValidation, "campo obrigatório ausente")

	_, registros := capturarErro(t, "json", "info", yerr)

	require.Len(t, registros, 2)
	assert.Equal(t, errors.GetDefaultHint(errors.ErrCodeValidation), registros[1]["sugestão"])
}

func TestHandleExecutionError_JSON_Debug(t *testing.T) {
	yerr := errors.New(errors.ErrCodeIO, "falha ao ler arquivo")

	_, registros := capturarErro(t, "json", "debug", yerr)

	require.Len(t, registros, 2)
	assert.Contains(t, registros[0], "details")
	assert.NotContains(t, registros[0], "message")
}

func TestHandleExecutionError_JSON_ErroGenerico(t *testing.T) {
	_, registros := capturarErro(t, "json", "info", fmt.Errorf("erro genérico inesperado"))

	assert.Equal(t, []map[string]any{
		{"level": "ERROR", "msg": "Falha inesperada", "erro": "erro genérico inesperado"},
		{"level": "INFO", "msg": "Dica", "sugestão": errors.GenericHint},
	}, registros)
}

func TestHandleExecutionError_SemInitConfig_UsaFlag(t *testing.T) {
	origFlag := logFormatFlag
	t.Cleanup(func() { logFormatFlag = origFlag })
	logFormatFlag = "json"

	saida, registros := capturarErro(t, "", "info", fmt.Errorf("flag desconhecida"))

	assert.Empty(t, saida)
	assert.Len(t, registros, 2)
}

// ========================================================
// traduzirErroDeUso e erroDeFlag — erros de uso do cobra
// ========================================================

func TestTraduzirErroDeUso_ComandoDesconhecido(t *testing.T) {
	err := traduzirErroDeUso(fmt.Errorf(`unknown command "comando-inexistente" for "yby"`), rootCmd)

	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Equal(t, errors.ErrCodeValidation, yerr.Code)
	assert.Equal(t, "comando desconhecido: yby comando-inexistente", yerr.Message)
	assert.Equal(t, "rode yby --help para ver os comandos", yerr.Hint)
}

func TestTraduzirErroDeUso_ComSugestoes(t *testing.T) {
	err := traduzirErroDeUso(fmt.Errorf("unknown command \"stat\" for \"yby\"\n\nDid you mean this?\n\tstatus\n\tsetup\n"), rootCmd)

	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Equal(t, "comando desconhecido: yby stat", yerr.Message)
	assert.Equal(t, "você quis dizer yby status ou yby setup?", yerr.Hint)
}

func TestTraduzirErroDeUso_Argumentos(t *testing.T) {
	err := traduzirErroDeUso(fmt.Errorf("accepts 1 arg(s), received 0"), statusCmd)

	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Equal(t, "argumentos inválidos para yby status", yerr.Message)
	assert.Equal(t, "rode yby status --help para ver o uso", yerr.Hint)
	assert.EqualError(t, yerr.Cause, "accepts 1 arg(s), received 0")
}

func TestTraduzirErroDeUso_MantemOsDemais(t *testing.T) {
	assert.NoError(t, traduzirErroDeUso(nil, rootCmd))

	generico := fmt.Errorf("falha qualquer")
	assert.Same(t, generico, traduzirErroDeUso(generico, rootCmd))

	yerr := errors.New(errors.ErrCodeIO, "unknown command \"x\" for \"yby\"")
	assert.Same(t, yerr, traduzirErroDeUso(yerr, rootCmd), "YbyError não é traduzido")
}

func TestErroDeFlag(t *testing.T) {
	err := erroDeFlag(statusCmd, fmt.Errorf("unknown flag: --xpto"))

	var yerr *errors.YbyError
	require.ErrorAs(t, err, &yerr)
	assert.Equal(t, errors.ErrCodeValidation, yerr.Code)
	assert.Equal(t, "flag inválida para yby status", yerr.Message)
	assert.Equal(t, "rode yby status --help para ver as flags", yerr.Hint)
	assert.NotNil(t, rootCmd.FlagErrorFunc(), "o rootCmd registra o erroDeFlag")
}
