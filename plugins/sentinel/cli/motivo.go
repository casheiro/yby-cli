//go:build k8s

package main

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// verbosDoKubernetes traduz o verbo de um "cannot <verbo> resource" do Kubernetes.
var verbosDoKubernetes = map[string]string{
	"list":   "listar",
	"get":    "ler",
	"watch":  "acompanhar",
	"create": "criar",
	"update": "alterar",
	"patch":  "alterar",
	"delete": "apagar",
}

// reNegado lê o verbo e o recurso da mensagem de um 403 do Kubernetes: `User "x" cannot
// list resource "deployments" in API group "apps"`.
var reNegado = regexp.MustCompile(`cannot (\w+) resource "([^"]+)"`)

// reProibido lê o recurso do começo da mensagem de um 403 (`deployments.apps is
// forbidden`), para quando o erro chega como texto, sem o StatusError.
var reProibido = regexp.MustCompile(`(?:^|: )([a-z0-9.-]+)(?: "[^"]*")? is forbidden`)

// trechosDeRede são os pedaços das mensagens do Go e do client-go quando o cluster não
// responde; uma biblioteca que embrulha o erro com %v perde o tipo, e só o texto fica.
var trechosDeRede = []string{
	"dial tcp", "connection refused", "connection reset", "i/o timeout",
	"no such host", "tls handshake timeout", "unexpected eof", ": eof", "network is unreachable",
	"the server is currently unable", "context deadline exceeded",
}

// motivoNaTela diz em PT-BR por que uma etapa do scan que consulta o cluster falhou, para
// a linha embaixo dela: sem permissão, credenciais recusadas ou cluster sem resposta. O
// erro cru (em inglês, com URL e usuário) fica para o log de depuração, onde quem chama o
// manda.
func motivoNaTela(err error) string {
	texto := strings.ToLower(err.Error())
	switch {
	case apierrors.IsForbidden(err) || strings.Contains(texto, "is forbidden"):
		return semPermissao(err)
	case apierrors.IsUnauthorized(err) || strings.Contains(texto, "unauthorized"):
		return "o cluster recusou as credenciais do kubeconfig"
	case erroDeRede(err, texto) || apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) || apierrors.IsServiceUnavailable(err):
		return "o cluster parou de responder"
	}
	return dicaDoLogDoPlugin + " para ver o motivo"
}

// motivoDaIA diz em PT-BR por que as recomendações da IA não vieram.
func motivoDaIA(err error) string {
	if erroDeRede(err, strings.ToLower(err.Error())) {
		return "o provider de IA não respondeu"
	}
	return dicaDoLogDoPlugin + " para ver o motivo"
}

// erroDeRede diz se err é de rede (o cluster ou o provider não respondeu).
func erroDeRede(err error, texto string) bool {
	var ne net.Error
	if errors.As(err, &ne) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	for _, t := range trechosDeRede {
		if strings.Contains(texto, t) {
			return true
		}
	}
	return false
}

// semPermissao monta "sem permissão para listar os deployments" com o verbo e o recurso
// do 403, quando dá para lê-los.
func semPermissao(err error) string {
	verbo, recurso := "acessar", ""
	if m := reNegado.FindStringSubmatch(err.Error()); m != nil {
		if v, ok := verbosDoKubernetes[m[1]]; ok {
			verbo = v
		}
		recurso = m[2]
	}
	var status apierrors.APIStatus
	if recurso == "" && errors.As(err, &status) && status.Status().Details != nil {
		recurso = status.Status().Details.Kind
	}
	if recurso == "" {
		if m := reProibido.FindStringSubmatch(err.Error()); m != nil {
			recurso = m[1]
		}
	}
	// "deployments.apps" vira "deployments"
	recurso, _, _ = strings.Cut(recurso, ".")
	if recurso == "" {
		return "sem permissão no cluster"
	}
	return "sem permissão para " + verbo + " os " + recurso
}
