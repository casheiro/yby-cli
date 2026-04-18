package cloud

import (
	"sync"
	"testing"
	"time"
)

func TestTokenCache_EmptyReturnsNil(t *testing.T) {
	var c TokenCache
	tok, ok := c.Get()
	if ok || tok != nil {
		t.Errorf("Get() em cache vazio deve retornar nil, false; got %v, %v", tok, ok)
	}
}

func TestTokenCache_ValidToken(t *testing.T) {
	var c TokenCache
	tok := &Token{Value: "tok1", ExpiresAt: time.Now().Add(2 * time.Minute)}
	c.Set(tok)
	got, ok := c.Get()
	if !ok || got == nil {
		t.Fatalf("Get() deve retornar token válido; got nil, %v", ok)
	}
	if got.Value != "tok1" {
		t.Errorf("Value = %q, want %q", got.Value, "tok1")
	}
}

func TestTokenCache_ExpiredToken(t *testing.T) {
	var c TokenCache
	tok := &Token{Value: "expirado", ExpiresAt: time.Now().Add(-1 * time.Second)}
	c.Set(tok)
	got, ok := c.Get()
	if ok || got != nil {
		t.Errorf("Get() com token expirado deve retornar nil, false; got %v, %v", got, ok)
	}
}

func TestTokenCache_TokenWithinMargin(t *testing.T) {
	var c TokenCache
	// Token expira em 30s — dentro da margem de 60s, deve ser tratado como expirado
	tok := &Token{Value: "margem", ExpiresAt: time.Now().Add(30 * time.Second)}
	c.Set(tok)
	got, ok := c.Get()
	if ok || got != nil {
		t.Errorf("Get() com token dentro da margem de 60s deve retornar nil, false; got %v, %v", got, ok)
	}
}

func TestTokenCache_TokenJustOutsideMargin(t *testing.T) {
	var c TokenCache
	// Token expira em 61s — fora da margem, deve ser retornado
	tok := &Token{Value: "fora-margem", ExpiresAt: time.Now().Add(61 * time.Second)}
	c.Set(tok)
	got, ok := c.Get()
	if !ok || got == nil {
		t.Errorf("Get() com token fora da margem de 60s deve retornar token; got nil, %v", ok)
	}
}

func TestTokenCache_Invalidate(t *testing.T) {
	var c TokenCache
	tok := &Token{Value: "valido", ExpiresAt: time.Now().Add(5 * time.Minute)}
	c.Set(tok)
	c.Invalidate()
	got, ok := c.Get()
	if ok || got != nil {
		t.Errorf("Get() após Invalidate deve retornar nil, false; got %v, %v", got, ok)
	}
}

func TestTokenCache_SetOverwrite(t *testing.T) {
	var c TokenCache
	c.Set(&Token{Value: "primeiro", ExpiresAt: time.Now().Add(5 * time.Minute)})
	c.Set(&Token{Value: "segundo", ExpiresAt: time.Now().Add(5 * time.Minute)})
	got, ok := c.Get()
	if !ok || got == nil {
		t.Fatalf("Get() deve retornar token após Set duplo; got nil, %v", ok)
	}
	if got.Value != "segundo" {
		t.Errorf("Value = %q, want %q", got.Value, "segundo")
	}
}

func TestTokenCache_Concurrency(t *testing.T) {
	var c TokenCache
	tok := &Token{Value: "concorrente", ExpiresAt: time.Now().Add(5 * time.Minute)}
	c.Set(tok)

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			switch idx % 3 {
			case 0:
				c.Get()
			case 1:
				c.Set(&Token{Value: "atualizado", ExpiresAt: time.Now().Add(5 * time.Minute)})
			case 2:
				c.Invalidate()
			}
		}(i)
	}

	wg.Wait()
	// Objetivo: verificar ausência de race conditions — o estado final é não-determinístico
}
