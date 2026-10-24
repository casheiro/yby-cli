package errors

import (
	"regexp"
	"strings"
)

// prefixoSaida é o "exit status N: " que antecede a saída de um comando no erro.
var prefixoSaida = regexp.MustCompile(`^exit status \d+: `)

// rodapesDoKubectl são frases fixas que o kubectl acrescenta ao motivo e não ajudam a
// entendê-lo.
var rodapesDoKubectl = []string{"; if you choose to ignore these errors, turn validation off with --validate=false"}

// prefixosDeErro marcam, na saída de git, helm e kubectl, a linha que diz o motivo.
var prefixosDeErro = []string{"fatal: ", "error: ", "Error: "}

// ResumirSaida resume a mensagem de um erro que traz a saída de um comando externo
// ("exit status 1: error: src refspec HEAD does not match any · error: failed to push
// some refs") para as pessoas: tira o "exit status N: " e fica com a primeira linha de
// erro, sem o prefixo; sem linha de erro, a última. As linhas podem vir separadas por
// quebra de linha ou por " · " (como o runner quieto as junta). O rodapé fixo do kubectl
// ("; if you choose to ignore these errors…") sai. Uma mensagem de uma linha só, sem
// esses marcadores, volta como veio.
func ResumirSaida(msg string) string {
	msg = prefixoSaida.ReplaceAllString(strings.TrimSpace(msg), "")
	for _, r := range rodapesDoKubectl {
		msg = strings.ReplaceAll(msg, r, "")
	}
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
