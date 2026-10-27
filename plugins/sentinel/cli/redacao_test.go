//go:build k8s

package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/backends"
)

// clienteComLogs é o cluster falso com os logs que o teste escolhe: o fake do client-go
// sempre devolve "fake logs".
type clienteComLogs struct {
	*fake.Clientset
	logs string
}

func (c clienteComLogs) CoreV1() typedcorev1.CoreV1Interface {
	return coreComLogs{c.Clientset.CoreV1(), c.logs}
}

type coreComLogs struct {
	typedcorev1.CoreV1Interface
	logs string
}

func (c coreComLogs) Pods(ns string) typedcorev1.PodInterface {
	return podsComLogs{c.CoreV1Interface.Pods(ns), ns, c.logs}
}

type podsComLogs struct {
	typedcorev1.PodInterface
	ns, logs string
}

func (p podsComLogs) GetLogs(nome string, _ *corev1.PodLogOptions) *rest.Request {
	cli := &fakerest.RESTClient{
		Client: fakerest.CreateHTTPClient(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(p.logs))}, nil
		}),
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         corev1.SchemeGroupVersion,
		VersionedAPIPath:     fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/log", p.ns, nome),
	}
	return cli.Request()
}

var _ kubernetes.Interface = clienteComLogs{}

// Segredos que aparecem em logs e eventos de verdade e não podem chegar ao provider.
const (
	jwtDeTeste    = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhcHAifQ.c2lnbmF0dXJh"
	senhaDeTeste  = "s3nh4F0rte"
	chaveAWSTeste = "AKIAABCDEFGHIJKLMNOP"
)

func TestInvestigate_RedigeSegredosAntesDaIA(t *testing.T) {
	pod, ev := podComProblema()
	ev.Message = "Back-off restarting failed container; env DB_PASSWORD=" + senhaDeTeste
	logs := strings.Join([]string{
		"GET /api/v1/pagamentos Authorization: Bearer " + jwtDeTeste,
		"conectando em postgres://app:" + senhaDeTeste + "@db.payments:5432/app",
		"aws: usando a chave " + chaveAWSTeste,
		"panic: dial tcp 10.0.0.7:5432: connect: connection refused",
	}, "\n")
	prov := &provedorFalso{resposta: respostaDaIA}
	usarDependencias(t, clienteComLogs{fake.NewSimpleClientset(pod, ev), logs}, nil, nil, prov)

	if _, _, err := rodar(t, "investigate", "worker-5d8b6-q7tvn", "-n", "payments", "--no-cache"); err != nil {
		t.Fatal(err)
	}
	if len(prov.recebidos) != 1 {
		t.Fatalf("esperava uma chamada à IA, houve %d", len(prov.recebidos))
	}
	enviado := prov.recebidos[0]
	for _, segredo := range []string{jwtDeTeste, senhaDeTeste, chaveAWSTeste} {
		if strings.Contains(enviado, segredo) {
			t.Errorf("o segredo %q foi para a IA:\n%s", segredo, enviado)
		}
	}
	// o diagnóstico continua lá
	for _, q := range []string{"[REDIGIDO]", "connection refused", "postgres://app:", "@db.payments:5432", "Back-off restarting failed container"} {
		if !strings.Contains(enviado, q) {
			t.Errorf("faltou %q no que foi para a IA:\n%s", q, enviado)
		}
	}
}

func TestScan_RedigeSegredosAntesDaIA(t *testing.T) {
	achado := backends.Finding{
		ID: "opa/env-secret", Severity: "high", Category: "pod-security",
		Resource: "Deployment/api", Namespace: "payments",
		Message:        "o contêiner api tem DB_PASSWORD=" + senhaDeTeste + " em texto puro no env",
		Recommendation: "use um Secret",
	}
	prov := &provedorFalso{resposta: "1. Use um Secret."}
	usarDependencias(t, fake.NewSimpleClientset(), []backends.SecurityBackend{backendFalso{nome: "opa", achados: []backends.Finding{achado}}}, nil, prov)

	if _, _, err := rodar(t, "scan", "-n", "payments"); err != nil {
		t.Fatal(err)
	}
	if len(prov.recebidos) != 1 {
		t.Fatalf("esperava uma chamada à IA, houve %d", len(prov.recebidos))
	}
	enviado := prov.recebidos[0]
	if strings.Contains(enviado, senhaDeTeste) {
		t.Errorf("a senha foi para a IA:\n%s", enviado)
	}
	if !strings.Contains(enviado, "Deployment/api") || !strings.Contains(enviado, "[REDIGIDO]") {
		t.Errorf("o achado redigido deveria ir para a IA:\n%s", enviado)
	}
}
