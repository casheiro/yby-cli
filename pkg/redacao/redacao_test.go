package redacao

import (
	"strings"
	"testing"
)

// casoSegredo: a saída não pode ter nenhum trecho de naoDeve e tem de manter cada
// trecho de deve (o contexto em volta do segredo).
type casoSegredo struct {
	nome, entrada string
	naoDeve, deve []string
}

var casosSegredo = []casoSegredo{
	// Esquema como valor de um nome sensível que não é Authorization.
	{"bearer em x-auth-token", "X-Auth-Token: Bearer abcdef1234567890xyz", []string{"abcdef1234567890xyz"}, []string{"X-Auth-Token: "}},
	{"bearer em token", "token: Bearer abcdef1234567890xyz", []string{"abcdef1234567890xyz"}, nil},
	{"bearer no curl", "curl -H 'X-Api-Key: Bearer abcdefghijklmnop123' https://x", []string{"abcdefghijklmnop123"}, []string{"https://x"}},
	{"bearer em api_token", "API_TOKEN=Bearer s3cr3tvalue", []string{"s3cr3tvalue"}, nil},
	{"basic em basic_auth", "basic_auth: Basic dXNlcjpodW50ZXIy", []string{"dXNlcjpodW50ZXIy"}, nil},

	// Marcação de markdown em volta do nome, como a IA escreve.
	{"md negrito no nome", "- **DB_PASSWORD**: hunter2", []string{"hunter2"}, []string{"**DB_PASSWORD**"}},
	{"md negrito password", "- **Password**: hunter2", []string{"hunter2"}, nil},
	{"md itálico", "*password*: hunter2", []string{"hunter2"}, nil},
	{"md código", "- `DB_PASSWORD`: `hunter2`", []string{"hunter2"}, []string{"`DB_PASSWORD`"}},
	{"md código com igual", "`token` = abc123", []string{"abc123"}, nil},

	// O esquema solto e o par seguinte são lidos juntos: o valor sai mesmo quando o
	// nome do par vira token do Bearer (o nome também sai: exagero aceito).
	{"oauth em yaml", "stringData:\n  token_type: Bearer\n  access_token: s3cr3tOpaqueValue123", []string{"s3cr3tOpaqueValue123"}, []string{"token_type: "}},
	{"bearer e par na linha de baixo", "bearer\ndb_password: hunter2xyz", []string{"hunter2xyz"}, nil},
	{"bearer antes de par", "Bearer DB_PASSWORD: hunter2xyz", []string{"hunter2xyz"}, nil},
	{"basic antes de par", "Basic dbPassword: hunter2xyz", []string{"hunter2xyz"}, nil},
	{"bearer basic", "token=Bearer Basic WwX580JW", []string{"WwX580JW"}, nil},

	// Pares encadeados: o valor do primeiro nome é outro nome sensível.
	{"pares encadeados", "token: password: hunter2xyz", []string{"hunter2xyz"}, nil},
	{"pares encadeados em markdown", "**Credentials:** password: hunter2xyz", []string{"hunter2xyz"}, nil},
	{"pares encadeados com espaço", "token: password = hunter2xyz", []string{"hunter2xyz"}, nil},

	// Segredo com cara de nome sensível e separador é redigido como valor (e, se for
	// lido também como nome, a linha de baixo sai como valor dele: exagero aceito).
	{"segredo terminado em dois-pontos", "password: mytoken:", []string{"mytoken"}, nil},
	{"segredo terminado em igual", "password: SuperSecret=", []string{"SuperSecret"}, nil},
	{"segredo com padding", "password: SuperSecret==", []string{"SuperSecret"}, nil},
	{"segredo com nome e texto depois", "password: mytoken: abc", []string{"mytoken", "abc"}, nil},
	{"segredo com linha de baixo", "password: mytoken:\nfim", []string{"mytoken"}, nil},
	{"nome com aspas embaixo do bearer", "token_type: Bearer\n\"access_token\": abc123def456ghi", []string{"abc123def456ghi"}, nil},
	{"par de dentro com valor embaixo", "credentials: password:\n  hunter2xyz", []string{"hunter2xyz"}, nil},
	{"env terminado em igual", "DB_PASSWORD=MinhaSenha=", []string{"MinhaSenha"}, nil},
	{"bearer terminado em igual", "Bearer abcdefKey=", []string{"abcdefKey"}, nil},

	// Token na linha de baixo do Bearer.
	{"bearer com token embaixo", "X-Auth-Token: Bearer\n  abc123def456ghi", []string{"abc123def456ghi"}, nil},
	{"bearer em prosa com token embaixo", "usei bearer\nabc123def456ghi", []string{"abc123def456ghi"}, nil},

	// Separador seguido de \v ou \f, que o sanitizeContent antigo redigia.
	{"tab vertical", "password=\vhunter2", []string{"hunter2"}, nil},
	{"form feed", "password=\fhunter2", []string{"hunter2"}, nil},

	// Nomes em PT-BR.
	{"senha", "senha: hunter2", []string{"hunter2"}, []string{"senha: "}},
	{"db_senha", "DB_SENHA=hunter2", []string{"hunter2"}, nil},

	// Tokens com formato próprio.
	{"github", "tok ghp_" + strings.Repeat("a", 36), []string{"ghp_"}, []string{"tok "}},
	{"openai", "k sk-" + strings.Repeat("A", 24), []string{"sk-AAAA"}, nil},
	{"openai proj solto", "chave sk-proj-abcDEF123ghiJKL456mnoPQR789stuVWXyz", []string{"sk-proj", "abcDEF123", "VWXyz"}, []string{"chave "}},
	{"anthropic solto", "chave sk-ant-api03-abcDEF123ghiJKL456mnoPQR789stuVWXyz-_AAAA", []string{"sk-ant", "abcDEF123", "_AAAA"}, []string{"chave "}},
	{"openai svcacct", "sk-svcacct-abcDEF123ghiJKL456mnoPQR789", []string{"abcDEF123"}, nil},
	{"stripe", "sk_live_abcdefghijklmnop1234", []string{"abcdefghij"}, nil},
	{"aws access", "id AKIAABCDEFGHIJKLMNOP", []string{"AKIAABCDEFGHIJKLMNOP"}, nil},
	{"slack", "xoxb-123-abc-def", []string{"xoxb-123"}, nil},
	{"jwt", "t eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.abc_DEF-123 fim", []string{"eyJhbGci", "abc_DEF"}, []string{" fim"}},

	// PEM.
	{"pem", "a\n-----BEGIN RSA PRIVATE KEY-----\nMIIEow\nabc\n-----END RSA PRIVATE KEY-----\nb", []string{"MIIEow", "abc"}, []string{"a\n", "\nb"}},
	{"pem truncado", "-----BEGIN PRIVATE KEY-----\nMIIEow", []string{"MIIEow"}, nil},
	{"pem openssh", "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaA\n-----END OPENSSH PRIVATE KEY-----", []string{"b3BlbnNzaA"}, nil},
	{"pem pgp", "-----BEGIN PGP PRIVATE KEY BLOCK-----\n\nlQOYBF\n=abcd\n-----END PGP PRIVATE KEY BLOCK-----\nfim", []string{"lQOYBF", "=abcd"}, []string{"fim"}},

	// Credencial em URL.
	{"url", "git clone https://user:s3nh4@github.com/x/y.git", []string{"s3nh4"}, []string{"https://user:", "@github.com/x/y.git"}},
	{"url usuario vazio", "redis://:hunter2@redis:6379/0", []string{"hunter2"}, []string{"redis://:", "@redis:6379/0"}},
	{"url usuario vazio em var", "REDIS_URL=redis://:hunter2@redis:6379/0", []string{"hunter2"}, []string{"REDIS_URL=redis://", "@redis:6379/0"}},
	{"url senha com arroba", "https://user:p@ss@host/x", []string{"p@ss", "ss@"}, []string{"https://user:", "@host/x"}},
	{"url senha com varias arrobas", "postgres://u:a@b@c@db:5432/app?x=y@z", []string{"a@b@c"}, []string{"@db:5432/app?x=y@z"}},
	{"url senha com barra", "https://u:ab/cd@host", []string{"ab/cd", "cd@"}, []string{"@host"}},
	{"url mongodb srv", "mongodb+srv://app:s3nh4@cluster0.x.net/db", []string{"s3nh4"}, []string{"@cluster0.x.net/db"}},

	// Pares nome=valor.
	{"aws secret", "aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", []string{"wJalrXUtnFEMI"}, nil},
	{"aws secret yaml", "AWS_SECRET_ACCESS_KEY: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", []string{"wJalrXUtnFEMI"}, []string{"AWS_SECRET_ACCESS_KEY: "}},
	{"password", "password=hunter2", []string{"hunter2"}, []string{"password="}},
	{"password json", `{"password": "hunter2"}`, []string{"hunter2"}, []string{`{"password": "`, `"}`}},
	{"token", "GITHUB_TOKEN=abc123", []string{"abc123"}, nil},
	{"secret", "CLIENT_SECRET: xyz789", []string{"xyz789"}, nil},
	{"client_secret", "client_secret: xyz789", []string{"xyz789"}, nil},
	{"api key", "api-key: k9k9k9", []string{"k9k9k9"}, nil},
	{"api key junto", "APIKEY=k9k9k9", []string{"k9k9k9"}, nil},
	{"access_key", "access_key=k9k9k9", []string{"k9k9k9"}, nil},
	{"secret_key", "secret_key=k9k9k9", []string{"k9k9k9"}, nil},
	{"private_key", "private_key=k9k9k9", []string{"k9k9k9"}, nil},
	{"credentials", "credentials: k9k9k9", []string{"k9k9k9"}, nil},
	{"credential", "DB_CREDENTIAL=k9k9k9", []string{"k9k9k9"}, nil},
	{"passwd", "passwd=hunter2", []string{"hunter2"}, nil},
	{"pwd", "DB_PWD=hunter2", []string{"hunter2"}, nil},
	{"pass", "DB_PASS=hunter2", []string{"hunter2"}, nil},
	{"suffix key", "SSH_KEY=zzz111", []string{"zzz111"}, nil},
	{"prefix key", "KEY_ID=zzz111", []string{"zzz111"}, nil},
	{"tls.key", "tls.key: LS0tLS1CRUdJTi", []string{"LS0tLS1CRUdJTi"}, nil},
	{"camel", "accessToken=qqq222", []string{"qqq222"}, nil},
	{"camel apiKey", "apiKey: qqq222", []string{"qqq222"}, nil},
	{"camel passWord", "passWord=hunter2", []string{"hunter2"}, nil},
	{"pascal PassWord", "PassWord: hunter2", []string{"hunter2"}, nil},
	{"colado dbpassword", "dbpassword=hunter2", []string{"hunter2"}, nil},
	{"pascal aws", `"SecretAccessKey": "wJalrXUtnFEMI/K7MDENG"`, []string{"wJalrXUtnFEMI"}, nil},
	{"camel aws", "secretAccessKey: wJalrXUtnFEMI", []string{"wJalrXUtnFEMI"}, nil},
	// Compostos com key colados a outra palavra (o api[_-]?key e o secret[_-]?key
	// antigos casavam em qualquer posição).
	{"OPENAIAPIKEY", "OPENAIAPIKEY=k9secretvalue", []string{"k9secretvalue"}, []string{"OPENAIAPIKEY="}},
	{"GOOGLEAPIKEY", "GOOGLEAPIKEY: k9secretvalue", []string{"k9secretvalue"}, nil},
	{"TWILIOAPIKEY", "TWILIOAPIKEY=k9secret", []string{"k9secret"}, nil},
	{"myapikey", "myapikey=k9secret", []string{"k9secret"}, nil},
	{"AWSSECRETKEY", "AWSSECRETKEY=wJalrXUtnFEMI", []string{"wJalrXUtnFEMI"}, []string{"AWSSECRETKEY="}},
	{"awssecretkey", "awssecretkey: wJalrXUtnFEMI", []string{"wJalrXUtnFEMI"}, nil},
	{"stripesecretkey", "stripesecretkey=k9secret", []string{"k9secret"}, nil},
	{"awsaccesskey", "awsaccesskey=k9secret", []string{"k9secret"}, nil},
	{"gitlabprivatekey", "gitlabprivatekey: k9secret", []string{"k9secret"}, nil},
	{"separador na linha de baixo", "DB_PASSWORD\n=hunter2", []string{"hunter2"}, nil},
	{"auth docker", `{"auths":{"r.io":{"auth":"dXNlcjpwYXNz"}}}`, []string{"dXNlcjpwYXNz"}, nil},
	{"basic_auth", "basic_auth: admin:hunter2", []string{"hunter2"}, nil},
	{"dockerconfigjson", ".dockerconfigjson: eyJhdXRocyI6e30=", []string{"eyJhdXRocyI6e30"}, nil},
	{"go :=", `password := "hunter2"`, []string{"hunter2"}, nil},
	{"ruby =>", `:password => 'hunter2'`, []string{"hunter2"}, nil},
	{"export", "export PASSWORD=hunter2", []string{"hunter2"}, []string{"export PASSWORD="}},
	{"flag com espaço", "mysql --password hunter2 -h db", []string{"hunter2"}, []string{"--password ", " -h db"}},
	{"flag com =", "app --token=abc123 --debug", []string{"abc123"}, []string{" --debug"}},

	// Um par não sensível não engole o sensível à frente.
	{"from-literal", "kubectl create secret generic db --from-literal=password=hunter2", []string{"hunter2"}, []string{"--from-literal=password="}},
	{"encadeado e", "a=1&password=hunter2", []string{"hunter2"}, []string{"a=1&password="}},
	{"encadeado virgula", "user=admin,password=hunter2", []string{"hunter2"}, []string{"user=admin,password="}},
	{"query token", "curl https://api.x.io/v1?access_token=ABCDEFGHIJKLMNOPQRSTUVWX", []string{"ABCDEFGH"}, nil},
	{"url em var", "URL=https://h/?token=abcdefghij", []string{"abcdefghij"}, nil},
	{"set api_key", "helm install x --set=api_key=k9k9k9", []string{"k9k9k9"}, nil},
	{"prefixo msg", "msg: password=hunter2", []string{"hunter2"}, nil},

	// Valor sem aspas vai até o espaço, com &, aspas soltas e o resto do token.
	{"e comercial", "password=p@ss&w0rd", []string{"w0rd"}, nil},
	{"e comercial env", "DB_PASSWORD=abc&def", []string{"def"}, nil},
	{"e comercial yaml", "password: p@ss&w0rd", []string{"w0rd"}, nil},
	{"aspa simples no meio", "password=hun'ter2 fim", []string{"hun", "ter2"}, []string{" fim"}},
	{"aspa dupla no meio", `password=hun"ter2 fim`, []string{"hun", "ter2"}, []string{" fim"}},
	{"aspa no meio yaml", `password: abc"def'ghi`, []string{"abc", "def", "ghi"}, nil},

	// Valor entre aspas vai até a aspa de fechamento, com aspas escapadas.
	{"aspas simples", "PASSWORD='p@ss&w0rd'", []string{"w0rd"}, []string{"PASSWORD='", "'"}},
	{"aspas simples dobradas", "password: 'it''s-s3cret'", []string{"s-s3cret"}, nil},
	{"aspa escapada", `password: "hun\"ter2"`, []string{"hun", "ter2"}, []string{`password: "`}},
	{"barra escapada", `password: "hun\\" fim`, []string{"hun"}, []string{" fim"}},
	{"aspas coladas", `password="hun"ter2 fim`, []string{"hun", "ter2"}, []string{" fim"}},
	{"aspas sem fechar", `password: "hunter2 e mais`, []string{"hunter2", "mais"}, nil},
	{"aspas json", `{"password":"hunter2","user":"ana"}`, []string{"hunter2"}, []string{`"user":"ana"}`}},
	{"aspas escapadas", `password=\"hunter2\"`, []string{"hunter2"}, nil},
	{"aspas escapadas coladas", `password=\"hun\"ter2\" fim`, []string{"hun", "ter2"}, []string{" fim"}},
	{"helm aspas escapadas", `helm install x --set password=\"hunter2\"`, []string{"hunter2"}, nil},
	{"json escapado", `curl -d "{\"password\":\"hunter2\"}"`, []string{"hunter2"}, nil},
	{"json escapado com aspa", `{\"password\":\"hun\\\"ter2\",\"user\":\"ana\"}`, []string{"hun", "ter2"}, []string{`\"user\":\"ana\"}`}},
	{"api_key escapado", `{\"api_key\":\"k9k9k9k9\"}`, []string{"k9k9k9k9"}, nil},
	{"aspas em varias linhas", "password: \"hun\n  ter2\"\nuser: ana", []string{"hun", "ter2"}, []string{"user: ana"}},
	{"aspas que nunca fecham", "password: \"hun\nter2 fim", []string{"hun", "ter2"}, []string{" fim"}},
	// Aspa que parecia fechar estava dentro do valor (casos do teste aleatório).
	{"aspa e fechamento", `api_key: "ske")hs&pkrx%mhd:"z"`, []string{"ske", "pkrx", "mhd"}, nil},
	{"aspa e virgula sem aspas", `db_password: "t1pbbfk",fbfZ9:qddwx fim`, []string{"t1pbbfk", "fbfZ9", "qddwx"}, []string{" fim"}},
	{"aspa e numero impar", `password="sg)";7jvyxvy;9ddzgq" fim`, []string{"7jvyxvy", "9ddzgq"}, []string{" fim"}},
	{"aspas simples vazias e resto", `password=''w2gn5l'f}dpjsj-b;lh'`, []string{"w2gn5l", "dpjsj"}, nil},

	// Marcação de markdown antes do valor.
	{"markdown negrito", "**Password:** hunter2", []string{"hunter2"}, []string{"**Password:** "}},
	{"markdown negrito embaixo", "**Password:**\n  hunter2", []string{"hunter2"}, nil},

	// Lista e mapa de fluxo: o miolo até o fechamento que casa.
	{"lista json", `"tokens": ["abc123", "def456"]`, []string{"abc123", "def456"}, []string{`"tokens": [`}},
	{"lista json indentada", "\"secrets\": [\n  \"abc123\",\n  \"def456\"\n]\nfim", []string{"abc123", "def456"}, []string{"fim"}},
	{"lista go", "map[password:[hunter2] user:[ana]]", []string{"hunter2"}, []string{"user:[ana]]"}},
	{"mapa de fluxo", `secret: {"value": "abc def"}`, []string{"abc", "def"}, nil},
	{"lista com fechamento no meio", "password: [abc]def fim", []string{"abc", "def"}, []string{" fim"}},
	{"lista com fechamento trocado", "password: [}3rXFI#zs fim", []string{"3rXFI"}, nil},
	{"lista go com fechamento no meio", "map[secret_key:[H]HRRz+T]]", []string{"HRRz"}, nil},

	// Valor na linha de baixo (YAML).
	{"linha de baixo", "password:\n  hunter2\nuser: ana", []string{"hunter2"}, []string{"password:\n", "user: ana"}},
	{"linha de baixo sem indentar", "password:\nhunter2", []string{"hunter2"}, nil},
	{"linha de baixo com branco", "password:\n\n  hunter2", []string{"hunter2"}, nil},
	{"linha de baixo continuada", "password:\n  hun\n  ter2\nuser: ana", []string{"hun", "ter2"}, []string{"user: ana"}},
	{"comentario e linha de baixo", "password: # a senha\n  hunter2", []string{"hunter2"}, nil},
	{"bloco literal", "tls.key: |\n  MIIEow\n\n  abc\nnext: x", []string{"MIIEow", "abc"}, []string{"tls.key: |\n", "next: x"}},
	{"bloco dobrado", "password: >-\n  hun\n  ter2\nuser: ana", []string{"hun", "ter2"}, []string{"user: ana"}},
	{"bloco em item de lista", "- password: |\n    hunter2\n  user: ana", []string{"hunter2"}, []string{"  user: ana"}},
	{"lista de tokens", "tokens:\n  - abc123\n  - def456\nfim: 1", []string{"abc123", "def456"}, []string{"fim: 1"}},
	{"lista no mesmo nivel", "secrets:\n- abc123\n- def456", []string{"abc123", "def456"}, nil},
	{"mapa aninhado", "credentials:\n  username: ana\n  password: hunter2", []string{"hunter2"}, []string{"username: ana"}},
	{"item de lista", "- password: hunter2\n  user: ana", []string{"hunter2"}, []string{"user: ana"}},

	// Authorization e Bearer/Basic.
	{"bearer", "Authorization: Bearer abcdef1234567890", []string{"abcdef1234567890"}, []string{"Authorization: Bearer "}},
	{"bearer curto", "Authorization: Bearer abc", []string{"abc"}, nil},
	{"basic", "Authorization: Basic dXNlcjpwYXNz", []string{"dXNlcjpwYXNz"}, []string{"Authorization: Basic "}},
	{"authorization sem esquema", "authorization=abc123", []string{"abc123"}, nil},
	{"authorization curl", `curl -H "Authorization: Bearer abcdef1234567890" https://api.x.io`, []string{"abcdef1234567890"}, []string{"https://api.x.io"}},
	{"authorization json", `{"Authorization": "Bearer abc123xyz"}`, []string{"abc123xyz"}, nil},
	{"authorization digest", `Authorization: Digest username="u", response="abc123"`, []string{"abc123"}, []string{"Authorization: Digest "}},
	{"proxy-authorization", "Proxy-Authorization: Basic dXNlcjpwYXNz", []string{"dXNlcjpwYXNz"}, nil},
	{"authorization token na linha de baixo", "Authorization: Bearer\n  abc123xyz\nfim", []string{"abc123xyz"}, []string{"fim"}},
	{"authorization valor sozinho", "Authorization: abc123\nfim", []string{"abc123"}, []string{"fim"}},
	{"authorization valor e texto", "authorization=abc123 fim", []string{"abc123"}, nil},
	{"authorization esquema desconhecido", "Authorization: Custom k9secret", []string{"k9secret"}, nil},
	// Delimitadores antes do esquema: %v de http.Header, json.Marshal, Python.
	{"authorization map go", "map[Accept:[*/*] Authorization:[Bearer sha256~AbCdEf0123456789xyz]]", []string{"sha256", "AbCdEf0123456789xyz"}, []string{"map[Accept:[*/*] Authorization:[Bearer "}},
	{"authorization json lista", `{"Authorization":["Bearer opaqueTOKEN123abc"]}`, []string{"opaqueTOKEN123abc"}, []string{`{"Authorization":["Bearer `}},
	{"authorization python", "{'Authorization': ['Bearer abc123def456']}", []string{"abc123def456"}, []string{"['Bearer "}},
	{"authorization parenteses", "Authorization: (Bearer x1)", []string{"x1"}, []string{"(Bearer "}},
	{"authorization angulos", "Authorization: <Bearer x1>", []string{"x1"}, []string{"<Bearer "}},
	{"authorization basic lista", `"Authorization": ["Basic dXNlcjpwYXNz"]`, []string{"dXNlcjpwYXNz"}, []string{`["Basic `}},
	{"authorization lista escapada", `{\"Authorization\":[\"Bearer abc123xyz\"]}`, []string{"abc123xyz"}, nil},
	{"authorization lista sem esquema", `Authorization: ["k9secret"]`, []string{"k9secret"}, nil},
	{"bearer solto angulos", "<Bearer x1>", []string{"x1"}, nil},
	{"bearer solto parenteses", "(Bearer xxLIWPc)", []string{"xxLIWPc"}, []string{"(Bearer ", ")"}},
	{"bearer solto misturado", "bearer YdjWXcjd", []string{"YdjWXcjd"}, nil},
	{"bearer solto com simbolos", `"Bearer INp}&V:O"`, []string{"INp}&V"}, nil},
	{"bearer solto lista", "['Bearer abc123def456']", []string{"abc123def456"}, nil},
	{"basic solto lista", `["Basic dXNlcjpwYXNz"]`, []string{"dXNlcjpwYXNz"}, nil},
	{"bearer solto curto com digito", "bearer 01d", []string{"01d"}, nil},
	{"bearer solto com =", "BEARER xyz==", []string{"xyz"}, nil},
	{"bearer solto com _", "bearer ab-cd_ef.gh", []string{"ab-cd_ef"}, nil},
	{"bearer solto", "usei bearer abcdef1234567890", []string{"abcdef1234567890"}, []string{"usei bearer "}},
	{"bearer solto com digito", "Bearer abc12345", []string{"abc12345"}, nil},
	{"basic solto", "basic dXNlcjpwYXNz", []string{"dXNlcjpwYXNz"}, nil},
	{"basic solto com =", "Basic YWRtaW46c2VuaGE=", []string{"YWRtaW46c2VuaGE"}, nil},

	// Formatos do Kubernetes: env com name/value e o kubectl describe de ConfigMap.
	{"env yaml", "env:\n- name: DB_PASSWORD\n  value: s3nh4\n- name: LOG\n  value: info", []string{"s3nh4"}, []string{"name: DB_PASSWORD", "value: info"}},
	{"env yaml entre aspas", "- name: DB_PASSWORD\n  value: \"s3nh4\"", []string{"s3nh4"}, nil},
	{"env yaml value antes", "- value: s3nh4\n  name: DB_PASSWORD\n- name: LOG\n  value: info", []string{"s3nh4"}, []string{"value: info"}},
	{"env yaml outra chave no meio", "- name: API_TOKEN\n  type: x\n  value: tk123", []string{"tk123"}, nil},
	{"env json", `{"name":"DB_PASSWORD","value":"s3nh4"}`, []string{"s3nh4"}, nil},
	{"env json lista", `[{"name":"LOG","value":"info"},{"name":"API_TOKEN","value":"tk123"}]`, []string{"tk123"}, []string{`"value":"info"`}},
	{"env json value antes", `{"value":"s3nh4","name":"DB_PASSWORD"}`, []string{"s3nh4"}, nil},
	{"env json indentado", "{\n    \"name\": \"DB_PASSWORD\",\n    \"value\": \"s3nh4\"\n}", []string{"s3nh4"}, nil},
	{"env json escapado", `{\"name\":\"DB_PASSWORD\",\"value\":\"s3nh4\"}`, []string{"s3nh4"}, nil},
	{"env fluxo", "env: [{name: DB_PASSWORD, value: s3nh4}]", []string{"s3nh4"}, nil},
	{"describe configmap", "Data\n====\nDB_PASSWORD:\n----\ns3nh4\n\nLOG_LEVEL:\n----\ninfo\n\nBinaryData\n====\n", []string{"s3nh4"}, []string{"LOG_LEVEL:\n----\ninfo", "BinaryData"}},
	{"describe configmap varias linhas", "password:\n----\nlinha1\n\nlinha2\n\nuser:\n----\nana\n\nEvents:  <none>", []string{"linha1", "linha2"}, []string{"user:\n----\nana", "Events:  <none>"}},
}

func TestRedigir_Segredos(t *testing.T) {
	for _, c := range casosSegredo {
		t.Run(c.nome, func(t *testing.T) {
			got := Redigir(c.entrada)
			if !strings.Contains(got, Marca) {
				t.Errorf("Redigir(%q) = %q, sem a marca", c.entrada, got)
			}
			for _, q := range c.naoDeve {
				if strings.Contains(got, q) {
					t.Errorf("Redigir(%q) = %q, vazou %q", c.entrada, got, q)
				}
			}
			for _, q := range c.deve {
				if !strings.Contains(got, q) {
					t.Errorf("Redigir(%q) = %q, perdeu %q", c.entrada, got, q)
				}
			}
		})
	}
}

// TestRedigir_Exato fixa a saída de casos em que a forma importa.
func TestRedigir_Exato(t *testing.T) {
	casos := []struct{ entrada, saida string }{
		{"password=hunter2 fim", "password=[REDIGIDO] fim"},
		{`password: "hun\"ter2"`, `password: "[REDIGIDO]"`},
		{`{"password":"x","user":"ana"}`, `{"password":"[REDIGIDO]","user":"ana"}`},
		{`{\"password\":\"x\"}`, `{\"password\":\"[REDIGIDO]\"}`},
		{"redis://:hunter2@redis:6379/0", "redis://:[REDIGIDO]@redis:6379/0"},
		{"https://user:p@ss@host/x", "https://user:[REDIGIDO]@host/x"},
		{"Authorization: Bearer abcdef1234567890", "Authorization: Bearer [REDIGIDO]"},
		{"password:\n  hunter2\nuser: ana", "password:\n  [REDIGIDO]\nuser: ana"},
		{"tls.key: |\n  a\n  b\nnext: x", "tls.key: |\n  [REDIGIDO]\n  [REDIGIDO]\nnext: x"},
		{"tokens:\n  - a1\n  - b2", "tokens:\n  - [REDIGIDO]\n  - [REDIGIDO]"},
		{"map[Accept:[*/*] Authorization:[Bearer sha256~AbCdEf0123456789xyz]]", "map[Accept:[*/*] Authorization:[Bearer [REDIGIDO]]]"},
		{`{"Authorization":["Bearer opaqueTOKEN123abc"]}`, `{"Authorization":["Bearer [REDIGIDO]"]}`},
		{"{'Authorization': ['Bearer abc123def456']}", "{'Authorization': ['Bearer [REDIGIDO]']}"},
		{"Authorization: (Bearer x1)", "Authorization: (Bearer [REDIGIDO])"},
		{`"Authorization": ["Basic dXNlcjpwYXNz"]`, `"Authorization": ["Basic [REDIGIDO]"]`},
		{"authorization=abc123 fim", "authorization=[REDIGIDO]"},
		{`Authorization: Digest username="u", response="abc"`, "Authorization: Digest [REDIGIDO]"},
		{"OPENAIAPIKEY=k9secretvalue fim", "OPENAIAPIKEY=[REDIGIDO] fim"},
		{"chave sk-proj-abcDEF123ghiJKL456mnoPQR789stuVWXyz fim", "chave [REDIGIDO] fim"},
		{"**Password:** hunter2 fim", "**Password:** [REDIGIDO] fim"},
		{`"tokens": ["a1", "b2"]`, `"tokens": [[REDIGIDO]]`},
		{"(Bearer xxLIWPc)", "(Bearer [REDIGIDO])"},
		{"- name: DB_PASSWORD\n  value: s3nh4\n- name: LOG\n  value: info", "- name: DB_PASSWORD\n  value: [REDIGIDO]\n- name: LOG\n  value: info"},
		{`{"name":"DB_PASSWORD","value":"s3nh4"}`, `{"name":"DB_PASSWORD","value":"[REDIGIDO]"}`},
		{"DB_PASSWORD:\n----\ns3nh4\n\nLOG:\n----\ninfo", "DB_PASSWORD:\n----\n[REDIGIDO]\n\nLOG:\n----\ninfo"},
	}
	for _, c := range casos {
		if got := Redigir(c.entrada); got != c.saida {
			t.Errorf("Redigir(%q) = %q, queria %q", c.entrada, got, c.saida)
		}
	}
}

func TestRedigir_NaoRedige(t *testing.T) {
	for _, s := range []string{
		"KEYCLOAK_URL=https://kc",
		"KEYBOARD_LAYOUT: abnt2",
		"monkey=banana",
		"TOKENIZER=bpe",
		"secretary=Ana",
		"authMethod: oidc",
		"https://github.com/casheiro/yby",
		"https://host:8080/caminho",
		"https://host:8080/pacotes/@escopo/x",
		"ssh://git@github.com:22/org/repo.git",
		"git@github.com:org/repo.git",
		"nome: web\nreplicas: 3",
		"sem segredo nenhum aqui",
		// "key" sozinho não é sensível.
		"- key: app\n  operator: In\n  values:\n  - web",
		"tolerations:\n- key: node-role.kubernetes.io/master\n  operator: Exists\n  effect: NoSchedule",
		"matchExpressions:\n  - key: app\n    operator: In",
		"env:\n- name: DB\n  valueFrom:\n    secretKeyRef:\n      name: db-creds\n      key: password",
		"kind: Secret\nmetadata:\n  name: db",
		// env que não é segredo; o value de um item não vale para o name de outro.
		"- name: LOG\n  value: info\n- name: KEYCLOAK_URL\n  value: http://kc",
		`[{"name":"LOG","value":"info"},{"name":"DB_PASSWORD","valueFrom":{"fieldRef":{"fieldPath":"status.podIP"}}}]`,
		"- value: info\n  name: LOG\n- name: DB_PASSWORD\n  valueFrom:\n    secretKeyRef:\n      name: db",
		"LOG_LEVEL:\n----\ninfo\n",
		// Basic e bearer em prosa.
		"Basic authentication is enabled",
		"use a basic configuration",
		"use basic authentication",
		"o cliente manda um bearer token",
		"o bearer token.",
		"o bearer é enviado",
		"Bearer authentication",
		// Nome de recurso com "sk-" no meio de palavra.
		"pod task-runner-deployment-7d9f8b-abcde",
		"desk-booking-service-deployment-abc",
	} {
		if got := Redigir(s); got != s {
			t.Errorf("Redigir(%q) = %q, queria igual", s, got)
		}
	}
}

// TestRedigir_ExageroAceito documenta o que é redigido sem ser segredo: o desenho
// prefere redigir demais a vazar.
func TestRedigir_ExageroAceito(t *testing.T) {
	for _, s := range []string{
		"**Secret:** use o Vault",                 // rótulo de markdown em prosa: some o "use"
		"secretName: db-creds",                    // nome de objeto Secret, não o segredo
		"max_tokens: 100",                         // contagem de tokens
		"Configure o token:\nfoo",                 // primeira palavra da linha de baixo
		"--password-file /etc/pwd",                // caminho depois de flag sensível
		"secretKeyRef: {name: db, key: password}", // referência em mapa de fluxo
		"Authorization: Custom realm=x, fim",      // esquema desconhecido: o resto da linha
		"bearer well-known",                       // hífen depois de bearer conta como token
	} {
		if got := Redigir(s); !strings.Contains(got, Marca) {
			t.Errorf("Redigir(%q) = %q, esperava redação (exagero aceito)", s, got)
		}
	}
}

func TestRedigir_Idempotente(t *testing.T) {
	for _, c := range casosSegredo {
		uma := Redigir(c.entrada)
		if duas := Redigir(uma); duas != uma {
			t.Errorf("%s: Redigir não é idempotente:\n1: %q\n2: %q", c.nome, uma, duas)
		}
	}
}

func TestRedigir_PreservaContexto(t *testing.T) {
	got := Redigir("antes https://u:p@h.io/x depois DB_PASSWORD=abc fim")
	for _, q := range []string{"antes https://", "@h.io/x depois", "DB_PASSWORD=", " fim"} {
		if !strings.Contains(got, q) {
			t.Errorf("faltou %q em %q", q, got)
		}
	}
}
