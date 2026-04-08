package cloud

import (
	"context"
	"time"
)

// Token representa um token de autenticação cloud com seu prazo de validade.
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// TokenGenerator gera tokens de autenticação cloud sob demanda.
type TokenGenerator interface {
	// GenerateToken gera um novo token de autenticação. Pode fazer chamada de rede.
	GenerateToken(ctx context.Context) (*Token, error)
}
