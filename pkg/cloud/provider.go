package cloud

import (
	"context"
	"time"
)

// CloudProvider abstrai interações com um provider cloud para gerenciamento de clusters K8s.
type CloudProvider interface {
	// Name retorna o identificador canônico do provider (ex: "aws", "azure", "gcp").
	Name() string

	// IsAvailable verifica se o CLI do provider está instalado. Sem chamadas de rede.
	IsAvailable(ctx context.Context) bool

	// CLIVersion retorna a versão do CLI do provider.
	CLIVersion(ctx context.Context) (string, error)

	// ListClusters lista os clusters disponíveis no provider.
	ListClusters(ctx context.Context, opts ListOptions) ([]ClusterInfo, error)

	// ConfigureKubeconfig configura o kubeconfig local para o cluster especificado.
	ConfigureKubeconfig(ctx context.Context, cluster ClusterInfo) error

	// ValidateCredentials verifica se as credenciais atuais são válidas.
	ValidateCredentials(ctx context.Context) (*CredentialStatus, error)

	// RefreshToken força a renovação do token de autenticação para o cluster.
	RefreshToken(ctx context.Context, cluster ClusterInfo) error
}

// ListOptions controla os filtros usados ao listar clusters.
type ListOptions struct {
	// Region filtra clusters por região (ex: "us-east-1", "eastus", "us-central1").
	Region string

	// Project filtra clusters por projeto (usado pelo GCP).
	Project string
}

// ClusterInfo contém metadados de um cluster K8s gerenciado.
type ClusterInfo struct {
	// Name é o nome do cluster.
	Name string

	// Region é a região onde o cluster está hospedado.
	Region string

	// Provider é o nome canônico do cloud provider (ex: "aws", "azure", "gcp").
	Provider string

	// Version é a versão do K8s do cluster (ex: "1.29").
	Version string

	// Status descreve o estado operacional do cluster (ex: "ACTIVE", "CREATING").
	Status string

	// Endpoint é a URL do API server K8s (ex: https://xyz.eks.amazonaws.com).
	Endpoint string

	// ARN é o Amazon Resource Name do cluster (específico AWS, vazio para outros providers).
	ARN string

	// ResourceGroup é o resource group Azure (vazio para outros providers).
	ResourceGroup string

	// ProjectID é o projeto GCP (vazio para outros providers).
	ProjectID string
}

// CredentialStatus reporta o estado atual de autenticação do provider.
type CredentialStatus struct {
	// Authenticated indica se as credenciais são válidas.
	Authenticated bool

	// Identity é a identidade autenticada (ex: ARN da AWS, email do GCP).
	Identity string

	// ExpiresAt é o timestamp de expiração do token, nil se não aplicável ou desconhecido.
	ExpiresAt *time.Time

	// Method descreve o método de autenticação usado (ex: "profile", "service-account").
	Method string
}
