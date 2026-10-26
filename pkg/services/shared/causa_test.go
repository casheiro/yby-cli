package shared

import (
	stdErr "errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

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

func TestSemLogDoKlog(t *testing.T) {
	klog := `E1024 15:46:20.317079   39364 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list"`
	assert.Equal(t, "Unable to connect to the server: dial tcp [::1]:8080",
		SemLogDoKlog(klog+"\n"+klog+"\nUnable to connect to the server: dial tcp [::1]:8080\n"))
	assert.Equal(t, klog, SemLogDoKlog(klog+"\n"), "só log: fica a última linha")
	assert.Equal(t, "error: current-context is not set", SemLogDoKlog("error: current-context is not set\r\n"))
	assert.Equal(t, "", SemLogDoKlog(""))
}

func TestComSaida(t *testing.T) {
	base := stdErr.New("exit status 1")
	err := ComSaida(base, []byte("error: current-context is not set\n"))
	assert.EqualError(t, err, "error: current-context is not set")
	assert.ErrorIs(t, err, base, "o erro original continua acessível")
	assert.Equal(t, "current-context is not set", CausaCurta(err))

	assert.Same(t, base, ComSaida(base, []byte("  \n")), "sem saída, fica o erro")
	assert.NoError(t, ComSaida(nil, []byte("qualquer coisa")))
}
