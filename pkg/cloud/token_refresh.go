package cloud

import (
	"context"
	"net/http"
	"sync"
)

// AutoRefreshTransport é um http.RoundTripper que injeta bearer tokens
// automaticamente e renova o token ao receber 401 Unauthorized.
type AutoRefreshTransport struct {
	Base      http.RoundTripper
	Generator TokenGenerator
	Cache     *TokenCache
	mu        sync.Mutex
}

// RoundTrip injeta o bearer token no header Authorization e executa a requisição.
// Em caso de 401, invalida o cache, gera um novo token e repete a requisição.
// Respostas 403 são propagadas sem retry (erro real de RBAC).
func (t *AutoRefreshTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.getToken(req.Context())
	if err != nil {
		return nil, err
	}

	// Clonar a request para não modificar a original
	reqClone := req.Clone(req.Context())
	reqClone.Header.Set("Authorization", "Bearer "+token)

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	resp, err := base.RoundTrip(reqClone)
	if err != nil {
		return nil, err
	}

	// 403 Forbidden: erro real de RBAC, propagar sem retry
	if resp.StatusCode == http.StatusForbidden {
		return resp, nil
	}

	// 401 Unauthorized: token expirado, tentar refresh
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()

		newToken, refreshErr := t.refreshToken(req.Context())
		if refreshErr != nil {
			return nil, refreshErr
		}

		// Repetir com novo token
		retryReq := req.Clone(req.Context())
		retryReq.Header.Set("Authorization", "Bearer "+newToken)

		return base.RoundTrip(retryReq)
	}

	return resp, nil
}

// getToken obtém o token do cache ou gera um novo.
func (t *AutoRefreshTransport) getToken(ctx context.Context) (string, error) {
	if tok, ok := t.Cache.Get(); ok {
		return tok.Value, nil
	}

	return t.refreshToken(ctx)
}

// refreshToken serializa refresh concorrente com mutex. Se outro goroutine já
// renovou o token, retorna o valor do cache sem gerar novo.
func (t *AutoRefreshTransport) refreshToken(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Verificar cache novamente (outro goroutine pode ter atualizado)
	if tok, ok := t.Cache.Get(); ok {
		return tok.Value, nil
	}

	t.Cache.Invalidate()

	newToken, err := t.Generator.GenerateToken(ctx)
	if err != nil {
		return "", err
	}

	t.Cache.Set(newToken)
	return newToken.Value, nil
}
