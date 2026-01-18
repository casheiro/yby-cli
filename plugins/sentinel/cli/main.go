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
			fmt.Printf("Error parsing YBY_PLUGIN_REQUEST: %v\n", err)
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

func handlePluginRequest(req plugin.PluginRequest) {
	switch req.Hook {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:    "sentinel",
			Version: "0.1.0",
			Hooks:   []string{"command"},
		})
	case "command":
		// Expect "yby sentinel investigate [pod-name] [namespace]"
		var podName, namespace string

		// Try to get from request Args (provided by CLI wrapper)
		// Logic: ["investigate", "pod-name", "namespace"]
		args := req.Args
		if len(args) > 0 && args[0] == "investigate" {
			if len(args) > 1 {
				podName = args[1]
			}
			if len(args) > 2 {
				namespace = args[2]
			}
		}

		// Fallback to Context if args missing
		if podName == "" && req.Context != nil {
			if p, ok := req.Context["pod"]; ok {
				podName = fmt.Sprintf("%v", p)
			}
			if n, ok := req.Context["namespace"]; ok {
				namespace = fmt.Sprintf("%v", n)
			}
		}

		if podName == "" {
			fmt.Println("❌ Pod name is required. Usage: yby sentinel investigate <pod> [namespace]")
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
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).Render("🛡️  Sentinel Investigation"))
	fmt.Printf("🔍 Fetching logs and events for pod '%s' in namespace '%s'...\n", podName, namespace)

	// 1. Get Pod Logs via kubectl
	cmdLogs := execCommand("kubectl", "logs", podName, "-n", namespace, "--tail=50")
	logsOut, err := cmdLogs.CombinedOutput()
	if err != nil {
		fmt.Printf("⚠️  Failed to get logs: %v\nOutput: %s\n", err, string(logsOut))
		// Continue? Maybe events help.
	}

	// 2. Get Events via kubectl
	// filtering by involvedObject.name involves field-selector which isn't always supported for simple pod name match on events
	// standard practice: kubectl get events -n namespace --field-selector involvedObject.name=podName
	cmdEvents := execCommand("kubectl", "get", "events", "-n", namespace,
		"--field-selector", fmt.Sprintf("involvedObject.name=%s", podName),
		"--sort-by='.lastTimestamp'",
		"-o", "json")

	eventsOut, err := cmdEvents.CombinedOutput()
	if err != nil {
		fmt.Printf("⚠️  Failed to get events: %v\n", err)
	}

	// Construct Context for AI
	realContext := fmt.Sprintf("LOGS:\n%s\n\nEVENTS (JSON):\n%s", string(logsOut), string(eventsOut))

	if len(strings.TrimSpace(realContext)) < 20 {
		fmt.Println("❌ No sufficient data (logs/events) gathered to analyze.")
		return
	}

	fmt.Println("🤖 Analyzing with AI...")

	ctx := context.Background()
	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		fmt.Println("❌ No AI provider available. Set OLLAMA_HOST or OPENAI_API_KEY.")
		return
	}

	analysis, err := provider.Completion(ctx, SentinelSystemPrompt, realContext)
	if err != nil {
		fmt.Printf("Error analyzing: %v\n", err)
		return
	}

	// Format Output
	fmt.Println(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).Render(analysis))
}

func respond(data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}
