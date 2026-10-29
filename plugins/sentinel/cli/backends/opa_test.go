//go:build k8s

package backends

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/allowlist"
	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/escopo"
)

// TestOPABackendName verifica que o nome do backend é "opa".
func TestOPABackendName(t *testing.T) {
	backend := &OPABackend{}
	if got := backend.Name(); got != "opa" {
		t.Errorf("Name() = %q, esperado %q", got, "opa")
	}
}

// TestOPABackendIsAvailable verifica que o backend está sempre disponível.
func TestOPABackendIsAvailable(t *testing.T) {
	backend := &OPABackend{}
	if !backend.IsAvailable() {
		t.Error("IsAvailable() = false, esperado true")
	}
}

// TestOPABackendInterfaceCompliance verifica que OPABackend implementa SecurityBackend.
func TestOPABackendInterfaceCompliance(t *testing.T) {
	var _ SecurityBackend = (*OPABackend)(nil)
}

// TestEvaluateClusterAdminPolicy verifica a detecção de bindings cluster-admin.
func TestEvaluateClusterAdminPolicy(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		input     map[string]interface{}
		wantCount int
		wantMsg   string
	}{
		{
			name: "binding suspeita deve gerar violação",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "custom-admin-binding"},
				"roleRef":  map[string]interface{}{"name": "cluster-admin"},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "User", "name": "evil-user", "namespace": ""},
				},
			},
			wantCount: 1,
			wantMsg:   "ClusterRoleBinding 'custom-admin-binding' concede cluster-admin a User 'evil-user'",
		},
		{
			// O nome do binding não decide: quem cria o binding escolhe o nome.
			name: "binding system: com subject fora da allowlist gera violação",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "system:controller-manager"},
				"roleRef":  map[string]interface{}{"name": "cluster-admin"},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "User", "name": "controller-manager", "namespace": ""},
				},
			},
			wantCount: 1,
		},
		{
			name: "binding helm-qualquer para uma SA qualquer gera violação",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "helm-qualquer"},
				"roleRef":  map[string]interface{}{"name": "cluster-admin"},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "ServiceAccount", "name": "app", "namespace": "default"},
				},
			},
			wantCount: 1,
			wantMsg:   "ClusterRoleBinding 'helm-qualquer' concede cluster-admin a ServiceAccount 'app'",
		},
		{
			name: "nome de controller fora do namespace dele gera violação",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "argocd-application-controller"},
				"roleRef":  map[string]interface{}{"name": "cluster-admin"},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "ServiceAccount", "name": "argocd-application-controller", "namespace": "default"},
				},
			},
			wantCount: 1,
		},
		{
			name: "binding cluster-admin padrão deve ser ignorada",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "cluster-admin"},
				"roleRef":  map[string]interface{}{"name": "cluster-admin"},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "Group", "name": "system:masters", "namespace": ""},
				},
			},
			wantCount: 0,
		},
		{
			name: "binding argocd deve ser ignorada",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "argocd-application-controller"},
				"roleRef":  map[string]interface{}{"name": "cluster-admin"},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "ServiceAccount", "name": "argocd-application-controller", "namespace": "argocd"},
				},
			},
			wantCount: 0,
		},
		{
			name: "system:masters deve ser ignorado como subject",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "custom-binding"},
				"roleRef":  map[string]interface{}{"name": "cluster-admin"},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "Group", "name": "system:masters", "namespace": ""},
				},
			},
			wantCount: 0,
		},
		{
			name: "binding sem cluster-admin não gera violação",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "reader-binding"},
				"roleRef":  map[string]interface{}{"name": "view"},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "User", "name": "reader", "namespace": ""},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations, err := evaluatePolicy(ctx, regoClusterAdminPolicy, tt.input)
			if err != nil {
				t.Fatalf("evaluatePolicy() erro: %v", err)
			}
			if len(violations) != tt.wantCount {
				t.Errorf("esperado %d violações, obteve %d: %+v", tt.wantCount, len(violations), violations)
			}
			if tt.wantMsg != "" && len(violations) > 0 && violations[0].msg != tt.wantMsg {
				t.Errorf("mensagem esperada %q, obteve %q", tt.wantMsg, violations[0].msg)
			}
		})
	}
}

// TestEvaluateSecretsAccessPolicy verifica a detecção de acesso a secrets.
func TestEvaluateSecretsAccessPolicy(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		input     map[string]interface{}
		wantCount int
	}{
		{
			name: "role com acesso a secrets deve gerar violação",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "custom-secrets-reader"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"get", "list"},
						"resources": []interface{}{"secrets"},
						"apiGroups": []interface{}{""},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "role system: deve ser ignorada",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "system:controller:token-cleaner"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"get", "list"},
						"resources": []interface{}{"secrets"},
						"apiGroups": []interface{}{""},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "role admin builtin deve ser ignorada",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "admin"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"*"},
						"resources": []interface{}{"secrets"},
						"apiGroups": []interface{}{""},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "edit por RoleBinding a uma SA qualquer deve ser ignorada",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "edit"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"get", "list", "watch"},
						"resources": []interface{}{"secrets"},
						"apiGroups": []interface{}{""},
					},
				},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "ServiceAccount", "name": "app", "namespace": "default", "pelo_cluster": false},
				},
			},
			wantCount: 0,
		},
		{
			name: "edit por ClusterRoleBinding a uma SA qualquer gera violação",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "edit"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"get", "list", "watch"},
						"resources": []interface{}{"secrets"},
						"apiGroups": []interface{}{""},
					},
				},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "ServiceAccount", "name": "app", "namespace": "default", "pelo_cluster": true},
				},
			},
			wantCount: 1,
		},
		{
			name: "role ligado só ao cert-manager deve ser ignorada",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "cert-manager-controller-certificates"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"get", "list", "watch"},
						"resources": []interface{}{"secrets"},
						"apiGroups": []interface{}{""},
					},
				},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "ServiceAccount", "name": "cert-manager", "namespace": "cert-manager"},
				},
			},
			wantCount: 0,
		},
		{
			name: "nome do cert-manager sem binding não é ignorado",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "cert-manager-controller-certificates"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"get", "list", "watch"},
						"resources": []interface{}{"secrets"},
						"apiGroups": []interface{}{""},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "role sem acesso a secrets não gera violação",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "custom-role"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"get", "list"},
						"resources": []interface{}{"pods"},
						"apiGroups": []interface{}{""},
					},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations, err := evaluatePolicy(ctx, regoSecretsAccessPolicy, tt.input)
			if err != nil {
				t.Fatalf("evaluatePolicy() erro: %v", err)
			}
			if len(violations) != tt.wantCount {
				t.Errorf("esperado %d violações, obteve %d: %+v", tt.wantCount, len(violations), violations)
			}
		})
	}
}

// TestEvaluateWildcardPolicy verifica a detecção de permissões wildcard.
func TestEvaluateWildcardPolicy(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		input     map[string]interface{}
		wantCount int
	}{
		{
			name: "role com wildcard em verbs deve gerar violação",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "custom-wildcard"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"*"},
						"resources": []interface{}{"pods"},
						"apiGroups": []interface{}{""},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "role com wildcard em resources deve gerar violação",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "custom-wildcard-res"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"get"},
						"resources": []interface{}{"*"},
						"apiGroups": []interface{}{""},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "role sem wildcard não gera violação",
			input: map[string]interface{}{
				"kind":     "ClusterRole",
				"metadata": map[string]interface{}{"name": "safe-role"},
				"rules": []interface{}{
					map[string]interface{}{
						"verbs":     []interface{}{"get", "list"},
						"resources": []interface{}{"pods", "services"},
						"apiGroups": []interface{}{""},
					},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations, err := evaluatePolicy(ctx, regoWildcardPolicy, tt.input)
			if err != nil {
				t.Fatalf("evaluatePolicy() erro: %v", err)
			}
			if len(violations) != tt.wantCount {
				t.Errorf("esperado %d violações, obteve %d: %+v", tt.wantCount, len(violations), violations)
			}
		})
	}
}

// TestEvaluateDefaultDenyPolicy verifica a detecção de ausência de default-deny.
func TestEvaluateDefaultDenyPolicy(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		input     map[string]interface{}
		wantCount int
	}{
		{
			name: "namespace sem policies deve gerar 2 violações (ingress + egress)",
			input: map[string]interface{}{
				"namespace": "default",
				"policies":  []interface{}{},
			},
			wantCount: 2,
		},
		{
			name: "namespace com default-deny ingress e egress não gera violação",
			input: map[string]interface{}{
				"namespace": "secure-ns",
				"policies": []interface{}{
					map[string]interface{}{
						"spec": map[string]interface{}{
							"podSelector": map[string]interface{}{
								"matchLabels": map[string]interface{}{},
							},
							"policyTypes": []interface{}{"Ingress"},
							"ingress":     []interface{}{},
							"egress":      []interface{}{},
						},
					},
					map[string]interface{}{
						"spec": map[string]interface{}{
							"podSelector": map[string]interface{}{
								"matchLabels": map[string]interface{}{},
							},
							"policyTypes": []interface{}{"Egress"},
							"ingress":     []interface{}{},
							"egress":      []interface{}{},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "namespace apenas com default-deny ingress deve gerar 1 violação (egress)",
			input: map[string]interface{}{
				"namespace": "partial-ns",
				"policies": []interface{}{
					map[string]interface{}{
						"spec": map[string]interface{}{
							"podSelector": map[string]interface{}{
								"matchLabels": map[string]interface{}{},
							},
							"policyTypes": []interface{}{"Ingress"},
							"ingress":     []interface{}{},
							"egress":      []interface{}{},
						},
					},
				},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations, err := evaluatePolicy(ctx, regoDefaultDenyPolicy, tt.input)
			if err != nil {
				t.Fatalf("evaluatePolicy() erro: %v", err)
			}
			if len(violations) != tt.wantCount {
				t.Errorf("esperado %d violações, obteve %d: %+v", tt.wantCount, len(violations), violations)
			}
		})
	}
}

// TestScanClusterWithFakeClient verifica o fluxo completo de scan com cliente fake.
func TestScanClusterWithFakeClient(t *testing.T) {
	ctx := doCluster(context.Background())
	backend := &OPABackend{}

	// Criar cliente fake com um ClusterRoleBinding suspeito
	client := fake.NewSimpleClientset(
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "suspicious-binding"},
			RoleRef: rbacv1.RoleRef{
				Name:     "cluster-admin",
				Kind:     "ClusterRole",
				APIGroup: "rbac.authorization.k8s.io",
			},
			Subjects: []rbacv1.Subject{
				{Kind: "User", Name: "suspicious-user"},
			},
		},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "secrets-reader"},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list"},
					Resources: []string{"secrets"},
					APIGroups: []string{""},
				},
			},
		},
	)

	findings, err := backend.ScanCluster(ctx, client, "default")
	if err != nil {
		t.Fatalf("ScanCluster() erro: %v", err)
	}

	// Deve encontrar: cluster-admin binding + secrets access + default-deny (ingress + egress)
	if len(findings) < 1 {
		t.Errorf("esperado pelo menos 1 finding, obteve %d", len(findings))
	}

	// Verificar que o finding de cluster-admin está presente
	foundClusterAdmin := false
	foundSecrets := false
	foundDefaultDeny := false
	for _, f := range findings {
		switch f.ID {
		case "opa/rbac_cluster_admin":
			foundClusterAdmin = true
			if f.Source != "opa" {
				t.Errorf("Source esperado 'opa', obteve %q", f.Source)
			}
			if f.Category != "rbac" {
				t.Errorf("Category esperado 'rbac', obteve %q", f.Category)
			}
		case "opa/rbac_secrets_access":
			foundSecrets = true
		case "opa/netpol_default_deny":
			foundDefaultDeny = true
			if f.Category != "network" {
				t.Errorf("Category esperado 'network', obteve %q", f.Category)
			}
		}
	}

	if !foundClusterAdmin {
		t.Error("finding opa/rbac_cluster_admin não encontrado")
	}
	if !foundSecrets {
		t.Error("finding opa/rbac_secrets_access não encontrado")
	}
	if !foundDefaultDeny {
		t.Error("finding opa/netpol_default_deny não encontrado")
	}
}

// TestScanCluster_ClusterForaDoAr: com toda leitura do cluster falhando, o OPA devolve a
// falha (e nenhum achado) em vez de "0 achados", e não escreve nada no log acima do debug.
func TestScanCluster_ClusterForaDoAr(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("*", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("dial tcp 127.0.0.1:6443: connect: connection refused")
	})
	var log bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(anterior) })

	findings, err := (&OPABackend{}).ScanCluster(context.Background(), client, "payments")
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("esperava a falha do cluster, obtido %v (%d achados)", err, len(findings))
	}
	var parcial *FalhaParcial
	if errors.As(err, &parcial) || len(findings) != 0 {
		t.Errorf("nenhuma política rodou: é a falha comum, sem achados: %v, %+v", err, findings)
	}
	if log.Len() > 0 {
		t.Errorf("as falhas ficam no log de depuração:\n%s", log.String())
	}
}

// TestScanCluster_FalhaParcial: sem permissão para os ClusterRoles, as outras políticas
// rodam, os achados delas valem e o erro diz quais ficaram de fora.
func TestScanCluster_FalhaParcial(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "clusterroles", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterroles"}, "", errors.New("RBAC"))
	})

	findings, err := (&OPABackend{}).ScanCluster(context.Background(), client, "payments")
	var parcial *FalhaParcial
	if !errors.As(err, &parcial) {
		t.Fatalf("esperava *FalhaParcial, obtido %v", err)
	}
	if strings.Join(parcial.Falhas, ",") != "acesso a secrets,wildcards" || !apierrors.IsForbidden(parcial) {
		t.Errorf("falhas inesperadas: %+v", parcial)
	}
	if len(findings) != 2 {
		t.Errorf("os achados do default-deny (ingress e egress) valem, obtido %+v", findings)
	}
}

// clusterRolesFalsos devolve n ClusterRoles com wildcard e acesso a secrets.
func clusterRolesFalsos(n int) []runtime.Object {
	var objs []runtime.Object
	for i := 0; i < n; i++ {
		objs = append(objs, &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("papel-%d", i)},
			Rules: []rbacv1.PolicyRule{
				{Verbs: []string{"*"}, Resources: []string{"secrets"}, APIGroups: []string{""}},
			},
		})
	}
	return objs
}

// TestScanCluster_ListaClusterRolesUmaVez: wildcards e acesso a secrets usam a mesma
// listagem de ClusterRoles.
func TestScanCluster_ListaClusterRolesUmaVez(t *testing.T) {
	client := fake.NewSimpleClientset(clusterRolesFalsos(3)...)
	listagens := 0
	client.PrependReactor("list", "clusterroles", func(k8stesting.Action) (bool, runtime.Object, error) {
		listagens++
		return false, nil, nil
	})

	if _, err := (&OPABackend{}).ScanCluster(doCluster(context.Background()), client, "payments"); err != nil {
		t.Fatalf("ScanCluster: %v", err)
	}
	if listagens != 1 {
		t.Errorf("ClusterRoles listados %d vezes, esperava 1", listagens)
	}
}

// TestScanCluster_CompilaCadaPoliticaUmaVez: o módulo Rego é preparado uma vez por
// política, não uma vez por recurso avaliado.
func TestScanCluster_CompilaCadaPoliticaUmaVez(t *testing.T) {
	limparPoliticasPreparadas()
	antes := compilacoesRego.Load()
	client := fake.NewSimpleClientset(clusterRolesFalsos(50)...)

	if _, err := (&OPABackend{}).ScanCluster(doCluster(context.Background()), client, "payments"); err != nil {
		t.Fatalf("ScanCluster: %v", err)
	}
	if got := compilacoesRego.Load() - antes; got > 4 {
		t.Errorf("%d compilações para 50 ClusterRoles, esperava no máximo uma por política (4)", got)
	}
}

// BenchmarkScanCluster mede o scan com 500 ClusterRoles.
func BenchmarkScanCluster(b *testing.B) {
	client := fake.NewSimpleClientset(clusterRolesFalsos(500)...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := (&OPABackend{}).ScanCluster(doCluster(context.Background()), client, "payments"); err != nil {
			b.Fatal(err)
		}
	}
}

// limparPoliticasPreparadas esvazia o cache de políticas preparadas.
func limparPoliticasPreparadas() {
	politicasPreparadas.Lock()
	defer politicasPreparadas.Unlock()
	politicasPreparadas.m = map[string]*rego.PreparedEvalQuery{}
}

// crb monta um ClusterRoleBinding para o papel com um subject.
func crb(nome, papel string, s rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: nome},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: papel},
		Subjects:   []rbacv1.Subject{s},
	}
}

// papelCuringa monta um ClusterRole com tudo em tudo.
func papelCuringa(nome string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: nome},
		Rules:      []rbacv1.PolicyRule{{Verbs: []string{"*"}, Resources: []string{"*"}, APIGroups: []string{"*"}}},
	}
}

// TestScanCluster_AllowlistPorSubject: a allowlist casa pelo subject. O "helm-qualquer"
// que dá cluster-admin a default/app aparece, e com ele o ClusterRole cluster-admin (que
// deixa de estar ligado no cluster todo só a system:masters); o binding e o ClusterRole
// do Argo CD e o cluster-admin padrão ficam de fora e são contados uma vez cada; um
// ClusterRole com nome do Argo CD ligado por RoleBinding a uma SA qualquer aparece.
func TestScanCluster_AllowlistPorSubject(t *testing.T) {
	sa := func(ns, nome string) rbacv1.Subject {
		return rbacv1.Subject{Kind: "ServiceAccount", Name: nome, Namespace: ns}
	}
	client := fake.NewSimpleClientset(
		crb("helm-qualquer", "cluster-admin", sa("default", "app")),
		crb("cluster-admin", "cluster-admin", rbacv1.Subject{Kind: "Group", Name: "system:masters"}),
		crb("argocd-application-controller", "argocd-application-controller", sa("argocd", "argocd-application-controller")),
		papelCuringa("argocd-application-controller"),
		papelCuringa("cluster-admin"),
		papelCuringa("argocd-maligno"),
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "qualquer", Namespace: "loja"},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "argocd-maligno"},
			Subjects:   []rbacv1.Subject{sa("loja", "app")},
		},
	)
	ctx, contagem := allowlist.ComContagem(doCluster(context.Background()))

	findings, err := (&OPABackend{}).ScanCluster(ctx, client, "payments")
	if err != nil {
		t.Fatalf("ScanCluster: %v", err)
	}
	var rbac []string
	for _, f := range findings {
		if f.Category == "rbac" {
			rbac = append(rbac, f.ID+" "+f.Resource+" "+f.Severity)
		}
	}
	quer := []string{
		"opa/rbac_cluster_admin ClusterRoleBinding/helm-qualquer critical",
		"opa/rbac_secrets_access ClusterRole/argocd-maligno high",
		"opa/rbac_secrets_access ClusterRole/cluster-admin high",
		"opa/rbac_wildcard ClusterRole/argocd-maligno high",
		"opa/rbac_wildcard ClusterRole/cluster-admin high",
	}
	if strings.Join(rbac, "|") != strings.Join(quer, "|") {
		t.Errorf("achados de RBAC:\n%s\nqueria:\n%s", strings.Join(rbac, "\n"), strings.Join(quer, "\n"))
	}
	if got := contagem.Total(); got != 2 {
		t.Errorf("ignorados pela allowlist = %d, queria 2 (binding cluster-admin e ClusterRole argocd-application-controller)", got)
	}
}

// TestScanCluster_PapelDoKubernetesNoClusterTodo: edit ligado por RoleBinding (vale num
// namespace só) fica de fora e é contado; o mesmo edit dado a default/app por um
// ClusterRoleBinding deixa a SA ler os secrets do cluster todo e aparece, sem contar.
func TestScanCluster_PapelDoKubernetesNoClusterTodo(t *testing.T) {
	edit := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "edit"},
		Rules:      []rbacv1.PolicyRule{{Verbs: []string{"get", "list", "watch"}, Resources: []string{"secrets"}, APIGroups: []string{""}}},
	}
	app := rbacv1.Subject{Kind: "ServiceAccount", Name: "app", Namespace: "default"}
	porRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "app-edit", Namespace: "default"},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "edit"},
		Subjects:   []rbacv1.Subject{app},
	}
	achadosDeSecrets := func(ctx context.Context, t *testing.T, client *fake.Clientset) []string {
		t.Helper()
		findings, err := (&OPABackend{}).ScanCluster(ctx, client, "default")
		if err != nil {
			t.Fatalf("ScanCluster: %v", err)
		}
		var r []string
		for _, f := range findings {
			if f.ID == "opa/rbac_secrets_access" {
				r = append(r, f.Resource)
			}
		}
		return r
	}

	t.Run("por RoleBinding", func(t *testing.T) {
		ctx, contagem := allowlist.ComContagem(context.Background())
		got := achadosDeSecrets(ctx, t, fake.NewSimpleClientset(edit.DeepCopy(), porRoleBinding.DeepCopy()))
		if len(got) != 0 || contagem.Total() != 1 {
			t.Errorf("esperava o edit ignorado e contado: %v, %d ignorados", got, contagem.Total())
		}
	})

	t.Run("por ClusterRoleBinding", func(t *testing.T) {
		client := fake.NewSimpleClientset(edit.DeepCopy(), porRoleBinding.DeepCopy(), crb("app-edit-tudo", "edit", app))
		ctx, contagem := allowlist.ComContagem(context.Background())
		got := achadosDeSecrets(ctx, t, client)
		if strings.Join(got, "|") != "ClusterRole/edit" || contagem.Total() != 0 {
			t.Errorf("esperava achado no ClusterRole/edit e nada ignorado: %v, %d ignorados", got, contagem.Total())
		}
	})
}

// TestAllowlist_GoERegoDecidemIgual: os checks internos (Go) e o OPA (Rego) leem a mesma
// lista. Cada controller da allowlist.Padrao, e a mesma SA fora do namespace dele, dá a
// mesma decisão nos dois lados, no binding de cluster-admin e no role curinga.
func TestAllowlist_GoERegoDecidemIgual(t *testing.T) {
	ctx := context.Background()
	var sujeitos []allowlist.Sujeito
	for _, c := range allowlist.Padrao.Controladores {
		for _, ns := range c.Namespaces {
			sujeitos = append(sujeitos,
				allowlist.Sujeito{Kind: "ServiceAccount", Nome: c.PrefixoSA + "x", Namespace: ns},
				allowlist.Sujeito{Kind: "ServiceAccount", Nome: c.PrefixoSA + "x", Namespace: ns + "-outro"},
			)
		}
	}
	for _, g := range append(append([]string(nil), allowlist.Padrao.GruposAdmin...), allowlist.Padrao.GruposDoSistema...) {
		sujeitos = append(sujeitos, allowlist.Sujeito{Kind: "Group", Nome: g})
	}
	for _, u := range allowlist.Padrao.UsuariosDoSistema {
		sujeitos = append(sujeitos, allowlist.Sujeito{Kind: "User", Nome: u})
	}
	sujeitos = append(sujeitos,
		allowlist.Sujeito{Kind: "Group", Nome: "system:authenticated"},
		allowlist.Sujeito{Kind: "ServiceAccount", Nome: "default", Namespace: allowlist.Padrao.NamespaceDoSistema},
	)

	for _, s := range sujeitos {
		binding := bindingToInput(rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "b"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"},
			Subjects:   []rbacv1.Subject{{Kind: s.Kind, Name: s.Nome, Namespace: s.Namespace}},
		})
		r, err := avaliarPolitica(ctx, regoClusterAdminPolicy, binding)
		if err != nil {
			t.Fatal(err)
		}
		if goIgnora := allowlist.Padrao.SujeitoPermitido(s); r.ignorado != goIgnora || (len(r.violacoes) == 0) != goIgnora {
			t.Errorf("cluster-admin para %+v: Rego ignorado=%v violações=%d, Go ignora=%v", s, r.ignorado, len(r.violacoes), goIgnora)
		}

		// O mesmo subject chegando por RoleBinding e por ClusterRoleBinding.
		pelaRole, peloCluster := s, s
		pelaRole.PeloCluster, peloCluster.PeloCluster = false, true
		for _, ligado := range []allowlist.Sujeito{pelaRole, peloCluster} {
			for _, papel := range append([]string{"qualquer", allowlist.Padrao.PrefixoDoSistema + "qualquer"}, allowlist.Padrao.PapeisDoKubernetes...) {
				sujeitosDoPapel := []allowlist.Sujeito{ligado}
				input := roleToInput("ClusterRole", papel, "", papelCuringa(papel).Rules, sujeitosDoPapel)
				r, err := avaliarPolitica(ctx, regoWildcardPolicy, input)
				if err != nil {
					t.Fatal(err)
				}
				if goIgnora := allowlist.Padrao.IgnorarPapel("ClusterRole", papel, sujeitosDoPapel); r.ignorado != goIgnora || (len(r.violacoes) == 0) != goIgnora {
					t.Errorf("ClusterRole %s ligado a %+v: Rego ignorado=%v violações=%d, Go ignora=%v", papel, ligado, r.ignorado, len(r.violacoes), goIgnora)
				}
			}
		}
	}
}

// TestScanCluster_SoORBACDoNamespace: no scan de um namespace só entra o RBAC que vale
// nele. O cluster-admin dado a other/x, o ClusterRole curinga sem binding e o ligado por
// RoleBinding em outro namespace ficam de fora; o ClusterRoleBinding que dá cluster-admin
// a payments/x aparece só com o subject do namespace, o ClusterRole ligado por RoleBinding
// de payments e o Role curinga de payments aparecem, cada um com o escopo dele.
func TestScanCluster_SoORBACDoNamespace(t *testing.T) {
	sa := func(ns, nome string) rbacv1.Subject {
		return rbacv1.Subject{Kind: "ServiceAccount", Name: nome, Namespace: ns}
	}
	rb := func(ns, nome, papel string, s rbacv1.Subject) *rbacv1.RoleBinding {
		return &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: nome, Namespace: ns},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: papel},
			Subjects:   []rbacv1.Subject{s},
		}
	}
	doPayments := crb("pay-admin", "cluster-admin", sa("payments", "x"))
	doPayments.Subjects = append([]rbacv1.Subject{{Kind: "User", Name: "bob"}}, doPayments.Subjects...)
	client := fake.NewSimpleClientset(
		crb("outro-admin", "cluster-admin", sa("other", "x")),
		doPayments,
		papelCuringa("curinga-solto"),
		papelCuringa("curinga-pay"),
		papelCuringa("curinga-loja"),
		rb("payments", "app-curinga", "curinga-pay", sa("payments", "app")),
		rb("loja", "app-curinga", "curinga-loja", sa("loja", "app")),
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "faz-tudo", Namespace: "payments"},
			Rules:      []rbacv1.PolicyRule{{Verbs: []string{"*"}, Resources: []string{"pods"}, APIGroups: []string{""}}},
		},
	)

	findings, err := (&OPABackend{}).ScanCluster(context.Background(), client, "payments")
	if err != nil {
		t.Fatalf("ScanCluster: %v", err)
	}
	var rbac []string
	for _, f := range findings {
		if f.Category == "rbac" {
			rbac = append(rbac, fmt.Sprintf("%s %s %s · %s", f.ID, f.Resource, f.Scope, f.Message))
		}
	}
	sort.Strings(rbac)
	quer := []string{
		"opa/rbac_cluster_admin ClusterRoleBinding/pay-admin cluster · ClusterRoleBinding 'pay-admin' concede cluster-admin a ServiceAccount 'x'",
		"opa/rbac_secrets_access ClusterRole/curinga-pay cluster · ClusterRole 'curinga-pay' permite leitura de secrets",
		"opa/rbac_wildcard ClusterRole/curinga-pay cluster · ClusterRole 'curinga-pay' possui permissões wildcard (*)",
		"opa/rbac_wildcard Role/faz-tudo namespace · Role 'faz-tudo' possui permissões wildcard (*)",
	}
	if strings.Join(rbac, "\n") != strings.Join(quer, "\n") {
		t.Errorf("achados de RBAC do namespace:\n%s\nqueria:\n%s", strings.Join(rbac, "\n"), strings.Join(quer, "\n"))
	}

	// Com --scope cluster, entra o RBAC do cluster todo: os 2 bindings (3 subjects) e os 3
	// ClusterRoles curinga, cada um nas duas políticas, e o Role curinga.
	findings, err = (&OPABackend{}).ScanCluster(doCluster(context.Background()), client, "payments")
	if err != nil {
		t.Fatalf("ScanCluster: %v", err)
	}
	porID := map[string]int{}
	for _, f := range findings {
		porID[f.ID]++
	}
	if porID["opa/rbac_cluster_admin"] != 3 || porID["opa/rbac_secrets_access"] != 3 || porID["opa/rbac_wildcard"] != 4 {
		t.Errorf("no escopo cluster, achados por check: %v", porID)
	}
}

// doCluster põe o escopo cluster no contexto: os testes que olham o RBAC do cluster todo.
func doCluster(ctx context.Context) context.Context { return escopo.Com(ctx, escopo.Cluster) }
