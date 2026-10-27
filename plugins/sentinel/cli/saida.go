//go:build k8s

package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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

// maxColunaRecurso limita a coluna dos recursos (com os 2 espaços de separação): um nome
// mais longo é encurtado no meio (encurtarRecurso), para a severidade e a descrição
// ficarem na coluna de todas as linhas.
const maxColunaRecurso = 32

// encurtarRecurso encurta um recurso para caber em n células, com reticências no meio do
// nome: em 30 células, "Deployment/local-path-provisioner (+6)" vira
// "Deployment/l…-provisioner (+6)".
// A contagem dos outros recursos do check (" (+6)") fica inteira.
func encurtarRecurso(recurso string, n int) string {
	if lipgloss.Width(recurso) <= n {
		return recurso
	}
	nome, resto := recurso, ""
	if i := strings.LastIndex(recurso, " (+"); i > 0 && strings.HasSuffix(recurso, ")") {
		nome, resto = recurso[:i], recurso[i:]
	}
	return reticenciasNoMeio(nome, n-lipgloss.Width(resto)) + resto
}

// reticenciasNoMeio corta o meio de s para caber em n células, guardando o começo e o
// fim, que distinguem os nomes do Kubernetes (o tipo e o sufixo).
func reticenciasNoMeio(s string, n int) string {
	runas := []rune(s)
	if len(runas) <= n {
		return s
	}
	reticencias := ui.G().Reticencias
	cabe := n - lipgloss.Width(reticencias)
	if cabe < 2 {
		return string(runas[:max(n, 0)])
	}
	esq := (cabe + 1) / 2
	dir := cabe - esq
	return string(runas[:esq]) + reticencias + string(runas[len(runas)-dir:])
}

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
		checkNaColuna(w, col, severidades[sev].tipo, encurtarRecurso(f.Resource, col.Nome-2), severidades[sev].singular, mensagemNaTela(f))
	}

	if op.fixDryRun || op.fix {
		escreverRemediacao(w, patches, op)
	}
	escreverRecomendacoes(w, report.Recommendations, caminho != "")

	var partes []ui.ParteResumo
	for _, s := range ordemSeveridades {
		if n := contagem[s]; n > 0 {
			partes = append(partes, ui.ParteResumo{Texto: contar(n, severidades[s].singular, severidades[s].plural), Tipo: severidades[s].tipo})
		}
	}
	ui.SummaryCom(w, partes, "")

	ui.Campos(w,
		ui.Campo{Rotulo: "fontes", Valor: nomesDasFontes(report.Sources)},
		ui.Campo{Rotulo: "allowlist", Valor: textoDaAllowlist(report.IgnoradosPelaAllowlist)},
		ui.Campo{Rotulo: "relatório", Valor: filepath.ToSlash(caminho), Caminho: true},
	)
	if errRelatorio != nil {
		slog.Debug("o relatório não foi salvo", "erro", errRelatorio)
		motivo := "não foi salvo em ~/.yby/reports; " + dicaDoLogDoPlugin + " para ver o motivo"
		if os.IsPermission(errRelatorio) {
			motivo = "não foi salvo: sem permissão para gravar em ~/.yby/reports"
		}
		ui.Check(w, ui.CheckAviso, "relatório", "", motivo)
	}
}

// minColunaDescricao é a largura mínima, em células, para a descrição de um achado
// quebrar dentro da coluna dela; mais estreita, ela desce para a linha de baixo, como no
// ui.CheckCom.
const minColunaDescricao = 30

// recuoDoCheck é a coluna onde o nome começa numa linha de ui.Check: a margem, o glifo
// do tipo e um espaço.
func recuoDoCheck(tipo ui.TipoCheck) int {
	g := ui.G()
	glifo := g.Parado
	switch tipo {
	case ui.CheckOk:
		glifo = g.Feito
	case ui.CheckAviso:
		glifo = g.Alerta
	case ui.CheckErro:
		glifo = g.Falha
	}
	return 3 + lipgloss.Width(glifo)
}

// continuarCheck escreve as linhas de continuação de uma linha de ui.Check, recuadas em
// recuo células e no estilo do texto dela (TintaSuave no neutro, Tinta nos outros).
func continuarCheck(w io.Writer, tipo ui.TipoCheck, recuo int, linhas []string) {
	s := ui.S()
	texto := s.Tinta
	if tipo == ui.CheckNeutro {
		texto = s.TintaSuave
	}
	for _, l := range linhas {
		fmt.Fprintln(w, strings.Repeat(" ", recuo)+texto.Render(l))
	}
}

// checkNaColuna é o ui.CheckCom de um achado em que a descrição longa quebra dentro da
// coluna dela, com as continuações recuadas sob o começo da descrição, em vez de descer
// inteira para baixo do recurso. Fora do terminal, quando cabe, ou quando a coluna teria
// menos de minColunaDescricao células, é o ui.CheckCom.
func checkNaColuna(w io.Writer, col ui.Colunas, tipo ui.TipoCheck, nome, versao, nota string) {
	recuo := recuoDoCheck(tipo) + col.Nome + col.Versao
	l := larguraTerminal(w)
	if l <= 0 || recuo+lipgloss.Width(nota) <= l || l-recuo < minColunaDescricao {
		ui.CheckCom(w, col, tipo, nome, versao, nota)
		return
	}
	linhas := ui.Quebrar(nota, l-recuo)
	ui.CheckCom(w, col, tipo, nome, versao, linhas[0])
	continuarCheck(w, tipo, recuo, linhas[1:])
}

// checkComCaminho é o ui.Check com um caminho de arquivo na nota: a home vira ~ e, quando
// o caminho não cabe na linha, ele desce para a de baixo, recuado sob o nome e quebrado só
// nos separadores (ui.QuebrarCaminho), nunca no meio de um nome.
func checkComCaminho(w io.Writer, tipo ui.TipoCheck, nome, versao, caminho string) {
	caminho = ui.AbreviarHome(caminho)
	col := ui.ColunasPara()
	recuo := recuoDoCheck(tipo)
	l := larguraTerminal(w)
	if l <= 0 || recuo+col.Nome+col.Versao+lipgloss.Width(caminho) <= l {
		ui.Check(w, tipo, nome, versao, caminho)
		return
	}
	ui.Check(w, tipo, nome, versao, "")
	continuarCheck(w, tipo, recuo, ui.QuebrarCaminho(caminho, l-recuo))
}

// maxLinhasRecomendacoes é quantas linhas das recomendações da IA o scan mostra na tela;
// o texto inteiro fica no relatório.
const maxLinhasRecomendacoes = 6

// escreverRecomendacoes mostra o começo das recomendações da IA, sem a marcação do
// markdown, e diz onde está o resto: no relatório, ou no -o markdown quando ele não foi
// salvo. Sem recomendações, não escreve nada.
func escreverRecomendacoes(w io.Writer, texto string, noRelatorio bool) {
	larg := larguraSaida(w) - 2
	var linhas []string
	for _, l := range strings.Split(strings.TrimSpace(texto), "\n") {
		if l = linhaSemMarkdown(l); l != "" {
			linhas = append(linhas, ui.Quebrar(l, larg)...)
		}
	}
	if len(linhas) == 0 {
		return
	}
	ui.Section(w, "recomendações da IA")
	s := ui.S()
	for _, l := range linhas[:min(len(linhas), maxLinhasRecomendacoes)] {
		fmt.Fprintln(w, "  "+s.Tinta.Render(l))
	}
	if len(linhas) <= maxLinhasRecomendacoes {
		return
	}
	resto := "o texto inteiro está no relatório"
	if !noRelatorio {
		resto = "o texto inteiro sai com -o markdown"
	}
	ui.Check(w, ui.CheckNeutro, resto, "", "")
}

// linhaSemMarkdown tira de uma linha das recomendações a marcação que não faz sentido no
// terminal: títulos (#), negrito (**), código (`) e réguas; o marcador de lista * vira -.
func linhaSemMarkdown(l string) string {
	l = strings.TrimSpace(l)
	if strings.Trim(l, "-=*_ ") == "" {
		return "" // régua ou linha vazia
	}
	l = strings.TrimSpace(strings.TrimLeft(l, "#"))
	if r, ok := strings.CutPrefix(l, "* "); ok {
		l = "- " + r
	}
	return strings.NewReplacer("**", "", "__", "", "`", "").Replace(l)
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

// textoDaAllowlist diz quantos recursos de RBAC a allowlist deixou de fora ("3
// ignorados", ao lado do rótulo allowlist); com nenhum, fica vazio e a linha não aparece.
func textoDaAllowlist(n int) string {
	if n == 0 {
		return ""
	}
	return contar(n, "ignorado", "ignorados")
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
	fmt.Fprintln(w, ui.Quadro("diagnóstico", strings.Join(diag, "\n"), larg, ui.QuadroSuave))

	fmt.Fprintln(w)
	sug := paragrafos(result.SuggestedFix, interno)
	fmt.Fprintln(w, ui.Quadro("sugestão de correção", strings.Join(sug, "\n"), larg, ui.QuadroSuave))

	// O comando fica fora do quadro e inteiro, para copiar: quebrado no meio, ou cortado
	// pelo quadro, ele não rodaria.
	if result.KubectlPatch != nil && *result.KubectlPatch != "none" && strings.TrimSpace(*result.KubectlPatch) != "" {
		ui.Section(w, "comando sugerido")
		for _, l := range strings.Split(strings.TrimSpace(*result.KubectlPatch), "\n") {
			fmt.Fprintln(w, "  "+s.Negrito.Render(strings.TrimRight(l, " \r")))
		}
	}
}
