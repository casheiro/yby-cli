package ai

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"sync"
	"time"
)

const (
	defaultEmbeddingCacheSize = 1000
	defaultEmbeddingCacheTTL  = 1 * time.Hour
)

// cacheEntry armazena um embedding em cache com timestamp.
type cacheEntry struct {
	key       string
	embedding []float32
	createdAt time.Time
}

// embeddingCache é um cache LRU com TTL para embeddings.
type embeddingCache struct {
	mu       sync.Mutex
	maxSize  int
	ttl      time.Duration
	items    map[string]*list.Element
	eviction *list.List
}

func newEmbeddingCache(maxSize int, ttl time.Duration) *embeddingCache {
	return &embeddingCache{
		maxSize:  maxSize,
		ttl:      ttl,
		items:    make(map[string]*list.Element),
		eviction: list.New(),
	}
}

// get retorna o embedding do cache se presente e não expirado.
func (c *embeddingCache) get(key string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	entry := elem.Value.(*cacheEntry)
	if time.Since(entry.createdAt) > c.ttl {
		// Expirado — remover
		c.eviction.Remove(elem)
		delete(c.items, key)
		return nil, false
	}

	// Mover para o início (mais recente)
	c.eviction.MoveToFront(elem)
	return entry.embedding, true
}

// put adiciona um embedding ao cache, evictando o mais antigo se necessário.
func (c *embeddingCache) put(key string, embedding []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Se já existe, atualizar
	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*cacheEntry)
		entry.embedding = embedding
		entry.createdAt = time.Now()
		c.eviction.MoveToFront(elem)
		return
	}

	// Evictar se necessário
	for c.eviction.Len() >= c.maxSize {
		oldest := c.eviction.Back()
		if oldest == nil {
			break
		}
		entry := oldest.Value.(*cacheEntry)
		delete(c.items, entry.key)
		c.eviction.Remove(oldest)
	}

	// Inserir novo
	entry := &cacheEntry{
		key:       key,
		embedding: embedding,
		createdAt: time.Now(),
	}
	elem := c.eviction.PushFront(entry)
	c.items[key] = elem
}

// CachedEmbeddingProvider é um decorator que intercepta apenas EmbedDocuments,
// utilizando um cache LRU com TTL para evitar chamadas redundantes.
type CachedEmbeddingProvider struct {
	inner Provider
	cache *embeddingCache
}

// NewCachedEmbeddingProvider cria um CachedEmbeddingProvider.
func NewCachedEmbeddingProvider(inner Provider, maxSize int, ttl time.Duration) *CachedEmbeddingProvider {
	return &CachedEmbeddingProvider{
		inner: inner,
		cache: newEmbeddingCache(maxSize, ttl),
	}
}

func (c *CachedEmbeddingProvider) Name() string { return c.inner.Name() }
func (c *CachedEmbeddingProvider) IsAvailable(ctx context.Context) bool {
	return c.inner.IsAvailable(ctx)
}

func (c *CachedEmbeddingProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.inner.Completion(ctx, systemPrompt, userPrompt)
}

func (c *CachedEmbeddingProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	return c.inner.StreamCompletion(ctx, systemPrompt, userPrompt, out)
}

func (c *CachedEmbeddingProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	return c.inner.GenerateGovernance(ctx, description)
}

// EmbedDocuments verifica o cache para cada texto, chama o provider apenas para misses,
// e retorna os resultados na ordem original.
func (c *CachedEmbeddingProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return c.inner.EmbedDocuments(ctx, texts)
	}

	results := make([][]float32, len(texts))
	var missIndices []int
	var missTexts []string
	hitCount := 0

	for i, text := range texts {
		key := hashText(text)
		if embedding, ok := c.cache.get(key); ok {
			results[i] = embedding
			hitCount++
		} else {
			missIndices = append(missIndices, i)
			missTexts = append(missTexts, text)
		}
	}

	missCount := len(missTexts)

	slog.Debug("cache.embeddings",
		"hits", hitCount,
		"misses", missCount,
		"total", len(texts),
	)

	if missCount == 0 {
		return results, nil
	}

	// Chamar provider para os misses
	missEmbeddings, err := c.inner.EmbedDocuments(ctx, missTexts)
	if err != nil {
		return nil, err
	}

	// Montar resultado final e popular cache
	for j, idx := range missIndices {
		results[idx] = missEmbeddings[j]
		key := hashText(missTexts[j])
		c.cache.put(key, missEmbeddings[j])
	}

	return results, nil
}

// hashText retorna o SHA-256 hex do texto para usar como chave de cache.
func hashText(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}
