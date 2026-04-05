package ai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implementa Provider para testes do RetryProvider.
type mockProvider struct {
	name           string
	available      bool
	completionFunc func(ctx context.Context, sys, usr string) (string, error)
	streamFunc     func(ctx context.Context, sys, usr string, out io.Writer) error
	embedFunc      func(ctx context.Context, texts []string) ([][]float32, error)
	governanceFunc func(ctx context.Context, desc string) (*GovernanceBlueprint, error)
}

func (m *mockProvider) Name() string                       { return m.name }
func (m *mockProvider) IsAvailable(_ context.Context) bool { return m.available }

func (m *mockProvider) Completion(ctx context.Context, sys, usr string) (string, error) {
	if m.completionFunc != nil {
		return m.completionFunc(ctx, sys, usr)
	}
	return "", nil
}

func (m *mockProvider) StreamCompletion(ctx context.Context, sys, usr string, out io.Writer) error {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, sys, usr, out)
	}
	return nil
}

func (m *mockProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if m.embedFunc != nil {
		return m.embedFunc(ctx, texts)
	}
	return nil, nil
}

func (m *mockProvider) GenerateGovernance(ctx context.Context, desc string) (*GovernanceBlueprint, error) {
	if m.governanceFunc != nil {
		return m.governanceFunc(ctx, desc)
	}
	return nil, nil
}

// fastRetryOpts retorna opcoes de retry rapidas para testes.
func fastRetryOpts() retry.Options {
	return retry.Options{
		InitialInterval:     10 * time.Millisecond,
		MaxInterval:         50 * time.Millisecond,
		MaxElapsedTime:      500 * time.Millisecond,
		RandomizationFactor: 0,
		Multiplier:          1.5,
	}
}

func TestRetryProvider_Name(t *testing.T) {
	inner := &mockProvider{name: "meu-provider"}
	rp := NewRetryProvider(inner, fastRetryOpts(), nil)
	assert.Equal(t, "meu-provider", rp.Name())
}

func TestRetryProvider_IsAvailable(t *testing.T) {
	inner := &mockProvider{name: "test", available: true}
	rp := NewRetryProvider(inner, fastRetryOpts(), nil)
	assert.True(t, rp.IsAvailable(context.Background()))

	inner2 := &mockProvider{name: "test", available: false}
	rp2 := NewRetryProvider(inner2, fastRetryOpts(), nil)
	assert.False(t, rp2.IsAvailable(context.Background()))
}

func TestRetryProvider_Completion_SucessoPrimeiraTentativa(t *testing.T) {
	inner := &mockProvider{
		name: "test",
		completionFunc: func(_ context.Context, _, _ string) (string, error) {
			return "resposta ok", nil
		},
	}
	rp := NewRetryProvider(inner, fastRetryOpts(), nil)

	result, err := rp.Completion(context.Background(), "sys", "usr")
	require.NoError(t, err)
	assert.Equal(t, "resposta ok", result)
}

func TestRetryProvider_Completion_RetryEm429(t *testing.T) {
	var calls atomic.Int32
	inner := &mockProvider{
		name: "test",
		completionFunc: func(_ context.Context, _, _ string) (string, error) {
			n := calls.Add(1)
			if n <= 2 {
				return "", &APIError{Provider: "test", StatusCode: 429, Body: "rate limited"}
			}
			return "sucesso apos retry", nil
		},
	}
	rp := NewRetryProvider(inner, fastRetryOpts(), nil)

	result, err := rp.Completion(context.Background(), "sys", "usr")
	require.NoError(t, err)
	assert.Equal(t, "sucesso apos retry", result)
	assert.Equal(t, int32(3), calls.Load(), "deveria ter tentado 3 vezes")
}

func TestRetryProvider_Completion_PermanenteEm401(t *testing.T) {
	var calls atomic.Int32
	inner := &mockProvider{
		name: "test",
		completionFunc: func(_ context.Context, _, _ string) (string, error) {
			calls.Add(1)
			return "", &APIError{Provider: "test", StatusCode: 401, Body: "unauthorized"}
		},
	}
	rp := NewRetryProvider(inner, fastRetryOpts(), nil)

	_, err := rp.Completion(context.Background(), "sys", "usr")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Equal(t, int32(1), calls.Load(), "nao deveria retentar em 401")
}

func TestRetryProvider_StreamCompletion_SemRetryAposEscrita(t *testing.T) {
	var calls atomic.Int32
	inner := &mockProvider{
		name: "test",
		streamFunc: func(_ context.Context, _, _ string, out io.Writer) error {
			calls.Add(1)
			// Escreve dados e depois falha
			_, _ = io.WriteString(out, "dados parciais")
			return &APIError{Provider: "test", StatusCode: 503, Body: "service unavailable"}
		},
	}
	rp := NewRetryProvider(inner, fastRetryOpts(), nil)

	var buf bytes.Buffer
	err := rp.StreamCompletion(context.Background(), "sys", "usr", &buf)
	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load(), "nao deveria retentar apos escrita parcial")
	assert.Equal(t, "dados parciais", buf.String())
}

func TestRetryProvider_StreamCompletion_RetrySeNaoEscreveu(t *testing.T) {
	var calls atomic.Int32
	inner := &mockProvider{
		name: "test",
		streamFunc: func(_ context.Context, _, _ string, out io.Writer) error {
			n := calls.Add(1)
			if n <= 1 {
				// Falha sem escrever nada
				return &APIError{Provider: "test", StatusCode: 503, Body: "service unavailable"}
			}
			_, _ = io.WriteString(out, "sucesso")
			return nil
		},
	}
	rp := NewRetryProvider(inner, fastRetryOpts(), nil)

	var buf bytes.Buffer
	err := rp.StreamCompletion(context.Background(), "sys", "usr", &buf)
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, "sucesso", buf.String())
}

func TestRetryProvider_EmbedDocuments_RetryEmErroDeRede(t *testing.T) {
	var calls atomic.Int32
	inner := &mockProvider{
		name: "test",
		embedFunc: func(_ context.Context, texts []string) ([][]float32, error) {
			n := calls.Add(1)
			if n <= 1 {
				// Erro de rede (sem APIError) — retentavel por padrao
				return nil, fmt.Errorf("connection reset by peer")
			}
			return [][]float32{{0.1, 0.2}}, nil
		},
	}
	rp := NewRetryProvider(inner, fastRetryOpts(), nil)

	result, err := rp.EmbedDocuments(context.Background(), []string{"texto"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int32(2), calls.Load(), "deveria ter retentado em erro de rede")
}

func TestRetryProvider_GenerateGovernance_Sucesso(t *testing.T) {
	expected := &GovernanceBlueprint{Domain: "fintech"}
	inner := &mockProvider{
		name: "test",
		governanceFunc: func(_ context.Context, _ string) (*GovernanceBlueprint, error) {
			return expected, nil
		},
	}
	rp := NewRetryProvider(inner, fastRetryOpts(), nil)

	result, err := rp.GenerateGovernance(context.Background(), "teste")
	require.NoError(t, err)
	assert.Equal(t, "fintech", result.Domain)
}

func TestRetryProvider_StatusCodesCustomizados(t *testing.T) {
	var calls atomic.Int32
	inner := &mockProvider{
		name: "test",
		completionFunc: func(_ context.Context, _, _ string) (string, error) {
			n := calls.Add(1)
			if n <= 1 {
				return "", &APIError{Provider: "test", StatusCode: 500, Body: "error"}
			}
			return "ok", nil
		},
	}
	// Incluir 500 como retentavel
	customStatuses := map[int]bool{500: true}
	rp := NewRetryProvider(inner, fastRetryOpts(), customStatuses)

	result, err := rp.Completion(context.Background(), "sys", "usr")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
	assert.Equal(t, int32(2), calls.Load())
}
