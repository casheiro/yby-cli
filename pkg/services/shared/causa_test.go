package shared

import (
	stdErr "errors"
	"fmt"
	"testing"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
)

func TestCausaCurta(t *testing.T) {
	saidaDoPush := fmt.Errorf("%w: %s", stdErr.New("exit status 1"),
		"error: src refspec HEAD does not match any · error: failed to push some refs to 'git://localhost:51234/repo.git'")
	casos := []struct {
		nome string
		err  error
		quer string
	}{
		{"nil", nil, ""},
		{"git push sem commit", ybyerrors.Wrap(saidaDoPush, ybyerrors.ErrCodeExec, "git push failed"),
			"src refspec HEAD does not match any"},
		{"erro simples dentro de YbyError",
			ybyerrors.Wrap(stdErr.New("no running pods found for service git-server"), ybyerrors.ErrCodePortForward, "failed to start port forwarder"),
			"no running pods found for service git-server"},
		{"YbyError sem causa", ybyerrors.New(ybyerrors.ErrCodePortForward, "tunnel not established"), "tunnel not established"},
		{"sem linha de erro fica a última", stdErr.New("exit status 2: Reading package lists... · E: Unable to locate package x"),
			"E: Unable to locate package x"},
		{"várias linhas", stdErr.New("aviso qualquer\nfatal: repository not found\n"), "repository not found"},
		{"só o código de saída", stdErr.New("exit status 1"), "exit status 1"},
		{"vazio", stdErr.New("  "), ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := CausaCurta(c.err); got != c.quer {
				t.Errorf("CausaCurta = %q; esperava %q", got, c.quer)
			}
		})
	}
}
