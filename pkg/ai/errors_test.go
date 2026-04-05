package ai

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIError_Error(t *testing.T) {
	err := &APIError{Provider: "gemini", StatusCode: 429, Body: "rate limited"}
	assert.Equal(t, "gemini retornou status 429: rate limited", err.Error())
}

func TestAPIError_ErrorsAs(t *testing.T) {
	original := &APIError{Provider: "openai", StatusCode: 500, Body: "internal"}
	wrapped := fmt.Errorf("chamada falhou: %w", original)

	var apiErr *APIError
	require.True(t, errors.As(wrapped, &apiErr))
	assert.Equal(t, "openai", apiErr.Provider)
	assert.Equal(t, 500, apiErr.StatusCode)
	assert.Equal(t, "internal", apiErr.Body)
}
