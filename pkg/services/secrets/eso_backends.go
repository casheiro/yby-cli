package secrets

import (
	"fmt"
	"strings"
)

// BackendOpts contém opções para geração de um ClusterSecretStore ESO.
type BackendOpts struct {
	Name      string
	Namespace string
	// Vault
	VaultServer string
	VaultPath   string
	// AWS
	AWSRegion string
	// GCP
	GCPProjectID string
	// Azure
	AzureVaultURL string
}

// GenerateSecretStore gera o YAML de um ClusterSecretStore para o backend especificado.
// Backends suportados: vault, aws, gcp, azure.
func GenerateSecretStore(backend string, opts BackendOpts) (string, error) {
	switch strings.ToLower(backend) {
	case "vault":
		return generateVaultStore(opts), nil
	case "aws":
		return generateAWSStore(opts), nil
	case "gcp":
		return generateGCPStore(opts), nil
	case "azure":
		return generateAzureStore(opts), nil
	default:
		return "", fmt.Errorf("backend desconhecido: %q. Use: vault, aws, gcp, azure", backend)
	}
}

func generateVaultStore(opts BackendOpts) string {
	server := opts.VaultServer
	if server == "" {
		server = "http://vault.vault.svc:8200"
	}
	path := opts.VaultPath
	if path == "" {
		path = "secret"
	}
	return fmt.Sprintf(`apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: %s-store
spec:
  provider:
    vault:
      server: "%s"
      path: "%s"
      version: v2
      auth:
        kubernetes:
          mountPath: kubernetes
          role: external-secrets
`, opts.Name, server, path)
}

func generateAWSStore(opts BackendOpts) string {
	region := opts.AWSRegion
	if region == "" {
		region = "us-east-1"
	}
	return fmt.Sprintf(`apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: %s-store
spec:
  provider:
    aws:
      service: SecretsManager
      region: %s
      auth:
        jwt:
          serviceAccountRef:
            name: external-secrets-sa
            namespace: external-secrets
`, opts.Name, region)
}

func generateGCPStore(opts BackendOpts) string {
	projectID := opts.GCPProjectID
	if projectID == "" {
		projectID = "meu-projeto"
	}
	return fmt.Sprintf(`apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: %s-store
spec:
  provider:
    gcpsm:
      projectID: "%s"
      auth:
        workloadIdentity:
          clusterLocation: us-central1
          clusterName: meu-cluster
          serviceAccountRef:
            name: external-secrets-sa
            namespace: external-secrets
`, opts.Name, projectID)
}

func generateAzureStore(opts BackendOpts) string {
	vaultURL := opts.AzureVaultURL
	if vaultURL == "" {
		vaultURL = "https://meu-keyvault.vault.azure.net"
	}
	return fmt.Sprintf(`apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: %s-store
spec:
  provider:
    azurekv:
      tenantId: "AZURE_TENANT_ID"
      vaultUrl: "%s"
      authType: WorkloadIdentity
      serviceAccountRef:
        name: external-secrets-sa
        namespace: external-secrets
`, opts.Name, vaultURL)
}
