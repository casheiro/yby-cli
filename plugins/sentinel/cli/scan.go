//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/redacao"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/allowlist"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/backends"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/escopo"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/profiles"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/remediation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ScanReport contém o resultado completo do scan de segurança.
type ScanReport struct {
	Namespace string `json:"namespace"`
	// Scope é o escopo do scan: "namespace" (só o RBAC que vale no namespace) ou
	// "cluster" (o RBAC do cluster todo, com --scope cluster).
	Scope           string                   `json:"scope"`
	Findings        []checks.SecurityFinding `json:"findings"`
	Sources         []string                 `json:"sources,omitempty"`
	Recommendations string                   `json:"recommendations,omitempty"`
	// IgnoradosPelaAllowlist conta os recursos de RBAC que a allowlist deixou de fora
	// (cada um uma vez, mesmo que mais de uma verificação o ignore).
	IgnoradosPelaAllowlist int `json:"ignored_by_allowlist"`
	// NaoRodaram diz o que o scan não conseguiu verificar, para o JSON não parecer limpo
	// quando faltou verificação.
	NaoRodaram []NaoRodou `json:"nao_rodaram,omitempty"`
	// Perfil é o perfil de conformidade do --profile; ForaDoPerfil conta os achados que
	// ele deixou de fora.
	Perfil       string `json:"perfil,omitempty"`
	ForaDoPerfil int    `json:"fora_do_perfil,omitempty"`
	// Contexto é o contexto do kubeconfig que o scan olhou; GeradoEm é a hora em que o
	// relatório foi gravado (só entra no markdown e no nome do arquivo).
	Contexto string    `json:"contexto,omitempty"`
	GeradoEm time.Time `json:"-"`
}

// NaoRodou registra as verificações de uma fonte que ficaram de fora. Verificacoes vazio
// quer dizer que a fonte inteira falhou; Motivo é o texto em PT-BR da tela.
type NaoRodou struct {
	Backend      string   `json:"backend"`
	Verificacoes []string `json:"verificacoes,omitempty"`
	Motivo       string   `json:"motivo"`
}

// Dependências do scan e do investigate, substituíveis nos testes.
var (
	novoClienteKube = getKubeClient
	nomeDoContexto  = contextoDoKube
	namespacePadrao = namespaceDoKube
	backendsDoScan  = func() []backends.SecurityBackend {
		return []backends.SecurityBackend{backends.NewPolarisBackend(getRestConfig), backends.NewOPABackend()}
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
		namespace = namespacePadrao()
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
	// O --fix pergunta antes de mudar o cluster; sem terminal na entrada, não há a quem
	// perguntar, e ele para aqui, antes do scan, em vez de aplicar calado.
	aplica := op.fix && !op.fixDryRun
	if aplica && !op.sim && !entradaTerminal() {
		return yerrors.New(yerrors.ErrCodeValidation, "o --fix pede confirmação, e a entrada não é um terminal").
			WithHint("rode num terminal, ou passe --yes para aplicar sem perguntar (veja antes o plano com --fix-dry-run)")
	}

	k8sClient, err := novoClienteKube()
	if err != nil {
		return semCluster(err)
	}
	if err := verificarNamespace(ctx, k8sClient, namespace); err != nil {
		return err
	}

	// O título só sai depois das verificações rápidas (perfil, cliente e namespace), para
	// o erro delas não ficar separado dele por duas linhas em branco.
	telaDasEtapas := saida
	if op.paraScripts() {
		telaDasEtapas = erros
	} else {
		titulo := "scan · namespace " + namespace
		if op.escopo == escopo.Cluster {
			titulo += " · escopo cluster"
		}
		if op.perfil != "" {
			titulo += " · perfil " + op.perfil
		}
		ui.Title(saida, "sentinel", titulo)
		fmt.Fprintln(saida)
	}

	prog := novoProgresso(telaDasEtapas)
	parar := animar(prog, erros)
	c, err := coletarScan(ctx, prog, k8sClient, namespace, selecionados, op)
	parar()
	if err != nil {
		return err
	}
	if err := escreverResultadoScan(saida, c, namespace, op); err != nil {
		return err
	}
	if len(c.report.Findings) == 0 || (!op.fix && !op.fixDryRun) {
		return nil
	}
	// No json e no markdown, o plano de correção vai para a tela das etapas, fora do
	// resultado; no terminal, ele já saiu no resultado.
	if op.formato != "" {
		escreverRemediacao(telaDasEtapas, c, op)
	}
	if !aplica {
		return nil
	}
	return aplicarCorrecoes(ctx, telaDasEtapas, erros, k8sClient, c, namespace, op)
}

// verificarNamespace lê o namespace antes do scan, como o investigate lê o pod: um
// namespace que não existe vira erro de validação (sem relatório nem achados do OPA para
// um namespace que não está lá), e um cluster que não responde para aqui, antes das
// etapas. Sem permissão para ler namespaces, o scan segue: cada etapa diz o que leu.
func verificarNamespace(ctx context.Context, client kubernetes.Interface, namespace string) error {
	_, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	switch {
	case err == nil:
		return nil
	case apierrors.IsNotFound(err):
		slog.Debug("o namespace não existe", "namespace", namespace, "erro", err)
		return yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("o namespace %s não existe no cluster", namespace)).
			WithHint("veja os namespaces com kubectl get ns ou no yby viz (tecla n)")
	case apierrors.IsForbidden(err):
		slog.Debug("sem permissão para ler o namespace; o scan segue", "namespace", namespace, "erro", err)
		return nil
	case apierrors.IsUnauthorized(err):
		return yerrors.Wrap(err, yerrors.ErrCodeValidation, "o cluster recusou as credenciais do kubeconfig").
			WithContext("namespace", namespace).
			WithHint("confira o contexto com kubectl config current-context e renove as credenciais")
	}
	return yerrors.Wrap(err, yerrors.ErrCodeClusterOffline, "não deu para consultar o cluster").
		WithContext("namespace", namespace).
		WithHint("confira se o cluster responde com kubectl get pods -n " + namespace)
}

// escreverResultadoScan escreve o json ou o markdown (no stdout ou no arquivo) ou, no
// formato terminal, o resumo na tela, com o relatório completo em ~/.yby/reports/.
//
// Sem achados, a linha só diz "nenhum problema" quando todas as verificações rodaram;
// senão ela é um aviso com quantas não rodaram (as etapas acima dizem quais). O código de
// saída continua 0, como num scan com achados: ele diz se o scan rodou, não o que achou.
func escreverResultadoScan(saida io.Writer, c coleta, namespace string, op opcoes) error {
	report := c.report
	switch op.formato {
	case "json":
		data, _ := json.MarshalIndent(report, "", "  ")
		return gravarRelatorio(saida, string(data), op.arquivo, "json")
	case "markdown":
		return gravarRelatorio(saida, exportScanMarkdown(report), op.arquivo, "markdown")
	}
	if len(report.Findings) == 0 {
		fmt.Fprintln(saida)
		switch {
		case c.naoRodaram > 0:
			ui.Check(saida, ui.CheckAviso, "nenhum problema nas verificações que rodaram · "+contar(c.naoRodaram, "não rodou", "não rodaram"), "", "")
		case report.Perfil != "":
			ui.Check(saida, ui.CheckOk, "nenhum problema do perfil "+report.Perfil, "", "")
		default:
			ui.Check(saida, ui.CheckOk, "nenhum problema de segurança encontrado", "", "")
		}
		ui.Campos(saida,
			ui.Campo{Rotulo: "perfil", Valor: textoDoPerfil(report)},
			ui.Campo{Rotulo: "fontes", Valor: nomesDasFontes(report.Sources)},
			ui.Campo{Rotulo: "allowlist", Valor: textoDaAllowlist(report.IgnoradosPelaAllowlist)},
		)
		return nil
	}
	caminho, errRelatorio := saveReportToGlobal(report, namespace)
	escreverScan(saida, c, op, caminho, errRelatorio)
	return nil
}

// coleta é o que coletarScan junta: o relatório, o plano de correção do --fix ou do
// --fix-dry-run com os patches que o servidor aceitou e recusou no dry-run e quantas
// verificações não rodaram (um backend que falhou por inteiro conta 1; as de uma falha
// parcial e os checks internos que falharam contam cada uma).
type coleta struct {
	report     ScanReport
	plano      remediation.Plano
	validos    []remediation.RemediationPatch
	recusados  []remediation.Falha
	naoRodaram int
}

// coletarScan roda os backends (ou os checks internos), as recomendações da IA e, com
// --fix, os patches, uma etapa de prog para cada; err é a falha que interrompe o scan. O
// motivo de uma etapa que falhou sai em PT-BR embaixo dela (motivoNaTela); o erro cru vai
// para o log de depuração.
func coletarScan(ctx context.Context, prog *ui.Progresso, client kubernetes.Interface, namespace string, selecionados []checks.SecurityCheck, op opcoes) (c coleta, err error) {
	var findings []checks.SecurityFinding
	var sources []string
	consultou := false
	backendRodou := false
	var primeiroErro error
	var naoRodou []NaoRodou
	// Os backends e os checks registram nesta contagem o que a allowlist deixou de fora,
	// e leem do contexto o escopo do RBAC (só o do namespace, ou o do cluster todo).
	ctx, ignorados := allowlist.ComContagem(ctx)
	ctx = escopo.Com(ctx, op.escopo)

	// 1. Backends de segurança (Polaris, OPA)
	for _, b := range backendsDoScan() {
		if !b.IsAvailable() {
			continue
		}
		prog.Iniciar(b.Name())
		bf, errBackend := b.ScanCluster(ctx, client, namespace)
		var parcial *backends.FalhaParcial
		if errBackend != nil && !errors.As(errBackend, &parcial) {
			prog.Falhar("falhou")
			slog.Debug("o backend falhou", "backend", b.Name(), "erro", errBackend)
			prog.Detalhe(motivoNaTela(errBackend))
			c.naoRodaram++
			naoRodou = append(naoRodou, NaoRodou{Backend: b.Name(), Motivo: motivoNaTela(errBackend)})
			if primeiroErro == nil {
				primeiroErro = errBackend
			}
			continue
		}
		consultou = true
		prog.Concluir(contar(len(bf), "achado", "achados"))
		if parcial != nil {
			// Os achados das outras verificações valem; as que não rodaram aparecem como
			// aviso na etapa, e o motivo vai para o log de depuração.
			slog.Debug("verificações do backend não rodaram", "backend", b.Name(), "erro", parcial.Causa)
			c.naoRodaram += len(parcial.Falhas)
			naoRodou = append(naoRodou, NaoRodou{Backend: b.Name(), Verificacoes: parcial.Falhas, Motivo: motivoNaTela(parcial.Causa)})
			prog.Avisar(fmt.Sprintf("%s: %s", contar(len(parcial.Falhas), "verificação não rodou", "verificações não rodaram"), strings.Join(parcial.Falhas, ", ")))
		}
		backendRodou = true
		sources = append(sources, b.Name())
		for _, f := range bf {
			findings = append(findings, checks.SecurityFinding{
				CheckID:        f.ID,
				Severity:       checks.Severity(f.Severity),
				Category:       checks.Category(f.Category),
				Namespace:      f.Namespace,
				Resource:       f.Resource,
				Resources:      f.Resources,
				Message:        f.Message,
				Recommendation: f.Recommendation,
				Scope:          f.Scope,
				Type:           f.Severity,
				Description:    f.Message,
			})
		}
	}

	// 2. Os checks internos são o plano B: só rodam quando nenhum backend consegue varrer
	// (nenhum disponível ou todos falharam). Backend que roda e não acha nada é resultado
	// limpo, não motivo para repetir a varredura com outro motor.
	if !backendRodou {
		sources = append(sources, "checks-internos")
		// Os pods são listados uma vez só para todos os checks.
		ctx = checks.ComCachePods(ctx)
		prog.Iniciar("checks internos")
		var falhas []string
		antes := len(findings)
		for _, check := range selecionados {
			cf, errCheck := check.Run(ctx, client, namespace)
			var naoVerif *checks.ErrNaoVerificado
			if errors.As(errCheck, &naoVerif) {
				// Não é falha de cluster nem achado: vira "não rodou" com o motivo.
				naoRodou = append(naoRodou, NaoRodou{Backend: "checks-internos", Verificacoes: []string{check.Name()}, Motivo: naoVerif.Motivo})
				c.naoRodaram++
				prog.Avisar(fmt.Sprintf("%s não rodou: %s", check.Name(), naoVerif.Motivo))
				continue
			}
			if errCheck != nil {
				slog.Debug("o check falhou", "check", check.Name(), "erro", errCheck)
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
			prog.Detalhe(motivoNaTela(primeiroErro))
			return coleta{}, yerrors.Wrap(primeiroErro, yerrors.ErrCodeClusterOffline,
				"não deu para consultar o cluster").
				WithContext("namespace", namespace).
				WithHint("confira se o cluster responde com kubectl get pods -n " + namespace)
		}
		prog.Concluir(contar(len(findings)-antes, "achado", "achados"))
		c.naoRodaram += len(falhas)
		if len(falhas) > 0 {
			naoRodou = append(naoRodou, NaoRodou{Backend: "checks-internos", Verificacoes: falhas, Motivo: motivoNaTela(primeiroErro)})
			prog.Avisar(fmt.Sprintf("%s: %s", contar(len(falhas), "check falhou", "checks falharam"), strings.Join(falhas, ", ")))
		}
	}

	// 3. Deduplicar findings (mesmo recurso + mesma mensagem) e, com --profile, deixar só
	// os do perfil, venham de que motor vierem; a IA e o --fix só veem estes.
	findings = deduplicateFindings(normalizarAchados(findings))
	findings, fora := filtrarPeloPerfil(findings, op.perfil)
	c.report = ScanReport{Namespace: namespace, Scope: string(escopo.De(ctx)), Findings: findings, Sources: sources, IgnoradosPelaAllowlist: ignorados.Total(), NaoRodaram: naoRodou, Perfil: op.perfil, ForaDoPerfil: fora, Contexto: nomeDoContexto()}
	if len(findings) == 0 {
		return c, nil
	}

	// 4. Recomendações via IA (o --no-ai as pula: o Bard, que chama o scan, já é a IA)
	if provider := provedorDeIA(ctx); provider != nil && !op.semIA {
		prog.Iniciar("recomendações da IA")
		// As mensagens dos achados podem trazer valores do cluster (env, anotações):
		// redigidas antes de ir a um provider que pode ser remoto.
		findingsJSON, _ := json.MarshalIndent(findings, "", "  ")
		recommendations, errIA := provider.Completion(ctx, prompts.Get("sentinel.scan"), redacao.Redigir(string(findingsJSON)))
		if errIA != nil {
			prog.Falhar("indisponíveis")
			slog.Debug("as recomendações da IA falharam", "provider", provider.Name(), "erro", errIA)
			prog.Detalhe(motivoDaIA(errIA))
		} else {
			c.report.Recommendations = recommendations
			prog.Concluir(provider.Name())
		}
	}

	// 5. Remediação: o plano (o dono de cada pod, só as correções seguras) e o dry-run no
	// servidor, que confere cada patch sem gravar nada. O --fix aplica depois do resultado,
	// com confirmação (aplicarCorrecoes).
	if !op.fix && !op.fixDryRun {
		return c, nil
	}
	prog.Iniciar("plano de correção")
	c.plano = remediation.Planejar(ctx, client, findings)
	c.validos, c.recusados = remediation.Aplicar(ctx, client, c.plano.Patches, true)
	if len(c.plano.Patches) == 0 {
		prog.Concluir("nenhum patch")
		return c, nil
	}
	prog.Concluir(contar(len(c.validos), "patch validado no servidor", "patches validados no servidor"))
	if len(c.recusados) > 0 {
		for _, f := range c.recusados {
			slog.Debug("o servidor recusou o patch no dry-run", "recurso", f.Patch.Recurso(), "erro", f.Err)
		}
		prog.Avisar(contar(len(c.recusados), "patch recusado pelo servidor", "patches recusados pelo servidor"))
	}
	return c, nil
}

// aplicarCorrecoes aplica os patches que o servidor aceitou no dry-run, depois de mostrar
// o plano: pergunta antes, a não ser com --yes, e diz em tela o que aplicou. Os patches
// que o servidor recusou (no dry-run ou agora) voltam como erro, para o código de saída
// dizer a scripts e à CI que nada (ou nem tudo) foi corrigido; o cancelamento não é erro.
func aplicarCorrecoes(ctx context.Context, tela, erros io.Writer, client kubernetes.Interface, c coleta, namespace string, op opcoes) error {
	falhas := c.recusados
	if len(c.validos) > 0 {
		if !op.sim {
			sim, err := confirmar(tela, "aplicar correções", perguntaDoFix(c.validos, namespace))
			if err != nil {
				return yerrors.Wrap(err, yerrors.ErrCodePlugin, "a confirmação do --fix falhou").
					WithHint("rode de novo, ou passe --yes para aplicar sem perguntar")
			}
			if !sim {
				fmt.Fprintln(tela)
				ui.Check(tela, ui.CheckNeutro, "nada foi aplicado: a correção foi cancelada", "", "")
				return nil
			}
		}
		fmt.Fprintln(tela)
		prog := novoProgresso(tela)
		parar := animar(prog, erros)
		prog.Iniciar("aplicar patches")
		_, agora := remediation.Aplicar(ctx, client, c.validos, false)
		if len(agora) > 0 {
			prog.Falhar(fmt.Sprintf("%d de %d falharam", len(agora), len(c.validos)))
			for _, f := range agora {
				slog.Debug("o servidor recusou o patch", "recurso", f.Patch.Recurso(), "erro", f.Err)
				prog.Detalhe(f.Patch.Recurso() + ": " + motivoNaTela(f.Err))
			}
		} else {
			prog.Concluir(contar(len(c.validos), "aplicado", "aplicados"))
		}
		parar()
		falhas = append(falhas, agora...)
	}
	if len(falhas) == 0 {
		return nil
	}
	return yerrors.Wrap(falhas[0], yerrors.ErrCodePlugin,
		fmt.Sprintf("o --fix não aplicou %d de %d patches", len(falhas), len(c.plano.Patches))).
		WithContext("namespace", namespace).
		WithHint(fmt.Sprintf("o motivo de cada um está na remediação ou na etapa aplicar patches; confira as permissões com kubectl auth can-i patch %ss -n %s",
			strings.ToLower(falhas[0].Patch.ResourceKind), namespace))
}

// perguntaDoFix é a pergunta da confirmação do --fix: quantos patches, onde, e que os pods
// são recriados (o rollout do template).
func perguntaDoFix(patches []remediation.RemediationPatch, namespace string) string {
	return fmt.Sprintf("aplicar %s no namespace %s?\nos pods dos recursos são recriados com a correção",
		contar(len(patches), "patch", "patches"), namespace)
}

// filtrarPeloPerfil deixa só os achados dos checks do perfil (profiles.Inclui) e conta os
// que ficaram de fora. Sem perfil, devolve todos.
func filtrarPeloPerfil(findings []checks.SecurityFinding, perfil string) (dentro []checks.SecurityFinding, fora int) {
	p, ok := profiles.GetProfile(perfil)
	if !ok {
		return findings, 0
	}
	for _, f := range findings {
		if p.Inclui(f.CheckID) {
			dentro = append(dentro, f)
		} else {
			fora++
		}
	}
	return dentro, fora
}

// textoDoPerfil diz o perfil aplicado e quantos achados ele deixou de fora ("pci-dss · 3
// achados fora do perfil"); sem perfil, fica vazio e a linha não aparece.
func textoDoPerfil(report ScanReport) string {
	if report.Perfil == "" || report.ForaDoPerfil == 0 {
		return report.Perfil
	}
	return report.Perfil + " · " + contar(report.ForaDoPerfil, "achado fora do perfil", "achados fora do perfil")
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
	checkComCaminho(saida, ui.CheckOk, "relatório", formato, filepath.ToSlash(arquivo))
	return nil
}

// deduplicateFindings remove findings duplicados (mesmo recurso + mesma mensagem).
func deduplicateFindings(findings []checks.SecurityFinding) []checks.SecurityFinding {
	seen := make(map[string]bool)
	var result []checks.SecurityFinding
	for _, f := range findings {
		key := fmt.Sprintf("%s|%s|%s|%s", f.CheckID, f.Resource, f.Category, f.Message)
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

	// O nome traz o contexto e a hora, para o segundo scan do dia (ou de outro cluster)
	// não apagar o primeiro.
	report.GeradoEm = relogio()
	base := "sentinel-scan-" + namespace
	if ctx := nomeSeguro(report.Contexto); ctx != "" {
		base += "-" + ctx
	}
	base += "-" + report.GeradoEm.Format("2006-01-02-150405")

	content := []byte(exportScanMarkdown(report))
	// Dois scans no mesmo segundo ganham um sufixo em vez de um sobrescrever o outro.
	for i := 1; ; i++ {
		nome := base + ".md"
		if i > 1 {
			nome = fmt.Sprintf("%s-%d.md", base, i)
		}
		reportPath := filepath.Join(reportsDir, nome)
		f, err := os.OpenFile(reportPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, err = f.Write(content)
		if errClose := f.Close(); err == nil {
			err = errClose
		}
		if err != nil {
			return "", err
		}
		return reportPath, nil
	}
}

// nomeSeguro deixa só o que cabe num nome de arquivo em qualquer sistema: o nome de um
// contexto pode ser um ARN do EKS (com ":" e "/"). Letras, números, ponto e hífen ficam;
// o resto vira "-".
func nomeSeguro(nome string) string {
	var sb strings.Builder
	for _, r := range nome {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteRune('-')
		}
	}
	return strings.Trim(sb.String(), "-.")
}

// exportScanMarkdown gera o relatório de scan em formato Markdown.
func exportScanMarkdown(report ScanReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Relatório de Segurança Sentinel — %s\n\n", report.Namespace))

	if report.Contexto != "" {
		sb.WriteString("**Contexto:** " + report.Contexto + "\n")
	}
	if !report.GeradoEm.IsZero() {
		sb.WriteString("**Gerado em:** " + report.GeradoEm.Format("2006-01-02 15:04:05") + "\n")
	}
	sb.WriteString("**Resumo:** " + resumoDoRelatorio(report.Findings) + "\n")
	if t := textoDoPerfil(report); t != "" {
		sb.WriteString("**Perfil:** " + t + "\n")
	}
	if report.Scope == string(escopo.Cluster) {
		sb.WriteString("**Escopo:** cluster (RBAC do cluster todo)\n")
	}
	if len(report.Sources) > 0 {
		sb.WriteString(fmt.Sprintf("**Backends:** %s\n", strings.Join(report.Sources, ", ")))
	}
	if t := textoDaAllowlist(report.IgnoradosPelaAllowlist); t != "" {
		sb.WriteString("**Allowlist:** " + t + "\n")
	}
	for _, n := range report.NaoRodaram {
		sb.WriteString("**Não rodou:** " + n.Backend)
		if len(n.Verificacoes) > 0 {
			sb.WriteString(" (" + strings.Join(n.Verificacoes, ", ") + ")")
		}
		sb.WriteString(" · " + n.Motivo + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString("## Vulnerabilidades\n")

	// Uma seção por categoria, na ordem da tela.
	var categoria checks.Category
	for i, f := range achadosPorSecao(report.Findings) {
		if i == 0 || f.Category != categoria {
			categoria = f.Category
			sb.WriteString("\n### " + tituloDaCategoria(categoria) + "\n\n")
		}
		icon := glifoDoRelatorio(severidades[severidadeDe(f)].tipo)
		sb.WriteString(fmt.Sprintf("- %s %s: %s\n", icon, recursosEmCodigo(f), f.Description))
	}

	if report.Recommendations != "" {
		sb.WriteString("\n## Recomendações\n\n")
		sb.WriteString(report.Recommendations + "\n")
	}

	return sb.String()
}

// recursosEmCodigo lista no markdown todos os recursos do achado, cada um em código.
func recursosEmCodigo(f checks.SecurityFinding) string {
	lista := f.Resources
	if len(lista) == 0 {
		lista = []string{f.Resource}
	}
	itens := make([]string, len(lista))
	for i, r := range lista {
		itens[i] = "`" + r + "`"
	}
	return strings.Join(itens, ", ")
}

// resumoDoRelatorio conta os achados por severidade com o mesmo mapa do terminal
// (severidadeDe), para o relatório e a tela do mesmo scan dizerem a mesma coisa.
func resumoDoRelatorio(findings []checks.SecurityFinding) string {
	contagem := map[string]int{}
	for _, f := range findings {
		contagem[severidadeDe(f)]++
	}
	var partes []string
	for _, s := range ordemSeveridades {
		if n := contagem[s]; n > 0 {
			partes = append(partes, contar(n, severidades[s].singular, severidades[s].plural))
		}
	}
	if len(partes) == 0 {
		return "nenhum achado"
	}
	return strings.Join(partes, ", ")
}

// glifoDoRelatorio dá o glifo de cada tipo de linha, o mesmo do ui.CheckCom no terminal. O
// relatório é um arquivo: usa sempre os glifos Unicode, sem depender do modo ASCII do
// terminal que rodou o scan.
func glifoDoRelatorio(tipo ui.TipoCheck) string {
	switch tipo {
	case ui.CheckErro:
		return ui.Unicode.Falha
	case ui.CheckAviso:
		return ui.Unicode.Alerta
	case ui.CheckOk:
		return ui.Unicode.Feito
	default:
		return ui.Unicode.Parado
	}
}
