//go:build k8s

package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/remediation"
	"github.com/charmbracelet/lipgloss"
)

// severidade descreve um nível de severidade na tela: a palavra ao lado do glifo, o
// plural do resumo e o tipo de linha (que dá o glifo e a cor).
type severidade struct {
	singular, plural string
	tipo             ui.TipoCheck
}

// ordemSeveridades vai da mais grave à mais leve.
var ordemSeveridades = []string{"critical", "high", "medium", "low", "info"}

var severidades = map[string]severidade{
	"critical": {"crítico", "críticos", ui.CheckErro},
	"high":     {"alto", "altos", ui.CheckAviso},
	"medium":   {"médio", "médios", ui.CheckAviso},
	"low":      {"baixo", "baixos", ui.CheckNeutro},
	"info":     {"informativo", "informativos", ui.CheckNeutro},
}

// severidadeDe devolve a severidade de f; os achados antigos só trazem Type, e o
// "warning" deles conta como médio.
func severidadeDe(f checks.SecurityFinding) string {
	s := string(f.Severity)
	if s == "" {
		s = f.Type
	}
	if s == "warning" {
		s = "medium"
	}
	if _, ok := severidades[s]; !ok {
		return "info"
	}
	return s
}

func posicaoSeveridade(s string) int {
	for i, o := range ordemSeveridades {
		if o == s {
			return i
		}
	}
	return len(ordemSeveridades)
}

// ordemCategorias é a ordem das seções do scan; as categorias fora dela vêm depois, em
// ordem alfabética.
var ordemCategorias = []checks.Category{
	checks.CategoryPodSecurity, checks.CategoryRBAC, checks.CategoryNetwork,
	checks.CategorySecrets, checks.CategorySupplyChain,
}

var nomesCategorias = map[checks.Category]string{
	checks.CategoryPodSecurity: "segurança dos pods",
	checks.CategoryRBAC:        "rbac",
	checks.CategoryNetwork:     "rede",
	checks.CategorySecrets:     "secrets",
	checks.CategorySupplyChain: "cadeia de suprimentos",
}

func nomeCategoria(c checks.Category) string {
	if n, ok := nomesCategorias[c]; ok {
		return n
	}
	if c == "" {
		return "outros"
	}
	return string(c)
}

func posicaoCategoria(c checks.Category) int {
	for i, o := range ordemCategorias {
		if o == c {
			return i
		}
	}
	return len(ordemCategorias)
}

// contar escreve "1 achado" ou "3 achados".
func contar(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// mensagensPolaris traduz, só na tela, as mensagens dos checks do Polaris, que a
// biblioteca escreve em inglês. O JSON e o relatório markdown continuam com o texto
// original; um check fora da lista também.
var mensagensPolaris = map[string]string{
	"automountServiceAccountToken":             "O token da ServiceAccount é montado automaticamente",
	"clusterrolebindingClusterAdmin":           "O ClusterRoleBinding aponta para o cluster-admin ou para um ClusterRole com curinga",
	"clusterrolebindingPodExecAttach":          "O ClusterRoleBinding aponta para um ClusterRole que permite pods/exec ou pods/attach, ou que não existe",
	"clusterrolePodExecAttach":                 "O ClusterRole permite pods/exec ou pods/attach",
	"cpuLimitsMissing":                         "Defina o limite de CPU",
	"cpuRequestsMissing":                       "Defina o request de CPU",
	"dangerousCapabilities":                    "O contêiner não deve ter capabilities perigosas",
	"deploymentMissingReplicas":                "Só uma réplica está agendada",
	"hostIPCSet":                               "O IPC do host não deve ser usado",
	"hostNetworkSet":                           "A rede do host não deve ser usada",
	"hostPathSet":                              "Volumes hostPath não são permitidos",
	"hostPIDSet":                               "O PID do host não deve ser usado",
	"hostPortSet":                              "A porta do host não deve ser usada",
	"hostProcess":                              "O acesso privilegiado ao host não é permitido",
	"hpaMaxAvailability":                       "O maxReplicas e o minReplicas do HPA devem ser diferentes",
	"hpaMinAvailability":                       "O minReplicas do HPA deve ser 2 ou mais",
	"insecureCapabilities":                     "O contêiner não deve ter capabilities inseguras",
	"linuxHardening":                           "Restrinja os privilégios do contêiner com AppArmor, Seccomp, SELinux ou tirando capabilities do Linux",
	"livenessProbeMissing":                     "Configure a liveness probe",
	"memoryLimitsMissing":                      "Defina o limite de memória",
	"memoryRequestsMissing":                    "Defina o request de memória",
	"metadataAndInstanceMismatched":            "O rótulo app.kubernetes.io/instance deve ser igual ao metadata.name",
	"missingNetworkPolicy":                     "Nenhuma NetworkPolicy seleciona os pods com regras de entrada e de saída",
	"missingPodDisruptionBudget":               "Falta um PodDisruptionBudget",
	"notReadOnlyRootFilesystem":                "O sistema de arquivos raiz deve ser somente leitura",
	"pdbDisruptionsIsZero":                     "O PodDisruptionBudget não permite nenhuma remoção voluntária",
	"pdbMinAvailableGreaterThanHPAMinReplicas": "O minAvailable do PodDisruptionBudget é maior que o minReplicas do HPA",
	"priorityClassNotSet":                      "Defina a priority class",
	"privilegeEscalationAllowed":               "A escalada de privilégio não deve ser permitida",
	"procMount":                                "O procMount não deve sair do padrão",
	"pullPolicyNotAlways":                      "A imagePullPolicy deve ser Always",
	"readinessProbeMissing":                    "Configure a readiness probe",
	"rolebindingClusterAdminClusterRole":       "O RoleBinding aponta para o cluster-admin ou para um ClusterRole com curinga",
	"rolebindingClusterAdminRole":              "O RoleBinding aponta para um Role com curinga",
	"rolebindingClusterRolePodExecAttach":      "O RoleBinding aponta para um ClusterRole que permite pods/exec ou pods/attach, ou que não existe",
	"rolebindingRolePodExecAttach":             "O RoleBinding aponta para um Role que permite pods/exec ou pods/attach, ou que não existe",
	"rolePodExecAttach":                        "O Role permite pods/exec ou pods/attach",
	"runAsPrivileged":                          "O contêiner não deve rodar como privilegiado",
	"runAsRootAllowed":                         "O contêiner não deve poder rodar como root",
	"sensitiveConfigmapContent":                "O ConfigMap parece ter conteúdo sensível nas chaves ou nos valores",
	"sensitiveContainerEnvVar":                 "O contêiner define variáveis de ambiente que parecem sensíveis",
	"tagNotSpecified":                          "Especifique a tag da imagem",
	"tlsSettingsMissing":                       "O Ingress não tem TLS configurado",
	"topologySpreadConstraint":                 "Configure uma topology spread constraint válida",
}

// mensagemNaTela devolve a mensagem de f como ela aparece na tela: a do Polaris em PT-BR,
// as outras como vieram.
func mensagemNaTela(f checks.SecurityFinding) string {
	if id, ok := strings.CutPrefix(f.CheckID, "polaris/"); ok {
		if m, ok := mensagensPolaris[id]; ok {
			return m
		}
	}
	if f.Message != "" {
		return f.Message
	}
	return f.Description
}

// maxColunaRecurso limita a coluna dos recursos: um nome mais longo empurra só a própria
// linha.
const maxColunaRecurso = 32

// escreverScan escreve o resultado do scan: uma seção por categoria com uma linha por
// achado (glifo e palavra da severidade, recurso e mensagem), a remediação do dry-run, o
// resumo por severidade, as fontes e o caminho do relatório.
func escreverScan(w io.Writer, report ScanReport, patches []remediation.RemediationPatch, op opcoes, caminho string, errRelatorio error) {
	achados := append([]checks.SecurityFinding(nil), report.Findings...)
	sort.SliceStable(achados, func(i, j int) bool {
		a, b := achados[i], achados[j]
		if pa, pb := posicaoCategoria(a.Category), posicaoCategoria(b.Category); pa != pb {
			return pa < pb
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if sa, sb := posicaoSeveridade(severidadeDe(a)), posicaoSeveridade(severidadeDe(b)); sa != sb {
			return sa < sb
		}
		if a.Resource != b.Resource {
			return a.Resource < b.Resource
		}
		return a.Message < b.Message
	})

	col := ui.Colunas{Nome: 12, Versao: 0}
	for _, f := range achados {
		col.Nome = max(col.Nome, min(lipgloss.Width(f.Resource)+2, maxColunaRecurso))
		col.Versao = max(col.Versao, lipgloss.Width(severidades[severidadeDe(f)].singular)+2)
	}

	contagem := map[string]int{}
	var categoria checks.Category
	for i, f := range achados {
		if i == 0 || f.Category != categoria {
			categoria = f.Category
			ui.Section(w, nomeCategoria(categoria))
		}
		sev := severidadeDe(f)
		contagem[sev]++
		ui.CheckCom(w, col, severidades[sev].tipo, f.Resource, severidades[sev].singular, mensagemNaTela(f))
	}

	if op.fixDryRun || op.fix {
		escreverRemediacao(w, patches, op)
	}

	var partes []ui.ParteResumo
	for _, s := range ordemSeveridades {
		if n := contagem[s]; n > 0 {
			partes = append(partes, ui.ParteResumo{Texto: contar(n, severidades[s].singular, severidades[s].plural), Tipo: severidades[s].tipo})
		}
	}
	ui.SummaryCom(w, partes, "")

	ui.Campos(w,
		ui.Campo{Rotulo: "fontes", Valor: nomesDasFontes(report.Sources)},
		ui.Campo{Rotulo: "relatório", Valor: caminho},
	)
	if errRelatorio != nil {
		ui.Check(w, ui.CheckAviso, "relatório", "", "não foi salvo: "+errRelatorio.Error())
	}
}

// escreverRemediacao lista os patches do --fix-dry-run (o --fix já os mostrou como etapa)
// ou avisa que os achados não têm patch.
func escreverRemediacao(w io.Writer, patches []remediation.RemediationPatch, op opcoes) {
	if len(patches) > 0 && !op.fixDryRun {
		return
	}
	titulo := "remediação"
	if op.fixDryRun {
		titulo += " · dry-run"
	}
	ui.Section(w, titulo)
	if len(patches) == 0 {
		ui.Check(w, ui.CheckNeutro, "nenhum patch de remediação para estes achados", "", "")
		return
	}
	itens := make([]string, len(patches))
	for i, p := range patches {
		itens[i] = fmt.Sprintf("%s %s/%s · %s · patch %s", p.ResourceKind, p.Namespace, p.ResourceName, p.Description, p.Patch)
	}
	ui.Passos(w, itens...)
	fmt.Fprintln(w)
	ui.Check(w, ui.CheckNeutro, "nada foi aplicado; rode com --fix para aplicar", "", "")
}

// nomesDasFontes devolve as fontes do scan como aparecem na tela; no JSON e no relatório
// os checks internos continuam "checks-internos".
func nomesDasFontes(fontes []string) string {
	nomes := make([]string, len(fontes))
	for i, f := range fontes {
		nomes[i] = strings.ReplaceAll(f, "checks-internos", "checks internos")
	}
	return strings.Join(nomes, ", ")
}

// larguraQuadro é a largura dos quadros do investigate em w.
func larguraQuadro(w io.Writer) int { return larguraSaida(w) }

// paragrafos quebra cada parágrafo de texto em linhas de até largura células.
func paragrafos(texto string, largura int) []string {
	var linhas []string
	for _, p := range strings.Split(strings.TrimSpace(texto), "\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			linhas = append(linhas, "")
			continue
		}
		linhas = append(linhas, ui.Quebrar(p, largura)...)
	}
	return linhas
}

// escreverAnalise escreve o diagnóstico e a sugestão do investigate em dois ui.Quadro.
func escreverAnalise(w io.Writer, result AnalysisResult) {
	larg := larguraQuadro(w)
	interno := larg - 4
	s := ui.S()

	var diag []string
	for _, l := range paragrafos(result.RootCause, interno) {
		diag = append(diag, s.Negrito.Render(l))
	}
	if d := strings.TrimSpace(result.TechnicalDetail); d != "" {
		diag = append(diag, "", strings.Join(paragrafos(d, interno), "\n"))
	}
	diag = append(diag, "", ui.Cota("confiança", float64(result.Confidence)/100, interno))
	fmt.Fprintln(w, ui.Quadro("diagnóstico", strings.Join(diag, "\n"), larg, ui.QuadroNormal))

	fmt.Fprintln(w)
	sug := paragrafos(result.SuggestedFix, interno)
	fmt.Fprintln(w, ui.Quadro("sugestão de correção", strings.Join(sug, "\n"), larg, ui.QuadroNormal))

	// O comando fica fora do quadro e inteiro, para copiar: quebrado no meio, ou cortado
	// pelo quadro, ele não rodaria.
	if result.KubectlPatch != nil && *result.KubectlPatch != "none" && strings.TrimSpace(*result.KubectlPatch) != "" {
		ui.Section(w, "comando sugerido")
		for _, l := range strings.Split(strings.TrimSpace(*result.KubectlPatch), "\n") {
			fmt.Fprintln(w, "  "+s.Negrito.Render(strings.TrimRight(l, " \r")))
		}
	}
}
