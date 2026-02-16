package ui

import (
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
)

// MockClient implements monitor.Client for testing
type MockClient struct {
	Pods []monitor.Pod
	Err  error
}

func (m *MockClient) GetPods() ([]monitor.Pod, error) {
	return m.Pods, m.Err
}

func TestModel_Init(t *testing.T) {
	client := &MockClient{
		Pods: []monitor.Pod{},
	}
	model := Model{client: client, pods: []monitor.Pod{}}

	cmd := model.Init()
	if cmd == nil {
		t.Error("Init returned nil command")
	}
}

func TestModel_Update(t *testing.T) {
	client := &MockClient{
		Pods: []monitor.Pod{
			{Name: "pod-1", Status: "Running", Namespace: "default"},
		},
	}
	// Initial state
	model := Model{client: client, pods: []monitor.Pod{}}

	// Simulator update logic directly?
	// Or trust the loop.
	// Test data reception
	podsMsg := client.Pods

	newModel, _ := model.Update(podsMsg)
	m := newModel.(Model)

	if len(m.pods) != 1 {
		t.Errorf("Expected 1 pod, got %d", len(m.pods))
	}
	if m.pods[0].Name != "pod-1" {
		t.Errorf("Expected pod-1, got %s", m.pods[0].Name)
	}
}

func TestModel_View_Error(t *testing.T) {
	model := Model{err: fmt.Errorf("connection refused")}
	view := model.View()

	// Expect error message
	if len(view) == 0 {
		t.Error("View is empty")
	}
	// Simple string check
	// Note: View contains color codes, so exact match is hard.
}
