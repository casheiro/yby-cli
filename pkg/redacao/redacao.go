// Package redacao tira segredos de texto antes de ele ir para disco, tela ou IA.
//
// É um pacote-folha (importa só regexp, sort e strings) para que o Bard, as tools e o
// Sentinel possam importá-lo sem ciclo.
//
// O desenho é conservador: na dúvida, redige. Exagerar (redigir a palavra de prosa
// que vem depois de "Secret:") é aceitável; deixar parte de um segredo no texto não é.
package redacao

import (
	"regexp"
	"sort"
	"strings"
)

// Marca é o que substitui o segredo.
const Marca = "[REDIGIDO]"

// padroes substituem o trecho inteiro pela Marca, onde quer que apareça.
var padroes = []*regexp.Regexp{
	// Bloco PEM de chave privada, inclusive PGP ("PRIVATE KEY BLOCK"), até o END
	// ou, se truncado, até o fim do texto.
	regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----.*?(?:-----END [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----|$)`),
	regexp.MustCompile(`ghp_[A-Za-z0-9_]{36,}`),        // GitHub personal access token
	regexp.MustCompile(`gho_[A-Za-z0-9_]{36,}`),        // GitHub OAuth token
	regexp.MustCompile(`ghu_[A-Za-z0-9_]{36,}`),        // GitHub user-to-server token
	regexp.MustCompile(`ghs_[A-Za-z0-9_]{36,}`),        // GitHub server-to-server token
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{22,}`), // GitHub fine-grained PAT
	// OpenAI (sk-proj-, sk-svcacct-, sk-admin-) e Anthropic (sk-ant-api03-), com
	// "-" e "_" no corpo. O \b evita nomes de recurso como task-runner-deployment.
	regexp.MustCompile(`\bsk-(?:proj-|ant-[a-z0-9]+-|svcacct-|admin-)?[A-Za-z0-9_\-]{20,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),                    // formato antigo, em qualquer posição
	regexp.MustCompile(`\b[rs]k_(?:live|test)_[A-Za-z0-9]{10,}`), // Stripe secret e restricted key
	regexp.MustCompile(`(?:AKIA|ASIA)[0-9A-Z]{16}`),              // AWS access key ID
	regexp.MustCompile(`xox[bporas]-[A-Za-z0-9\-]+`),             // Slack tokens
	// JWT: três segmentos base64url, os dois primeiros começam em "eyJ" ({").
	regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]*`),
}

// Redigir troca pela Marca os segredos conhecidos do texto: primeiro blocos PEM,
// tokens com formato próprio e senha em URL; depois, de uma vez, os tokens depois
// de "Bearer"/"Basic", os valores de nomes sensíveis e os values de env com name
// sensível.
//
// Esses três últimos são achados sobre o mesmo texto e redigidos pela união dos
// trechos, sem que um veja o resultado do outro. Aplicados em sequência, um
// escondia o que o outro precisava ler: em "X-Auth-Token: Bearer x" o par redigia
// "Bearer" e o token sobrava; em "token: password: x" o primeiro par engolia o
// nome do segundo e o valor sobrava. Com a união, o pior caso é redigir a mais.
func Redigir(texto string) string {
	for _, p := range padroes {
		texto = p.ReplaceAllString(texto, Marca)
	}
	texto = redigirURLs(texto)
	ivs := append(intervalosEsquemas(texto), intervalosPares(texto)...)
	return aplicar(texto, append(ivs, intervalosEnv(texto)...))
}

// aplicar troca pela Marca a união dos trechos. Trechos que se tocam viram uma
// marca só, para a segunda passada não ver duas marcas coladas como um valor.
func aplicar(texto string, ivs []intervalo) string {
	sort.Slice(ivs, func(a, b int) bool { return ivs[a].ini < ivs[b].ini })
	var out strings.Builder
	pos := 0
	for k := 0; k < len(ivs); k++ {
		ini, fim := ivs[k].ini, ivs[k].fim
		if fim <= ini || fim <= pos {
			continue
		}
		if ini < pos {
			ini = pos
		}
		for k+1 < len(ivs) && ivs[k+1].ini <= fim {
			k++
			if ivs[k].fim > fim {
				fim = ivs[k].fim
			}
		}
		out.WriteString(texto[pos:ini])
		out.WriteString(Marca)
		pos = fim
	}
	out.WriteString(texto[pos:])
	return out.String()
}

// ---------------------------------------------------------------------------
// Senha em URL

// esquemaURL acha o começo de uma URL (scheme://).
var esquemaURL = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://`)

// redigirURLs redige a senha de scheme://usuario:senha@host, com usuário vazio
// (redis://:senha@host) e com "@" na senha (redige até o último "@" antes do host).
func redigirURLs(texto string) string {
	var out strings.Builder
	pos := 0
	for _, loc := range esquemaURL.FindAllStringIndex(texto, -1) {
		if loc[0] < pos {
			continue
		}
		ini := loc[1]
		fim := ini + indiceEspaco(texto[ini:])
		a, b, ok := senhaURL(texto[ini:fim])
		if !ok {
			continue
		}
		out.WriteString(texto[pos : ini+a])
		out.WriteString(Marca)
		pos = ini + b
	}
	out.WriteString(texto[pos:])
	return out.String()
}

// senhaURL recebe o que vem depois de "scheme://" até o primeiro espaço e devolve
// o intervalo da senha: do primeiro ":" da autoridade até o último "@" que não
// tenha "/", "?" nem "#" entre ele e o "@" anterior. host:porta/caminho@x não é
// credencial.
func senhaURL(tok string) (int, int, bool) {
	fimAutoridade := strings.IndexAny(tok, "/?#")
	if fimAutoridade < 0 {
		fimAutoridade = len(tok)
	}
	c := strings.IndexByte(tok, ':')
	if c < 0 || c >= fimAutoridade || strings.IndexByte(tok[:c], '@') >= 0 {
		return 0, 0, false
	}
	i := strings.IndexByte(tok[c:], '@')
	if i < 0 {
		return 0, 0, false
	}
	a := c + i
	for {
		prox := strings.IndexByte(tok[a+1:], '@')
		if prox < 0 || strings.ContainsAny(tok[a+1:a+1+prox], "/?#") {
			break
		}
		a += 1 + prox
	}
	senha := tok[c+1 : a]
	if senha == "" {
		return 0, 0, false
	}
	if j := strings.IndexAny(senha, "/?#"); j >= 0 && soDigitos(senha[:j]) {
		return 0, 0, false // host:porta/caminho@algo
	}
	return c + 1, a, true
}

// ---------------------------------------------------------------------------
// Nomes sensíveis

// nomes casa cada nome seguido de separador (=, :, :=, =>, ": " de JSON, \": de
// JSON dentro de string), inclusive com o separador na linha de baixo (como o \s*
// do sanitizeContent antigo), ou de espaço (só vale para flags --nome valor). O valor é
// lido à parte e cada nome é olhado sozinho, então um par não sensível nunca
// engole um sensível à frente (--from-literal=password=x, a=1&password=x). Depois
// do nome cabe a marcação de markdown que a IA põe em volta dele (**DB_PASSWORD**:,
// `token` =); o "_" já entra no próprio nome.
var nomes = regexp.MustCompile(`(\\?["']?)([A-Za-z0-9_.\-]+)(\\?["']?[*` + "`" + `]{0,3})(?:\s*(?::=|=>|=|:)[ \t\v\f]*|([ \t]+))`)

// palavrasSensiveis tornam o nome sensível quando aparecem como palavra (separada
// por _, -, . ou fronteira de camelCase) ou como o segmento inteiro (passWord).
var palavrasSensiveis = map[string]bool{
	"password": true, "passwords": true, "passwd": true, "pwd": true, "pass": true,
	"passphrase": true, "secret": true, "secrets": true, "token": true, "tokens": true,
	"apikey": true, "accesskey": true, "secretkey": true, "privatekey": true,
	"sshkey": true, "masterkey": true, "encryptionkey": true, "signingkey": true,
	"credential": true, "credentials": true, "dockerconfigjson": true, "dockercfg": true,
	"senha": true, "senhas": true,
}

// sufixosSensiveis pegam palavras coladas: dbpassword, clientsecret, accesstoken,
// OPENAIAPIKEY, AWSSECRETKEY, gitlabprivatekey, dbsenha. "key" sozinho não entra.
var sufixosSensiveis = []string{
	"password", "passwd", "passphrase", "secret", "secrets", "token", "tokens", "senha",
	"credential", "credentials", "apikey", "apikeys", "secretkey", "accesskey",
	"privatekey", "sshkey", "signingkey", "encryptionkey", "masterkey", "authkey",
}

// referencias, como última palavra, fazem do nome a referência a um segredo, e não ele.
var referencias = map[string]bool{"name": true, "names": true, "ref": true, "refs": true}

// nomeSensivel diz se o nome guarda segredo e se é um cabeçalho Authorization.
// "key" sozinho não é sensível (key: app em matchExpressions, tolerations e
// secretKeyRef); em composto é (SSH_KEY, KEY_ID, apiKey, tls.key).
func nomeSensivel(nome string) (sensivel, autorizacao bool) {
	segmentos := strings.FieldsFunc(nome, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	var palavras []string
	for _, s := range segmentos {
		palavras = append(palavras, separarCamelCase(s)...)
	}
	temKey := false
	for _, p := range palavras {
		if p == "key" || p == "keys" {
			temKey = true
		}
		if p == "authorization" {
			autorizacao = true
		}
	}
	if autorizacao {
		return true, true
	}
	// Nome e referência não são o segredo (secretName, SecretName do describe,
	// secretKeyRef, secret_ref): quem diagnostica precisa saber qual Secret falta.
	if n := len(palavras); n > 1 && referencias[palavras[n-1]] {
		return false, false
	}
	if temKey && len(palavras) > 1 {
		return true, false
	}
	// "auth" só como última palavra: auth, basic_auth, basicAuth (authMethod não).
	if n := len(palavras); n > 0 && palavras[n-1] == "auth" {
		return true, false
	}
	candidatas := append([]string{}, palavras...)
	for _, s := range segmentos {
		candidatas = append(candidatas, strings.ToLower(s))
	}
	for _, p := range candidatas {
		if palavrasSensiveis[p] {
			return true, false
		}
		for _, suf := range sufixosSensiveis {
			if strings.HasSuffix(p, suf) {
				return true, false
			}
		}
		if strings.HasPrefix(p, "password") || strings.HasPrefix(p, "passwd") {
			return true, false
		}
	}
	return false, false
}

// separarCamelCase quebra camelCase/PascalCase em palavras minúsculas (ASCII):
// SecretAccessKey -> secret, access, key; APIKey -> api, key.
func separarCamelCase(s string) []string {
	var palavras []string
	ini := 0
	for i := 1; i < len(s); i++ {
		c, ant := s[i], s[i-1]
		if !maiuscula(c) {
			continue
		}
		if minuscula(ant) || digito(ant) || (maiuscula(ant) && i+1 < len(s) && minuscula(s[i+1])) {
			palavras = append(palavras, strings.ToLower(s[ini:i]))
			ini = i
		}
	}
	if ini < len(s) {
		palavras = append(palavras, strings.ToLower(s[ini:]))
	}
	return palavras
}

// intervalo é um trecho [ini, fim) do texto que vira Marca.
type intervalo struct{ ini, fim int }

// intervalosPares devolve os trechos do valor de cada nome sensível. Cada nome é
// lido no texto original, mesmo que esteja dentro do valor de outro (token:
// password: x): a união dos trechos redige os dois.
func intervalosPares(texto string) []intervalo {
	var ivs []intervalo
	for _, loc := range nomes.FindAllStringSubmatchIndex(texto, -1) {
		nome := texto[loc[4]:loc[5]]
		if loc[8] >= 0 && !strings.HasPrefix(nome, "--") {
			continue // nome seguido só de espaço: vale só para flags
		}
		sensivel, autorizacao := nomeSensivel(nome)
		if !sensivel {
			continue
		}
		ivs = append(ivs, redigirValor(texto, loc[1], coluna(texto, loc[0]), autorizacao)...)
	}
	return ivs
}

// redigirValor devolve, em ordem, os trechos do valor que começa em i (logo depois
// do separador). col é a coluna do nome, usada para achar o valor YAML nas linhas
// de baixo.
func redigirValor(texto string, i, col int, autorizacao bool) []intervalo {
	fimLinha := i + indiceByte(texto[i:], '\n')
	linha := strings.TrimRight(texto[i:fimLinha], " \t\r")
	if linha == "" {
		if ivs, ok := valorDescribe(texto, fimLinha); ok {
			return ivs
		}
		return valorAbaixo(texto, fimLinha, col)
	}
	if indicadorBloco.MatchString(linha) {
		return bloco(texto, fimLinha, col)
	}
	if tok := texto[i : i+indiceEspaco(texto[i:])]; !marcacao.MatchString(tok) && inofensivo.MatchString(tok) {
		return nil
	}
	switch {
	case strings.HasPrefix(linha, Marca):
		// Já redigido (Redigir é idempotente): a marca não é lida como lista.
		return []intervalo{{i, i + indiceEspaco(texto[i:])}}
	case linha[0] == '#':
		// Comentário YAML ou senha começando em "#": redige o token e olha a linha de baixo.
		ivs := []intervalo{{i, i + indiceEspaco(texto[i:])}}
		return append(ivs, valorAbaixo(texto, fimLinha, col)...)
	case strings.HasPrefix(linha, `\"`):
		return entreAspas(texto, i, 2, fimLinha, fimAspasEscapadas)
	case linha[0] == '"':
		return entreAspas(texto, i, 1, fimLinha, fimAspasDuplas)
	case linha[0] == '\'':
		return entreAspas(texto, i, 1, fimLinha, fimAspasSimples)
	case autorizacao:
		return []intervalo{valorAutorizacao(texto, i, i+len(linha))}
	case linha[0] == '[' || linha[0] == '{':
		return colecao(texto, i, fimLinha)
	}
	tok := i + indiceEspaco(texto[i:])
	if marcacao.MatchString(texto[i:tok]) {
		// **Password:** hunter2: o "**" é marcação de markdown e o valor vem depois.
		// Redige o que vem depois (na linha ou embaixo) e a marcação fica.
		resto := strings.TrimLeft(texto[tok:fimLinha], " \t")
		if strings.TrimRight(resto, " \t\r") == "" {
			return valorAbaixo(texto, fimLinha, col)
		}
		j := fimLinha - len(resto)
		return []intervalo{{j, j + indiceEspaco(resto)}}
	}
	return []intervalo{{i, tok}}
}

// inofensivo casa um valor que não pode ser segredo: booleano, nulo ou o curinga "*",
// sozinho ou numa lista, com aspas e com a pontuação ou a marcação de markdown que a
// prosa põe depois (automountServiceAccountToken: false`:**, secrets: ["*"]`).
var inofensivo = regexp.MustCompile("^(?:[\"']?(?i:true|false|null|~)[\"']?|\\[[\"']?\\*[\"']?\\]|[\"']?\\*[\"']?)[`*_:,.;)]*$")

// marcacao casa um valor que é só marcação de markdown (**, __, `).
var marcacao = regexp.MustCompile("^[*_`]{1,3}$")

// colecao redige o miolo de uma lista ou mapa de fluxo ([a, b] ou {a: b}) até o
// fechamento que casa com a abertura em i, pulando o que está entre aspas. O
// fechamento pode estar até linhasAspas linhas abaixo (JSON indentado); o miolo é
// redigido linha a linha. Sem fechamento, redige até o fim da linha. Se o que vem
// colado depois do fechamento não for limpo (restoLimpo), o fechamento estava
// dentro do valor ([abc]def) e o token inteiro é redigido.
func colecao(texto string, i, fimLinha int) []intervalo {
	limite := fimLinha
	for n := 0; n < linhasAspas && limite < len(texto); n++ {
		limite += 1 + indiceByte(texto[limite+1:], '\n')
	}
	f := fechamento(texto[i:limite])
	if f < 0 {
		return []intervalo{{i, i + len(strings.TrimRight(texto[i:fimLinha], " \t\r"))}}
	}
	fecha := i + f
	depois := fecha + 1
	fimToken := depois + indiceEspaco(texto[depois:])
	if !restoLimpo.MatchString(texto[depois:fimToken]) && fecha < fimLinha {
		return []intervalo{{i, fimToken}}
	}
	var ivs []intervalo
	ini := i + 1
	for ini < fecha {
		fim := ini + indiceByte(texto[ini:fecha], '\n')
		seg := texto[ini:fim]
		a := len(seg) - len(strings.TrimLeft(seg, " \t\r"))
		b := len(strings.TrimRight(seg, " \t\r"))
		if a < b {
			ivs = append(ivs, intervalo{ini + a, ini + b})
		}
		ini = fim + 1
	}
	if !restoLimpo.MatchString(texto[depois:fimToken]) {
		ivs = append(ivs, intervalo{fecha, fimToken})
	}
	return ivs
}

// fechamento devolve o índice do "]" ou "}" que fecha a abertura em s[0], com
// aninhamento e pulando strings entre aspas; -1 se não fecha ou se um fechamento
// não casa com a abertura ([a}).
func fechamento(s string) int {
	var pilha []byte
	var aspa byte
	for j := 0; j < len(s); j++ {
		c := s[j]
		switch {
		case aspa != 0:
			if c == aspa && barrasAntes(s, j)%2 == 0 {
				aspa = 0
			}
		case c == '"' || c == '\'':
			aspa = c
		case c == '[':
			pilha = append(pilha, ']')
		case c == '{':
			pilha = append(pilha, '}')
		case c == ']' || c == '}':
			if len(pilha) == 0 || pilha[len(pilha)-1] != c {
				return -1
			}
			pilha = pilha[:len(pilha)-1]
			if len(pilha) == 0 {
				return j
			}
		}
	}
	return -1
}

// indicadorBloco casa o indicador de escalar em bloco do YAML (|, >, |-, >+, |2).
var indicadorBloco = regexp.MustCompile(`^[|>][0-9+\-]*(?:[ \t]+#.*)?$`)

// pareceChave diz se a linha (sem indentação) é uma entrada de mapa YAML.
var pareceChave = regexp.MustCompile(`^["']?[A-Za-z0-9_.\-/]+["']?[ \t]*:(?:[ \t]|$)`)

// linhasAspas é quantas linhas abaixo a aspa de fechamento é procurada (o YAML
// aceita escalar entre aspas em várias linhas).
const linhasAspas = 10

// entreAspas redige o conteúdo entre as aspas que abrem em i (abre é o tamanho do
// delimitador). A aspa de fechamento pode estar até linhasAspas linhas abaixo; se
// não houver, redige até o fim da linha e o primeiro token da linha de baixo. O
// resto do token (até o espaço) depois da aspa de fechamento só fica se for limpo:
// nada, fechamentos (",", ";", ")", "]", "}") ou o próximo par de um JSON
// (,"user":"ana"}), com aspas em par. Senão a aspa que parecia fechar estava dentro
// do valor ("hun"ter2, "a",b") e o token inteiro é redigido, aspas inclusive.
func entreAspas(texto string, i, abre, fimLinha int, fecha func(string) int) []intervalo {
	limite := fimLinha
	for n := 0; n < linhasAspas && limite < len(texto); n++ {
		limite += 1 + indiceByte(texto[limite+1:], '\n')
	}
	f := fecha(texto[i+abre : limite])
	if f < 0 {
		ivs := []intervalo{{i + abre, i + len(strings.TrimRight(texto[i:fimLinha], " \t\r"))}}
		return append(ivs, primeiroTokenAbaixo(texto, fimLinha)...)
	}
	ini, fim := i+abre, i+abre+f
	depois := fim + abre
	fimToken := depois + indiceEspaco(texto[depois:])
	resto := texto[depois:fimToken]
	aspa := texto[depois-1 : depois]
	if strings.Count(resto, aspa)%2 == 1 || !restoLimpo.MatchString(resto) {
		return []intervalo{{i, fimToken}}
	}
	return []intervalo{{ini, fim}}
}

// primeiroTokenAbaixo devolve o primeiro token da próxima linha não vazia.
func primeiroTokenAbaixo(texto string, nl int) []intervalo {
	for ini := nl + 1; ini < len(texto); {
		fim := ini + indiceByte(texto[ini:], '\n')
		ind, cont := indentacao(texto[ini:fim])
		if c := strings.TrimRight(cont, " \t\r"); c != "" {
			return []intervalo{{ini + ind, ini + ind + indiceEspaco(c)}}
		}
		ini = fim + 1
	}
	return nil
}

// restoLimpo casa o que pode vir colado depois de um valor entre aspas sem ser
// parte dele: fechamentos e, depois de uma vírgula, o próximo nome entre aspas
// com separador (JSON). Nome sem aspas (,user=x) não conta: o token é redigido.
var restoLimpo = regexp.MustCompile(`^[,;)\]}]*(?:,\\?["'][A-Za-z0-9_.\-]+\\?["'][=:].*)?$`)

// fimAspasDuplas acha a aspa dupla de fechamento (precedida por número par de "\").
func fimAspasDuplas(s string) int {
	for j := 0; j < len(s); j++ {
		if s[j] == '"' && barrasAntes(s, j)%2 == 0 {
			return j
		}
	}
	return -1
}

// fimAspasEscapadas acha o \" de fechamento de um valor dentro de string JSON
// (\"valor\"). O \" de fechamento tem 4k+1 barras antes da aspa; uma aspa escapada
// dentro do valor (\\\") tem 4k+3. Devolve o índice da barra.
func fimAspasEscapadas(s string) int {
	for j := 0; j < len(s); j++ {
		if s[j] == '"' && barrasAntes(s, j)%4 == 1 {
			return j - 1
		}
	}
	return -1
}

// fimAspasSimples acha a aspa simples de fechamento; duas aspas simples seguidas
// (escape do YAML) são uma aspa dentro do valor.
func fimAspasSimples(s string) int {
	for j := 0; j < len(s); j++ {
		if s[j] != '\'' {
			continue
		}
		if j+1 < len(s) && s[j+1] == '\'' {
			j++
			continue
		}
		return j
	}
	return -1
}

// esquemasToken são os esquemas de Authorization cujo segredo é um token só.
var esquemasToken = map[string]bool{"bearer": true, "basic": true, "token": true}

// esquemasLinha são os esquemas conhecidos cujo segredo ocupa o resto da linha
// (parâmetros separados por vírgula). O nome do esquema fica.
var esquemasLinha = map[string]bool{
	"digest": true, "negotiate": true, "ntlm": true, "aws4-hmac-sha256": true,
	"oauth": true, "hoba": true, "mutual": true, "scram-sha-1": true,
	"scram-sha-256": true, "vapid": true, "hawk": true, "dpop": true, "gnap": true,
}

// esquemaAuth casa "Esquema valor" no começo do valor de Authorization; o valor
// pode estar na linha de baixo ("Bearer" sozinho no fim da linha).
var esquemaAuth = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9\-]*)(\s+)\S`)

// aberturas são os delimitadores que podem vir antes do esquema: lista do Go
// (map[Authorization:[Bearer x]]), JSON e Python (["Bearer x"], ['Bearer x']),
// (Bearer x), <Bearer x> e aspas escapadas.
const aberturas = "[({<\"'\\ \t"

// fechamentos são os delimitadores que podem vir colados depois do token.
const fechamentos = "])}>\"'\\,;"

// valorAutorizacao mantém o esquema (Bearer, Basic, Digest) e redige o resto. Pula
// os delimitadores de abertura antes do esquema; Bearer, Basic e Token redigem o
// token até o espaço (os fechamentos colados ficam); os esquemas de esquemasLinha
// redigem o resto da linha. Sem esquema reconhecível (valor solto, esquema
// desconhecido, "[Bearer]"), redige do começo do valor até o fim da linha.
func valorAutorizacao(texto string, i, fimLinha int) intervalo {
	tudo := intervalo{i, fimLinha}
	ini := i
	for ini < fimLinha && strings.IndexByte(aberturas, texto[ini]) >= 0 {
		ini++
	}
	m := esquemaAuth.FindStringSubmatchIndex(texto[ini:])
	if m == nil {
		return tudo
	}
	esquema := strings.ToLower(texto[ini+m[2] : ini+m[3]])
	tok := ini + m[1] - 1
	switch {
	case esquemasToken[esquema]:
		fim := tok + indiceEspaco(texto[tok:])
		if strings.HasPrefix(texto[tok:], Marca) {
			fim = tok + len(Marca) // já redigido
		} else if ini > i {
			// Só tira fechamentos colados quando houve abertura.
			if f := tok + len(strings.TrimRight(texto[tok:fim], fechamentos)); f > tok {
				fim = f
			}
		}
		return intervalo{tok, fim}
	case esquemasLinha[esquema] && tok < fimLinha:
		return intervalo{tok, fimLinha}
	case tok > fimLinha:
		// Valor sozinho na linha (o "esquema" é o próprio valor).
		return intervalo{i, fimLinha}
	}
	return tudo
}

// valorAbaixo trata o nome sem valor na linha (YAML "password:" e o valor embaixo).
// Linha mais indentada que o nome: escalar continuado ou lista de escalares é
// redigido; mapa aninhado fica (os nomes dele são olhados um a um). Linha não
// indentada que não é chave, lista nem cerca de código: redige o primeiro token,
// como o sanitizeContent antigo fazia.
func valorAbaixo(texto string, nl, col int) []intervalo {
	var ivs []intervalo
	primeira := true
	for ini := nl + 1; nl < len(texto) && ini <= len(texto); {
		fim := ini + indiceByte(texto[ini:], '\n')
		ind, cont := indentacao(texto[ini:fim])
		conteudo := strings.TrimRight(cont, " \t\r")
		if conteudo == "" {
			if fim >= len(texto) {
				break
			}
			ini = fim + 1
			continue
		}
		cIni := ini + ind
		cFim := cIni + len(conteudo)
		lista := conteudo == "-" || strings.HasPrefix(conteudo, "- ")
		switch {
		case pareceChave.MatchString(conteudo):
			return ivs
		case lista && ind >= col:
			item := strings.TrimLeft(conteudo[1:], " \t")
			if item == "" || pareceChave.MatchString(item) {
				return ivs
			}
			ivs = append(ivs, intervalo{cFim - len(item), cFim})
		case ind > col:
			ivs = append(ivs, intervalo{cIni, cFim})
		case primeira && !lista && !strings.HasPrefix(conteudo, "```") && !strings.HasPrefix(conteudo, "~~~"):
			return []intervalo{{cIni, cIni + indiceEspaco(conteudo)}}
		default:
			return ivs
		}
		primeira = false
		if fim >= len(texto) {
			break
		}
		ini = fim + 1
	}
	return ivs
}

// separadorDescribe é a linha que o kubectl describe de um ConfigMap põe entre a
// chave e o valor ("DB_PASSWORD:", "----", o valor, uma linha em branco).
const separadorDescribe = "----"

// valorDescribe trata o valor de um ConfigMap no kubectl describe: se a linha de
// baixo de nl é o separador, redige de uma vez tudo o que vem depois dele até a
// próxima chave (linha "X:" seguida de separador), o BinaryData, o Events ou o fim
// do texto, inclusive as linhas em branco do meio. ok diz se era esse formato.
func valorDescribe(texto string, nl int) (ivs []intervalo, ok bool) {
	if nl >= len(texto) {
		return nil, false
	}
	ini := nl + 1
	fim := ini + indiceByte(texto[ini:], '\n')
	if strings.TrimRight(texto[ini:fim], " \t\r") != separadorDescribe {
		return nil, false
	}
	primeiro, ultimo := -1, -1
	for fim < len(texto) {
		ini = fim + 1
		fim = ini + indiceByte(texto[ini:], '\n')
		linha := strings.TrimRight(texto[ini:fim], " \t\r")
		if fimDoDescribe(texto, linha, fim) {
			break
		}
		if strings.TrimSpace(linha) != "" {
			if primeiro < 0 {
				primeiro = ini
			}
			ultimo = ini + len(linha)
		}
	}
	if primeiro < 0 {
		return nil, true
	}
	return []intervalo{{primeiro, ultimo}}, true
}

// fimDoDescribe diz se a linha (que termina em fim) fecha o valor de um ConfigMap
// no kubectl describe: a próxima chave, o BinaryData ou o Events.
func fimDoDescribe(texto, linha string, fim int) bool {
	if linha == "BinaryData" || strings.HasPrefix(linha, "Events:") {
		return true
	}
	if linha == "" || espaco(linha[0]) || !strings.HasSuffix(linha, ":") || fim >= len(texto) {
		return false
	}
	prox := texto[fim+1:]
	return strings.TrimRight(prox[:indiceByte(prox, '\n')], " \t\r") == separadorDescribe
}

// bloco redige as linhas de um escalar em bloco YAML (| ou >): todas as linhas mais
// indentadas que o nome, com linhas em branco no meio.
func bloco(texto string, nl, col int) []intervalo {
	var ivs []intervalo
	for ini := nl + 1; nl < len(texto) && ini <= len(texto); {
		fim := ini + indiceByte(texto[ini:], '\n')
		ind, cont := indentacao(texto[ini:fim])
		conteudo := strings.TrimRight(cont, " \t\r")
		if conteudo != "" {
			if ind <= col {
				break
			}
			ivs = append(ivs, intervalo{ini + ind, ini + ind + len(conteudo)})
		}
		if fim >= len(texto) {
			break
		}
		ini = fim + 1
	}
	return ivs
}

// ---------------------------------------------------------------------------
// Env do Kubernetes (name e value)

// chaveEnv casa as chaves name e value de um item de env, em YAML, JSON (inclusive
// dentro de string, \"name\":) e mapa de fluxo. O separador é só ":".
var chaveEnv = regexp.MustCompile(`(\\?["']?)(name|value)(\\?["']?)[ \t]*:[ \t]*`)

// nomeDoEnv lê o valor da chave name (o nome da variável), com ou sem aspas.
var nomeDoEnv = regexp.MustCompile(`^\\?["']?([A-Za-z0-9_.\-]+)`)

// chaveDeEnv é uma chave name ou value achada no texto: ini e fim delimitam a chave
// com o separador, col é a coluna dela e nomeSensivel diz, numa chave name, se a
// variável guarda segredo.
type chaveDeEnv struct {
	ini, fim, col int
	ehNome        bool
	nomeSensivel  bool
}

// intervalosEnv redige o value dos itens de env cujo name é sensível
// ("- name: DB_PASSWORD" e "  value: x"), com o value antes ou depois do name e
// outras chaves no meio. O value é o mais próximo do name, para frente ou para
// trás, sem outro name no caminho e no mesmo objeto (mesmoItem); assim o value de
// um item nunca é lido como o de outro, e o name de um secretKeyRef aninhado
// encerra a busca.
func intervalosEnv(texto string) []intervalo {
	var chaves []chaveDeEnv
	for _, loc := range chaveEnv.FindAllStringSubmatchIndex(texto, -1) {
		if loc[0] > 0 && strings.IndexByte(caracteresDeNome, texto[loc[0]-1]) >= 0 {
			continue // parte de outra chave (hostname:, file.name:)
		}
		c := chaveDeEnv{ini: loc[0], fim: loc[1], col: coluna(texto, loc[0]), ehNome: texto[loc[4]:loc[5]] == "name"}
		if c.ehNome {
			if m := nomeDoEnv.FindStringSubmatch(texto[loc[1]:]); m != nil {
				c.nomeSensivel, _ = nomeSensivel(m[1])
			}
		}
		chaves = append(chaves, c)
	}
	var ivs []intervalo
	for i, c := range chaves {
		if !c.nomeSensivel {
			continue
		}
		for _, passo := range []int{1, -1} {
			j := i + passo
			if j < 0 || j >= len(chaves) || chaves[j].ehNome {
				continue
			}
			a, b := c, chaves[j]
			if passo < 0 {
				a, b = b, c
			}
			if mesmoItem(texto, a, b) {
				v := chaves[j]
				ivs = append(ivs, redigirValor(texto, v.fim, v.col, false)...)
			}
		}
	}
	return ivs
}

// caracteresDeNome são os que podem vir colados antes de uma chave que não é a
// name ou a value (hostname, file.name).
const caracteresDeNome = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_.-"

// mesmoItem diz se as chaves a e b (a antes de b) são do mesmo item de env. Na
// mesma linha (JSON compacto, mapa de fluxo), não pode haver "{" nem "}" entre
// elas. Em linhas diferentes (YAML, JSON indentado), as duas estão na mesma coluna,
// nenhuma linha do meio fica à esquerda dela (fim do objeto, item novo da lista) e
// a linha de b começa nela: "- value:" abaixo de um name é outro item.
func mesmoItem(texto string, a, b chaveDeEnv) bool {
	entre := texto[a.fim:b.ini]
	if !strings.Contains(entre, "\n") {
		return !strings.ContainsAny(entre, "{}")
	}
	if a.col != b.col {
		return false
	}
	linhas := strings.Split(texto[a.fim:b.ini+1], "\n")[1:]
	for k, l := range linhas {
		ind, cont := indentacao(l)
		if strings.TrimRight(cont, " \t\r") == "" && k < len(linhas)-1 {
			continue
		}
		if ind < b.col || (k == len(linhas)-1 && ind != b.col) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Bearer e Basic soltos

// esquemaSolto casa "Bearer x" e "Basic x" fora de um par Authorization, inclusive
// com o token na linha de baixo (o \s+ do sanitizeContent antigo).
var esquemaSolto = regexp.MustCompile(`(?i)\b(bearer|basic)(\s+)(\S+)`)

// intervalosEsquemas devolve o token depois de Bearer/Basic só quando ele tem cara
// de token, para não redigir prosa ("Basic authentication is enabled"). Quando o
// token é recusado, a busca recomeça nele, e não depois dele: em "Bearer Basic x",
// o "Basic" recusado como token ainda é esquema. Um nome de par na linha de baixo
// (token_type: Bearer / access_token: x) pode ser redigido como token: é exagero
// aceito, e o valor dele sai pelos pares.
func intervalosEsquemas(texto string) []intervalo {
	var ivs []intervalo
	busca := 0
	for busca < len(texto) {
		loc := esquemaSolto.FindStringSubmatchIndex(texto[busca:])
		if loc == nil {
			break
		}
		for k := range loc {
			loc[k] += busca
		}
		ini, fim := loc[6], loc[7]
		tok := strings.TrimRight(texto[ini:fim], pontuacaoFinal)
		if tok == "" {
			tok = texto[ini:fim]
		}
		// Pontuação de prosa e fechamentos colados ("Bearer x1)", "o bearer token.")
		// ficam de fora da avaliação e do trecho redigido.
		if strings.HasPrefix(texto[ini:fim], Marca) ||
			!pareceToken(strings.ToLower(texto[loc[2]:loc[3]]), tok) {
			busca = ini
			continue
		}
		ivs = append(ivs, intervalo{ini, ini + len(tok)})
		busca = fim
	}
	return ivs
}

// pontuacaoFinal é o que pode vir colado no fim de um token sem ser parte dele.
const pontuacaoFinal = ".,;:!?])}>\"'\\"

// pareceToken diz se o que vem depois de Bearer ou Basic soltos é credencial.
// Bearer: 16+ caracteres, ou qualquer caractere que não seja letra (dígito, "-",
// "_", "~", "+", "/", "=", "&"...), ou maiúscula no meio de minúsculas (aBcD). Fica
// só palavra de letras em minúsculas, Capitalizada ou em maiúsculas com menos de
// 16 letras ("bearer token", "Bearer authentication"). Basic: 8+ caracteres com
// dígito, "+", "/", "=" ou maiúscula no meio de minúsculas (base64 de
// usuario:senha); "basic configuration" fica.
func pareceToken(esquema, tok string) bool {
	if strings.HasPrefix(tok, Marca) {
		return false
	}
	soLetras, temMinuscula, maiusculaNoMeio := true, false, false
	for i := 0; i < len(tok); i++ {
		c := tok[i]
		if !maiuscula(c) && !minuscula(c) && c < 0x80 {
			soLetras = false // bytes de UTF-8 contam como letra (prosa acentuada)
		}
		if minuscula(c) {
			temMinuscula = true
		}
		if i > 0 && maiuscula(c) {
			maiusculaNoMeio = true
		}
	}
	misturado := temMinuscula && maiusculaNoMeio
	if esquema == "bearer" {
		return len(tok) >= 16 || !soLetras || misturado
	}
	if len(tok) < 8 {
		return false
	}
	return strings.ContainsAny(tok, "0123456789+/=") || misturado
}

// ---------------------------------------------------------------------------
// Auxiliares ASCII

func maiuscula(c byte) bool { return c >= 'A' && c <= 'Z' }
func minuscula(c byte) bool { return c >= 'a' && c <= 'z' }
func digito(c byte) bool    { return c >= '0' && c <= '9' }
func espaco(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

func soDigitos(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !digito(s[i]) {
			return false
		}
	}
	return true
}

// indiceEspaco devolve onde começa o primeiro espaço de s (ou len(s)).
func indiceEspaco(s string) int {
	for i := 0; i < len(s); i++ {
		if espaco(s[i]) {
			return i
		}
	}
	return len(s)
}

// indiceByte é strings.IndexByte com len(s) quando não acha.
func indiceByte(s string, b byte) int {
	if i := strings.IndexByte(s, b); i >= 0 {
		return i
	}
	return len(s)
}

// barrasAntes conta as "\" seguidas que vêm antes de s[j].
func barrasAntes(s string, j int) int {
	n := 0
	for k := j - 1; k >= 0 && s[k] == '\\'; k-- {
		n++
	}
	return n
}

// coluna devolve a coluna (em bytes) de i na linha dele.
func coluna(texto string, i int) int {
	return i - (strings.LastIndexByte(texto[:i], '\n') + 1)
}

// indentacao devolve quantos espaços e tabs abrem a linha e o resto dela.
func indentacao(linha string) (int, string) {
	n := 0
	for n < len(linha) && (linha[n] == ' ' || linha[n] == '\t') {
		n++
	}
	return n, linha[n:]
}
