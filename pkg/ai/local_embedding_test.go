package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadLocalEmbeddingModel(t *testing.T) {
	prefix := "/" + localEmbeddingModelID + "/resolve/main/"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Responde com o próprio caminho do arquivo para conferir o destino
		_, _ = w.Write([]byte(strings.TrimPrefix(r.URL.Path, prefix)))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), localEmbeddingModelDir)
	require.NoError(t, downloadLocalEmbeddingModel(context.Background(), server.URL, dest))

	// Todos os arquivos ficam na raiz, inclusive onnx/model.onnx
	for _, file := range localEmbeddingFiles {
		data, err := os.ReadFile(filepath.Join(dest, filepath.Base(file)))
		require.NoError(t, err)
		assert.Equal(t, file, string(data))
	}

	parts, _ := filepath.Glob(filepath.Join(dest, "*.part"))
	assert.Empty(t, parts, "não deve sobrar arquivo temporário")
}

func TestDownloadLocalEmbeddingModel_ErroHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "model.onnx") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	dest := t.TempDir()
	err := downloadLocalEmbeddingModel(context.Background(), server.URL, dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "onnx/model.onnx")
	assert.Contains(t, err.Error(), "404")

	// Sem tokenizer.json o download é tratado como incompleto e refeito
	assert.NoFileExists(t, filepath.Join(dest, "tokenizer.json"))
	assert.NoFileExists(t, filepath.Join(dest, "model.onnx.part"))
}

func TestHuggingFaceEndpoint(t *testing.T) {
	t.Setenv("HF_ENDPOINT", "")
	assert.Equal(t, "https://huggingface.co", huggingFaceEndpoint())

	t.Setenv("HF_ENDPOINT", "https://hf.mirror.local/")
	assert.Equal(t, "https://hf.mirror.local", huggingFaceEndpoint())
}
