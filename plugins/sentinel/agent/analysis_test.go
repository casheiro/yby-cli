package main

import (
	"testing"
)

func TestAnalyzeEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    K8sEvent
		wantCrit bool
	}{
		{
			name: "Normal Event",
			event: K8sEvent{
				Reason:  "Started",
				Message: "Started container foo",
				Type:    "Normal",
			},
			wantCrit: false,
		},
		{
			name: "CrashLoopBackOff in Message",
			event: K8sEvent{
				Reason:  "BackOff",
				Message: "Back-off restarting failed container. CrashLoopBackOff...",
				Type:    "Warning",
			},
			wantCrit: true,
		},
		{
			name: "CrashLoopBackOff in Reason",
			event: K8sEvent{
				Reason:  "CrashLoopBackOff",
				Message: "Container failed",
				Type:    "Warning",
			},
			wantCrit: true,
		},
		{
			name: "OOMKilled Detection",
			event: K8sEvent{
				Reason:  "OOMKilled",
				Message: "Container limit exceeded",
				Type:    "Warning",
			},
			wantCrit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AnalyzeEvent(tt.event); got != tt.wantCrit {
				t.Errorf("AnalyzeEvent() = %v, want %v", got, tt.wantCrit)
			}
		})
	}
}
