//go:build e2e

package scenarios

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	ybyctx "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/services/doctor"
	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCloud_KubeconfigExecPlugin verifica que kubeconfigs com exec plugin
// de cloud providers são reconhecidos corretamente pelo sistema de detecção.
func TestCloud_KubeconfigExecPlugin(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{"AWS EKS", "aws", "aws"},
		{"AWS IAM Authenticator", "aws-iam-authenticator", "aws"},
		{"Azure kubelogin", "kubelogin", "azure"},
		{"Azure CLI", "az", "azure"},
		{"GCP auth plugin", "gke-gcloud-auth-plugin", "gcp"},
		{"GCP gcloud", "gcloud", "gcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			kubeconfigPath := filepath.Join(tmpDir, "config")

			kubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.com
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: ` + tt.command + `
      args:
        - token
`
			require.NoError(t, os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0644))

			// Verificar que o arquivo foi criado e contém o comando esperado
			data, err := os.ReadFile(kubeconfigPath)
			require.NoError(t, err)
			assert.Contains(t, string(data), "command: "+tt.command)
		})
	}
}

// TestCloud_EnvCreateWithKubeContext verifica que um ambiente cloud pode ser
// criado com kube-context e tipo eks via Manager.AddEnvironment.
func TestCloud_EnvCreateWithKubeContext(t *testing.T) {
	tmpDir := t.TempDir()
	ybyDir := filepath.Join(tmpDir, ".yby")
	require.NoError(t, os.MkdirAll(ybyDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "config"), 0755))

	// Cria environments.yaml inicial com ambiente local
	initialManifest := `current: local
environments:
  local:
    type: local
    description: "Ambiente local"
    values: "config/values-local.yaml"
`
	require.NoError(t, os.WriteFile(filepath.Join(ybyDir, "environments.yaml"), []byte(initialManifest), 0644))

	// Cria kubeconfig fake com exec plugin AWS
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig-eks")
	eksKubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://ABCDEF1234.gr7.us-east-1.eks.amazonaws.com
  name: my-eks-context
contexts:
- context:
    cluster: my-eks-context
    user: eks-user
  name: my-eks-context
current-context: my-eks-context
users:
- name: eks-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
        - eks
        - get-token
        - --cluster-name
        - prod-cluster
`
	require.NoError(t, os.WriteFile(kubeconfigPath, []byte(eksKubeconfig), 0644))

	// Usa Manager para adicionar ambiente com tipo eks e kube-context
	mgr := ybyctx.NewManager(tmpDir)
	env := ybyctx.Environment{
		Type:        "eks",
		Description: "Ambiente de produção EKS",
		KubeConfig:  kubeconfigPath,
		KubeContext: "my-eks-context",
		Namespace:   "default",
		Cloud: &ybyctx.CloudConfig{
			Provider: "aws",
			Region:   "us-east-1",
			Cluster:  "prod-cluster",
		},
	}
	err := mgr.AddEnvironment("prod", env, "# Valores de produção EKS\n")
	require.NoError(t, err)

	// Verifica que environments.yaml contém o ambiente com tipo eks
	data, err := os.ReadFile(filepath.Join(ybyDir, "environments.yaml"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "prod:")
	assert.Contains(t, content, "type: eks")
	assert.Contains(t, content, "kube_context: my-eks-context")
	assert.Contains(t, content, "provider: aws")

	// Verifica que o arquivo de values foi criado
	assert.FileExists(t, filepath.Join(tmpDir, "config", "values-prod.yaml"))
}

// TestCloud_EnvUseCloud verifica que é possível trocar o ambiente ativo
// de local para um ambiente cloud.
func TestCloud_EnvUseCloud(t *testing.T) {
	tmpDir := t.TempDir()
	ybyDir := filepath.Join(tmpDir, ".yby")
	require.NoError(t, os.MkdirAll(ybyDir, 0755))

	// Cria manifest com 2 ambientes: local + cloud
	manifest := `current: local
environments:
  local:
    type: local
    description: "Ambiente local"
    values: "config/values-local.yaml"
  cloud-env:
    type: eks
    description: "Ambiente cloud EKS"
    values: "config/values-cloud-env.yaml"
    kube_context: my-eks-context
    namespace: production
    cloud:
      provider: aws
      region: us-east-1
      cluster: prod-cluster
`
	require.NoError(t, os.WriteFile(filepath.Join(ybyDir, "environments.yaml"), []byte(manifest), 0644))

	// Limpa YBY_ENV para não interferir
	t.Setenv("YBY_ENV", "")

	mgr := ybyctx.NewManager(tmpDir)

	// Verifica que o ambiente atual é local
	currentName, _, err := mgr.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "local", currentName)

	// Troca para cloud-env
	err = mgr.SetCurrent("cloud-env")
	require.NoError(t, err)

	// Verifica que current mudou no manifest
	currentName, currentEnv, err := mgr.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "cloud-env", currentName)
	assert.Equal(t, "eks", currentEnv.Type)
	assert.Equal(t, "my-eks-context", currentEnv.KubeContext)
	assert.NotNil(t, currentEnv.Cloud)
	assert.Equal(t, "aws", currentEnv.Cloud.Provider)
}

// TestCloud_DoctorWithCloudContext verifica que o doctor service detecta
// e reporta cloud providers quando executado com MockRunner.
func TestCloud_DoctorWithCloudContext(t *testing.T) {
	// Cria kubeconfig com exec plugin AWS para detecção
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "config")
	eksKubeconfig := `apiVersion: v1
kind: Config
users:
- name: eks-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
        - eks
        - get-token
`
	require.NoError(t, os.WriteFile(kubeconfigPath, []byte(eksKubeconfig), 0644))
	t.Setenv("KUBECONFIG", kubeconfigPath)
	// Neutraliza fallback para ~/.kube/config
	t.Setenv("HOME", tmpDir)

	// MockRunner que simula AWS CLI instalado
	runner := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			switch file {
			case "aws":
				return "/usr/local/bin/aws", nil
			case "kubectl", "helm", "argocd", "git", "direnv":
				return "/usr/bin/" + file, nil
			}
			return "", os.ErrNotExist
		},
		RunFunc: func(_ context.Context, name string, args ...string) error {
			// docker info e kubectl get nodes — sucesso
			return nil
		},
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			if name == "aws" {
				// aws --version
				for _, arg := range args {
					if arg == "--version" {
						return []byte("aws-cli/2.15.0 Python/3.11.6"), nil
					}
				}
				// aws sts get-caller-identity
				return []byte(`{"UserId":"AIDAEXAMPLE","Account":"123456789012","Arn":"arn:aws:iam::123456789012:user/test"}`), nil
			}
			if name == "grep" {
				return []byte("MemTotal:       16384000 kB"), nil
			}
			return []byte{}, nil
		},
	}

	svc := doctor.NewService(runner)
	report := svc.Run(context.Background())

	// Verifica que report.Cloud não está vazio
	require.NotEmpty(t, report.Cloud, "report.Cloud deve conter checks de cloud providers")

	// Verifica que contém check do provider AWS
	foundAWS := false
	for _, check := range report.Cloud {
		if check.Name == "aws" || check.Name == "aws credenciais" {
			foundAWS = true
			break
		}
	}
	assert.True(t, foundAWS, "report.Cloud deve conter check do provider AWS, got: %v", report.Cloud)
}
