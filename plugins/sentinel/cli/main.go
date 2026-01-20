package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/charmbracelet/lipgloss"
)

var execCommand = exec.Command

func main() {
	var req plugin.PluginRequest

	// 1. Check for Environment Variable Protocol
	if envReq := os.Getenv("YBY_PLUGIN_REQUEST"); envReq != "" {
		if err := json.Unmarshal([]byte(envReq), &req); err != nil {
			fmt.Printf("Erro ao analisar YBY_PLUGIN_REQUEST: %v\n", err)
			os.Exit(1)
		}
		handlePluginRequest(req)
		return
	}

	// 2. Fallback to Stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		if err := json.NewDecoder(os.Stdin).Decode(&req); err == nil {
			handlePluginRequest(req)
			return
		}
	}

	// Fallback/Mock
	handlePluginRequest(plugin.PluginRequest{Hook: "command"})
}

// AnalysisResult define a estrutura esperada da resposta da IA
type AnalysisResult struct {
	RootCause       string  `json:"root_cause"`
	TechnicalDetail string  `json:"technical_detail"`
	Confidence      int     `json:"confidence"`
	SuggestedFix    string  `json:"suggested_fix"`
	KubectlPatch    *string `json:"kubectl_patch"`
}

func handlePluginRequest(req plugin.PluginRequest) {
	switch req.Hook {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:    "sentinel",
			Version: "0.2.0",
			Hooks:   []string{"command"},
		})
	case "command":
		// Expect "yby sentinel investigate [pod-name] [flags]"
		// Flags: -n/--namespace
		var podName, namespace string
		args := req.Args

		// Parser simples de argumentos
		if len(args) > 0 {
			if args[0] == "investigate" {
				// Remove o comando "investigate" da lista para processar o resto
				remainingArgs := args[1:]

				for i := 0; i < len(remainingArgs); i++ {
					arg := remainingArgs[i]

					// Verifica flags de namespace
					if arg == "-n" || arg == "--namespace" {
						if i+1 < len(remainingArgs) {
							namespace = remainingArgs[i+1]
							i++ // Avança o próximo, pois já foi consumido como valor
						}
						continue
					}

					// Se não é flag e podName ainda está vazio, deve ser o nome do pod
					if !strings.HasPrefix(arg, "-") && podName == "" {
						podName = arg
					}
				}
			}
		}

		// Fallback to Context if needed
		if podName == "" && req.Context != nil {
			if p, ok := req.Context["pod"]; ok {
				podName = fmt.Sprintf("%v", p)
			}
		}
		if namespace == "" && req.Context != nil {
			if n, ok := req.Context["namespace"]; ok {
				namespace = fmt.Sprintf("%v", n)
			}
		}

		if podName == "" {
			fmt.Println("❌ Nome do Pod é obrigatório. Uso: yby sentinel investigate <pod> [-n namespace]")
			return
		}

		if namespace == "" {
			namespace = "default"
		}

		investigate(podName, namespace)
	default:
		os.Exit(0)
	}
}

func investigate(podName, namespace string) {
	// Configuração de Estilo
	width := 80 // Largura confortável para leitura
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).Padding(0, 1)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		BorderForeground(lipgloss.Color("63")).
		Width(width) // Força quebra de linha

	// Estilos de texto internos também precisam respeitar ou serem menores,
	// mas o box com Width já deve forçar o wrap do conteúdo string.
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(width - 4) // width - padding/border

	fmt.Println(titleStyle.Render(fmt.Sprintf("\n🛡️  Sentinel Investigation: %s/%s", namespace, podName)))

	// 1. Get Pod Logs via kubectl
	fmt.Print("🔍 Coletando logs...")
	cmdLogs := execCommand("kubectl", "logs", podName, "-n", namespace, "--tail=50")
	logsOut, err := cmdLogs.CombinedOutput()
	if err != nil {
		fmt.Printf("\r⚠️  Falha ao obter logs (%v). Continuando...\n", err)
	} else {
		fmt.Println("\r✅ Logs coletados")
	}

	// 2. Get Events via kubectl
	fmt.Print("🔍 Coletando eventos...")
	cmdEvents := execCommand("kubectl", "get", "events", "-n", namespace,
		"--field-selector", fmt.Sprintf("involvedObject.name=%s", podName),
		"--sort-by=.lastTimestamp",
		"-o", "json")

	eventsOut, err := cmdEvents.CombinedOutput()
	if err != nil {
		fmt.Printf("\r⚠️  Falha ao obter eventos. Continuando...\n")
	} else {
		fmt.Println("\r✅ Eventos coletados")
	}

	// 3. Get Metrics (CPU/RAM)
	fmt.Print("🔍 Coletando métricas...")
	cmdMetrics := execCommand("kubectl", "top", "pod", podName, "-n", namespace, "--no-headers")
	metricsOut, errMet := cmdMetrics.CombinedOutput()
	metricsStr := ""
	if errMet != nil {
		// Silencioso sobre métricas, comum não ter metrics-server
		metricsStr = "Metrics unavailable (metrics-server likely missing)"
		fmt.Println("\r⚠️  Métricas indisponíveis")
	} else {
		metricsStr = string(metricsOut)
		fmt.Println("\r✅ Métricas coletadas")
	}

	// Construct Context for AI
	realContext := fmt.Sprintf("LOGS:\n%s\n\nEVENTS (JSON):\n%s\n\nMETRICS:\n%s", string(logsOut), string(eventsOut), metricsStr)

	if len(strings.TrimSpace(realContext)) < 20 {
		fmt.Println("❌ Dados insuficientes (logs/eventos) coletados para análise.")
		return
	}

	fmt.Println("\n🤖 Analisando com IA...")

	ctx := context.Background()
	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		fmt.Println("❌ Nenhum provedor de IA disponível. Defina OLLAMA_HOST ou OPENAI_API_KEY.")
		return
	}

	analysisJSON, err := provider.Completion(ctx, SentinelSystemPrompt, realContext)
	if err != nil {
		fmt.Printf("Erro na chamada da IA: %v\n", err)
		return
	}

	// Parse JSON output
	var result AnalysisResult
	// Tentar limpar blocos de código markdown se houver (```json ... ```)
	analysisClean := strings.ReplaceAll(analysisJSON, "```json", "")
	analysisClean = strings.ReplaceAll(analysisClean, "```", "")

	if err := json.Unmarshal([]byte(analysisClean), &result); err != nil {
		fmt.Printf("⚠️  Erro ao parsear resposta da IA: %v\nConteúdo bruto:\n%s\n", err, analysisJSON)
		return
	}

	// Render Result
	var sb strings.Builder

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Width(width-4).Render(fmt.Sprintf("\n🚨 Causa Raiz: %s", result.RootCause)) + "\n")
	sb.WriteString(labelStyle.Render(result.TechnicalDetail) + "\n\n")

	confidenceColor := "46" // Green
	if result.Confidence < 80 {
		confidenceColor = "220" // Yellow
	}
	if result.Confidence < 50 {
		confidenceColor = "196" // Red
	}
	sb.WriteString(fmt.Sprintf("Confiança: %s%%\n", lipgloss.NewStyle().Foreground(lipgloss.Color(confidenceColor)).Render(fmt.Sprintf("%d", result.Confidence))))

	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("\n💡 Sugestão de Correção:") + "\n")
	sb.WriteString(lipgloss.NewStyle().Width(width-4).Render(result.SuggestedFix) + "\n")

	if result.KubectlPatch != nil && *result.KubectlPatch != "none" && *result.KubectlPatch != "" {
		sb.WriteString(lipgloss.NewStyle().Bold(true).Render("\n🛠️  Comando Sugerido:") + "\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Width(width-4).Render(*result.KubectlPatch) + "\n")
	}

	fmt.Println(boxStyle.Render(sb.String()))
}

func respond(data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}
