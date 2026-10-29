//go:build k8s

package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	yerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/backends"
	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/checks"
	"github.com/muesli/termenv"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// achadoEscalacao é o achado dos checks internos que o --fix corrige: o pod
// web-5d8b6-q7tvn é do ReplicaSet web-5d8b6, que é do Deployment web.
var achadoEscalacao = checks.SecurityFinding{
	CheckID: "POD_PRIVILEGE_ESCALATION", Severity: checks.SeverityMedium, Type: "medium",
	Category: checks.CategoryPodSecurity, Pod: "web-5d8b6-q7tvn", Container: "app", Namespace: "payments",
	Resource: "web-5d8b6-q7tvn/app", Message: "Container 'app' no pod 'web-5d8b6-q7tvn' permite escalação de privilégios",
}

// clusterDoDeployment monta o cluster falso com a cadeia Pod → ReplicaSet → Deployment e
// guarda os patches que chegam ao servidor. O dry-run é respondido pelo reactor sem
// gravar nada, como no servidor de verdade; falhaReal, quando não é nil, é a resposta
// do patch de verdade.
func clusterDoDeployment(falhaReal error) (*fake.Clientset, *[]k8stesting.PatchActionImpl) {
	controle := true
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "payments", UID: "dep-web"},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "web:1.0"}}},
		}},
	}
	rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
		Name: "web-5d8b6", Namespace: "payments", UID: "rs-web",
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "web", UID: "dep-web", Controller: &controle}},
	}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-5d8b6-q7tvn", Namespace: "payments",
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "web-5d8b6", UID: "rs-web", Controller: &controle}},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "web:1.0"}}},
	}
	client := fake.NewSimpleClientset(dep, rs, pod)
	var patches []k8stesting.PatchActionImpl
	client.PrependReactor("patch", "*", func(a k8stesting.Action) (bool, runtime.Object, error) {
		p := a.(k8stesting.PatchActionImpl)
		patches = append(patches, p)
		if len(p.PatchOptions.DryRun) > 0 {
			return true, dep, nil
		}
		if falhaReal != nil {
			return true, nil, falhaReal
		}
		return false, nil, nil
	})
	return client, &patches
}

// templateDoWeb devolve o securityContext do contêiner app no template do Deployment web.
func templateDoWeb(t *testing.T, client *fake.Clientset) *corev1.SecurityContext {
	t.Helper()
	dep, err := client.AppsV1().Deployments("payments").Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return dep.Spec.Template.Spec.Containers[0].SecurityContext
}

// TestScan_FixDryRunValidaNoServidor: o patch vai para o dono do pod (o Deployment, no
// caminho do template), o servidor o valida com DryRun All e nada é gravado.
func TestScan_FixDryRunValidaNoServidor(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	client, patches := clusterDoDeployment(nil)
	usarDependencias(t, client, nil, []checks.SecurityCheck{checkFalso{achados: []checks.SecurityFinding{achadoEscalacao}}}, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--fix-dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if len(*patches) != 1 {
		t.Fatalf("esperava 1 patch no servidor (o dry-run), obtidos %d", len(*patches))
	}
	p := (*patches)[0]
	if p.GetResource().Resource != "deployments" || p.Name != "web" || strings.Join(p.PatchOptions.DryRun, ",") != metav1.DryRunAll {
		t.Errorf("o dry-run vai ao Deployment web com DryRun All: %s %s %v", p.GetResource().Resource, p.Name, p.PatchOptions.DryRun)
	}
	if !strings.Contains(string(p.Patch), `"template":{"spec":{"containers":[{"name":"app","securityContext":{"allowPrivilegeEscalation":false}}]}}`) {
		t.Errorf("o patch muda o template dos pods: %s", p.Patch)
	}
	if sc := templateDoWeb(t, client); sc != nil {
		t.Errorf("o dry-run não grava nada: %+v", sc)
	}
	conferirGolden(t, "scan-dry-run.golden", saida)
}

// TestScan_FixSemTerminalPedeYes: fora de um terminal, o --fix não tem a quem perguntar e
// não aplica nada sem --yes.
func TestScan_FixSemTerminalPedeYes(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	client, patches := clusterDoDeployment(nil)
	usarDependencias(t, client, nil, []checks.SecurityCheck{checkFalso{achados: []checks.SecurityFinding{achadoEscalacao}}}, nil)

	_, _, err := rodar(t, "scan", "-n", "payments", "--fix")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || !strings.Contains(ye.Hint, "--yes") {
		t.Fatalf("esperava ERR_VALIDATION com --yes na dica, obtido %v", err)
	}
	if len(*patches) != 0 || templateDoWeb(t, client) != nil {
		t.Errorf("sem confirmação, nada vai ao servidor: %d patches", len(*patches))
	}
}

// TestScan_FixComYesAplicaNoDono: com --yes, o patch passa pelo dry-run e é aplicado no
// Deployment, não no pod.
func TestScan_FixComYesAplicaNoDono(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	client, patches := clusterDoDeployment(nil)
	usarDependencias(t, client, nil, []checks.SecurityCheck{checkFalso{achados: []checks.SecurityFinding{achadoEscalacao}}}, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--fix", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if len(*patches) != 2 || len((*patches)[0].PatchOptions.DryRun) == 0 || len((*patches)[1].PatchOptions.DryRun) != 0 {
		t.Fatalf("esperava o dry-run e depois o patch de verdade: %+v", *patches)
	}
	sc := templateDoWeb(t, client)
	if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Errorf("o template do Deployment ficou com allowPrivilegeEscalation false: %+v", sc)
	}
	for _, trecho := range []string{"REMEDIAÇÃO", "Deployment/web", "allowPrivilegeEscalation (vazio) → false", "✓ aplicar patches   1 aplicado"} {
		if !strings.Contains(saida, trecho) {
			t.Errorf("faltou %q:\n%s", trecho, saida)
		}
	}
}

// TestScan_FixPerguntaAntes: num terminal, o --fix mostra o plano e só aplica se a pessoa
// confirmar.
func TestScan_FixPerguntaAntes(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	for _, aceita := range []bool{false, true} {
		client, patches := clusterDoDeployment(nil)
		usarDependencias(t, client, nil, []checks.SecurityCheck{checkFalso{achados: []checks.SecurityFinding{achadoEscalacao}}}, nil)
		entradaTerminal = func() bool { return true }
		var perguntas []string
		confirmar = func(_ io.Writer, _, pergunta string) (bool, error) {
			perguntas = append(perguntas, pergunta)
			return aceita, nil
		}

		saida, _, err := rodar(t, "scan", "-n", "payments", "--fix")
		if err != nil {
			t.Fatal(err)
		}
		if len(perguntas) != 1 || !strings.Contains(perguntas[0], "1 patch") {
			t.Errorf("pergunta uma vez, com o tamanho do plano: %q", perguntas)
		}
		aplicado := templateDoWeb(t, client) != nil
		if aplicado != aceita {
			t.Errorf("aceita=%v, aplicado=%v (%d patches no servidor)\n%s", aceita, aplicado, len(*patches), saida)
		}
		if !aceita && !strings.Contains(saida, "nada foi aplicado") {
			t.Errorf("o cancelamento diz que nada foi aplicado:\n%s", saida)
		}
	}
}

// TestScan_FixQueFalhaSaiComErro: o patch que o servidor recusa sai como erro depois do
// resultado, com a dica da permissão do tipo do recurso.
func TestScan_FixQueFalhaSaiComErro(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	negado := apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "deployments"}, "web", errors.New(`User "dev" cannot patch resource "deployments"`))
	client, _ := clusterDoDeployment(negado)
	usarDependencias(t, client, nil, []checks.SecurityCheck{checkFalso{achados: []checks.SecurityFinding{achadoEscalacao}}}, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--fix", "--yes")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodePlugin || !strings.Contains(ye.Message, "1 de 1") || !strings.Contains(ye.Hint, "kubectl auth can-i patch deployments") {
		t.Fatalf("patches que falharam saem como erro com hint, obtido %v", err)
	}
	for _, trecho := range []string{"✕ aplicar patches", "1 de 1 falharam", "RESUMO"} {
		if !strings.Contains(saida, trecho) {
			t.Errorf("faltou %q:\n%s", trecho, saida)
		}
	}
}

// TestScan_FixDoPolaris: o achado do Polaris já vem no controlador; o --fix corrige os
// contêineres do template que ainda não têm a correção.
func TestScan_FixDoPolaris(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	client, patches := clusterDoDeployment(nil)
	polaris := backendFalso{nome: "polaris", achados: []backends.Finding{{
		ID: "polaris/insecureCapabilities", Severity: "medium", Category: "pod-security",
		Resource: "Deployment/web", Namespace: "payments", Message: "Container should not have insecure capabilities",
	}}}
	usarDependencias(t, client, []backends.SecurityBackend{polaris}, nil, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--fix-dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if len(*patches) != 1 || !strings.Contains(string((*patches)[0].Patch), `"capabilities":{"drop":["ALL"]}`) {
		t.Fatalf("esperava o dry-run do drop ALL no Deployment web: %+v", *patches)
	}
	if !strings.Contains(saida, "capabilities.drop (vazio) → [ALL]") {
		t.Errorf("a remediação mostra o que muda:\n%s", saida)
	}
}
