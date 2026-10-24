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
