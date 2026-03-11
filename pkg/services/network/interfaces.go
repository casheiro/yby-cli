package network

import "context"

// AccessOptions contains configuration for accessing the cluster
type AccessOptions struct {
	TargetContext string
}

// AccessService manages the establishing of access tunnels to cluster services
type AccessService interface {
	Run(ctx context.Context, opts AccessOptions) error
}

// ClusterNetworkManager abstracts interactions with the cluster for networking and secrets
type ClusterNetworkManager interface {
	GetCurrentContext() (string, error)
	GetSecretValue(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error)
	HasService(ctx context.Context, kubeContext, ns, serviceName string) bool
	PortForward(ctx context.Context, kubeContext, ns, resource, ports string) error
	CreateToken(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error)
	KillPortForward(port string)
}

// LocalContainerManager abstracts running local containers (e.g. Docker)
type LocalContainerManager interface {
	IsAvailable() bool
	StartGrafana(ctx context.Context) error
}
