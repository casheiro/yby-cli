package monitor

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// RetryClient envolve um Client com lógica de retry e backoff exponencial
type RetryClient struct {
	inner      Client
	maxRetries int
	baseDelay  time.Duration
}

// Inner retorna o client interno envolto pelo retry
func (r *RetryClient) Inner() Client {
	return r.inner
}

// Contexto devolve o contexto do kubeconfig do client interno, quando ele o conhece.
func (r *RetryClient) Contexto() string {
	if c, ok := r.inner.(interface{ Contexto() string }); ok {
		return c.Contexto()
	}
	return ""
}

// Kubeconfig devolve o arquivo do kubeconfig pedido ao client interno, quando ele o conhece.
func (r *RetryClient) Kubeconfig() string {
	if c, ok := r.inner.(interface{ Kubeconfig() string }); ok {
		return c.Kubeconfig()
	}
	return ""
}

// NamespaceInicial devolve o namespace por onde a TUI começa, do client
// interno, quando ele o conhece.
func (r *RetryClient) NamespaceInicial() string {
	if c, ok := r.inner.(interface{ NamespaceInicial() string }); ok {
		return c.NamespaceInicial()
	}
	return ""
}

// Sondar repassa a sonda ao client interno, sem retry: o relógio tenta de novo
// no próximo tique. Client que não sonda não tem o que confirmar.
func (r *RetryClient) Sondar() error {
	if s, ok := r.inner.(Sondador); ok {
		return s.Sondar()
	}
	return nil
}

// NewRetryClient cria um RetryClient com configuração padrão (3 tentativas, 1s base)
func NewRetryClient(inner Client) *RetryClient {
	return &RetryClient{
		inner:      inner,
		maxRetries: 3,
		baseDelay:  time.Second,
	}
}

// permanente diz se tentar de novo não muda a resposta: o servidor negou
// (RBAC), recusou a credencial, não achou o recurso ou recusou o pedido; a
// credencial do exec não saiu; ou a leitura passou do prazo, que repetida
// faria um tique esperar três prazos (o relógio tenta de novo no próximo).
func permanente(err error) bool {
	return apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsNotFound(err) ||
		apierrors.IsInvalid(err) || ErroDeCredencial(err) || errors.Is(err, context.DeadlineExceeded)
}

// repetir faz a leitura com até maxRetries tentativas e backoff exponencial;
// o erro permanente volta na hora, como veio.
func repetir[T any](r *RetryClient, filter ListFilter, ler func(ListFilter) (T, error)) (T, error) {
	var zero T
	var lastErr error
	for i := 0; i < r.maxRetries; i++ {
		data, err := ler(filter)
		if err == nil {
			return data, nil
		}
		if permanente(err) {
			return zero, err
		}
		lastErr = err
		if i < r.maxRetries-1 {
			time.Sleep(r.baseDelay * time.Duration(1<<uint(i)))
		}
	}
	return zero, fmt.Errorf("falha após %d tentativas: %w", r.maxRetries, lastErr)
}

// GetPods tenta obter pods com retry e backoff exponencial
func (r *RetryClient) GetPods(filter ListFilter) ([]Pod, error) {
	return repetir(r, filter, r.inner.GetPods)
}

// GetDeployments tenta obter deployments com retry e backoff exponencial
func (r *RetryClient) GetDeployments(filter ListFilter) ([]Deployment, error) {
	return repetir(r, filter, r.inner.GetDeployments)
}

// GetServices tenta obter services com retry e backoff exponencial
func (r *RetryClient) GetServices(filter ListFilter) ([]Service, error) {
	return repetir(r, filter, r.inner.GetServices)
}

// GetNodes tenta obter nodes com retry e backoff exponencial
func (r *RetryClient) GetNodes(filter ListFilter) ([]Node, error) {
	return repetir(r, filter, r.inner.GetNodes)
}

// GetStatefulSets tenta obter statefulsets com retry e backoff exponencial
func (r *RetryClient) GetStatefulSets(filter ListFilter) ([]StatefulSet, error) {
	return repetir(r, filter, r.inner.GetStatefulSets)
}

// GetJobs tenta obter jobs com retry e backoff exponencial
func (r *RetryClient) GetJobs(filter ListFilter) ([]Job, error) {
	return repetir(r, filter, r.inner.GetJobs)
}

// GetIngresses tenta obter ingresses com retry e backoff exponencial
func (r *RetryClient) GetIngresses(filter ListFilter) ([]Ingress, error) {
	return repetir(r, filter, r.inner.GetIngresses)
}

// GetConfigMaps tenta obter configmaps com retry e backoff exponencial
func (r *RetryClient) GetConfigMaps(filter ListFilter) ([]ConfigMap, error) {
	return repetir(r, filter, r.inner.GetConfigMaps)
}

// GetEvents tenta obter eventos com retry e backoff exponencial
func (r *RetryClient) GetEvents(filter ListFilter) ([]Event, error) {
	return repetir(r, filter, r.inner.GetEvents)
}
