//go:build k8s

package remediation

import (
	"context"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

// TestAplicar_DryRunNoServidor: o dry-run manda DryRun All e o field manager do Sentinel;
// o tipo que o --fix não corrige vira falha.
func TestAplicar_DryRunNoServidor(t *testing.T) {
	client := clusterDeTeste()
	plano := Planejar(context.Background(), client, []checks.SecurityFinding{achadoDoPod("POD_PRIVILEGE_ESCALATION", "web-1-a", "app")})
	var opcoes []metav1.PatchOptions
	client.PrependReactor("patch", "*", func(a k8stesting.Action) (bool, runtime.Object, error) {
		opcoes = append(opcoes, a.(k8stesting.PatchActionImpl).PatchOptions)
		return false, nil, nil
	})
	patches := append(plano.Patches, RemediationPatch{ResourceKind: "Pod", ResourceName: "solto", Namespace: "ns", Patch: "{}"})

	aceitos, falhas := Aplicar(context.Background(), client, patches, true)
	if len(aceitos) != 1 || len(falhas) != 1 || falhas[0].Patch.Recurso() != "Pod/solto" {
		t.Fatalf("aceitos %+v, falhas %+v", aceitos, falhas)
	}
	if len(opcoes) != 1 || strings.Join(opcoes[0].DryRun, ",") != metav1.DryRunAll || opcoes[0].FieldManager != "yby-sentinel" {
		t.Errorf("opções do patch: %+v", opcoes)
	}

	aceitos, falhas = Aplicar(context.Background(), client, plano.Patches, false)
	if len(aceitos) != 1 || len(falhas) != 0 || len(opcoes[1].DryRun) != 0 {
		t.Fatalf("aplicar de verdade: %+v %+v %+v", aceitos, falhas, opcoes)
	}
	dep, err := client.AppsV1().Deployments("ns").Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sc := dep.Spec.Template.Spec.Containers[0].SecurityContext
	if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Errorf("o template ficou com allowPrivilegeEscalation false: %+v", sc)
	}
	if dep.Spec.Template.Spec.Containers[1].SecurityContext.Capabilities == nil {
		t.Error("o contêiner que não estava no patch continua como estava")
	}
}
