//go:build k8s

package remediation

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// gerenciadorDeCampos é o field manager dos patches do --fix: o managedFields do recurso
// diz quem mudou o securityContext.
const gerenciadorDeCampos = "yby-sentinel"

// Aplicar manda os patches ao servidor. Com dryRun, o servidor confere cada um (esquema,
// validação e admissão) sem gravar nada. Devolve os que passaram e as falhas.
func Aplicar(ctx context.Context, client kubernetes.Interface, patches []RemediationPatch, dryRun bool) (aceitos []RemediationPatch, falhas []Falha) {
	opts := metav1.PatchOptions{FieldManager: gerenciadorDeCampos}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	for _, p := range patches {
		if err := aplicar(ctx, client, p, opts); err != nil {
			falhas = append(falhas, Falha{Patch: p, Err: err})
			continue
		}
		aceitos = append(aceitos, p)
	}
	return aceitos, falhas
}

func aplicar(ctx context.Context, client kubernetes.Interface, p RemediationPatch, opts metav1.PatchOptions) error {
	tipo, dados := k8stypes.StrategicMergePatchType, []byte(p.Patch)
	var err error
	switch p.ResourceKind {
	case "Deployment":
		_, err = client.AppsV1().Deployments(p.Namespace).Patch(ctx, p.ResourceName, tipo, dados, opts)
	case "StatefulSet":
		_, err = client.AppsV1().StatefulSets(p.Namespace).Patch(ctx, p.ResourceName, tipo, dados, opts)
	case "DaemonSet":
		_, err = client.AppsV1().DaemonSets(p.Namespace).Patch(ctx, p.ResourceName, tipo, dados, opts)
	case "CronJob":
		_, err = client.BatchV1().CronJobs(p.Namespace).Patch(ctx, p.ResourceName, tipo, dados, opts)
	default:
		err = fmt.Errorf("o --fix não corrige %s", p.ResourceKind)
	}
	return err
}
