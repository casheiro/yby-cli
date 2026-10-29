//go:build !windows

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fora do Windows, o arquivo sai na hora.
func TestAgendarRemocao_ApagaNaHora(t *testing.T) {
	alvo := filepath.Join(t.TempDir(), "yby.bak")
	require.NoError(t, os.WriteFile(alvo, []byte("binário"), 0o755))

	require.NoError(t, agendarRemocao(alvo))
	_, err := os.Stat(alvo)
	assert.True(t, os.IsNotExist(err))
}
