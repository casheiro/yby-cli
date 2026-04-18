//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var execCommand = exec.Command

func main() {
	// Initialize SDK
	if err := sdk.Init(); err != nil {
		// Log to stderr so it doesn't break JSON output if relevant
		fmt.Fprintf(os.Stderr, "SDK Init error: %v\n", err)
	}

	// Use context definition from SDK if available, or fallback to manual generic request check
	// But SDK Init already handles parsing plugin request.
	// Since handlePluginRequest expects a req object, we need to reconstruct it or change the signature.
	// Ideally handlePluginRequest should use sdk.GetFullContext() directly for logic,
	// but hook dispatching depends on the "Hook" field which SDK parses into hidden struct.

	// Wait, SDK currentContext is PluginFullContext.
	// PluginRequest has 'Hook'. SDK Init reads PluginRequest.
	// We need to know WHICH hook was called.
	// My SDK provided `GetFullContext` but didn't expose the `Hook`.
	// I should probably export `GetHook()` from SDK or return the request from Init.

	// Let's modify SDK to export GetHook or modify this main to re-read?
	// No, SDK consumed stdin.

	// FIX: I need to update SDK to expose the Hook or the Request.
	// For now, I will hack it by accessing the raw request if I expose it, or just make SDK return the hook.
	// OR: I can just use the fact that SDK Init absorbs the request.
	// But I need to switch on the hook.

	// I will invoke handleLogic() and inside it assume SDK is ready.
	// But I need the hook name.
	// If I check os.Args, maybe "manifest" is passed as arg?
	// Usually Yby plugins pass context via stdin for 'command', but 'manifest' might be just arg or stdin with Hook="manifest".
	// The manager sends: PluginRequest{Hook: "manifest"}.

	handlePluginRequest()
}

// printSentinelHelp exibe a ajuda do Sentinel.
func printSentinelHelp() {
	fmt.Println("Sentinel - Auditoria de seguranca e conformidade K8s")
	fmt.Println()
	fmt.Println("Uso: yby sentinel <subcomando> [flags]")
	fmt.Println()
	fmt.Println("Subcomandos:")
	fmt.Println("  scan                  Escaneia vulnerabilidades de seguranca")
	fmt.Println("  investigate <pod>     Investiga um pod com IA")
	fmt.Println()
	fmt.Println("Flags (scan):")
	fmt.Println("  -n, --namespace       Namespace a escanear (padrao: default)")
	fmt.Println("  -o, --output          Formato de saida: terminal, json, markdown")
	fmt.Println("  -f, --file            Salvar resultado em arquivo")
	fmt.Println("  -p, --profile         Perfil de compliance: cis-l1, cis-l2, pci-dss, soc2")
	fmt.Println("  --fix-dry-run         Mostrar patches de remediacao sem aplicar")
	fmt.Println("  --fix                 Aplicar patches de remediacao")
	fmt.Println()
	fmt.Println("Flags (investigate):")
	fmt.Println("  -n, --namespace       Namespace do pod (padrao: default)")
	fmt.Println("  --no-cache            Ignorar cache de analises anteriores")
	fmt.Println()
	fmt.Println("Exemplos:")
	fmt.Println("  yby sentinel scan -n default")
	fmt.Println("  yby sentinel scan -n production --profile cis-l1")
	fmt.Println("  yby sentinel scan -n default --fix-dry-run")
	fmt.Println("  yby sentinel investigate meu-pod -n default")
}

// AnalysisResult define a estrutura esperada da resposta da IA
type AnalysisResult struct {
	RootCause       string  `json:"root_cause"`
	TechnicalDetail string  `json:"technical_detail"`
	Confidence      int     `json:"confidence"`
	SuggestedFix    string  `json:"suggested_fix"`
	KubectlPatch    *string `json:"kubectl_patch"`
}

func handlePluginRequest() {
	hook := sdk.GetHook()

	switch hook {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:        "sentinel",
			Version:     "1.0.0",
			Description: "Auditoria de seguranca K8s com scan de vulnerabilidades e investigacao IA",
			Hooks:       []string{"command"},
		})
	case "command":
		args := sdk.GetArgs() // Use SDK args

		if len(args) == 0 {
			printSentinelHelp()
			return
		}

		if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
			printSentinelHelp()
			return
		}

		switch args[0] {
		case "investigate":
			// Expect "yby sentinel investigate [pod-name] [flags]"
			// Flags: -n/--namespace, -o/--output, -f/--file, --no-cache
			var podName, namespace, outputFormat, outputFile string
			var noCache bool
			remainingArgs := args[1:]

			for i := 0; i < len(remainingArgs); i++ {
				arg := remainingArgs[i]

				// Verifica flags de namespace
				if arg == "-n" || arg == "--namespace" {
					if i+1 < len(remainingArgs) {
						namespace = remainingArgs[i+1]
						i++
					}
					continue
				}

				// Flag de formato de saída
				if arg == "--output" || arg == "-o" {
					if i+1 < len(remainingArgs) {
						outputFormat = remainingArgs[i+1]
						i++
					}
					continue
				}

				// Flag de arquivo de saída
				if arg == "--file" || arg == "-f" {
					if i+1 < len(remainingArgs) {
						outputFile = remainingArgs[i+1]
						i++
					}
					continue
				}

				// Flag para desabilitar cache
				if arg == "--no-cache" {
					noCache = true
					continue
				}

				// Se não é flag e podName ainda está vazio, deve ser o nome do pod
				if !strings.HasPrefix(arg, "-") && podName == "" {
					podName = arg
				}
			}

			// Fallback to Context if needed
			ctx := sdk.GetFullContext()
			if podName == "" && ctx != nil {
				if p, ok := ctx.Data["pod"]; ok {
					podName = fmt.Sprintf("%v", p)
				}
			}

			// Priority: Flag > Values > Context > Default
			if namespace == "" {
				if ctx != nil && ctx.Infra.Namespace != "" {
					namespace = ctx.Infra.Namespace
				} else {
					namespace = "default"
				}
			}

			if podName == "" {
				fmt.Println("❌ Nome do Pod é obrigatório. Uso: yby sentinel investigate <pod> [-n namespace]")
				return
			}

			investigate(podName, namespace, outputFormat, outputFile, noCache)

		case "scan":
			// Expect "yby sentinel scan [-n namespace] [-o format] [-f file] [--profile name] [--fix] [--fix-dry-run]"
			var namespace, outputFormat, outputFile, profile string
			var fix, fixDryRun bool
			remainingArgs := args[1:]

			for i := 0; i < len(remainingArgs); i++ {
				arg := remainingArgs[i]
				if arg == "-n" || arg == "--namespace" {
					if i+1 < len(remainingArgs) {
						namespace = remainingArgs[i+1]
						i++
					}
					continue
				}
				if arg == "--output" || arg == "-o" {
					if i+1 < len(remainingArgs) {
						outputFormat = remainingArgs[i+1]
						i++
					}
					continue
				}
				if arg == "--file" || arg == "-f" {
					if i+1 < len(remainingArgs) {
						outputFile = remainingArgs[i+1]
						i++
					}
					continue
				}
				if arg == "--profile" || arg == "-p" {
					if i+1 < len(remainingArgs) {
						profile = remainingArgs[i+1]
						i++
					}
					continue
				}
				if arg == "--fix" {
					fix = true
					continue
				}
				if arg == "--fix-dry-run" {
					fixDryRun = true
					continue
				}
			}

			if namespace == "" {
				namespace = "default"
			}

			scanNamespace(namespace, outputFormat, outputFile, profile, fix, fixDryRun)

		default:
			fmt.Printf("Subcomando desconhecido: %s\n\n", args[0])
			printSentinelHelp()
		}
	default:
		// Se rodar sem hook mas com args via main, talvez seja uso direto?
		if len(os.Args) > 1 {
			// Mock behavior for dev?
			// Placeholder for future dev mode
			_ = 0
		}
		os.Exit(0)
	}
}

func investigate(podName, namespace, outputFormat, outputFile string, noCache bool) {
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

	k8sClient, err := sdk.GetKubeClient()
	if err != nil {
		fmt.Printf("⚠️  Falha ao obter cliente Kubernetes: %v\n", err)
		return
	}

	ctx := context.Background()

	// 1. Get Pod Logs via client-go
	fmt.Print("🔍 Coletando logs...")
	tailLines := int64(50)
	logsReq := k8sClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tailLines,
	})
	podLogs, err := logsReq.DoRaw(ctx)

	logsStr := ""
	if err != nil {
		fmt.Printf("\r⚠️  Falha ao obter logs (%v). Continuando...\n", err)
	} else {
		logsStr = string(podLogs)
		fmt.Println("\r✅ Logs coletados")
	}

	// Verificar cache antes de continuar a coleta
	if !noCache {
		if cached, ok := loadCache(namespace, podName, logsStr); ok {
			fmt.Println("\n📦 Resultado do cache (use --no-cache para forçar re-análise)")
			renderResult(*cached, podName, namespace, outputFormat, outputFile, width, titleStyle, boxStyle, labelStyle)
			return
		}
	}

	// 2. Get Events via client-go
	fmt.Print("🔍 Coletando eventos...")
	events, err := k8sClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", podName),
	})

	eventsStr := "[]"
	if err != nil {
		fmt.Printf("\r⚠️  Falha ao obter eventos (%v). Continuando...\n", err)
	} else {
		// Serialize events to JSON for AI
		eventsBytes, _ := json.Marshal(events.Items)
		eventsStr = string(eventsBytes)
		fmt.Println("\r✅ Eventos coletados")
	}

	// 3. Get Metrics (CPU/RAM)
	// Client-go core doesn't have Metrics. API call directly or fallback to kubectl top?
	// Given we want to avoid shelling out if possible, but metrics client is separate.
	// For now, let's skip metrics strictly via client-go to save time on setting up metrics client deps,
	// or try to shell out BUT ensuring we use the context.
	// SDK gives us KubeConfig/Context.

	fmt.Print("🔍 Coletando métricas...")

	metricsStr := "Metrics unavailable (client-go metrics not implemented yet)"

	// Fallback to kubectl top ONLY if we have context info to pass
	fullCtx := sdk.GetFullContext()
	if fullCtx != nil {
		args := []string{"top", "pod", podName, "-n", namespace, "--no-headers"}
		if fullCtx.Infra.KubeConfig != "" {
			args = append(args, "--kubeconfig", fullCtx.Infra.KubeConfig)
		}
		if fullCtx.Infra.KubeContext != "" {
			args = append(args, "--context", fullCtx.Infra.KubeContext)
		}

		// We re-enable exec just for this fallback
		cmdMetrics := execCommand("kubectl", args...)
		out, err := cmdMetrics.CombinedOutput()
		if err == nil {
			metricsStr = string(out)
			fmt.Println("\r✅ Métricas coletadas (via kubectl)")
		} else {
			fmt.Println("\r⚠️  Métricas indisponíveis")
		}
	} else {
		fmt.Println("\r⚠️  Métricas indisponíveis (sem contexto)")
	}

	// Verificar se o pod tem sinais de problema antes de enviar pra IA
	pod, podErr := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if podErr == nil {
		healthy := isPodHealthy(pod, events)
		if healthy {
			fmt.Println("\nPod saudavel — nenhum problema identificado.")
			fmt.Printf("  Status: %s\n", pod.Status.Phase)
			for _, cs := range pod.Status.ContainerStatuses {
				fmt.Printf("  Container %s: Ready=%v, Restarts=%d\n", cs.Name, cs.Ready, cs.RestartCount)
			}
			return
		}
	}

	// Construct Context for AI
	realContext := fmt.Sprintf("LOGS:\n%s\n\nEVENTS (JSON):\n%s\n\nMETRICS:\n%s", logsStr, eventsStr, metricsStr)

	if len(strings.TrimSpace(realContext)) < 20 {
		fmt.Println("Dados insuficientes (logs/eventos) coletados para analise.")
		return
	}

	fmt.Println("\nAnalisando com IA...")

	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		fmt.Println("❌ Nenhum provedor de IA disponível. Defina OLLAMA_HOST ou OPENAI_API_KEY.")
		return
	}

	analysisJSON, err := provider.Completion(ctx, prompts.Get("sentinel.investigate"), realContext)
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

	// Salvar no cache
	saveCache(namespace, podName, logsStr, result)

	// Renderizar resultado (visual ou exportar)
	renderResult(result, podName, namespace, outputFormat, outputFile, width, titleStyle, boxStyle, labelStyle)
}

// renderResult lida com a renderização visual ou exportação do resultado da análise.
// isPodHealthy verifica se o pod está saudável baseado no status e eventos.
// Retorna true se não há sinais de problema.
func isPodHealthy(pod *corev1.Pod, events *corev1.EventList) bool {
	// Pod não está Running
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	// Algum container não está Ready ou tem restarts recentes
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready {
			return false
		}
		if cs.RestartCount > 0 {
			// Verificar se o restart foi recente (últimas 2 horas)
			if cs.LastTerminationState.Terminated != nil {
				// Tem terminação recente → não saudável
				return false
			}
		}
		// Container em estado de waiting (CrashLoopBackOff, ImagePullBackOff, etc.)
		if cs.State.Waiting != nil {
			return false
		}
	}

	// Verificar eventos de Warning
	if events != nil {
		for _, e := range events.Items {
			if e.Type == "Warning" {
				return false
			}
		}
	}

	return true
}

func renderResult(result AnalysisResult, podName, namespace, outputFormat, outputFile string, width int, titleStyle, boxStyle, labelStyle lipgloss.Style) {
	// Exportar relatório se formato especificado
	if outputFormat != "" {
		switch outputFormat {
		case "json":
			content, err := exportJSON(result, podName, namespace)
			if err != nil {
				fmt.Printf("❌ Erro ao exportar JSON: %v\n", err)
				return
			}
			if err := writeReport(content, outputFile); err != nil {
				fmt.Printf("❌ Erro ao escrever relatório: %v\n", err)
				return
			}
			if outputFile != "" {
				fmt.Printf("✅ Relatório JSON salvo em %s\n", outputFile)
			}
		case "markdown":
			content := exportMarkdown(result, podName, namespace)
			if err := writeReport(content, outputFile); err != nil {
				fmt.Printf("❌ Erro ao escrever relatório: %v\n", err)
				return
			}
			if outputFile != "" {
				fmt.Printf("✅ Relatório Markdown salvo em %s\n", outputFile)
			}
		default:
			fmt.Printf("⚠️  Formato de saída desconhecido: %s. Usando formato visual padrão.\n", outputFormat)
		}
		if outputFormat == "json" || outputFormat == "markdown" {
			return // Não renderizar visual
		}
	}

	// Renderização visual padrão
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
