package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kubectlFalso responde como um kubeconfig com só o contexto ctx-a (namespace nsa) e,
// conforme atual, com ou sem current-context; guarda os comandos.
type kubectlFalso struct {
	atual    string
	comandos []string
}

func (k *kubectlFalso) rodar(args ...string) (string, error) {
	linha := strings.Join(args, " ")
	k.comandos = append(k.comandos, linha)
	contexto := k.atual
	if len(args) > 1 && args[0] == "--context" {
		contexto, args = args[1], args[2:]
	}
	switch strings.Join(args, " ") {
	case "config current-context":
		// Como o kubectl: lê o arquivo e ignora o --context.
		if k.atual == "" {
			return "", errors.New("error: current-context is not set")
		}
		return k.atual + "\n", nil
	case "config get-contexts ctx-a -o name":
		return "ctx-a\n", nil
	case "config view --minify --output=jsonpath={..namespace}":
		if contexto == "ctx-a" {
			return "nsa", nil
		}
		return "", errors.New("contexto inexistente")
	}
	if strings.HasPrefix(strings.Join(args, " "), "config get-contexts ") {
		return "", errors.New("error: context not found")
	}
	if contexto != "ctx-a" {
		return "", errors.New("contexto inexistente")
	}
	if args[0] == "get" && args[1] == "pods" {
		return `{"items":[{"metadata":{"name":"worker-1"},"status":{"phase":"Running"}}]}`, nil
	}
	return `{"items":[]}`, nil
}

// Com --context, o contexto é conferido pelo nome: vale num kubeconfig sem
// current-context (o Viz aberto pelo ↵ do Cloud), e o kubectl segue esse contexto.
func TestEnriquecer_ContextoSemCurrentContext(t *testing.T) {
	k := &kubectlFalso{}
	cc := enriquecer(k.rodar, "ctx-a", "")
	require.NotNil(t, cc, "o contexto dado existe: o contexto do cluster não pode sumir")
	assert.Equal(t, "ctx-a", cc.Cluster)
	assert.Equal(t, "nsa", cc.Namespace)
	require.Len(t, cc.Pods, 1)
	assert.Equal(t, "worker-1", cc.Pods[0].Name)
	for _, c := range k.comandos {
		assert.NotContains(t, c, "current-context", "com --context o current-context não vale")
	}
}

// Um --context que não existe no kubeconfig não passa, mesmo com current-context.
func TestEnriquecer_ContextoInexistente(t *testing.T) {
	k := &kubectlFalso{atual: "ctx-a"}
	assert.Nil(t, enriquecer(k.rodar, "ctx-sumiu", ""))
}

// Sem --context, vale o current-context e o namespace dele; o -n troca o namespace.
func TestEnriquecer_SemContexto(t *testing.T) {
	cc := enriquecer((&kubectlFalso{atual: "ctx-a"}).rodar, "", "")
	require.NotNil(t, cc)
	assert.Equal(t, "ctx-a", cc.Cluster)
	assert.Equal(t, "nsa", cc.Namespace)

	cc = enriquecer((&kubectlFalso{atual: "ctx-a"}).rodar, "", "payments")
	require.NotNil(t, cc)
	assert.Equal(t, "payments", cc.Namespace)

	assert.Nil(t, enriquecer((&kubectlFalso{}).rodar, "", ""), "sem current-context nem --context, nada")
}
