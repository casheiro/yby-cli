package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Event structure from kubectl get events -o json (simplified)
type K8sEvent struct {
	InvolvedObject struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"involvedObject"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

type EventList struct {
	Items []K8sEvent `json:"items"`
}

func main() {
	fmt.Println("🛡️  Iniciando Agente Sentinel...")

	// Ensure kubectl is available
	_, err := exec.LookPath("kubectl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ kubectl not found. The agent needs kubectl to interact with the cluster.\n")
		os.Exit(1)
	}

	fmt.Println("👀 Monitorando Eventos Kubernetes por 'CrashLoopBackOff'...")

	// Native implementation with client-go is better, but to keep "Integrity" without
	// massive refactor of dependencies right now, we use a robust kubectl polling
	// or watch wrapper. The "Shell" concept allows this.
	//
	// Robust approach: Run kubectl get events -w --all-namespaces

	cmd := exec.Command("kubectl", "get", "events", "--all-namespaces", "--watch", "--output", "json")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	decoder := json.NewDecoder(stdout)
	for {
		// kubectl get events -o json --watch outputs stream of single JSON objects for modifications
		// Note: The specific output format of kubectl watch json can vary (WatchEvent vs native object).
		// Standard json output with watch wraps in {"type":"ADDED", "object": {...}}

		var watchEvent struct {
			Type   string   `json:"type"`
			Object K8sEvent `json:"object"`
		}

		if err := decoder.Decode(&watchEvent); err != nil {
			if err == io.EOF {
				break
			}
			// Tolerant decoding
			continue
		}

		// Analysis Logic
		go AnalyzeEvent(watchEvent.Object)
	}

	_ = cmd.Wait() // Should not reach here normally
}

// AnalyzeEvent checks if an event is critical. Returns true if critical.
func AnalyzeEvent(evt K8sEvent) bool {
	// Simple Heuristic for now (Brain in the Shell logic)
	// In a full implementation, this binary would forward to the CLI or central brain.
	// We check for CrashLoopBackOff specifically.
	isCritical := strings.Contains(evt.Message, "CrashLoopBackOff") ||
		strings.Contains(evt.Reason, "CrashLoopBackOff") ||
		strings.Contains(evt.Message, "OOMKilled") ||
		strings.Contains(evt.Reason, "OOMKilled")

	if isCritical {
		fmt.Printf("🚨 DETECTED CRITICAL EVENT: Pod %s (%s)\n", evt.InvolvedObject.Name, evt.Message)
		// Action: Could trigger webhook, call yby sentinel cli, etc.
	}
	return isCritical
}
