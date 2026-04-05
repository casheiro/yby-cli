package monitor

import (
	"fmt"
	"time"
)

// RetryClient envolve um Client com lógica de retry e backoff exponencial
type RetryClient struct {
	inner      Client
	maxRetries int
	baseDelay  time.Duration
}

// NewRetryClient cria um RetryClient com configuração padrão (3 tentativas, 1s base)
func NewRetryClient(inner Client) *RetryClient {
	return &RetryClient{
		inner:      inner,
		maxRetries: 3,
		baseDelay:  time.Second,
	}
}

// GetPods tenta obter pods com retry e backoff exponencial
func (r *RetryClient) GetPods(filter ListFilter) ([]Pod, error) {
	var lastErr error
	for i := 0; i < r.maxRetries; i++ {
		data, err := r.inner.GetPods(filter)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if i < r.maxRetries-1 {
			time.Sleep(r.baseDelay * time.Duration(1<<uint(i)))
		}
	}
	return nil, fmt.Errorf("falha após %d tentativas: %w", r.maxRetries, lastErr)
}

// GetDeployments tenta obter deployments com retry e backoff exponencial
func (r *RetryClient) GetDeployments(filter ListFilter) ([]Deployment, error) {
	var lastErr error
	for i := 0; i < r.maxRetries; i++ {
		data, err := r.inner.GetDeployments(filter)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if i < r.maxRetries-1 {
			time.Sleep(r.baseDelay * time.Duration(1<<uint(i)))
		}
	}
	return nil, fmt.Errorf("falha após %d tentativas: %w", r.maxRetries, lastErr)
}

// GetServices tenta obter services com retry e backoff exponencial
func (r *RetryClient) GetServices(filter ListFilter) ([]Service, error) {
	var lastErr error
	for i := 0; i < r.maxRetries; i++ {
		data, err := r.inner.GetServices(filter)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if i < r.maxRetries-1 {
			time.Sleep(r.baseDelay * time.Duration(1<<uint(i)))
		}
	}
	return nil, fmt.Errorf("falha após %d tentativas: %w", r.maxRetries, lastErr)
}

// GetNodes tenta obter nodes com retry e backoff exponencial
func (r *RetryClient) GetNodes(filter ListFilter) ([]Node, error) {
	var lastErr error
	for i := 0; i < r.maxRetries; i++ {
		data, err := r.inner.GetNodes(filter)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if i < r.maxRetries-1 {
			time.Sleep(r.baseDelay * time.Duration(1<<uint(i)))
		}
	}
	return nil, fmt.Errorf("falha após %d tentativas: %w", r.maxRetries, lastErr)
}
