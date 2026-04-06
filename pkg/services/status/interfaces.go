package status

import "context"

// ClusterInspector abstrai operações de inspeção do cluster Kubernetes.
// Cada método retorna a saída textual do comando e um eventual erro.
type ClusterInspector interface {
	GetNodes(ctx context.Context) (string, error)
	GetArgoCDPods(ctx context.Context) (string, error)
	GetIngresses(ctx context.Context) (string, error)
	GetScaledObjects(ctx context.Context) (string, error)
	GetKeplerPods(ctx context.Context) (string, error)
}
