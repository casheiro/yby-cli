package cloud

import (
	"sync"
	"time"
)

// tokenMargin é o intervalo antes da expiração real em que um token é considerado
// expirado pelo cache, evitando uso de token prestes a vencer.
const tokenMargin = 60 * time.Second

// TokenCache é um cache thread-safe para tokens de autenticação cloud.
// Considera tokens como expirados tokenMargin antes da expiração real.
type TokenCache struct {
	mu    sync.RWMutex
	token *Token
}

// Get retorna o token armazenado se válido (não expirado com margem de 60s).
// Retorna nil, false quando o cache está vazio ou o token expirou.
func (c *TokenCache) Get() (*Token, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.token == nil {
		return nil, false
	}
	if time.Now().Add(tokenMargin).After(c.token.ExpiresAt) {
		return nil, false
	}
	return c.token, true
}

// Set armazena um token no cache.
func (c *TokenCache) Set(t *Token) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = t
}

// Invalidate remove o token armazenado, forçando refresh na próxima chamada a Get.
func (c *TokenCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = nil
}
