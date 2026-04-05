package ai

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
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
