package cmd

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/testutil"
)

func TestRunnerQuieto_GuardaASaida(t *testing.T) {
	var chamadas []string
	base := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			chamadas = append(chamadas, "combined "+name)
			if name == "helm" {
				return []byte("linha 1\nlinha 2\n\nError: timed out\r\nINSTALLATION FAILED\n"), fmt.Errorf("exit status 1")
			}
			return []byte("ok"), nil
		},
		RunStdinOutputFunc: func(ctx context.Context, stdin, name string, args ...string) ([]byte, error) {
			chamadas = append(chamadas, "stdin "+name+" "+stdin)
			return nil, fmt.Errorf("exit status 1")
		},
	}
	r := runnerQuieto{base}

	assert.NoError(t, r.Run(context.Background(), "k3d", "cluster", "create"))
	err := r.Run(context.Background(), "helm", "upgrade")
	assert.EqualError(t, err, "exit status 1: linha 2 · Error: timed out · INSTALLATION FAILED")
	// sem saída, o erro volta como está
	assert.EqualError(t, r.RunStdin(context.Background(), "apiVersion: v1", "kubectl", "apply"), "exit status 1")
	assert.Equal(t, []string{"combined k3d", "combined helm", "stdin kubectl apiVersion: v1"}, chamadas)
}

func TestDicaDe(t *testing.T) {
	interno := errors.New(errors.ErrCodeCmdNotFound, "k3d não encontrado").WithHint("rode yby setup")
	assert.Equal(t, "rode yby setup", dicaDe(fmt.Errorf("falha: %w", interno), "padrão"))
	assert.Equal(t, "rode yby setup", dicaDe(errors.Wrap(interno, errors.ErrCodeExec, "envolto"), "padrão"))
	assert.Equal(t, "padrão", dicaDe(fmt.Errorf("sem dica"), "padrão"))
	assert.Equal(t, "padrão", dicaDe(errors.New(errors.ErrCodeExec, "sem dica"), "padrão"))
}

func TestNovoProgressoPadrao(t *testing.T) {
	var b bytes.Buffer
	p := novoProgresso(&b, "a")
	assert.False(t, p.Terminal(), "um buffer não é terminal: as linhas saem uma vez cada")
	p.Iniciar("a")
	p.Concluir("")
	assert.NotContains(t, b.String(), "\x1b[")
}
