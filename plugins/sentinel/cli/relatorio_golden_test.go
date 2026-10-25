//go:build k8s

package main

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
)

// atualizarGolden regrava os arquivos golden: go test -tags k8s ./plugins/sentinel/cli/ -run <Teste> -update
var atualizarGolden = flag.Bool("update", false, "regrava os arquivos golden em testdata/")

// conferirGolden compara obtido com testdata/<nome>; com -update, regrava o arquivo.
func conferirGolden(t *testing.T, nome, obtido string) {
	t.Helper()
	caminho := filepath.Join("testdata", nome)
	if *atualizarGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(obtido), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	esperado, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("golden %s ausente (rode com -update): %v", caminho, err)
	}
	if string(esperado) != obtido {
		t.Errorf("saída diferente do golden %s\n--- esperado ---\n%s\n--- obtido ---\n%s", caminho, esperado, obtido)
	}
}

// achadosDeExemplo são os achados das goldens: dois backends, severidades e categorias
// variadas, fora de ordem, como chegam do Polaris (que agrupa num map).
func achadosDeExemplo() []checks.SecurityFinding {
	return []checks.SecurityFinding{
		{CheckID: "polaris/memoryLimitsMissing", Severity: "high", Type: "high", Category: checks.CategoryPodSecurity, Namespace: "payments", Resource: "Deployment/web (+2)", Message: "Memory limits should be set", Description: "Memory limits should be set", Recommendation: "Defina resources.limits.memory"},
		{CheckID: "opa/rbac-cluster-admin", Severity: "critical", Type: "critical", Category: checks.CategoryRBAC, Namespace: "payments", Resource: "ClusterRoleBinding/ci-deployer", Message: "ServiceAccount payments/ci com cluster-admin", Description: "ServiceAccount payments/ci com cluster-admin", Recommendation: "Substitua cluster-admin por roles mais restritivos"},
		{CheckID: "polaris/runAsRootAllowed", Severity: "critical", Type: "critical", Category: checks.CategoryPodSecurity, Namespace: "payments", Resource: "Deployment/api", Message: "Should not be allowed to run as root", Description: "Should not be allowed to run as root"},
		{CheckID: "opa/netpol-default-deny", Severity: "medium", Type: "medium", Category: checks.CategoryNetwork, Namespace: "payments", Resource: "Namespace/payments", Message: "Namespace sem NetworkPolicy default-deny", Description: "Namespace sem NetworkPolicy default-deny"},
		{CheckID: "polaris/pullPolicyNotAlways", Severity: "low", Type: "low", Category: checks.CategorySupplyChain, Namespace: "payments", Resource: "StatefulSet/redis", Message: "Image pull policy should be Always", Description: "Image pull policy should be Always"},
	}
}

// dataDoRelatorio é a linha **Data:** do relatório do investigate, que vem de time.Now.
var dataDoRelatorio = regexp.MustCompile(`\*\*Data:\*\* [0-9: -]+`)

// TestRelatorioMarkdown_NaoMuda prende o formato dos relatórios markdown (o do scan, salvo
// em ~/.yby/reports, e o do investigate): a migração da saída para o pkg/ui não os toca,
// salvo os emojis de severidade do scan, trocados pelos glifos ▲, ✕ e ○, e o resumo, que
// passou a contar por severidade como o terminal (T034).
func TestRelatorioMarkdown_NaoMuda(t *testing.T) {
	scan := exportScanMarkdown(ScanReport{
		Namespace:       "payments",
		Findings:        achadosDeExemplo(),
		Sources:         []string{"polaris", "opa"},
		Recommendations: "1. Tire o cluster-admin do ci-deployer.\n2. Defina limites de memória.",
	})
	conferirGolden(t, "relatorio-scan.md", scan)

	patch := `kubectl -n payments set env deploy/worker DB_HOST=postgres.payments`
	inv := exportMarkdown(AnalysisResult{
		RootCause:       "O worker não encontra o banco",
		TechnicalDetail: "DB_HOST aponta para localhost e o contêiner reinicia a cada 30 s.",
		Confidence:      82,
		SuggestedFix:    "Aponte DB_HOST para o Service do Postgres.",
		KubectlPatch:    &patch,
	}, "worker-5d8b6-q7tvn", "payments")
	conferirGolden(t, "relatorio-investigate.md", dataDoRelatorio.ReplaceAllString(inv, "**Data:** <data>"))
}
