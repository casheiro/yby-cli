//go:build k8s

package remediation

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// ApplyPatches aplica os patches de remediação no cluster.
func ApplyPatches(ctx context.Context, client kubernetes.Interface, patches []RemediationPatch) []error {
	var errs []error
	for _, p := range patches {
		err := applyPatch(ctx, client, p)
		if err != nil {
			errs = append(errs, fmt.Errorf("falha ao aplicar patch em %s/%s: %w", p.Namespace, p.ResourceName, err))
		}
	}
	return errs
}

func applyPatch(ctx context.Context, client kubernetes.Interface, p RemediationPatch) error {
	patchType := k8stypes.StrategicMergePatchType
	if p.PatchType == "json-patch" {
		patchType = k8stypes.JSONPatchType
	}

	switch p.ResourceKind {
	case "Pod":
		_, err := client.CoreV1().Pods(p.Namespace).Patch(ctx, p.ResourceName, patchType, []byte(p.Patch), metav1.PatchOptions{})
		return err
	case "Deployment":
		_, err := client.AppsV1().Deployments(p.Namespace).Patch(ctx, p.ResourceName, patchType, []byte(p.Patch), metav1.PatchOptions{})
		return err
	default:
		return fmt.Errorf("tipo de recurso '%s' não suportado para patching", p.ResourceKind)
	}
}
