package main

import (
	"bytes"
	"errors"
	"testing"

	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/plugins/bard/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLerArgs(t *testing.T) {
	casos := []struct {
		nome string
		args []string
		quer opcoesBard
	}{
		{"sem args", nil, opcoesBard{}},
		{"ajuda", []string{"--help"}, opcoesBard{ajuda: true}},
		{"-p junta o resto", []string{"-p", "lista", "os", "pods"}, opcoesBard{temPrompt: true, prompt: "lista os pods"}},
		{"-p sem pergunta", []string{"-p"}, opcoesBard{temPrompt: true}},
		{
			"o pedido do Viz",
			[]string{"--pergunta", "Por que o pod payments/worker está em CrashLoopBackOff (14 reinícios)?", "--context", "k3d-yby-local", "--namespace", "payments", "--voltar", "Viz"},
			opcoesBard{pergunta: "Por que o pod payments/worker está em CrashLoopBackOff (14 reinícios)?", contexto: "k3d-yby-local", namespace: "payments", voltar: "Viz"},
		},
		{"com igual e -n", []string{"--pergunta=Explique o nó k3d-0.", "-n", "kube-system"}, opcoesBard{pergunta: "Explique o nó k3d-0.", namespace: "kube-system"}},
		{"flags antes do -p", []string{"--context", "prod", "-p", "e", "agora?"}, opcoesBard{contexto: "prod", temPrompt: true, prompt: "e agora?"}},
		{"args desconhecidos ficam de fora", []string{"--verbose", "--pergunta", "oi"}, opcoesBard{pergunta: "oi"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			o, err := lerArgs(c.args)
			require.NoError(t, err)
			assert.Equal(t, c.quer, o)
		})
	}
}

func TestLerArgs_FlagSemValor(t *testing.T) {
	for _, args := range [][]string{{"--pergunta"}, {"--pergunta", "  "}, {"--context="}, {"-n"}, {"--voltar"}} {
		_, err := lerArgs(args)
		var ye *yerrors.YbyError
		if assert.True(t, errors.As(err, &ye), "args %q", args) {
			assert.Equal(t, yerrors.ErrCodeValidation, ye.Code)
			assert.Contains(t, ye.Message, "falta o valor de")
			assert.NotEmpty(t, ye.GetHint())
		}
	}
}

// O --context e o --namespace do Viz valem no cabeçalho mesmo sem resposta do kubectl.
func TestAplicarCluster(t *testing.T) {
	var c tui.Config
	aplicarCluster(&c, nil, opcoesBard{contexto: "k3d-yby-local", namespace: "payments"})
	assert.Equal(t, "k3d-yby-local", c.Cluster)
	assert.Equal(t, "payments", c.Namespace)

	c = tui.Config{}
	aplicarCluster(&c, &ClusterContext{Cluster: "atual", Namespace: "default"}, opcoesBard{namespace: "payments"})
	assert.Equal(t, "atual", c.Cluster)
	assert.Equal(t, "payments", c.Namespace)

	c = tui.Config{}
	aplicarCluster(&c, &ClusterContext{Cluster: "atual", Namespace: "default"}, opcoesBard{})
	assert.Equal(t, "atual", c.Cluster)
	assert.Equal(t, "default", c.Namespace)
}

// O ctrl+c da TUI sai com 130, calado; outro erro segue.
func TestSaidaDaConversa(t *testing.T) {
	var s saidaSilenciosa
	if assert.True(t, errors.As(saidaDaConversa(tui.ErrInterrompido), &s)) {
		assert.Equal(t, 130, s.codigo)
		var b bytes.Buffer
		assert.Equal(t, 130, sairComErro(&b, saidaDaConversa(tui.ErrInterrompido)))
		assert.Empty(t, b.String(), "o ctrl+c não escreve nada")
	}
	assert.NoError(t, saidaDaConversa(nil))
	outro := errors.New("terminal")
	assert.Equal(t, outro, saidaDaConversa(outro))
}
