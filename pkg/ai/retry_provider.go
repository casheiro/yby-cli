package ai

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/casheiro/yby-cli/pkg/retry"
)

// DefaultRetryableStatusCodes define quais status HTTP devem ser retentados.
var DefaultRetryableStatusCodes = map[int]bool{
	429: true, // Too Many Requests
	502: true, // Bad Gateway
	503: true, // Service Unavailable
}

// RetryProvider e um decorator que implementa Provider e adiciona retry
// com backoff exponencial via pkg/retry.
type RetryProvider struct {
	inner             Provider
	opts              retry.Options
	retryableStatuses map[int]bool
}

// NewRetryProvider cria um RetryProvider que envolve o provider informado.
// Se statuses for nil, usa DefaultRetryableStatusCodes.
func NewRetryProvider(inner Provider, opts retry.Options, statuses map[int]bool) *RetryProvider {
	if statuses == nil {
		statuses = DefaultRetryableStatusCodes
	}
	return &RetryProvider{inner: inner, opts: opts, retryableStatuses: statuses}
}

func (r *RetryProvider) Name() string                         { return r.inner.Name() }
func (r *RetryProvider) IsAvailable(ctx context.Context) bool { return r.inner.IsAvailable(ctx) }

// Completion executa a chamada com retry automatico em erros retentaveis.
func (r *RetryProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var result string
	err := retry.Do(ctx, r.opts, func() error {
		res, err := r.inner.Completion(ctx, systemPrompt, userPrompt)
		if err != nil {
			return r.classifyError(err)
		}
		result = res
		return nil
	})
	return result, err
}

// StreamCompletion executa streaming com retry. Se ja escreveu bytes no writer,
// nao retenta para evitar dados duplicados.
func (r *RetryProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	sw := &safeWriter{w: out}
	return retry.Do(ctx, r.opts, func() error {
		err := r.inner.StreamCompletion(ctx, systemPrompt, userPrompt, sw)
		if err != nil {
			if sw.hasWritten.Load() {
				// Ja escreveu dados no writer, nao faz sentido retentar
				return backoff.Permanent(err)
			}
			return r.classifyError(err)
		}
		return nil
	})
}

// EmbedDocuments executa a chamada de embeddings com retry automatico.
func (r *RetryProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	var result [][]float32
	err := retry.Do(ctx, r.opts, func() error {
		res, err := r.inner.EmbedDocuments(ctx, texts)
		if err != nil {
			return r.classifyError(err)
		}
		result = res
		return nil
	})
	return result, err
}

// GenerateGovernance executa a geracao de governance com retry automatico.
func (r *RetryProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	var result *GovernanceBlueprint
	err := retry.Do(ctx, r.opts, func() error {
		res, err := r.inner.GenerateGovernance(ctx, description)
		if err != nil {
			return r.classifyError(err)
		}
		result = res
		return nil
	})
	return result, err
}

// classifyError determina se o erro e retentavel baseado no status HTTP.
// Erros que nao sao APIError ou cujo status nao esta na lista sao tratados
// como permanentes (nao retentaveis).
func (r *RetryProvider) classifyError(err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if !r.retryableStatuses[apiErr.StatusCode] {
			return backoff.Permanent(err)
		}

		// Respeitar Retry-After do servidor quando disponível
		if apiErr.RetryAfter > 0 {
			slog.Warn("erro do provider de IA, aguardando retry-after",
				"provider", r.inner.Name(),
				"status", apiErr.StatusCode,
				"retry_after", apiErr.RetryAfter,
			)
			time.Sleep(apiErr.RetryAfter)
		} else {
			slog.Warn("erro do provider de IA, retentando",
				"provider", r.inner.Name(),
				"status", apiErr.StatusCode,
			)
		}
		return err
	}
	// Erros que nao sao APIError (ex: rede) sao retentaveis por padrao
	return err
}

// safeWriter rastreia se ja escreveu bytes para evitar retry apos escrita parcial.
type safeWriter struct {
	w          io.Writer
	hasWritten atomic.Bool
}

func (sw *safeWriter) Write(p []byte) (n int, err error) {
	n, err = sw.w.Write(p)
	if n > 0 {
		sw.hasWritten.Store(true)
	}
	return
}
