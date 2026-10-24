package shared

import (
	stdErr "errors"
	"regexp"
	"strings"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
)

// prefixoSaida é o "exit status N: " que antecede a saída de um comando no erro.
var prefixoSaida = regexp.MustCompile(`^exit status \d+: `)

// prefixosDeErro marcam, na saída de git, helm e kubectl, a linha que diz o motivo.
var prefixosDeErro = []string{"fatal: ", "error: ", "Error: "}

// CausaCurta resume err numa nota curta para as pessoas: a mensagem do erro mais interno,
// sem o código dos YbyError. Quando ela traz a saída de um comando ("exit status 1: error:
// src refspec HEAD does not match any · error: failed to push some refs"), fica a primeira
// linha de erro, sem o prefixo; sem linha de erro, a última. err nil devolve "".
func CausaCurta(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	for e := err; e != nil; {
		var ye *ybyerrors.YbyError
		if !stdErr.As(e, &ye) {
			msg = e.Error()
			break
		}
		if ye.Cause == nil {
			msg = ye.Message
			break
		}
		e = ye.Cause
	}

	msg = prefixoSaida.ReplaceAllString(strings.TrimSpace(msg), "")
	var partes []string
	for _, linha := range strings.Split(strings.ReplaceAll(msg, "\r\n", "\n"), "\n") {
		for _, p := range strings.Split(linha, " · ") {
			if p = strings.TrimSpace(p); p != "" {
				partes = append(partes, p)
			}
		}
	}
	if len(partes) == 0 {
		return ""
	}
	for _, p := range partes {
		for _, prefixo := range prefixosDeErro {
			if strings.HasPrefix(p, prefixo) {
				return strings.TrimSpace(strings.TrimPrefix(p, prefixo))
			}
		}
	}
	return partes[len(partes)-1]
}

// reLinhaKlog acha as linhas de log do client-go que o kubectl escreve no stderr
// ("E1024 15:46:20.317079 39364 memcache.go:265] ...").
var reLinhaKlog = regexp.MustCompile(`^[IWEF]\d{4} \d{2}:\d{2}:\d{2}\.\d+ +\d+ [^ ]+:\d+\] `)

// SemLogDoKlog tira da saída do kubectl as linhas de log do client-go, que repetem a
// mesma falha várias vezes, e deixa a mensagem final ("Unable to connect to the
// server: ..."). Se só houver linhas de log, fica a última.
func SemLogDoKlog(saida string) string {
	var resto []string
	ultimaDeLog := ""
	for _, l := range strings.Split(strings.TrimRight(saida, "\n"), "\n") {
		l = strings.TrimRight(l, "\r")
		if reLinhaKlog.MatchString(l) {
			ultimaDeLog = l
			continue
		}
		if strings.TrimSpace(l) != "" {
			resto = append(resto, l)
		}
	}
	if len(resto) == 0 {
		return ultimaDeLog
	}
	return strings.Join(resto, "\n")
}

// erroComSaida é o erro de um comando externo com a mensagem que ele escreveu.
type erroComSaida struct {
	saida string
	err   error
}

func (e *erroComSaida) Error() string { return e.saida }
func (e *erroComSaida) Unwrap() error { return e.err }

// ComSaida troca o "exit status 1" de err pela mensagem que o comando escreveu (sem as
// linhas de log do client-go), para a causa do erro dizer o motivo. O erro original
// continua acessível por errors.Is/As. Sem saída, devolve err.
func ComSaida(err error, saida []byte) error {
	msg := strings.TrimSpace(SemLogDoKlog(string(saida)))
	if err == nil || msg == "" {
		return err
	}
	return &erroComSaida{saida: msg, err: err}
}
