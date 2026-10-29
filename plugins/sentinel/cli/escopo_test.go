//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	yerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/backends"
	"github.com/casheiro/yby-cli/v5/plugins/sentinel/cli/escopo"
	"github.com/muesli/termenv"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// backendDoEscopo guarda o escopo que o scan pôs no contexto e devolve um achado de um
// ClusterRoleBinding (escopo cluster) e um de um Deployment, sem escopo.
type backendDoEscopo struct{ visto *escopo.Escopo }

func (b backendDoEscopo) Name() string      { return "opa" }
func (b backendDoEscopo) IsAvailable() bool { return true }
func (b backendDoEscopo) ScanCluster(ctx context.Context, _ kubernetes.Interface, _ string) ([]backends.Finding, error) {
	*b.visto = escopo.De(ctx)
	return []backends.Finding{
		{ID: "opa/rbac_cluster_admin", Severity: "critical", Category: "rbac", Resource: "ClusterRoleBinding/ci", Message: "cluster-admin", Scope: "cluster"},
		{ID: "polaris/runAsRootAllowed", Severity: "critical", Category: "pod-security", Resource: "Deployment/api", Namespace: "payments", Message: "root"},
	}, nil
}

// O --scope chega aos backends pelo contexto, e o JSON diz o escopo do scan e o de cada
// achado (namespace quando o motor não diz).
func TestScan_EscopoNoContextoENoJSON(t *testing.T) {
	for _, tc := range []struct {
		nome   string
		args   []string
		escopo escopo.Escopo
	}{
		{"padrão", nil, escopo.Namespace},
		{"namespace", []string{"--scope", "namespace"}, escopo.Namespace},
		{"cluster", []string{"--scope=cluster"}, escopo.Cluster},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
			var visto escopo.Escopo
			usarDependencias(t, fake.NewSimpleClientset(), []backends.SecurityBackend{backendDoEscopo{&visto}}, nil, nil)

			saida, _, err := rodar(t, append([]string{"scan", "-n", "payments", "-o", "json", "--no-ai"}, tc.args...)...)
			if err != nil {
				t.Fatal(err)
			}
			if visto != tc.escopo {
				t.Errorf("o backend viu o escopo %q, queria %q", visto, tc.escopo)
			}
			var r struct {
				Scope    string `json:"scope"`
				Findings []struct {
					Resource string `json:"resource"`
					Scope    string `json:"scope"`
				} `json:"findings"`
			}
			if err := json.Unmarshal([]byte(saida), &r); err != nil {
				t.Fatalf("JSON inválido: %v\n%s", err, saida)
			}
			if r.Scope != string(tc.escopo) {
				t.Errorf("scope do relatório = %q, queria %q", r.Scope, tc.escopo)
			}
			escopos := map[string]string{}
			for _, f := range r.Findings {
				escopos[f.Resource] = f.Scope
			}
			if escopos["ClusterRoleBinding/ci"] != "cluster" || escopos["Deployment/api"] != "namespace" {
				t.Errorf("scope dos achados: %v", escopos)
			}
		})
	}
}

// No terminal, o título diz quando o scan é do cluster todo.
func TestScan_EscopoClusterNoTitulo(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	var visto escopo.Escopo
	usarDependencias(t, fake.NewSimpleClientset(), []backends.SecurityBackend{backendDoEscopo{&visto}}, nil, nil)

	saida, _, err := rodar(t, "scan", "-n", "payments", "--scope", "cluster", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "scan · namespace payments · escopo cluster") {
		t.Errorf("o título não diz o escopo cluster:\n%s", saida)
	}
	if md := exportScanMarkdown(ScanReport{Namespace: "payments", Scope: "cluster"}); !strings.Contains(md, "**Escopo:** cluster") {
		t.Errorf("o relatório não diz o escopo cluster:\n%s", md)
	}
}

// Um escopo que não existe é erro de validação, com a dica dos valores.
func TestLerOpcoes_EscopoDesconhecido(t *testing.T) {
	_, err := lerOpcoes("scan", []string{"--scope", "tudo"})
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || !strings.Contains(ye.Hint, "namespace ou cluster") {
		t.Fatalf("esperava ERR_VALIDATION com a dica dos escopos, obtido %v", err)
	}
	if _, err := lerOpcoes("scan", []string{"--scope"}); err == nil {
		t.Error("--scope sem valor tem de ser erro")
	}
}
