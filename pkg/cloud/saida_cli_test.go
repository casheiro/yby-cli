package cloud

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
)

// saidaSSOVencido é o que o AWS CLI v2 escreve no stderr com o token do SSO vencido (sai
// com 255).
const saidaSSOVencido = "\nError when retrieving token from sso: Token has expired and refresh failed\n"

func TestErroDoCLI_LevaAUltimaLinhaUtil(t *testing.T) {
	casos := []struct {
		nome, saida, quer string
	}{
		{"sso vencido", saidaSSOVencido,
			"aws eks get-token: exit status 255: Error when retrieving token from sso: Token has expired and refresh failed"},
		{"sem saída", "", "aws eks get-token: exit status 255"},
		{"só espaços", "\n  \r\n", "aws eks get-token: exit status 255"},
		{"json antes do erro", "{\n  \"clusters\": []\n}\nAn error occurred (AccessDenied) when calling X\n",
			"aws eks get-token: exit status 255: An error occurred (AccessDenied) when calling X"},
		{"token no meio", "falhou com eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.abc e ya29.a0Af-xyz",
			"aws eks get-token: exit status 255: falhou com *** e ***"},
		{"campo de segredo", `refresh token=abc123 recusado`,
			"aws eks get-token: exit status 255: refresh token=*** recusado"},
		{"chave de acesso", "chave ASIAEXEMPLOFALSO0001 recusada",
			"aws eks get-token: exit status 255: chave *** recusada"},
		{"bearer opaco", "Authorization: Bearer abc.def-123 recusado",
			"aws eks get-token: exit status 255: Authorization: Bearer *** recusado"},
		{"client secret", "client_secret=sup3rS3cret inválido",
			"aws eks get-token: exit status 255: client_secret=*** inválido"},
		{"api key", `resposta {"api_key": "k-1"} recusada`,
			`aws eks get-token: exit status 255: resposta {"api_key": ***} recusada`},
		{"valor entre aspas com espaço", `password: "a b c" recusada`,
			"aws eks get-token: exit status 255: password: *** recusada"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			err := erroDoCLI("aws eks get-token", []byte(c.saida), errors.New("exit status 255"))
			if err.Error() != c.quer {
				t.Errorf("erro = %q, quer %q", err.Error(), c.quer)
			}
		})
	}
}

func TestErroDoCLI_AparaLinhaLonga(t *testing.T) {
	err := erroDoCLI("az aks list", []byte(strings.Repeat("a", 500)), errors.New("exit status 1"))
	linha := strings.TrimPrefix(err.Error(), "az aks list: exit status 1: ")
	if n := len([]rune(linha)); n != maxSaidaNoErro {
		t.Errorf("linha com %d caracteres, quer %d", n, maxSaidaNoErro)
	}
	if !strings.HasSuffix(linha, "…") {
		t.Errorf("linha aparada sem reticências: %q", linha)
	}
}

func TestErroDoCLI_PreservaACausa(t *testing.T) {
	causa := errors.New("exit status 255")
	err := erroDoCLI("aws eks list-clusters", []byte(saidaSSOVencido), causa)
	if !errors.Is(err, causa) {
		t.Error("o erro do CLI perdeu a causa")
	}
}

// Os providers levam a saída do CLI ao erro: o painel, o toast e o audit log contam o
// motivo, e não só o código de saída.
func TestProviders_ErroComASaidaDoCLI(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(saidaSSOVencido), errors.New("exit status 255")
		},
	}
	ctx := context.Background()
	aws := &awsProvider{runner: runner}
	casos := map[string]error{}
	_, casos["aws list"] = aws.ListClusters(ctx, ListOptions{})
	casos["aws refresh"] = aws.RefreshToken(ctx, ClusterInfo{Name: "staging"})
	_, casos["aws cred"] = aws.ValidateCredentials(ctx)
	az := &azureProvider{runner: runner}
	_, casos["az list"] = az.ListClusters(ctx, ListOptions{})
	casos["az refresh"] = az.RefreshToken(ctx, ClusterInfo{})
	g := &gcpProvider{runner: runner}
	_, casos["gcp list"] = g.ListClusters(ctx, ListOptions{})
	casos["gcp refresh"] = g.RefreshToken(ctx, ClusterInfo{})
	for nome, err := range casos {
		if err == nil || !strings.HasSuffix(err.Error(), ": exit status 255: Error when retrieving token from sso: Token has expired and refresh failed") {
			t.Errorf("%s: erro sem a saída do CLI: %v", nome, err)
		}
	}
}

func TestRegiaoDoARN(t *testing.T) {
	casos := map[string]string{
		"arn:aws:eks:us-east-1:482193759310:cluster/prod-core": "us-east-1",
		"arn:aws:eks:sa-east-1:1:cluster/a":                    "sa-east-1",
		"":                                                     "",
		"prod-core":                                            "",
	}
	for arn, quer := range casos {
		if r := regiaoDoARN(arn); r != quer {
			t.Errorf("regiaoDoARN(%q) = %q, quer %q", arn, r, quer)
		}
	}
}

// Sem --region, o CLI usa a região configurada; ela vem no ARN do describe-cluster.
func TestAWSProvider_ListClusters_RegiaoDoARN(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if args[1] == "list-clusters" {
				return []byte(`{"clusters":["prod-core"]}`), nil
			}
			return []byte(`{"cluster":{"name":"prod-core","version":"1.30","status":"ACTIVE","arn":"arn:aws:eks:us-east-1:482193759310:cluster/prod-core"}}`), nil
		},
	}
	clusters, err := (&awsProvider{runner: runner}).ListClusters(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 || clusters[0].Region != "us-east-1" {
		t.Fatalf("região = %+v, quer us-east-1", clusters)
	}
}

// A região das variáveis de ambiente vale antes da do ~/.aws/config, que o `aws configure
// get region` lê sozinho; AWS_REGION antes de AWS_DEFAULT_REGION.
func TestAWSProvider_RegiaoConfigurada_VariaveisDeAmbiente(t *testing.T) {
	chamouOCLI := false
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			chamouOCLI = true
			return []byte("sa-east-1\n"), nil
		},
	}
	a := &awsProvider{runner: runner}
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "eu-west-1")
	if r := a.regiaoConfigurada(context.Background()); r != "eu-west-1" {
		t.Errorf("com AWS_DEFAULT_REGION: %q, quer eu-west-1", r)
	}
	t.Setenv("AWS_REGION", "us-west-2")
	if r := a.regiaoConfigurada(context.Background()); r != "us-west-2" {
		t.Errorf("com AWS_REGION: %q, quer us-west-2", r)
	}
	if chamouOCLI {
		t.Error("com a região no ambiente, o CLI não devia ser chamado")
	}
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	if r := a.regiaoConfigurada(context.Background()); r != "sa-east-1" {
		t.Errorf("sem variável: %q, quer a do CLI (sa-east-1)", r)
	}
}

// Sem describe (e sem ARN), vale a região configurada do CLI, lida uma vez por listagem.
func TestAWSProvider_ListClusters_SemDescribeUsaARegiaoDoCLI(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	leiturasDaRegiao := 0
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			switch strings.Join(args[:2], " ") {
			case "eks list-clusters":
				return []byte(`{"clusters":["a","b"]}`), nil
			case "configure get":
				leiturasDaRegiao++
				return []byte("sa-east-1\n"), nil
			}
			return []byte("\nAn error occurred (AccessDenied)"), errors.New("exit status 254")
		},
	}
	clusters, err := (&awsProvider{runner: runner}).ListClusters(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range clusters {
		if c.Region != "sa-east-1" {
			t.Errorf("%s: região %q, quer sa-east-1", c.Name, c.Region)
		}
	}
	if leiturasDaRegiao != 1 {
		t.Errorf("a região do CLI foi lida %d vezes, quer 1", leiturasDaRegiao)
	}
}

func TestAzureProvider_ContaEAAssinatura(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if args[1] == "show" {
				return []byte(`{"id":"7f3c2a91-4b6e-4d1a-9c55-2e8f0b6d4a13","user":{"name":"neto@casheiro.onmicrosoft.com","type":"user"}}`), nil
			}
			return nil, errors.New("sem token")
		},
	}
	cred, err := (&azureProvider{runner: runner}).ValidateCredentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cred.Account != "7f3c2a91-4b6e-4d1a-9c55-2e8f0b6d4a13" || cred.Identity != "neto@casheiro.onmicrosoft.com" {
		t.Errorf("credencial = %+v", cred)
	}
}
