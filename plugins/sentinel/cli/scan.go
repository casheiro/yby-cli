//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/backends"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/profiles"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/remediation"
	"k8s.io/client-go/kubernetes"
)

// ScanReport contém o resultado completo do scan de segurança.
type ScanReport struct {
	Namespace       string                   `json:"namespace"`
	Findings        []checks.SecurityFinding `json:"findings"`
	Sources         []string                 `json:"sources,omitempty"`
	Recommendations string                   `json:"recommendations,omitempty"`
}

// Dependências do scan e do investigate, substituíveis nos testes.
var (
	novoClienteKube = getKubeClient
	backendsDoScan  = func() []backends.SecurityBackend {
		return []backends.SecurityBackend{backends.NewPolarisBackend(), backends.NewOPABackend()}
	}
	checksDoScan = checks.GetAll
	provedorDeIA = func(ctx context.Context) ai.Provider { return ai.GetProvider(ctx, "auto") }
)

// semCluster é o erro de quando o cliente Kubernetes não pode ser montado.
func semCluster(err error) error {
	return yerrors.Wrap(err, yerrors.ErrCodeClusterOffline, "não deu para montar o cliente do Kubernetes").
		WithHint("confira o kubeconfig e o contexto com kubectl config current-context")
}

// scanNamespace examina o namespace e escreve o resultado em saida. O título e as etapas
// vão para saida, ou para erros quando o resultado é para scripts (json ou markdown sem
// arquivo).
func scanNamespace(ctx context.Context, saida, erros io.Writer, op opcoes) error {
	namespace := op.namespace
	if namespace == "" {
		namespace = "default"
	}
	var selecionados []checks.SecurityCheck
	if op.perfil != "" {
		p, ok := profiles.GetProfile(op.perfil)
		if !ok {
			return yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("o perfil %s não existe", op.perfil)).
				WithHint("use --profile " + listaComOu(nomesDosPerfis()))
		}
		selecionados = checks.GetByIDs(p.CheckIDs)
	} else {
		selecionados = checksDoScan()
	}

	k8sClient, err := novoClienteKube()
	if err != nil {
		return semCluster(err)
	}

	// O título só sai depois das verificações rápidas (perfil e cliente), para o erro
	// delas não ficar separado dele por duas linhas em branco.
	telaDasEtapas := saida
	if op.paraScripts() {
		telaDasEtapas = erros
	} else {
		ui.Title(saida, "sentinel", "scan · namespace "+namespace)
		fmt.Fprintln(saida)
	}

	prog := novoProgresso(telaDasEtapas)
	parar := animar(prog, erros)
	report, patches, falhaPatches, err := coletarScan(ctx, prog, k8sClient, namespace, selecionados, op)
	parar()
	if err != nil {
		return err
	}
	if err := escreverResultadoScan(saida, report, patches, namespace, op); err != nil {
		return err
	}
	// Patches que o --fix não aplicou saem como erro depois do resultado, para o código de
	// saída dizer a scripts e à CI que nada (ou nem tudo) foi corrigido.
	return falhaPatches
}

// escreverResultadoScan escreve o json ou o markdown (no stdout ou no arquivo) ou, no
// formato terminal, o resumo na tela, com o relatório completo em ~/.yby/reports/.
func escreverResultadoScan(saida io.Writer, report ScanReport, patches []remediation.RemediationPatch, namespace string, op opcoes) error {
	switch op.formato {
	case "json":
		data, _ := json.MarshalIndent(report, "", "  ")
		return gravarRelatorio(saida, string(data), op.arquivo, "json")
	case "markdown":
		return gravarRelatorio(saida, exportScanMarkdown(report), op.arquivo, "markdown")
	}
	if len(report.Findings) == 0 {
		fmt.Fprintln(saida)
		ui.Check(saida, ui.CheckOk, "nenhum problema de segurança encontrado", "", "")
		ui.Campos(saida, ui.Campo{Rotulo: "fontes", Valor: nomesDasFontes(report.Sources)})
		return nil
	}
	caminho, errRelatorio := saveReportToGlobal(report, namespace)
	escreverScan(saida, report, patches, op, caminho, errRelatorio)
	return nil
}

// coletarScan roda os backends (ou os checks internos), as recomendações da IA e, com
// --fix, os patches, uma etapa de prog para cada. Devolve os patches gerados com --fix ou
// --fix-dry-run e, em falhaPatches, o erro dos patches que o --fix não aplicou; err é a
// falha que interrompe o scan.
func coletarScan(ctx context.Context, prog *ui.Progresso, client kubernetes.Interface, namespace string, selecionados []checks.SecurityCheck, op opcoes) (report ScanReport, patches []remediation.RemediationPatch, falhaPatches, err error) {
	var findings []checks.SecurityFinding
	var sources []string
	consultou := false
	var primeiroErro error

	// 1. Backends de segurança (Polaris, OPA)
	for _, b := range backendsDoScan() {
		if !b.IsAvailable() {
			continue
		}
		prog.Iniciar(b.Name())
		bf, errBackend := b.ScanCluster(ctx, client, namespace)
		if errBackend != nil {
			prog.Falhar("falhou")
			prog.Detalhe(errBackend.Error())
			if primeiroErro == nil {
				primeiroErro = errBackend
			}
			continue
		}
		consultou = true
		prog.Concluir(contar(len(bf), "achado", "achados"))
		if len(bf) > 0 {
			sources = append(sources, b.Name())
		}
		for _, f := range bf {
			findings = append(findings, checks.SecurityFinding{
				CheckID:        f.ID,
				Severity:       checks.Severity(f.Severity),
				Category:       checks.Category(f.Category),
				Namespace:      f.Namespace,
				Resource:       f.Resource,
				Message:        f.Message,
				Recommendation: f.Recommendation,
				Type:           f.Severity,
				Description:    f.Message,
			})
		}
	}

	// 2. Sem achados dos backends, os checks internos
	if len(sources) == 0 {
		sources = append(sources, "checks-internos")
		prog.Iniciar("checks internos")
		var falhas []string
		antes := len(findings)
		for _, check := range selecionados {
			cf, errCheck := check.Run(ctx, client, namespace)
			if errCheck != nil {
				falhas = append(falhas, check.Name())
				if primeiroErro == nil {
					primeiroErro = errCheck
				}
				continue
			}
			consultou = true
			findings = append(findings, cf...)
		}
		if !consultou && primeiroErro != nil {
			prog.Falhar("falhou")
			return ScanReport{}, nil, nil, yerrors.Wrap(primeiroErro, yerrors.ErrCodeClusterOffline,
				"não deu para consultar o cluster").
				WithContext("namespace", namespace).
				WithHint("confira se o cluster responde com kubectl get pods -n " + namespace)
		}
		prog.Concluir(contar(len(findings)-antes, "achado", "achados"))
		if len(falhas) > 0 {
			prog.Avisar(fmt.Sprintf("%s: %s", contar(len(falhas), "check falhou", "checks falharam"), strings.Join(falhas, ", ")))
		}
	}

	// 3. Deduplicar findings (mesmo recurso + mesma mensagem)
	findings = deduplicateFindings(findings)
	report = ScanReport{Namespace: namespace, Findings: findings, Sources: sources}
	if len(findings) == 0 {
		return report, nil, nil, nil
	}

	// 4. Recomendações via IA
	if provider := provedorDeIA(ctx); provider != nil {
		prog.Iniciar("recomendações da IA")
		findingsJSON, _ := json.MarshalIndent(findings, "", "  ")
		recommendations, errIA := provider.Completion(ctx, prompts.Get("sentinel.scan"), string(findingsJSON))
		if errIA != nil {
			prog.Falhar("indisponíveis")
			prog.Detalhe(errIA.Error())
		} else {
			report.Recommendations = recommendations
			prog.Concluir(provider.Name())
		}
	}

	// 5. Remediação
	if !op.fix && !op.fixDryRun {
		return report, nil, nil, nil
	}
	patches = remediation.GeneratePatches(findings)
	if op.fix && !op.fixDryRun && len(patches) > 0 {
		prog.Iniciar("aplicar patches")
		if errs := remediation.ApplyPatches(ctx, client, patches); len(errs) > 0 {
			prog.Falhar(fmt.Sprintf("%d de %d falharam", len(errs), len(patches)))
			for _, e := range errs {
				prog.Detalhe(e.Error())
			}
			falhaPatches = yerrors.Wrap(errs[0], yerrors.ErrCodePlugin,
				fmt.Sprintf("o --fix não aplicou %d de %d patches", len(errs), len(patches))).
				WithContext("namespace", namespace).
				WithHint("o motivo de cada um está na etapa aplicar patches; confira as permissões com kubectl auth can-i patch deployments -n " + namespace)
		} else {
			prog.Concluir(contar(len(patches), "aplicado", "aplicados"))
		}
	}
	return report, patches, falhaPatches, nil
}

// gravarRelatorio escreve o json ou o markdown no stdout ou, com -f, no arquivo; neste
// caso confirma em saida onde ele ficou.
func gravarRelatorio(saida io.Writer, conteudo, arquivo, formato string) error {
	if arquivo == "" {
		fmt.Fprint(saida, conteudo)
		return nil
	}
	if err := writeReport(conteudo, arquivo); err != nil {
		return yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para gravar o relatório").
			WithContext("arquivo", arquivo).
			WithHint("confira se a pasta existe e se você pode escrever nela")
	}
	fmt.Fprintln(saida)
	ui.Check(saida, ui.CheckOk, "relatório", formato, arquivo)
	return nil
}

// deduplicateFindings remove findings duplicados (mesmo recurso + mesma mensagem).
func deduplicateFindings(findings []checks.SecurityFinding) []checks.SecurityFinding {
	seen := make(map[string]bool)
	var result []checks.SecurityFinding
	for _, f := range findings {
		key := fmt.Sprintf("%s|%s|%s", f.Resource, f.Category, f.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, f)
	}
	return result
}

// saveReportToGlobal salva o relatório completo em ~/.yby/reports/.
func saveReportToGlobal(report ScanReport, namespace string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	reportsDir := filepath.Join(home, ".yby", "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return "", err
	}

	date := relogio().Format("2006-01-02")
	filename := fmt.Sprintf("sentinel-scan-%s-%s.md", namespace, date)
	reportPath := filepath.Join(reportsDir, filename)

	content := exportScanMarkdown(report)
	if err := os.WriteFile(reportPath, []byte(content), 0644); err != nil {
		return "", err
	}

	return reportPath, nil
}

// exportScanMarkdown gera o relatório de scan em formato Markdown.
func exportScanMarkdown(report ScanReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Relatório de Segurança Sentinel — %s\n\n", report.Namespace))

	criticals := 0
	warnings := 0
	for _, f := range report.Findings {
		if f.Type == "critical" {
			criticals++
		} else {
			warnings++
		}
	}

	sb.WriteString(fmt.Sprintf("**Resumo:** %d criticos, %d avisos\n", criticals, warnings))
	if len(report.Sources) > 0 {
		sb.WriteString(fmt.Sprintf("**Backends:** %s\n", strings.Join(report.Sources, ", ")))
	}
	sb.WriteString("\n")
	sb.WriteString("## Vulnerabilidades\n\n")

	for _, f := range report.Findings {
		icon := "⚠️"
		if f.Type == "critical" {
			icon = "🚨"
		}
		sb.WriteString(fmt.Sprintf("- %s **[%s]** `%s`: %s\n", icon, f.Category, f.Resource, f.Description))
	}

	if report.Recommendations != "" {
		sb.WriteString("\n## Recomendações\n\n")
		sb.WriteString(report.Recommendations + "\n")
	}

	return sb.String()
}
