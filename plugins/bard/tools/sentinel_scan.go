package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/casheiro/yby-cli/v5/pkg/plugin"
)

func init() {
	Register(&Tool{
		Name:        "sentinel_scan",
		Description: "Executa scan de segurança no cluster via plugin Sentinel",
		Intents:     []string{"security_scan", "scan_vulnerabilities", "check_security"},
		Parameters: []ToolParam{
			{Name: "namespace", Description: "Namespace a escanear. Se vazio, usa o namespace atual", Required: false},
		},
		Execute: executeSentinelScan,
	})
}

func executeSentinelScan(ctx context.Context, params map[string]string) (string, error) {
	if err := conferirParams(params, "namespace"); err != nil {
		return "", err
	}
	binaryPath, err := discoverPluginBinary("sentinel")
	if err != nil {
		return "", fmt.Errorf("sentinel não disponível: %w", err)
	}

	resp, err := invokePlugin(ctx, binaryPath, pedidoSentinelScan(params))
	if err != nil {
		return "", err
	}

	// Formatar resultado como texto legível
	return formatScanResults(resp.Data), nil
}

// pedidoSentinelScan é o pedido ao Sentinel para o scan, no cluster e no namespace da
// sessão quando a IA não diz o namespace. As recomendações da IA do Sentinel ficam de fora
// (--no-ai): quem analisa os achados é o próprio Bard.
func pedidoSentinelScan(params map[string]string) plugin.PluginRequest {
	args := []string{"scan"}
	if ns := namespaceDe(params); ns != "" {
		args = append(args, "-n", ns)
	}
	return pedidoAoPlugin(append(args, "-o", "json", "--no-ai"))
}

// msgScanSemRelatorio é a saída quando o Sentinel não devolve o relatório. Não é "nenhum
// achado": um Sentinel antigo, que respondia noutro formato, fazia o Bard dizer que o
// namespace estava limpo.
const msgScanSemRelatorio = "O Sentinel não devolveu o relatório do scan: não dá para dizer se há achados. Sugira rodar yby sentinel scan no terminal e conferir se o yby e os plugins estão na mesma versão (yby plugin update)."

// Tetos do resumo do scan. O resumo inteiro fica em tetoResumoScan, abaixo dos 8 KB que o
// Fluxo dá à saída da ferramenta: se passasse, o corte do Fluxo tiraria o meio da lista, e
// a IA leria menos achados do que o resumo anuncia. A lista fica com o que sobra do
// cabeçalho, do que não rodou e dos comandos, com um piso; cada achado e cada linha do que
// não rodou têm teto próprio, e só os primeiros backends que não rodaram são listados.
const (
	tetoResumoScan    = 7 * 1024
	pisoListaScan     = 2 * 1024
	tetoLinhaAchado   = 1024
	tetoLinhaNaoRodou = 300
	maxNaoRodaram     = 6
	maxRecursosResumo = 10
	reservaFechoLista = 200 // o título da lista e o "… e mais N achados (…)"
)

// ordemSeveridade é a ordem do mais grave para o menos; severidade fora dela vem depois.
var ordemSeveridade = []string{"critical", "high", "medium", "low", "info"}

// relatorioScan é a parte do JSON do scan do Sentinel (ScanReport) que vai ao resumo.
type relatorioScan struct {
	Namespace  string       `json:"namespace"`
	Scope      string       `json:"scope"`
	Contexto   string       `json:"contexto"`
	Findings   []achadoScan `json:"findings"`
	Sources    []string     `json:"sources"`
	Ignorados  int          `json:"ignored_by_allowlist"`
	NaoRodaram []naoRodouIA `json:"nao_rodaram"`
	Perfil     string       `json:"perfil"`
	Fora       int          `json:"fora_do_perfil"`
}

type achadoScan struct {
	CheckID        string   `json:"check_id"`
	Severity       string   `json:"severity"`
	Resource       string   `json:"resource"`
	Resources      []string `json:"resources"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation"`
	Scope          string   `json:"scope"`
}

type naoRodouIA struct {
	Backend      string   `json:"backend"`
	Verificacoes []string `json:"verificacoes"`
	Motivo       string   `json:"motivo"`
}

// formatScanResults resume o scan para a IA: o escopo, a contagem por severidade, os
// achados mais graves com os recursos de cada um e o que não rodou. O JSON inteiro de um
// cluster grande não cabe no prompt; o relatório completo fica com o Sentinel.
func formatScanResults(data interface{}) string {
	if data == nil {
		// O scan sempre devolve o relatório, mesmo sem achados: sem ele, nada se sabe.
		return msgScanSemRelatorio
	}
	compacto, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	var rel relatorioScan
	campos, ok := data.(map[string]interface{})
	if _, temAchados := campos["findings"]; !ok || !temAchados || json.Unmarshal(compacto, &rel) != nil {
		// Fora do formato do scan: vai como veio, em JSON compacto.
		return string(compacto)
	}
	return resumirScan(rel)
}

// resumirScan monta o texto do resumo do scan.
func resumirScan(rel relatorioScan) string {
	var sb strings.Builder
	sb.WriteString("Scan de segurança do Sentinel")
	if rel.Namespace != "" {
		sb.WriteString(" · namespace " + rel.Namespace)
	}
	if rel.Scope != "" {
		sb.WriteString(" · escopo " + rel.Scope)
	}
	if rel.Contexto != "" {
		sb.WriteString(" · contexto " + rel.Contexto)
	}
	sb.WriteString("\n")
	if len(rel.Sources) > 0 {
		sb.WriteString("Fontes: " + strings.Join(rel.Sources, ", ") + "\n")
	}
	if rel.Perfil != "" {
		fmt.Fprintf(&sb, "Perfil: %s (%d achados fora dele)\n", rel.Perfil, rel.Fora)
	}

	achados := ordenarPorSeveridade(rel.Findings)
	if len(achados) == 0 {
		sb.WriteString("Nenhum achado.\n")
	} else {
		fmt.Fprintf(&sb, "%s: %s\n", plural(len(achados), "achado", "achados"), contagemPorSeveridade(achados))
	}
	if rel.Ignorados > 0 {
		fmt.Fprintf(&sb, "Recursos de RBAC ignorados pela allowlist: %d\n", rel.Ignorados)
	}
	cauda := naoRodaram(rel.NaoRodaram) + orientacaoScan(rel)
	tetoLista := max(tetoResumoScan-sb.Len()-len(cauda)-reservaFechoLista, pisoListaScan)

	if len(achados) > 0 {
		var linhas []string
		tamanho := 0
		for _, a := range achados {
			linha := "\n- " + linhaAchado(a)
			if len(linhas) > 0 && tamanho+len(linha) > tetoLista {
				break
			}
			linhas = append(linhas, linha)
			tamanho += len(linha)
		}
		n := len(linhas)
		if n < len(achados) {
			fmt.Fprintf(&sb, "\nOs %d mais graves (os outros não couberam aqui):", n)
		} else {
			sb.WriteString("\nDo mais grave para o menos:")
		}
		for _, l := range linhas {
			sb.WriteString(l)
		}
		if resto := len(achados) - n; resto > 0 {
			fmt.Fprintf(&sb, "\n… e mais %s (%s)", plural(resto, "achado", "achados"), contagemPorSeveridade(achados[n:]))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(cauda)
	return sb.String()
}

// naoRodaram lista o que o scan não conseguiu verificar: os primeiros maxNaoRodaram, cada
// um com até tetoLinhaNaoRodou bytes, e quantos ficaram de fora.
func naoRodaram(lista []naoRodouIA) string {
	if len(lista) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\nNão rodaram:")
	for _, nr := range lista[:min(len(lista), maxNaoRodaram)] {
		linha := nr.Backend
		if len(nr.Verificacoes) > 0 {
			linha += " (" + strings.Join(nr.Verificacoes, ", ") + ")"
		}
		sb.WriteString("\n- " + cortarTexto(linha+": "+nr.Motivo, tetoLinhaNaoRodou))
	}
	if resto := len(lista) - maxNaoRodaram; resto > 0 {
		fmt.Fprintf(&sb, "\n… e mais %d (o relatório completo traz todos)", resto)
	}
	sb.WriteString("\n")
	return sb.String()
}

// cortarTexto corta s em até n bytes, sem partir um caractere, e marca o corte com "…".
func cortarTexto(s string, n int) string {
	if len(s) <= n {
		return s
	}
	corte := n - len("…")
	for corte > 0 && !utf8.RuneStart(s[corte]) {
		corte--
	}
	return s[:corte] + "…"
}

// corrigidosPeloSentinel são as verificações que o --fix do Sentinel corrige
// (plugins/sentinel/cli/remediation); o POD_CAPABILITIES crítico (SYS_ADMIN) fica de fora.
var corrigidosPeloSentinel = map[string]bool{
	"POD_PRIVILEGE_ESCALATION":           true,
	"polaris/privilegeEscalationAllowed": true,
	"POD_CAPABILITIES":                   true,
	"polaris/insecureCapabilities":       true,
}

// orientacaoScan diz à IA o que o usuário pode rodar a partir do scan: o relatório
// completo e, quando há achado que o Sentinel corrige, o --fix-dry-run. O Bard não roda
// nenhum dos dois; a IA os cita na resposta. Fecha com o risco das correções de
// securityContext, que a IA costuma sugerir sem avisar o que quebra.
func orientacaoScan(rel relatorioScan) string {
	comando := "yby sentinel scan"
	if rel.Namespace != "" {
		comando += " -n " + rel.Namespace
	}
	if rel.Scope == "cluster" {
		comando += " --scope cluster"
	}
	if rel.Perfil != "" {
		comando += " -p " + rel.Perfil
	}
	var sb strings.Builder
	sb.WriteString("\nComandos do yby para o usuário rodar (cite os que ajudarem; o Bard não os executa):\n")
	fmt.Fprintf(&sb, "- %s: o relatório completo, com todos os achados e recursos, gravado em ~/.yby/reports\n", comando)
	corrige := false
	for _, a := range rel.Findings {
		if corrigidosPeloSentinel[a.CheckID] && !(a.CheckID == "POD_CAPABILITIES" && a.Severity == "critical") {
			corrige = true
			break
		}
	}
	if corrige {
		fmt.Fprintf(&sb, "- %s --fix-dry-run para ver e --fix para aplicar: o Sentinel corrige a escalada de privilégio e as capabilities no template do dono de cada pod (Deployment, StatefulSet, DaemonSet, CronJob); recurso gerido pelo Argo CD fica de fora e se corrige no Git\n", comando)
	}
	if kube.contexto != "" {
		fmt.Fprintf(&sb, "Esses comandos usam o contexto atual do kubeconfig; o scan foi no contexto %s.\n", kube.contexto)
	}
	sb.WriteString("Ao sugerir securityContext, diga o que pode quebrar: runAsNonRoot e runAsUser falham com imagem que roda como root, e readOnlyRootFilesystem pede um emptyDir nos diretórios em que o app escreve (/tmp, cache).\n")
	return sb.String()
}

// posicaoSeveridade é a posição da severidade em ordemSeveridade, ou depois de todas.
func posicaoSeveridade(sev string) int {
	for i, s := range ordemSeveridade {
		if s == sev {
			return i
		}
	}
	return len(ordemSeveridade)
}

// ordenarPorSeveridade devolve os achados do mais grave para o menos, mantendo a ordem do
// Sentinel entre os de mesma severidade.
func ordenarPorSeveridade(achados []achadoScan) []achadoScan {
	ordenados := append([]achadoScan(nil), achados...)
	sort.SliceStable(ordenados, func(i, j int) bool {
		return posicaoSeveridade(ordenados[i].Severity) < posicaoSeveridade(ordenados[j].Severity)
	})
	return ordenados
}

// contagemPorSeveridade escreve "2 critical, 1 high", na ordem de gravidade.
func contagemPorSeveridade(achados []achadoScan) string {
	contagem := map[string]int{}
	var outras []string
	for _, a := range achados {
		if posicaoSeveridade(a.Severity) == len(ordemSeveridade) && contagem[a.Severity] == 0 {
			outras = append(outras, a.Severity)
		}
		contagem[a.Severity]++
	}
	var partes []string
	for _, sev := range append(append([]string{}, ordemSeveridade...), outras...) {
		if contagem[sev] > 0 {
			partes = append(partes, fmt.Sprintf("%d %s", contagem[sev], sev))
		}
	}
	return strings.Join(partes, ", ")
}

// linhaAchado escreve um achado numa linha: severidade, verificação, recursos (até o
// teto), escopo de cluster, mensagem e recomendação.
func linhaAchado(a achadoScan) string {
	recursos := a.Resources
	if len(recursos) == 0 && a.Resource != "" {
		recursos = []string{a.Resource}
	}
	texto := strings.Join(recursos[:min(len(recursos), maxRecursosResumo)], ", ")
	if resto := len(recursos) - maxRecursosResumo; resto > 0 {
		texto += " e mais " + plural(resto, "recurso", "recursos")
	}
	linha := fmt.Sprintf("[%s] %s · %s", a.Severity, a.CheckID, texto)
	if a.Scope == "cluster" {
		linha += " (cluster)"
	}
	linha += " · " + a.Message
	if a.Recommendation != "" {
		linha += " · Recomendação: " + a.Recommendation
	}
	return cortarTexto(linha, tetoLinhaAchado)
}

// plural escreve "1 achado" ou "N achados".
func plural(n int, um, varios string) string {
	if n == 1 {
		return "1 " + um
	}
	return fmt.Sprintf("%d %s", n, varios)
}
