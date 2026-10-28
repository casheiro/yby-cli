//go:build integration

package ai

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Testes de integração com o modelo de embeddings local de verdade ───────────
// Executar com: go test -tags integration -run TestLocalEmbedding_ModeloReal -v ./pkg/ai/
// Baixam o modelo (~120 MB) do HuggingFace numa home temporária, nunca no ~/.yby real.
// Com YBY_TESTE_MODELO_LOCAL apontando para uma pasta com o modelo já baixado, os
// arquivos são copiados de lá em vez de baixados.

// modeloRealNaHomeTemporaria põe o modelo local numa home temporária e devolve o provider.
func modeloRealNaHomeTemporaria(t *testing.T) *LocalEmbeddingProvider {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if origem := os.Getenv("YBY_TESTE_MODELO_LOCAL"); origem != "" {
		dest := filepath.Join(home, ".yby", "models", localEmbeddingModelDir)
		require.NoError(t, os.MkdirAll(dest, 0o755))
		for _, f := range localEmbeddingFiles {
			dados, err := os.ReadFile(filepath.Join(origem, f.destino))
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dest, f.destino), dados, 0o644))
		}
	}

	p := NewLocalEmbeddingProvider()
	t.Cleanup(p.Destroy)
	require.NoError(t, PrepararEmbeddings(context.Background(), p, shared.EtapasMudas{}))
	return p
}

// TestLocalEmbedding_ModeloReal_TrechosDensos: um UKI com kubeconfig, certificado, hex ou
// YAML denso derrubava o index inteiro num pânico do grafo acima de 512 tokens. Com o
// corte em tokens, todos viram vetores, no mesmo lote, sem demorar.
func TestLocalEmbedding_ModeloReal_TrechosDensos(t *testing.T) {
	p := modeloRealNaHomeTemporaria(t)

	nomes := []string{"prosa"}
	textos := []string{"O Argo CD sincroniza as aplicações do repositório com o cluster de produção."}
	for nome, texto := range trechosDensos() {
		nomes = append(nomes, nome)
		textos = append(textos, texto)
	}
	// O hex de 1992 bytes do relato: no modelo novo passava de 1900 tokens.
	nomes = append(nomes, "hex-1992")
	textos = append(textos, "## Digest\n"+strings.Repeat("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", 32)[:1992])

	inicio := time.Now()
	vetores, err := p.EmbedDocuments(context.Background(), textos)
	require.NoError(t, err)
	require.Len(t, vetores, len(textos))
	for i, v := range vetores {
		assert.Len(t, v, 384, nomes[i])
	}
	t.Logf("%d textos em %s", len(textos), time.Since(inicio))

	for i, texto := range textos {
		preparado := prepararParaModelo(texto, p.limiteTokens, p.contarTokens)
		assert.LessOrEqual(t, p.contarTokens(preparado), p.limiteTokens, nomes[i])
	}
	assert.Equal(t, 512, p.limiteTokens)
}
