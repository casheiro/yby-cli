package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateSecretStore_Vault(t *testing.T) {
	yaml, err := GenerateSecretStore("vault", BackendOpts{Name: "meu", Namespace: "ns"})
	assert.NoError(t, err)
	assert.Contains(t, yaml, "kind: ClusterSecretStore")
	assert.Contains(t, yaml, "meu-store")
	assert.Contains(t, yaml, "vault")
}

func TestGenerateSecretStore_AWS(t *testing.T) {
	yaml, err := GenerateSecretStore("aws", BackendOpts{Name: "aws", AWSRegion: "us-west-2"})
	assert.NoError(t, err)
	assert.Contains(t, yaml, "SecretsManager")
	assert.Contains(t, yaml, "us-west-2")
}

func TestGenerateSecretStore_GCP(t *testing.T) {
	yaml, err := GenerateSecretStore("gcp", BackendOpts{Name: "gcp", GCPProjectID: "meu-proj"})
	assert.NoError(t, err)
	assert.Contains(t, yaml, "gcpsm")
	assert.Contains(t, yaml, "meu-proj")
}

func TestGenerateSecretStore_Azure(t *testing.T) {
	yaml, err := GenerateSecretStore("azure", BackendOpts{Name: "az", AzureVaultURL: "https://meu.vault.azure.net"})
	assert.NoError(t, err)
	assert.Contains(t, yaml, "azurekv")
	assert.Contains(t, yaml, "meu.vault.azure.net")
}

func TestGenerateSecretStore_BackendDesconhecido(t *testing.T) {
	_, err := GenerateSecretStore("inexistente", BackendOpts{Name: "x"})
	assert.ErrorContains(t, err, "backend desconhecido")
}

func TestGenerateSecretStore_DefaultsVault(t *testing.T) {
	yaml, err := GenerateSecretStore("vault", BackendOpts{Name: "v"})
	assert.NoError(t, err)
	assert.Contains(t, yaml, "http://vault.vault.svc:8200")
	assert.Contains(t, yaml, "secret")
}

func TestGenerateSecretStore_DefaultsAWS(t *testing.T) {
	yaml, err := GenerateSecretStore("aws", BackendOpts{Name: "a"})
	assert.NoError(t, err)
	assert.Contains(t, yaml, "us-east-1")
}

func TestGenerateSecretStore_DefaultsGCP(t *testing.T) {
	yaml, err := GenerateSecretStore("gcp", BackendOpts{Name: "g"})
	assert.NoError(t, err)
	assert.Contains(t, yaml, "meu-projeto")
}

func TestGenerateSecretStore_DefaultsAzure(t *testing.T) {
	yaml, err := GenerateSecretStore("azure", BackendOpts{Name: "z"})
	assert.NoError(t, err)
	assert.Contains(t, yaml, "meu-keyvault")
}

func TestGenerateSecretStore_CaseInsensitive(t *testing.T) {
	yaml, err := GenerateSecretStore("VAULT", BackendOpts{Name: "v"})
	assert.NoError(t, err)
	assert.Contains(t, yaml, "ClusterSecretStore")
}
