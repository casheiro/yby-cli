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
			Version:     "0.2.0",
			Description: "Auditoria de segurança e conformidade (CIS/NSA)",
			Hooks:       []string{"command"},
		})
	case "command":
		// Expect "yby sentinel investigate [pod-name] [flags]"
		// Flags: -n/--namespace
		var podName, namespace string
		args := sdk.GetArgs() // Use SDK args

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

		investigate(podName, namespace)
	default:
		// Se rodar sem hook mas com args via main, talvez seja uso direto?
		if len(os.Args) > 1 {
			// Mock behavior for dev?
		}
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
		cmdMetrics := exec.Command("kubectl", args...)
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

	// Construct Context for AI
	realContext := fmt.Sprintf("LOGS:\n%s\n\nEVENTS (JSON):\n%s\n\nMETRICS:\n%s", logsStr, eventsStr, metricsStr)

	if len(strings.TrimSpace(realContext)) < 20 {
		fmt.Println("❌ Dados insuficientes (logs/eventos) coletados para análise.")
		return
	}

	fmt.Println("\n🤖 Analisando com IA...")

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
