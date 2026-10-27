//go:build k8s

package remediation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// clusterDeTeste tem o Deployment web (contêineres app e sidecar, este já endurecido) com
// dois pods, o CronJob backup com um pod, o Deployment argo do Argo CD e um pod sem dono.
func clusterDeTeste() *fake.Clientset {
	nao, sim := false, true
	endurecido := &corev1.SecurityContext{AllowPrivilegeEscalation: &nao, Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}
	web := corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}, {Name: "sidecar", SecurityContext: endurecido}}}
	argo := &appsv1.Deployment{ObjectMeta: meta("argo", "", ""), Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: podSpec()}}}
	argo.Labels = map[string]string{rotuloDoArgo: "loja"}
	privilegiado := podSpec()
	privilegiado.Containers[0].SecurityContext = &corev1.SecurityContext{Privileged: &sim}
	objs := []runtime.Object{
		&appsv1.Deployment{ObjectMeta: meta("web", "", ""), Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: web}}},
		&appsv1.ReplicaSet{ObjectMeta: meta("web-1", "Deployment", "web")},
		pod("web-1-a", "ReplicaSet", "web-1"),
		pod("web-1-b", "ReplicaSet", "web-1"),
		&batchv1.CronJob{ObjectMeta: meta("backup", "", ""), Spec: batchv1.CronJobSpec{JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: podSpec()}}}}},
		&batchv1.Job{ObjectMeta: meta("backup-1", "CronJob", "backup")},
		pod("backup-1-a", "Job", "backup-1"),
		argo,
		&appsv1.DaemonSet{ObjectMeta: meta("cni", "", ""), Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: privilegiado}}},
		pod("solto", "", ""),
	}
	return fake.NewSimpleClientset(objs...)
}

func achadoDoPod(checkID, pod, conteiner string) checks.SecurityFinding {
	return checks.SecurityFinding{CheckID: checkID, Severity: checks.SeverityMedium, Pod: pod, Container: conteiner, Namespace: "ns"}
}

func achadoDoPolaris(checkID string, recursos ...string) checks.SecurityFinding {
	return checks.SecurityFinding{CheckID: checkID, Severity: checks.SeverityMedium, Resource: recursos[0], Resources: recursos, Namespace: "ns"}
}

// TestPlanejar_SobeAoDonoEJuntaNumPatch: os achados dos dois pods do Deployment e o do
// Polaris viram um patch só, no template do Deployment, só no contêiner que precisa.
func TestPlanejar_SobeAoDonoEJuntaNumPatch(t *testing.T) {
	plano := Planejar(context.Background(), clusterDeTeste(), []checks.SecurityFinding{
		achadoDoPod("POD_PRIVILEGE_ESCALATION", "web-1-a", "app"),
		achadoDoPod("POD_PRIVILEGE_ESCALATION", "web-1-b", "app"),
		achadoDoPolaris("polaris/insecureCapabilities", "Deployment/web"),
	})
	if len(plano.Patches) != 1 || len(plano.Recusas) != 0 {
		t.Fatalf("esperava 1 patch e nenhuma recusa: %+v", plano)
	}
	p := plano.Patches[0]
	if p.Recurso() != "Deployment/web" || p.Namespace != "ns" || p.PatchType != "strategic-merge" {
		t.Errorf("o patch vai ao Deployment: %+v", p)
	}
	esperado := `{"spec":{"template":{"spec":{"containers":[{"name":"app","securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}}}`
	if p.Patch != esperado {
		t.Errorf("patch:\n%s\nesperado:\n%s", p.Patch, esperado)
	}
	mudancas := strings.Join(p.Mudancas, "\n")
	if mudancas != "contêiner app: allowPrivilegeEscalation (vazio) → false\ncontêiner app: capabilities.drop (vazio) → [ALL]" {
		t.Errorf("mudanças:\n%s", mudancas)
	}
	if p.Description != "bloqueia a escalação de privilégios e tira as capabilities" {
		t.Errorf("descrição: %q", p.Description)
	}
}

// TestPlanejar_CronJobNoJobTemplate: o pod de um Job de CronJob corrige o CronJob, no
// caminho do jobTemplate.
func TestPlanejar_CronJobNoJobTemplate(t *testing.T) {
	plano := Planejar(context.Background(), clusterDeTeste(), []checks.SecurityFinding{achadoDoPod("POD_PRIVILEGE_ESCALATION", "backup-1-a", "app")})
	if len(plano.Patches) != 1 || plano.Patches[0].Recurso() != "CronJob/backup" {
		t.Fatalf("esperava o patch no CronJob: %+v", plano)
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(plano.Patches[0].Patch), &patch); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plano.Patches[0].Patch, `{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"containers":`) {
		t.Errorf("caminho do template do CronJob: %s", plano.Patches[0].Patch)
	}
}

// TestPlanejar_Recusas: pod sem dono, recurso do Argo CD e contêiner privilegiado não
// recebem patch, cada um com o motivo; os checks sem correção segura só são contados.
func TestPlanejar_Recusas(t *testing.T) {
	plano := Planejar(context.Background(), clusterDeTeste(), []checks.SecurityFinding{
		achadoDoPod("POD_PRIVILEGE_ESCALATION", "solto", "app"),
		achadoDoPolaris("polaris/privilegeEscalationAllowed", "Deployment/argo", "DaemonSet/cni"),
		achadoDoPod("POD_ROOT_CONTAINER", "web-1-a", "app"),
		achadoDoPod("POD_READONLY_ROOTFS", "web-1-a", "app"),
		achadoDoPod("POD_RESOURCE_LIMITS", "web-1-a", "app"),
		achadoDoPolaris("polaris/runAsRootAllowed", "Deployment/web"),
		{CheckID: "POD_CAPABILITIES", Severity: checks.SeverityCritical, Pod: "web-1-a", Container: "app", Namespace: "ns"},
	})
	if len(plano.Patches) != 0 {
		t.Errorf("nenhum patch: %+v", plano.Patches)
	}
	if plano.SemCorrecao != 5 {
		t.Errorf("5 achados sem correção automática, obtidos %d", plano.SemCorrecao)
	}
	esperadas := map[string]string{
		"DaemonSet/cni":   "é privilegiado",
		"Deployment/argo": "gerido pelo Argo CD (Application loja)",
		"Pod/solto":       "pod não tem dono",
	}
	if len(plano.Recusas) != len(esperadas) {
		t.Fatalf("recusas: %+v", plano.Recusas)
	}
	for _, r := range plano.Recusas {
		if !strings.Contains(r.Motivo, esperadas[r.Recurso]) {
			t.Errorf("%s: motivo %q, esperado com %q", r.Recurso, r.Motivo, esperadas[r.Recurso])
		}
	}
}

// TestPlanejar_JaCorrigido: o template que já tem a correção (o pod é de antes dela) não
// recebe patch.
func TestPlanejar_JaCorrigido(t *testing.T) {
	plano := Planejar(context.Background(), clusterDeTeste(), []checks.SecurityFinding{achadoDoPod("POD_PRIVILEGE_ESCALATION", "web-1-a", "sidecar")})
	if len(plano.Patches) != 0 || len(plano.Recusas) != 0 {
		t.Errorf("nada a corrigir: %+v", plano)
	}
}
