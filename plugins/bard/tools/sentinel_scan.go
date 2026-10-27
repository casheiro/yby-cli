package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/casheiro/yby-cli/pkg/plugin"
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

// Teto do resumo do scan: os achados mais graves e, em cada um, os recursos listados.
const (
	maxAchadosResumo  = 10
	maxRecursosResumo = 10
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
		return "Nenhum finding reportado."
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

	if len(achados) > 0 {
		n := min(len(achados), maxAchadosResumo)
		if n < len(achados) {
			fmt.Fprintf(&sb, "\nOs %d mais graves:", n)
		} else {
			sb.WriteString("\nDo mais grave para o menos:")
		}
		for _, a := range achados[:n] {
			sb.WriteString("\n- " + linhaAchado(a))
		}
		if resto := len(achados) - n; resto > 0 {
			fmt.Fprintf(&sb, "\n… e mais %s (%s)", plural(resto, "achado", "achados"), contagemPorSeveridade(achados[n:]))
		}
		sb.WriteString("\n")
	}

	if len(rel.NaoRodaram) > 0 {
		sb.WriteString("\nNão rodaram:")
		for _, nr := range rel.NaoRodaram {
			sb.WriteString("\n- " + nr.Backend)
			if len(nr.Verificacoes) > 0 {
				sb.WriteString(" (" + strings.Join(nr.Verificacoes, ", ") + ")")
			}
			sb.WriteString(": " + nr.Motivo)
		}
		sb.WriteString("\n")
	}
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
	return linha
}

// plural escreve "1 achado" ou "N achados".
func plural(n int, um, varios string) string {
	if n == 1 {
		return "1 " + um
	}
	return fmt.Sprintf("%d %s", n, varios)
}
