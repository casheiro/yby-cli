package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssets_ContainsExpectedEntries(t *testing.T) {
	// Verifica que o embed.FS possui entradas
	entries, err := Assets.ReadDir("assets")
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "Assets deve conter ao menos um arquivo ou diretório em assets/")

	// Verifica que os diretórios esperados estão presentes
	expected := map[string]bool{
		"argo-workflows": false,
		"charts":         false,
		"config":         false,
		"manifests":      false,
	}

	for _, entry := range entries {
		if _, ok := expected[entry.Name()]; ok {
			expected[entry.Name()] = true
		}
	}

	for name, found := range expected {
		assert.True(t, found, "diretório esperado %q não encontrado em assets/", name)
	}
}

func TestAssets_ReadDir_InvalidPath(t *testing.T) {
	_, err := Assets.ReadDir("nonexistent")
	assert.Error(t, err)
}
