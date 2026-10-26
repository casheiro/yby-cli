//go:build k8s

package backends

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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
			name: "binding system: deve ser ignorada",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "system:controller-manager"},
				"roleRef":  map[string]interface{}{"name": "cluster-admin"},
				"subjects": []interface{}{
					map[string]interface{}{"kind": "User", "name": "controller-manager", "namespace": ""},
				},
			},
			wantCount: 0,
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
			name: "role cert-manager deve ser ignorada",
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
			wantCount: 0,
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
	ctx := context.Background()
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
