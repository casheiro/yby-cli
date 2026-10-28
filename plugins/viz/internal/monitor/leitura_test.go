package monitor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// clientQueConta é um Client que devolve sempre o mesmo erro e conta as chamadas.
type clientQueConta struct {
	err      error
	chamadas atomic.Int32
}

func (c *clientQueConta) falhar() error { c.chamadas.Add(1); return c.err }

func (c *clientQueConta) GetPods(ListFilter) ([]Pod, error)               { return nil, c.falhar() }
func (c *clientQueConta) GetDeployments(ListFilter) ([]Deployment, error) { return nil, c.falhar() }
func (c *clientQueConta) GetServices(ListFilter) ([]Service, error)       { return nil, c.falhar() }
func (c *clientQueConta) GetNodes(ListFilter) ([]Node, error)             { return nil, c.falhar() }
func (c *clientQueConta) GetStatefulSets(ListFilter) ([]StatefulSet, error) {
	return nil, c.falhar()
}
func (c *clientQueConta) GetJobs(ListFilter) ([]Job, error)             { return nil, c.falhar() }
func (c *clientQueConta) GetIngresses(ListFilter) ([]Ingress, error)    { return nil, c.falhar() }
func (c *clientQueConta) GetConfigMaps(ListFilter) ([]ConfigMap, error) { return nil, c.falhar() }
func (c *clientQueConta) GetEvents(ListFilter) ([]Event, error)         { return nil, c.falhar() }

// leiturasDe são as nove listas do Client, uma por aba.
func leiturasDe(c Client) map[string]func() error {
	f := ListFilter{}
	return map[string]func() error{
		"pods":         func() error { _, err := c.GetPods(f); return err },
		"deployments":  func() error { _, err := c.GetDeployments(f); return err },
		"services":     func() error { _, err := c.GetServices(f); return err },
		"nodes":        func() error { _, err := c.GetNodes(f); return err },
		"statefulsets": func() error { _, err := c.GetStatefulSets(f); return err },
		"jobs":         func() error { _, err := c.GetJobs(f); return err },
		"ingresses":    func() error { _, err := c.GetIngresses(f); return err },
		"configmaps":   func() error { _, err := c.GetConfigMaps(f); return err },
		"eventos":      func() error { _, err := c.GetEvents(f); return err },
	}
}

// TestRetryClient_NaoRepeteErroPermanente: negado, credencial recusada, recurso
// que não existe, pedido inválido, credencial do exec que não saiu e prazo
// estourado voltam na primeira tentativa, com o erro de origem.
func TestRetryClient_NaoRepeteErroPermanente(t *testing.T) {
	pods := schema.GroupResource{Resource: "pods"}
	casos := map[string]error{
		"forbidden":    apierrors.NewForbidden(pods, "", errors.New("RBAC")),
		"unauthorized": apierrors.NewUnauthorized("token vencido"),
		"notfound":     apierrors.NewNotFound(pods, "web-1"),
		"invalid":      apierrors.NewInvalid(schema.GroupKind{Kind: "Pod"}, "web-1", field.ErrorList{field.Required(field.NewPath("spec"), "")}),
		"credencial":   fmt.Errorf(`Get "https://eks": getting credentials: exec: executable aws failed with exit code 255`),
		"prazo":        fmt.Errorf("falha ao listar pods: %w", context.DeadlineExceeded),
	}
	for nome, causa := range casos {
		for leitura := range leiturasDe(&FakeClient{}) {
			inner := &clientQueConta{err: fmt.Errorf("falha ao listar %s: %w", leitura, causa)}
			r := &RetryClient{inner: inner, maxRetries: 3, baseDelay: time.Millisecond}
			err := leiturasDe(r)[leitura]()
			if n := inner.chamadas.Load(); n != 1 {
				t.Errorf("%s em %s: %d tentativas, esperada 1", nome, leitura, n)
			}
			if !errors.Is(err, causa) {
				t.Errorf("%s em %s: o erro de origem se perdeu: %v", nome, leitura, err)
			}
		}
	}
}

// TestRetryClient_RepeteErroPassageiro: a queda de conexão segue com as três tentativas.
func TestRetryClient_RepeteErroPassageiro(t *testing.T) {
	for leitura := range leiturasDe(&FakeClient{}) {
		inner := &clientQueConta{err: errors.New("dial tcp 127.0.0.1:6443: connection refused")}
		r := &RetryClient{inner: inner, maxRetries: 3, baseDelay: time.Millisecond}
		if err := leiturasDe(r)[leitura](); err == nil {
			t.Fatalf("%s: esperava erro", leitura)
		}
		if n := inner.chamadas.Load(); n != 3 {
			t.Errorf("%s: %d tentativas, esperadas 3", leitura, n)
		}
	}
}

// servidorParado responde nada até o cliente desistir: um cluster que aceita a
// conexão e não devolve a lista.
func servidorParado(t *testing.T) *httptest.Server {
	t.Helper()
	fim := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-fim:
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(fim) }) // roda antes do Close: solta os handlers presos
	return srv
}

// TestK8sClient_LeituraComPrazo: cada lista desiste no prazo de leitura em vez
// de esperar para sempre o servidor que não responde.
func TestK8sClient_LeituraComPrazo(t *testing.T) {
	srv := servidorParado(t)
	cs, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	c := NewK8sClientDe(cs)
	c.prazo = 50 * time.Millisecond
	for leitura, ler := range leiturasDe(c) {
		volta := make(chan error, 1)
		go func() { volta <- ler() }()
		select {
		case err := <-volta:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("%s: esperava prazo estourado, veio %v", leitura, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: a leitura não voltou no prazo", leitura)
		}
	}
}

// TestErroDeCredencial: o erro do plugin exec do kubeconfig (o client-go não o
// tipa) é reconhecido com o texto que o client-go escreve de verdade.
func TestErroDeCredencial(t *testing.T) {
	srv := servidorParado(t)
	cfg := &rest.Config{Host: srv.URL, ExecProvider: &clientcmdapi.ExecConfig{
		APIVersion:      "client.authentication.k8s.io/v1",
		Command:         "yby-plugin-de-credencial-que-nao-existe",
		InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
	}}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c := NewK8sClientDe(cs)
	c.prazo = 5 * time.Second
	_, err = c.GetPods(ListFilter{})
	if err == nil || !ErroDeCredencial(err) {
		t.Fatalf("o exec que falha deveria ser erro de credencial: %v", err)
	}
	if ErroDeCredencial(errors.New("dial tcp: connection refused")) || ErroDeCredencial(nil) {
		t.Fatal("só a falha do exec é erro de credencial")
	}
	// Pelo RetryClient, o erro chega na primeira tentativa e continua reconhecível.
	inner := &clientQueConta{err: err}
	r := &RetryClient{inner: inner, maxRetries: 3, baseDelay: time.Millisecond}
	if _, err := r.GetPods(ListFilter{}); !ErroDeCredencial(err) || inner.chamadas.Load() != 1 {
		t.Fatalf("RetryClient: %d tentativas, %v", inner.chamadas.Load(), err)
	}
}
