package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RealRunner implements Runner using os/exec
type RealRunner struct{}

func (r *RealRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *RealRunner) RunCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func (r *RealRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// RealFilesystem implements Filesystem using os
type RealFilesystem struct{}

func (f *RealFilesystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (f *RealFilesystem) WriteFile(name string, d []byte, p fs.FileMode) error {
	return os.WriteFile(name, d, p)
}

func (f *RealFilesystem) MkdirAll(p string, perm fs.FileMode) error {
	return os.MkdirAll(p, perm)
}

func (f *RealFilesystem) Stat(n string) (fs.FileInfo, error) {
	return os.Stat(n)
}

func (f *RealFilesystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (f *RealFilesystem) WalkDir(r string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(r, fn)
}

// RealK8sClient implements K8sClient using kubectl commands
type RealK8sClient struct {
	Runner Runner
}

func (k *RealK8sClient) WaitPodReady(ctx context.Context, label, ns string, timeout int) error {
	return k.Runner.Run(ctx, "kubectl", "wait", "--for=condition=Ready", "pod", "-l", label, "-n", ns, fmt.Sprintf("--timeout=%ds", timeout))
}

func (k *RealK8sClient) WaitCRD(ctx context.Context, crdName string, timeout int) error {
	return k.Runner.Run(ctx, "kubectl", "wait", "--for", "condition=established", fmt.Sprintf("--timeout=%ds", timeout), "crd/"+crdName)
}

func (k *RealK8sClient) NamespaceExists(ctx context.Context, ns string) (bool, error) {
	err := k.Runner.Run(ctx, "kubectl", "get", "namespace", ns)
	return err == nil, nil
}

func (k *RealK8sClient) CreateNamespace(ctx context.Context, ns string) error {
	out, err := k.Runner.RunCombinedOutput(ctx, "kubectl", "create", "namespace", ns)
	if err != nil && strings.Contains(string(out), "already exists") {
		return nil
	}
	return err
}

func (k *RealK8sClient) ApplyManifest(ctx context.Context, path string, namespace string) error {
	args := []string{"apply", "-f", path}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	return k.Runner.Run(ctx, "kubectl", args...)
}

func (k *RealK8sClient) PatchApplication(ctx context.Context, name, ns, patch string) error {
	return k.Runner.Run(ctx, "kubectl", "patch", "application", name, "-n", ns, "--type", "merge", "-p", patch)
}
