package main

import (
	"encoding/json"
	"testing"
)

func TestAnalyzeEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    K8sEvent
		wantCrit bool
	}{
		{
			name: "Evento normal",
			event: K8sEvent{
				Reason:  "Started",
				Message: "Started container foo",
				Type:    "Normal",
			},
			wantCrit: false,
		},
		{
			name: "CrashLoopBackOff na mensagem",
			event: K8sEvent{
				Reason:  "BackOff",
				Message: "Back-off restarting failed container. CrashLoopBackOff...",
				Type:    "Warning",
			},
			wantCrit: true,
		},
		{
			name: "CrashLoopBackOff na razão",
			event: K8sEvent{
				Reason:  "CrashLoopBackOff",
				Message: "Container failed",
				Type:    "Warning",
			},
			wantCrit: true,
		},
		{
			name: "Detecção de OOMKilled",
			event: K8sEvent{
				Reason:  "OOMKilled",
				Message: "Container limit exceeded",
				Type:    "Warning",
			},
			wantCrit: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := AnalyzeEvent(tc.event); got != tc.wantCrit {
				t.Errorf("AnalyzeEvent() = %v, esperado %v", got, tc.wantCrit)
			}
		})
	}
}

func TestNotifyPayload_Serialization(t *testing.T) {
	payload := NotifyPayload{
		Timestamp: "2024-01-01T00:00:00Z",
		Pod:       "test-pod",
		Kind:      "Pod",
		Reason:    "CrashLoopBackOff",
		Message:   "back-off restarting failed container",
		EventType: "Warning",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("falha ao serializar payload: %v", err)
	}
	var decoded NotifyPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("falha ao desserializar payload: %v", err)
	}
	if decoded.Pod != "test-pod" {
		t.Errorf("pod esperado 'test-pod', obtido %q", decoded.Pod)
	}
	if decoded.Reason != "CrashLoopBackOff" {
		t.Errorf("razão esperada 'CrashLoopBackOff', obtida %q", decoded.Reason)
	}
}

func TestSendWebhook_ServidorInvalido(t *testing.T) {
	// Testar que sendWebhook não causa panic com URL inválida
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("sendWebhook causou panic: %v", r)
		}
	}()
	sendWebhook("http://localhost:1/invalid", []byte(`{"test": true}`))
}
