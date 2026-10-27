package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
)

// APIError representa um erro HTTP de um provider de IA, com status code
// para que o middleware de retry possa classificar erros retentaveis.
type APIError struct {
	Provider   string
	StatusCode int
	Body       string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s retornou status %d: %s", e.Provider, e.StatusCode, e.Body)
}

// NewAPIErrorFromResponse cria um APIError a partir de uma resposta HTTP,
// parseando o header Retry-After quando presente.
func NewAPIErrorFromResponse(provider string, resp *http.Response, body []byte) *APIError {
	apiErr := &APIError{
		Provider:   provider,
		StatusCode: resp.StatusCode,
		Body:       string(body),
	}

	if ra := resp.Header.Get("Retry-After"); ra != "" {
		// Tentar como segundos inteiros
		if seconds, err := strconv.Atoi(ra); err == nil {
			apiErr.RetryAfter = time.Duration(seconds) * time.Second
		} else {
			// Tentar como HTTP-date (RFC 7231)
			if t, err := time.Parse(time.RFC1123, ra); err == nil {
				delay := time.Until(t)
				if delay > 0 {
					apiErr.RetryAfter = delay
				}
			}
		}
	}

	return apiErr
}

// ErroRetentavel diz se vale tentar de novo a chamada que falhou com err:
// status HTTP 429, 502 ou 503, erro de rede (inclusive prazo da chamada),
// stream cortado no meio e YbyError de timeout ou de serviço fora do alcance.
// O resto é permanente: chave inválida, modelo inexistente, prompt acima do
// limite de tokens, configuração, falha de um provider CLI e cancelamento.
func ErroRetentavel(err error) bool {
	return erroRetentavel(err, DefaultRetryableStatusCodes)
}

// erroRetentavel é o ErroRetentavel com os status HTTP retentáveis informados.
func erroRetentavel(err error, statuses map[int]bool) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return statuses[apiErr.StatusCode]
	}
	// O CLI que nem inicia (fora do PATH, sem permissão, formato inválido) devolve um
	// syscall.Errno, que também é um net.Error: tentar de novo não muda nada.
	var pathErr *fs.PathError
	var execErr *exec.Error
	if errors.As(err, &pathErr) || errors.As(err, &execErr) {
		return false
	}
	// O context.DeadlineExceeded também é um net.Error: o prazo de uma
	// chamada vale nova tentativa, e o fim do contexto de quem chamou já
	// encerra o retry.Do por conta própria.
	var netErr net.Error
	if errors.As(err, &netErr) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var ybyErr *ybyerrors.YbyError
	if errors.As(err, &ybyErr) {
		return ybyErr.Code == ybyerrors.ErrCodeNetworkTimeout || ybyErr.Code == ybyerrors.ErrCodeUnreachable
	}
	return false
}
