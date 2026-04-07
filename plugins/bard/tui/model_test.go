package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestNew verifica que o modelo é criado com os valores corretos.
func TestNew(t *testing.T) {
	config := Config{
		SystemPrompt: "test prompt",
		SessionID:    "20260405-143022",
		Namespace:    "default",
		Cluster:      "k3d-test",
		AIModel:      "gpt-4",
	}

	model := New(nil, nil, config)

	if model.state != stateIdle {
		t.Errorf("estado inicial esperado idle, obtido %d", model.state)
	}
	if model.config.SessionID != "20260405-143022" {
		t.Errorf("sessionID inesperado: %s", model.config.SessionID)
	}
	if model.config.Cluster != "k3d-test" {
		t.Errorf("cluster inesperado: %s", model.config.Cluster)
	}
}

// TestRenderStatusBar verifica que a status bar contém as informações corretas.
func TestRenderStatusBar(t *testing.T) {
	config := Config{
		SessionID: "test-session",
		Namespace: "production",
		Cluster:   "k3d-local",
		AIModel:   "gpt-4",
	}

	model := New(nil, nil, config)
	model.width = 120

	bar := model.renderStatusBar()

	if !strings.Contains(bar, "k3d-local") {
		t.Error("status bar deveria conter o cluster")
	}
	if !strings.Contains(bar, "production") {
		t.Error("status bar deveria conter o namespace")
	}
	if !strings.Contains(bar, "test-session") {
		t.Error("status bar deveria conter a sessão")
	}
	if !strings.Contains(bar, "gpt-4") {
		t.Errorf("status bar deveria conter o modelo, obteve: %q", bar)
	}
	if !strings.Contains(bar, "idle") {
		t.Errorf("status bar deveria conter o status idle, obteve: %q", bar)
	}
}

// TestUpdateWindowSize verifica que o modelo responde a mudanças de tamanho.
func TestUpdateWindowSize(t *testing.T) {
	model := New(nil, nil, Config{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m := updated.(Model)

	if m.width != 100 {
		t.Errorf("largura esperada 100, obtida %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("altura esperada 40, obtida %d", m.height)
	}
}

// TestUpdateCtrlC verifica que Ctrl+C causa saída.
func TestUpdateCtrlC(t *testing.T) {
	model := New(nil, nil, Config{})

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Ctrl+C deveria retornar um comando de saída")
	}
}

// TestResponseMsg verifica processamento de resposta da IA.
func TestResponseMsg(t *testing.T) {
	model := New(nil, nil, Config{})
	model.state = stateStreaming

	updated, _ := model.Update(responseMsg{content: "resposta da IA"})
	m := updated.(Model)

	if m.state != stateIdle {
		t.Error("estado deveria voltar para idle após resposta")
	}
	if len(m.messages) != 1 {
		t.Fatalf("esperava 1 mensagem, obteve %d", len(m.messages))
	}
	if m.messages[0].role != "assistant" {
		t.Errorf("role esperado 'assistant', obtido '%s'", m.messages[0].role)
	}
	if m.messages[0].content != "resposta da IA" {
		t.Errorf("conteúdo inesperado: %s", m.messages[0].content)
	}
}

// TestResponseMsg_ComErro verifica processamento de erro da IA.
func TestResponseMsg_ComErro(t *testing.T) {
	model := New(nil, nil, Config{})
	model.state = stateStreaming

	updated, _ := model.Update(responseMsg{err: fmt.Errorf("falha na IA")})
	m := updated.(Model)

	if m.state != stateIdle {
		t.Error("estado deveria voltar para idle após erro")
	}
	if len(m.messages) != 1 {
		t.Fatalf("esperava 1 mensagem, obteve %d", len(m.messages))
	}
	if m.messages[0].role != "error" {
		t.Errorf("role esperado 'error', obtido '%s'", m.messages[0].role)
	}
}

// TestView_SemTamanho verifica que View sem tamanho mostra loading.
func TestView_SemTamanho(t *testing.T) {
	model := New(nil, nil, Config{})
	view := model.View()
	if !strings.Contains(view, "Carregando") {
		t.Error("view sem tamanho deveria mostrar 'Carregando'")
	}
}
