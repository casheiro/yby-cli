package ai

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedEmbeddingProvider_CacheHit(t *testing.T) {
	var calls atomic.Int32
	inner := &mockProvider{
		name: "test",
		embedFunc: func(_ context.Context, texts []string) ([][]float32, error) {
			calls.Add(1)
			results := make([][]float32, len(texts))
			for i := range texts {
				results[i] = []float32{0.1, 0.2}
			}
			return results, nil
		},
	}
	cached := NewCachedEmbeddingProvider(inner, 100, time.Hour)

	ctx := context.Background()

	// Primeira chamada — miss
	r1, err := cached.EmbedDocuments(ctx, []string{"hello", "world"})
	require.NoError(t, err)
	assert.Len(t, r1, 2)
	assert.Equal(t, int32(1), calls.Load())

	// Segunda chamada com os mesmos textos — cache hit
	r2, err := cached.EmbedDocuments(ctx, []string{"hello", "world"})
	require.NoError(t, err)
	assert.Len(t, r2, 2)
	assert.Equal(t, int32(1), calls.Load(), "não deveria chamar provider novamente")
}

func TestCachedEmbeddingProvider_PartialHit(t *testing.T) {
	var calls atomic.Int32
	var lastTexts []string
	inner := &mockProvider{
		name: "test",
		embedFunc: func(_ context.Context, texts []string) ([][]float32, error) {
			calls.Add(1)
			lastTexts = texts
			results := make([][]float32, len(texts))
			for i, text := range texts {
				if text == "hello" {
					results[i] = []float32{1.0, 0.0}
				} else {
					results[i] = []float32{0.0, 1.0}
				}
			}
			return results, nil
		},
	}
	cached := NewCachedEmbeddingProvider(inner, 100, time.Hour)
	ctx := context.Background()

	// Primeira chamada com "hello"
	_, err := cached.EmbedDocuments(ctx, []string{"hello"})
	require.NoError(t, err)
	assert.Equal(t, int32(1), calls.Load())

	// Segunda chamada com "hello" e "world" — "hello" é hit, "world" é miss
	results, err := cached.EmbedDocuments(ctx, []string{"hello", "world"})
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, []string{"world"}, lastTexts, "deveria chamar provider apenas com miss")

	// Verificar ordem dos resultados
	assert.InDelta(t, float32(1.0), results[0][0], 0.001, "hello do cache")
	assert.InDelta(t, float32(0.0), results[1][0], 0.001, "world do provider")
}

func TestCachedEmbeddingProvider_TTLExpiry(t *testing.T) {
	var calls atomic.Int32
	inner := &mockProvider{
		name: "test",
		embedFunc: func(_ context.Context, texts []string) ([][]float32, error) {
			calls.Add(1)
			results := make([][]float32, len(texts))
			for i := range texts {
				results[i] = []float32{0.1}
			}
			return results, nil
		},
	}

	// TTL de 1ms para testar expiração
	cached := NewCachedEmbeddingProvider(inner, 100, time.Millisecond)
	ctx := context.Background()

	_, err := cached.EmbedDocuments(ctx, []string{"hello"})
	require.NoError(t, err)
	assert.Equal(t, int32(1), calls.Load())

	// Esperar TTL expirar
	time.Sleep(5 * time.Millisecond)

	_, err = cached.EmbedDocuments(ctx, []string{"hello"})
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load(), "deveria chamar provider novamente após TTL")
}

func TestCachedEmbeddingProvider_LRUEviction(t *testing.T) {
	var calls atomic.Int32
	inner := &mockProvider{
		name: "test",
		embedFunc: func(_ context.Context, texts []string) ([][]float32, error) {
			calls.Add(1)
			results := make([][]float32, len(texts))
			for i := range texts {
				results[i] = []float32{0.1}
			}
			return results, nil
		},
	}

	// Cache de tamanho 2
	cached := NewCachedEmbeddingProvider(inner, 2, time.Hour)
	ctx := context.Background()

	// Inserir "a" e "b" (preenche o cache)
	_, _ = cached.EmbedDocuments(ctx, []string{"a"})
	_, _ = cached.EmbedDocuments(ctx, []string{"b"})
	assert.Equal(t, int32(2), calls.Load())

	// Inserir "c" — deve evictar "a" (LRU)
	_, _ = cached.EmbedDocuments(ctx, []string{"c"})
	assert.Equal(t, int32(3), calls.Load())

	// "a" deve ser miss novamente
	_, _ = cached.EmbedDocuments(ctx, []string{"a"})
	assert.Equal(t, int32(4), calls.Load(), "a deveria ter sido evictado")

	// "c" deve ser hit (ainda no cache), "b" deve ser miss (evictado por "a")
	_, _ = cached.EmbedDocuments(ctx, []string{"c"})
	assert.Equal(t, int32(4), calls.Load(), "c deveria estar em cache")

	_, _ = cached.EmbedDocuments(ctx, []string{"b"})
	assert.Equal(t, int32(5), calls.Load(), "b deveria ter sido evictado")
}

func TestCachedEmbeddingProvider_PassthroughMethods(t *testing.T) {
	inner := &mockProvider{
		name:      "test-provider",
		available: true,
		completionFunc: func(_ context.Context, _, _ string) (string, error) {
			return "resposta", nil
		},
	}
	cached := NewCachedEmbeddingProvider(inner, 100, time.Hour)

	assert.Equal(t, "test-provider", cached.Name())
	assert.True(t, cached.IsAvailable(context.Background()))

	result, err := cached.Completion(context.Background(), "sys", "usr")
	require.NoError(t, err)
	assert.Equal(t, "resposta", result)
}

func TestCachedEmbeddingProvider_Empty(t *testing.T) {
	inner := &mockProvider{
		name: "test",
		embedFunc: func(_ context.Context, texts []string) ([][]float32, error) {
			return nil, nil
		},
	}
	cached := NewCachedEmbeddingProvider(inner, 100, time.Hour)

	results, err := cached.EmbedDocuments(context.Background(), []string{})
	require.NoError(t, err)
	assert.Nil(t, results)
}
