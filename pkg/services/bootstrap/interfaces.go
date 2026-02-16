package bootstrap

import (
	"context"
	"io/fs"
)

// Runner abstracts command execution (sh, helm, kubectl)
type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
	RunCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error)
	LookPath(file string) (string, error)
}

// Filesystem abstracts file operations
type Filesystem interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Stat(name string) (fs.FileInfo, error)
	UserHomeDir() (string, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
}

// K8sClient abstracts specific Kubernetes operations requested by the bootstrap process
type K8sClient interface {
	WaitPodReady(ctx context.Context, label, ns string, timeoutSeconds int) error
	WaitCRD(ctx context.Context, crdName string, timeoutSeconds int) error
	NamespaceExists(ctx context.Context, ns string) (bool, error)
	CreateNamespace(ctx context.Context, ns string) error
	ApplyManifest(ctx context.Context, path string, namespace string) error
	PatchApplication(ctx context.Context, name, namespace, patch string) error
}
