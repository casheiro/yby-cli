package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// K8sEvent representa um evento Kubernetes simplificado obtido via kubectl.
type K8sEvent struct {
	InvolvedObject struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"involvedObject"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

// EventList representa uma lista de eventos Kubernetes.
type EventList struct {
	Items []K8sEvent `json:"items"`
}

func main() {
	slog.Info("Iniciando Agente Sentinel")

	// Verificar se kubectl está disponível
	_, err := exec.LookPath("kubectl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubectl não encontrado. O agente precisa do kubectl para interagir com o cluster.\n")
		os.Exit(1)
	}

	slog.Info("Monitorando eventos Kubernetes", "padrões", "CrashLoopBackOff, OOMKilled")

	cmd := exec.Command("kubectl", "get", "events", "--all-namespaces", "--watch", "--output", "json")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("falha ao criar pipe de stdout", "erro", err)
		fmt.Fprintf(os.Stderr, "erro fatal: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		slog.Error("falha ao iniciar kubectl", "erro", err)
		fmt.Fprintf(os.Stderr, "erro fatal: %v\n", err)
		os.Exit(1)
	}

	decoder := json.NewDecoder(stdout)
	for {
		var watchEvent struct {
			Type   string   `json:"type"`
			Object K8sEvent `json:"object"`
		}

		if err := decoder.Decode(&watchEvent); err != nil {
			if err == io.EOF {
				break
			}
			// Decodificação tolerante a erros
			continue
		}

		go AnalyzeEvent(watchEvent.Object)
	}

	_ = cmd.Wait()
}

// AnalyzeEvent verifica se um evento é crítico. Retorna true se for crítico.
// Caso o evento seja crítico, envia notificação via webhook (se configurado).
func AnalyzeEvent(evt K8sEvent) bool {
	isCritical := strings.Contains(evt.Message, "CrashLoopBackOff") ||
		strings.Contains(evt.Reason, "CrashLoopBackOff") ||
		strings.Contains(evt.Message, "OOMKilled") ||
		strings.Contains(evt.Reason, "OOMKilled")

	if isCritical {
		slog.Warn("evento crítico detectado", "pod", evt.InvolvedObject.Name, "mensagem", evt.Message, "razão", evt.Reason)
		// Notificar via webhook se configurado
		notifyWebhook(evt)
	}
	return isCritical
}
