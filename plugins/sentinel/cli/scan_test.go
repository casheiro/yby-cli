//go:build k8s

package main

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// --- checkRootContainer ---

func TestCheckRootContainer_RunAsRoot(t *testing.T) {
	var uid int64 = 0
	container := corev1.Container{
		Name: "app",
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: &uid,
		},
	}
	pod := corev1.Pod{}
	pod.Name = "pod-root"

	findings := checkRootContainer(pod, container, "default")

	if len(findings) == 0 {
		t.Fatal("esperava pelo menos um finding para container rodando como root")
	}
	found := false
	for _, f := range findings {
		if f.Type == "critical" && f.Category == "root_container" {
			found = true
			break
		}
	}
	if !found {
		t.Error("esperava finding crítico de root_container para RunAsUser=0")
	}
}

func TestCheckRootContainer_SemSecurityContext(t *testing.T) {
	container := corev1.Container{
		Name:            "app",
		SecurityContext: nil,
	}
	pod := corev1.Pod{}
	pod.Name = "pod-nosec"

	findings := checkRootContainer(pod, container, "default")

	if len(findings) == 0 {
		t.Fatal("esperava finding para container sem SecurityContext")
	}
	found := false
	for _, f := range findings {
		if f.Type == "warning" && f.Category == "root_container" {
			found = true
			break
		}
	}
	if !found {
		t.Error("esperava finding de aviso de root_container quando SecurityContext é nil")
	}
}

func TestCheckRootContainer_RunAsNonRootTrue(t *testing.T) {
	nonRoot := true
	container := corev1.Container{
		Name: "app",
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot: &nonRoot,
		},
	}
	pod := corev1.Pod{}
	pod.Name = "pod-nonroot"

	findings := checkRootContainer(pod, container, "default")

	if len(findings) != 0 {
		t.Errorf("esperava zero findings para container com RunAsNonRoot=true, obteve %d", len(findings))
	}
}

// --- checkResourceLimits ---

func TestCheckResourceLimits_SemLimites(t *testing.T) {
	container := corev1.Container{
		Name: "app",
	}
	pod := corev1.Pod{}
	pod.Name = "pod-nolimits"

	findings := checkResourceLimits(pod, container, "default")

	if len(findings) == 0 {
		t.Fatal("esperava finding para container sem limites de recursos")
	}
	if findings[0].Type != "warning" || findings[0].Category != "no_limits" {
		t.Errorf("esperava finding warning/no_limits, obteve %s/%s", findings[0].Type, findings[0].Category)
	}
}

func TestCheckResourceLimits_ComLimites(t *testing.T) {
	container := corev1.Container{
		Name: "app",
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
	}
	pod := corev1.Pod{}
	pod.Name = "pod-limits"

	findings := checkResourceLimits(pod, container, "default")

	if len(findings) != 0 {
		t.Errorf("esperava zero findings para container com limites definidos, obteve %d", len(findings))
	}
}

// --- checkImagePullPolicy ---

func TestCheckImagePullPolicy_NaoAlways(t *testing.T) {
	container := corev1.Container{
		Name:            "app",
		ImagePullPolicy: corev1.PullIfNotPresent,
	}
	pod := corev1.Pod{}
	pod.Name = "pod-pullpolicy"

	findings := checkImagePullPolicy(pod, container, "default")

	if len(findings) == 0 {
		t.Fatal("esperava finding para ImagePullPolicy != Always")
	}
	if findings[0].Type != "warning" || findings[0].Category != "image_pull_policy" {
		t.Errorf("esperava finding warning/image_pull_policy, obteve %s/%s", findings[0].Type, findings[0].Category)
	}
}

func TestCheckImagePullPolicy_Always(t *testing.T) {
	container := corev1.Container{
		Name:            "app",
		ImagePullPolicy: corev1.PullAlways,
	}
	pod := corev1.Pod{}
	pod.Name = "pod-always"

	findings := checkImagePullPolicy(pod, container, "default")

	if len(findings) != 0 {
		t.Errorf("esperava zero findings para ImagePullPolicy=Always, obteve %d", len(findings))
	}
}

// --- checkExposedSecrets ---

func TestCheckExposedSecrets_EnvHardcoded(t *testing.T) {
	container := corev1.Container{
		Name: "app",
		Env: []corev1.EnvVar{
			{Name: "DB_PASSWORD", Value: "senha-hardcoded"},
		},
	}
	pod := corev1.Pod{}
	pod.Name = "pod-secrets"

	findings := checkExposedSecrets(pod, container, "default")

	if len(findings) == 0 {
		t.Fatal("esperava finding para env var sensível com valor hardcoded")
	}
	if findings[0].Type != "critical" || findings[0].Category != "exposed_secrets" {
		t.Errorf("esperava finding critical/exposed_secrets, obteve %s/%s", findings[0].Type, findings[0].Category)
	}
	if !strings.Contains(findings[0].Description, "DB_PASSWORD") {
		t.Errorf("descrição deveria mencionar o nome da variável DB_PASSWORD, obteve: %s", findings[0].Description)
	}
}

func TestCheckExposedSecrets_EnvViaSecretKeyRef(t *testing.T) {
	container := corev1.Container{
		Name: "app",
		Env: []corev1.EnvVar{
			{
				Name: "DB_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "db-secret"},
						Key:                  "password",
					},
				},
			},
		},
	}
	pod := corev1.Pod{}
	pod.Name = "pod-secretref"

	findings := checkExposedSecrets(pod, container, "default")

	if len(findings) != 0 {
		t.Errorf("esperava zero findings para env via secretKeyRef, obteve %d", len(findings))
	}
}

// --- exportScanMarkdown ---

func TestExportScanMarkdown_SemFindings(t *testing.T) {
	report := ScanReport{
		Namespace: "prod",
		Findings:  []SecurityFinding{},
	}

	content := exportScanMarkdown(report)

	if !strings.Contains(content, "prod") {
		t.Error("markdown deveria conter o nome do namespace")
	}
	if !strings.Contains(content, "0 críticos, 0 avisos") {
		t.Errorf("resumo esperado '0 críticos, 0 avisos', conteúdo: %s", content)
	}
	if strings.Contains(content, "## Recomendações") {
		t.Error("não deveria conter seção de recomendações quando não há findings")
	}
}

func TestExportScanMarkdown_ContaCriticaisEAvisos(t *testing.T) {
	report := ScanReport{
		Namespace: "staging",
		Findings: []SecurityFinding{
			{Type: "critical", Category: "root_container", Resource: "pod-a/app", Namespace: "staging", Description: "roda como root"},
			{Type: "critical", Category: "exposed_secrets", Resource: "pod-b/app", Namespace: "staging", Description: "senha hardcoded"},
			{Type: "warning", Category: "no_limits", Resource: "pod-c/app", Namespace: "staging", Description: "sem limites"},
		},
	}

	content := exportScanMarkdown(report)

	if !strings.Contains(content, "2 críticos, 1 avisos") {
		t.Errorf("resumo esperado '2 críticos, 1 avisos', conteúdo: %s", content)
	}
}

func TestExportScanMarkdown_ComRecomendacoes(t *testing.T) {
	report := ScanReport{
		Namespace: "default",
		Findings: []SecurityFinding{
			{Type: "warning", Category: "no_limits", Resource: "pod-a/app", Namespace: "default", Description: "sem limites"},
		},
		Recommendations: "Adicione limites de CPU e memória a todos os containers.",
	}

	content := exportScanMarkdown(report)

	if !strings.Contains(content, "## Recomendações") {
		t.Error("markdown deveria conter seção '## Recomendações'")
	}
	if !strings.Contains(content, "Adicione limites de CPU e memória") {
		t.Errorf("markdown deveria conter o texto das recomendações, conteúdo: %s", content)
	}
}
