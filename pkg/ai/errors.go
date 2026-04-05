package ai

import "fmt"

// APIError representa um erro HTTP de um provider de IA, com status code
// para que o middleware de retry possa classificar erros retentaveis.
type APIError struct {
	Provider   string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s retornou status %d: %s", e.Provider, e.StatusCode, e.Body)
}
