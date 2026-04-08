package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithHint(t *testing.T) {
	err := New(ErrCodeValidation, "campo obrigatório ausente").
		WithHint("Informe o campo --name na chamada.")

	assert.Equal(t, "Informe o campo --name na chamada.", err.Hint)
}

func TestGetHint_CustomHint(t *testing.T) {
	err := New(ErrCodeValidation, "erro").
		WithHint("Dica personalizada")

	assert.Equal(t, "Dica personalizada", err.GetHint())
}

func TestGetHint_DefaultFromRegistry(t *testing.T) {
	err := New(ErrCodeClusterOffline, "cluster não responde")

	hint := err.GetHint()
	assert.Contains(t, hint, "kubectl cluster-info")
}

func TestGetHint_NoHintAvailable(t *testing.T) {
	err := New("ERR_UNKNOWN_CODE", "erro desconhecido")

	assert.Equal(t, "", err.GetHint())
}

func TestGetDefaultHint_AllCodes(t *testing.T) {
	codes := []string{
		ErrCodeIO, ErrCodeCmdNotFound, ErrCodeExec,
		ErrCodeNetworkTimeout, ErrCodeUnreachable, ErrCodePortForward,
		ErrCodeClusterOffline, ErrCodeManifest, ErrCodeHelm,
		ErrCodeValidation, ErrCodeConfig,
		ErrCodePlugin, ErrCodePluginRPC, ErrCodePluginNotFound,
		ErrCodeScaffold, ErrCodeTokenLimit,
		ErrCodeCloudTokenExpired, ErrCodeCloudCLIMissing, ErrCodeCloudModelDisabled,
	}

	for _, code := range codes {
		hint := GetDefaultHint(code)
		assert.NotEmpty(t, hint, "código %s deveria ter hint padrão", code)
	}
}

func TestGetDefaultHint_UnknownCode(t *testing.T) {
	hint := GetDefaultHint("ERR_NONEXISTENT")
	assert.Empty(t, hint)
}
