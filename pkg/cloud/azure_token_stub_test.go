//go:build !azure

package cloud

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/testutil"
)

// O token vai para o servidor de API do AKS, e a validade sai do expires_on (UTC).
func TestTokenAKSPeloCLI_PublicoEEpoch(t *testing.T) {
	var argumentos []string
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			argumentos = args
			return []byte(`{"accessToken":"tok","expiresOn":"2026-10-25 09:18:00.000000","expires_on":1792930680}`), nil
		},
	}
	tok, err := (&AzureTokenGenerator{Runner: runner}).GenerateToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(argumentos, " "), "--resource "+aksServidor) {
		t.Errorf("sem o público do AKS: %v", argumentos)
	}
	if tok.Value != "tok" || !tok.ExpiresAt.Equal(time.Unix(1792930680, 0)) {
		t.Errorf("token = %+v", tok)
	}
}

// Sem expires_on (az antigo), o expiresOn é a hora local, não UTC.
func TestTokenAKSPeloCLI_ExpiresOnNaHoraLocal(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(context.Context, string, ...string) ([]byte, error) {
			return []byte(`{"accessToken":"tok","expiresOn":"2026-10-25 09:18:00.000000"}`), nil
		},
	}
	tok, err := (&AzureAdvancedTokenGenerator{Runner: runner}).GenerateToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	quer := time.Date(2026, 10, 25, 9, 18, 0, 0, time.Local)
	if !tok.ExpiresAt.Equal(quer) {
		t.Errorf("ExpiresAt = %v, quer %v", tok.ExpiresAt, quer)
	}
}

func TestTokenAKSPeloCLI_Erros(t *testing.T) {
	semLogin := &testutil.MockRunner{
		RunCombinedOutputFunc: func(context.Context, string, ...string) ([]byte, error) {
			return []byte("ERROR: Please run 'az login' to setup account.\n"), errors.New("exit status 1")
		},
	}
	if _, err := tokenAKSPeloCLI(context.Background(), semLogin); err == nil || !strings.Contains(err.Error(), "az login") {
		t.Errorf("erro sem a saída do az: %v", err)
	}
	semValidade := &testutil.MockRunner{
		RunCombinedOutputFunc: func(context.Context, string, ...string) ([]byte, error) {
			return []byte(`{"accessToken":"tok","expiresOn":"amanhã"}`), nil
		},
	}
	if _, err := tokenAKSPeloCLI(context.Background(), semValidade); err == nil {
		t.Error("validade ilegível deveria dar erro")
	}
}
