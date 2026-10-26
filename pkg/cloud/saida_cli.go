package cloud

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Os CLIs das clouds (aws, az, gcloud) dizem o motivo da falha na saída, e o código de
// saída sozinho ("exit status 255") não ajuda ninguém. erroDoCLI junta ao erro a última
// linha útil da saída, para a frase da linha de estado, o toast e o audit log contarem o
// motivo (ex.: o SSO vencido da AWS).

// maxSaidaNoErro é o tamanho máximo, em caracteres, da linha da saída levada ao erro.
const maxSaidaNoErro = 240

// segredosNaSaida casa com tokens e chaves que um CLI pode ecoar: JWT (Entra ID), token de
// acesso do Google, token do EKS, chaves de acesso da AWS, o valor depois de "Bearer" e os
// valores de campos de segredo (entre aspas, o valor inteiro). O grupo 1 ou o 2, quando
// casa, é o prefixo que fica à vista ("Bearer ", "password: ").
var segredosNaSaida = regexp.MustCompile(`eyJ[\w-]+\.[\w.-]+|ya29\.[\w.-]+|k8s-aws-v1\.[\w-]+|\b(?:AKIA|ASIA)[A-Z0-9]{16}\b|(?i:\b(bearer\s+))\S+|(?i:("?(?:access_?token|secret_?access_?key|session_?token|client_?secret|api_?key|token|password|senha)"?\s*[:=]\s*))(?:"[^"]*"?|[^\s",}]+"?)`)

// erroDoCLI é o erro de uma chamada ao CLI com a última linha útil da saída: "aws eks
// get-token: exit status 255: Error when retrieving token from sso: Token has expired and
// refresh failed". Sem saída útil, só o comando e o erro.
func erroDoCLI(comando string, saida []byte, err error) error {
	if linha := linhaUtilDaSaida(saida); linha != "" {
		return fmt.Errorf("%s: %w: %s", comando, err, linha)
	}
	return fmt.Errorf("%s: %w", comando, err)
}

// linhaUtilDaSaida é a última linha não vazia da saída que não é pedaço de JSON (a saída é
// a combinada, e o stdout pode ter vindo antes do erro), sem segredos e aparada.
func linhaUtilDaSaida(saida []byte) string {
	linhas := strings.Split(strings.ReplaceAll(string(saida), "\r\n", "\n"), "\n")
	for i := len(linhas) - 1; i >= 0; i-- {
		l := strings.TrimSpace(linhas[i])
		if l == "" || pedacoDeJSON(l) {
			continue
		}
		l = segredosNaSaida.ReplaceAllStringFunc(l, func(s string) string {
			m := segredosNaSaida.FindStringSubmatch(s)
			for _, prefixo := range m[1:] {
				if prefixo != "" {
					return prefixo + "***"
				}
			}
			return "***"
		})
		if utf8.RuneCountInString(l) > maxSaidaNoErro {
			l = string([]rune(l)[:maxSaidaNoErro-1]) + "…"
		}
		return l
	}
	return ""
}

// pedacoDeJSON diz se a linha é parte de um JSON impresso (chaves, colchetes, "campo": valor).
func pedacoDeJSON(l string) bool {
	switch l[0] {
	case '{', '}', '[', ']':
		return true
	case '"':
		return strings.Contains(l, `":`)
	}
	return false
}

// regiaoDoARN tira a região de um ARN (arn:aws:eks:us-east-1:482100009310:cluster/nome);
// vazio se o ARN não tiver.
func regiaoDoARN(arn string) string {
	partes := strings.SplitN(arn, ":", 6)
	if len(partes) < 6 || partes[0] != "arn" {
		return ""
	}
	return partes[3]
}
