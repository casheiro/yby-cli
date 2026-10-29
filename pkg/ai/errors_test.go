package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os/exec"
	"syscall"
	"testing"

	ybyerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
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

func TestErroRetentavel(t *testing.T) {
	erroDeRede := &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}
	casos := []struct {
		nome string
		err  error
		quer bool
	}{
		{"nil", nil, false},
		{"429", &APIError{Provider: "p", StatusCode: 429}, true},
		{"502 embrulhado", fmt.Errorf("chamada: %w", &APIError{Provider: "p", StatusCode: 502}), true},
		{"503", &APIError{Provider: "p", StatusCode: 503}, true},
		{"401", &APIError{Provider: "p", StatusCode: 401}, false},
		{"404 modelo inexistente", &APIError{Provider: "p", StatusCode: 404}, false},
		{"rede", fmt.Errorf("falha ao chamar ollama: %w", erroDeRede), true},
		{"fim inesperado do stream", fmt.Errorf("lendo: %w", io.ErrUnexpectedEOF), true},
		{"prazo da chamada", fmt.Errorf("falha: %w", context.DeadlineExceeded), true},
		{"cancelado", fmt.Errorf("falha: %w", context.Canceled), false},
		{"limite de tokens", ybyerrors.New(ybyerrors.ErrCodeTokenLimit, "prompt grande"), false},
		{"config", ybyerrors.New(ybyerrors.ErrCodeConfig, "chave ausente"), false},
		{"timeout de rede do yby", ybyerrors.New(ybyerrors.ErrCodeNetworkTimeout, "demorou"), true},
		{"yby embrulhando 503", ybyerrors.Wrap(&APIError{Provider: "p", StatusCode: 503}, ybyerrors.ErrCodeExec, "x"), true},
		{"CLI que falhou", errors.New("claude cli falhou: exit status 1"), false},
		// O syscall.Errno é um net.Error: o CLI que nem inicia não pode passar por falha de rede.
		{"CLI que não inicia", fmt.Errorf("claude cli falhou: %w",
			&fs.PathError{Op: "fork/exec", Path: "/bin/claude", Err: syscall.ENOEXEC}), false},
		{"CLI fora do PATH", fmt.Errorf("claude cli falhou: %w",
			&exec.Error{Name: "claude", Err: syscall.ENOENT}), false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			assert.Equal(t, c.quer, ErroRetentavel(c.err))
		})
	}
}
