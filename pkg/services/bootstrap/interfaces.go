package bootstrap

import (
	"context"
)

// Runner and Filesystem are imported from shared

// K8sClient abstracts specific Kubernetes operations requested by the bootstrap process
type K8sClient interface {
	WaitPodReady(ctx context.Context, label, ns string, timeoutSeconds int) error
	WaitCRD(ctx context.Context, crdName string, timeoutSeconds int) error
	NamespaceExists(ctx context.Context, ns string) (bool, error)
	CreateNamespace(ctx context.Context, ns string) error
	ApplyManifest(ctx context.Context, path string, namespace string) error
	PatchApplication(ctx context.Context, name, namespace, patch string) error
	WaitApplicationHealthy(ctx context.Context, name, namespace string, timeoutSeconds int) error
}
