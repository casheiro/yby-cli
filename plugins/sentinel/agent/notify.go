package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// NotifyPayload é o payload enviado via webhook.
type NotifyPayload struct {
	Timestamp string `json:"timestamp"`
	Pod       string `json:"pod"`
	Kind      string `json:"kind"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	EventType string `json:"event_type"`
}

// notifyWebhook envia notificação para os webhooks configurados.
func notifyWebhook(evt K8sEvent) {
	payload := NotifyPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Pod:       evt.InvolvedObject.Name,
		Kind:      evt.InvolvedObject.Kind,
		Reason:    evt.Reason,
		Message:   evt.Message,
		EventType: evt.Type,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("falha ao serializar payload de notificação", "erro", err)
		return
	}

	// Webhook genérico
	if url := os.Getenv("SENTINEL_WEBHOOK_URL"); url != "" {
		sendWebhook(url, data)
	}

	// Slack webhook
	if url := os.Getenv("SENTINEL_SLACK_WEBHOOK"); url != "" {
		slackPayload := map[string]string{
			"text": fmt.Sprintf("*Sentinel Alert*\nPod: `%s`\nRazão: %s\nMensagem: %s", evt.InvolvedObject.Name, evt.Reason, evt.Message),
		}
		slackData, _ := json.Marshal(slackPayload)
		sendWebhook(url, slackData)
	}
}

// sendWebhook envia um POST HTTP com retry simples.
func sendWebhook(url string, data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			slog.Error("falha ao criar request de webhook", "url", url, "erro", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Warn("falha ao enviar webhook", "url", url, "tentativa", attempt, "erro", err)
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt*2) * time.Second)
				continue
			}
			slog.Error("webhook falhou após todas as tentativas", "url", url)
			return
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			slog.Info("webhook enviado com sucesso", "url", url, "status", resp.StatusCode)
			return
		}

		slog.Warn("webhook retornou status inesperado", "url", url, "status", resp.StatusCode, "tentativa", attempt)
		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}
}
