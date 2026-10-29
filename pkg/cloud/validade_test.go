package cloud

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/testutil"
)

// runnerPorComando responde cada comando pela chave do mapa que for prefixo dele
// ("aws sts get-caller-identity"); o resto falha.
func runnerPorComando(saidas map[string]string) *testutil.MockRunner {
	return &testutil.MockRunner{
		RunStdinOutputFunc: func(_ context.Context, _ string, nome string, args ...string) ([]byte, error) {
			cmd := nome + " " + strings.Join(args, " ")
			for prefixo, saida := range saidas {
				if strings.HasPrefix(cmd, prefixo) {
					return []byte(saida), nil
				}
			}
			return nil, errors.New("comando inesperado: " + cmd)
		},
	}
}

func TestValidateCredentials_PreencheAValidade(t *testing.T) {
	ctx := context.Background()
	esperado := time.Date(2026, 10, 25, 12, 18, 0, 0, time.UTC)

	aws := &awsProvider{runner: runnerPorComando(map[string]string{
		"aws sts get-caller-identity":      `{"Account":"482100009310","Arn":"arn:aws:sts::482100009310:assumed-role/x/y"}`,
		"aws configure export-credentials": `{"Version":1,"AccessKeyId":"A","Expiration":"2026-10-25T12:18:00+00:00"}`,
	})}
	st, err := aws.ValidateCredentials(ctx)
	if err != nil || st.ExpiresAt == nil || !st.ExpiresAt.Equal(esperado) {
		t.Fatalf("aws: ExpiresAt = %v, err = %v", st.ExpiresAt, err)
	}

	az := &azureProvider{runner: runnerPorComando(map[string]string{
		"az account show":             `{"user":{"name":"ana@empresa.com.br"},"state":"Enabled"}`,
		"az account get-access-token": `{"expiresOn":"2026-10-25 09:18:00.000000","expires_on":` + strconv.FormatInt(esperado.Unix(), 10) + `}`,
	})}
	st, err = az.ValidateCredentials(ctx)
	if err != nil || st.ExpiresAt == nil || !st.ExpiresAt.Equal(esperado) {
		t.Fatalf("azure: ExpiresAt = %v, err = %v", st.ExpiresAt, err)
	}

	gcp := &gcpProvider{runner: runnerPorComando(map[string]string{
		"gcloud auth list":            `[{"account":"yby@ml.iam.gserviceaccount.com","status":"ACTIVE"}]`,
		"gcloud config config-helper": `{"credential":{"access_token":"x","token_expiry":"2026-10-25T12:18:00Z"}}`,
	})}
	st, err = gcp.ValidateCredentials(ctx)
	if err != nil || st.ExpiresAt == nil || !st.ExpiresAt.Equal(esperado) {
		t.Fatalf("gcp: ExpiresAt = %v, err = %v", st.ExpiresAt, err)
	}
}

// Sem a validade (CLI antigo, chave de IAM fixa), a credencial continua válida e o
// ExpiresAt fica nil.
func TestValidateCredentials_SemValidadeNaoFalha(t *testing.T) {
	aws := &awsProvider{runner: runnerPorComando(map[string]string{
		"aws sts get-caller-identity": `{"Account":"1","Arn":"arn:aws:iam::1:user/ana"}`,
	})}
	st, err := aws.ValidateCredentials(context.Background())
	if err != nil || !st.Authenticated || st.ExpiresAt != nil {
		t.Fatalf("status = %+v, err = %v", st, err)
	}
}

func TestLerInstante(t *testing.T) {
	if lerInstante("", time.UTC) != nil || lerInstante("ontem", time.UTC) != nil {
		t.Fatal("texto vazio ou inválido deve dar nil")
	}
	got := lerInstante("2026-10-25 09:18:00.000000", time.FixedZone("BRT", -3*3600))
	if got == nil || !got.Equal(time.Date(2026, 10, 25, 12, 18, 0, 0, time.UTC)) {
		t.Fatalf("hora local do az: %v", got)
	}
}
