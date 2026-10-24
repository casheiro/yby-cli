package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResumirSaida(t *testing.T) {
	casos := []struct{ nome, msg, quer string }{
		{"saída do git com várias linhas", "exit status 1: error: src refspec HEAD does not match any · error: failed to push some refs", "src refspec HEAD does not match any"},
		{"fatal do git", "exit status 128: fatal: not a git repository", "not a git repository"},
		{"Error do helm em outra linha", "exit status 1: aviso qualquer\nError: chart not found", "chart not found"},
		{"sem linha de erro fica a última", "exit status 1: linha 1 · linha 2 · Unable to connect to the server", "Unable to connect to the server"},
		{"mensagem simples volta como veio", "open /x/y.yaml: no such file or directory", "open /x/y.yaml: no such file or directory"},
		{"só o exit status", "exit status 1", "exit status 1"},
		{"rodapé do kubectl sai", `exit status 1: error: error validating "STDIN": failed to download openapi; if you choose to ignore these errors, turn validation off with --validate=false`, `error validating "STDIN": failed to download openapi`},
		{"vazia", "  ", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			assert.Equal(t, c.quer, ResumirSaida(c.msg))
		})
	}
}
