//go:build k8s

package remediation

import (
	"context"
	"errors"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// meta monta o ObjectMeta de um recurso de "ns" com o controller dono (kind/nome), ou sem
// dono com kind vazio.
func meta(nome, kindDono, nomeDono string) metav1.ObjectMeta {
	m := metav1.ObjectMeta{Name: nome, Namespace: "ns"}
	if kindDono != "" {
		controle := true
		m.OwnerReferences = []metav1.OwnerReference{{Kind: kindDono, Name: nomeDono, Controller: &controle}}
	}
	return m
}

func podSpec() corev1.PodSpec {
	return corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}
}

func pod(nome, kindDono, nomeDono string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: meta(nome, kindDono, nomeDono), Spec: podSpec()}
}

func TestDonoDe(t *testing.T) {
	objs := []runtime.Object{
		&appsv1.Deployment{ObjectMeta: meta("web", "", ""), Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: podSpec()}}},
		&appsv1.ReplicaSet{ObjectMeta: meta("web-1", "Deployment", "web")},
		pod("web-1-a", "ReplicaSet", "web-1"),
		&batchv1.CronJob{ObjectMeta: meta("backup", "", ""), Spec: batchv1.CronJobSpec{JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: podSpec()}}}}},
		&batchv1.Job{ObjectMeta: meta("backup-1", "CronJob", "backup")},
		pod("backup-1-a", "Job", "backup-1"),
		&batchv1.Job{ObjectMeta: meta("migrar", "", "")},
		pod("migrar-a", "Job", "migrar"),
		&appsv1.StatefulSet{ObjectMeta: meta("db", "", ""), Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: podSpec()}}},
		pod("db-0", "StatefulSet", "db"),
		&appsv1.DaemonSet{ObjectMeta: meta("agente", "", ""), Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: podSpec()}}},
		pod("agente-x", "DaemonSet", "agente"),
		pod("solto", "", ""),
		&appsv1.ReplicaSet{ObjectMeta: meta("rs-solto", "", "")},
		pod("rs-solto-a", "ReplicaSet", "rs-solto"),
		pod("rollout-a", "Rollout", "rollout"),
	}
	client := fake.NewSimpleClientset(objs...)

	casos := []struct {
		kind, nome string
		alvo       string // "Kind/nome", ou vazio quando o recurso é recusado
		motivo     string // trecho do motivo da recusa
	}{
		{"Pod", "web-1-a", "Deployment/web", ""},
		{"Pod", "backup-1-a", "CronJob/backup", ""},
		{"Pod", "db-0", "StatefulSet/db", ""},
		{"Pod", "agente-x", "DaemonSet/agente", ""},
		{"Deployment", "web", "Deployment/web", ""},
		{"Pod", "migrar-a", "", "Job não tem CronJob"},
		{"Pod", "solto", "", "pod não tem dono"},
		{"Pod", "rs-solto-a", "", "ReplicaSet não tem Deployment"},
		{"Pod", "rollout-a", "", "não corrige Rollout"},
	}
	for _, c := range casos {
		a, err := donoDe(context.Background(), client, "ns", c.kind, c.nome)
		if c.alvo != "" {
			if err != nil || a.recurso() != c.alvo || a.spec == nil || len(a.spec.Containers) != 1 {
				t.Errorf("%s/%s: esperava %s com o template, obtido %q (%v)", c.kind, c.nome, c.alvo, a.recurso(), err)
			}
			continue
		}
		var nc naoCorrigivel
		if !errors.As(err, &nc) || !strings.Contains(nc.motivo, c.motivo) {
			t.Errorf("%s/%s: esperava a recusa %q, obtido %v", c.kind, c.nome, c.motivo, err)
		}
	}

	// Recurso que sumiu é erro de leitura, não recusa.
	_, err := donoDe(context.Background(), client, "ns", "Pod", "sumiu")
	var nc naoCorrigivel
	if err == nil || errors.As(err, &nc) || motivoDaLeitura(err) != "o recurso não existe mais no cluster" {
		t.Errorf("pod que sumiu: %v", err)
	}
}

func TestGeridoPeloArgo(t *testing.T) {
	casos := []struct {
		meta metav1.ObjectMeta
		app  string
	}{
		{metav1.ObjectMeta{Annotations: map[string]string{anotacaoDoArgo: "payments:apps/Deployment:payments/web"}}, "payments"},
		{metav1.ObjectMeta{Labels: map[string]string{rotuloDoArgo: "payments"}}, "payments"},
		// O rótulo do Helm não é do Argo CD.
		{metav1.ObjectMeta{Labels: map[string]string{"app.kubernetes.io/instance": "web", "app.kubernetes.io/managed-by": "Helm"}}, ""},
	}
	for _, c := range casos {
		app, gerido := geridoPeloArgo(&appsv1.Deployment{ObjectMeta: c.meta})
		if app != c.app || gerido != (c.app != "") {
			t.Errorf("%+v: obtido %q %v", c.meta, app, gerido)
		}
	}
}
