package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
)

// MockClient implementa monitor.Client para testes
type MockClient struct {
	Pods         []monitor.Pod
	Deployments  []monitor.Deployment
	Services     []monitor.Service
	Nodes        []monitor.Node
	StatefulSets []monitor.StatefulSet
	Jobs         []monitor.Job
	Ingresses    []monitor.Ingress
	ConfigMaps   []monitor.ConfigMap
	Events       []monitor.Event
	Err          error
}

func (m *MockClient) GetPods(_ monitor.ListFilter) ([]monitor.Pod, error)               { return m.Pods, m.Err }
func (m *MockClient) GetDeployments(_ monitor.ListFilter) ([]monitor.Deployment, error) { return m.Deployments, m.Err }
func (m *MockClient) GetServices(_ monitor.ListFilter) ([]monitor.Service, error)       { return m.Services, m.Err }
func (m *MockClient) GetNodes(_ monitor.ListFilter) ([]monitor.Node, error)             { return m.Nodes, m.Err }
func (m *MockClient) GetStatefulSets(_ monitor.ListFilter) ([]monitor.StatefulSet, error) { return m.StatefulSets, m.Err }
func (m *MockClient) GetJobs(_ monitor.ListFilter) ([]monitor.Job, error)                 { return m.Jobs, m.Err }
func (m *MockClient) GetIngresses(_ monitor.ListFilter) ([]monitor.Ingress, error)        { return m.Ingresses, m.Err }
func (m *MockClient) GetConfigMaps(_ monitor.ListFilter) ([]monitor.ConfigMap, error)     { return m.ConfigMaps, m.Err }
func (m *MockClient) GetEvents(_ monitor.ListFilter) ([]monitor.Event, error)             { return m.Events, m.Err }

// --- Testes de criação do Model ---

// TestModel_NewModel_ComClient verifica que o modelo é criado sem erro
// quando um client válido é fornecido.
func TestModel_NewModel_ComClient(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	if model.err != nil {
		t.Errorf("esperava err nil, obtido: %v", model.err)
	}
	if model.client == nil {
		t.Error("esperava client não nil")
	}
}

// TestModel_NewModel_SemClient verifica que o modelo é criado com erro
// quando o client é nil.
func TestModel_NewModel_SemClient(t *testing.T) {
	model := NewModel(nil)

	if model.err == nil {
		t.Error("esperava err não nil para client nil")
	}
	if model.client != nil {
		t.Error("esperava client nil")
	}
}

// --- Testes de Init ---

// TestModel_Init verifica que Init retorna comandos quando há client válido.
func TestModel_Init(t *testing.T) {
	client := &MockClient{
		Pods: []monitor.Pod{},
	}
	model := NewModel(client)

	cmd := model.Init()
	if cmd == nil {
		t.Error("Init retornou comando nil com client válido")
	}
}

// TestModel_Init_SemClient verifica que Init retorna nil quando há erro.
func TestModel_Init_SemClient(t *testing.T) {
	model := NewModel(nil)

	cmd := model.Init()
	if cmd != nil {
		t.Error("Init deveria retornar nil quando há erro")
	}
}

// --- Testes de navegação por Tab ---

// TestModel_Update_TabNavigation verifica a navegação entre abas com a tecla tab.
func TestModel_Update_TabNavigation(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	// Tab avança para próxima aba
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	m := newModel.(Model)

	if m.activeTab != TabDeployments {
		t.Errorf("esperava aba TabDeployments (1), obtido %d", m.activeTab)
	}

	// Mais um tab
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newModel.(Model)

	if m.activeTab != TabServices {
		t.Errorf("esperava aba TabServices (2), obtido %d", m.activeTab)
	}
}

// TestModel_Update_ShiftTabNavigation verifica navegação reversa com shift+tab.
func TestModel_Update_ShiftTabNavigation(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	// Shift+tab volta para última aba (wrap around)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m := newModel.(Model)

	if m.activeTab != TabEvents {
		t.Errorf("esperava aba TabEvents (8), obtido %d", m.activeTab)
	}
}

// TestModel_Update_NumberKeys verifica navegação direta por teclas numéricas.
func TestModel_Update_NumberKeys(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	testes := []struct {
		tecla    string
		esperado ResourceTab
	}{
		{"1", TabPods},
		{"2", TabDeployments},
		{"3", TabServices},
		{"4", TabNodes},
		{"5", TabStatefulSets},
		{"6", TabJobs},
		{"7", TabIngresses},
		{"8", TabConfigMaps},
		{"9", TabEvents},
	}

	for _, tt := range testes {
		t.Run(fmt.Sprintf("tecla_%s", tt.tecla), func(t *testing.T) {
			newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.tecla)})
			m := newModel.(Model)

			if m.activeTab != tt.esperado {
				t.Errorf("tecla '%s': esperava aba %d, obtido %d", tt.tecla, tt.esperado, m.activeTab)
			}
		})
	}
}

// --- Testes de scroll ---

// TestModel_Update_Scroll verifica seleção com j e k.
func TestModel_Update_Scroll(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	// Definir height pequeno e conteúdo suficiente para permitir scroll
	model.height = 10
	for i := 0; i < 20; i++ {
		model.pods = append(model.pods, monitor.Pod{
			Name: fmt.Sprintf("pod-%d", i), Status: "Running", Namespace: "default", CPU: "10m", Memory: "64Mi",
		})
	}

	// Mover seleção para baixo com j
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m := newModel.(Model)

	if m.selectedIndex != 1 {
		t.Errorf("esperava selectedIndex 1 após 'j', obtido %d", m.selectedIndex)
	}

	// Mais um j
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newModel.(Model)

	if m.selectedIndex != 2 {
		t.Errorf("esperava selectedIndex 2 após segundo 'j', obtido %d", m.selectedIndex)
	}

	// Mover seleção para cima com k
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newModel.(Model)

	if m.selectedIndex != 1 {
		t.Errorf("esperava selectedIndex 1 após 'k', obtido %d", m.selectedIndex)
	}

	// k não pode ir abaixo de 0
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newModel.(Model)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newModel.(Model)

	if m.selectedIndex != 0 {
		t.Errorf("esperava selectedIndex 0 (mínimo), obtido %d", m.selectedIndex)
	}
}

// TestModel_Update_TabResetScroll verifica que trocar de aba reseta o scroll e a seleção.
func TestModel_Update_TabResetScroll(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.height = 10
	for i := 0; i < 20; i++ {
		model.pods = append(model.pods, monitor.Pod{
			Name: fmt.Sprintf("pod-%d", i), Status: "Running", Namespace: "default", CPU: "10m", Memory: "64Mi",
		})
	}

	// Mover seleção
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m := newModel.(Model)

	if m.selectedIndex != 1 {
		t.Fatalf("esperava selectedIndex 1, obtido %d", m.selectedIndex)
	}

	// Trocar aba reseta scroll e seleção
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newModel.(Model)

	if m.scrollY != 0 {
		t.Errorf("esperava scrollY 0 após trocar aba, obtido %d", m.scrollY)
	}
	if m.selectedIndex != 0 {
		t.Errorf("esperava selectedIndex 0 após trocar aba, obtido %d", m.selectedIndex)
	}
}

// --- Testes de mensagens de dados ---

// TestModel_Update_PodMsg verifica recepção de dados de pods.
func TestModel_Update_PodMsg(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	pods := []monitor.Pod{
		{Name: "pod-1", Status: "Running", Namespace: "default"},
		{Name: "pod-2", Status: "Pending", Namespace: "kube-system"},
	}

	newModel, _ := model.Update(pods)
	m := newModel.(Model)

	if len(m.pods) != 2 {
		t.Errorf("esperado 2 pods, obtido %d", len(m.pods))
	}
	if m.pods[0].Name != "pod-1" {
		t.Errorf("esperado 'pod-1', obtido '%s'", m.pods[0].Name)
	}
}

// TestModel_Update_DeploymentMsg verifica recepção de dados de deployments.
func TestModel_Update_DeploymentMsg(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	deps := []monitor.Deployment{
		{Name: "deploy-1", Namespace: "default", Replicas: 3, Ready: 3, Available: 3},
	}

	newModel, _ := model.Update(deps)
	m := newModel.(Model)

	if len(m.deployments) != 1 {
		t.Errorf("esperado 1 deployment, obtido %d", len(m.deployments))
	}
	if m.deployments[0].Name != "deploy-1" {
		t.Errorf("esperado 'deploy-1', obtido '%s'", m.deployments[0].Name)
	}
}

// TestModel_Update_ServiceMsg verifica recepção de dados de services.
func TestModel_Update_ServiceMsg(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	svcs := []monitor.Service{
		{Name: "svc-1", Namespace: "default", Type: "ClusterIP", ClusterIP: "10.0.0.1", Ports: "80/TCP"},
	}

	newModel, _ := model.Update(svcs)
	m := newModel.(Model)

	if len(m.services) != 1 {
		t.Errorf("esperado 1 service, obtido %d", len(m.services))
	}
	if m.services[0].Name != "svc-1" {
		t.Errorf("esperado 'svc-1', obtido '%s'", m.services[0].Name)
	}
}

// TestModel_Update_NodeMsg verifica recepção de dados de nodes.
func TestModel_Update_NodeMsg(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	nodes := []monitor.Node{
		{Name: "node-1", Status: "Ready", CPUCapacity: "4", MemoryCapacity: "8Gi", Version: "v1.29.0"},
	}

	newModel, _ := model.Update(nodes)
	m := newModel.(Model)

	if len(m.nodes) != 1 {
		t.Errorf("esperado 1 node, obtido %d", len(m.nodes))
	}
	if m.nodes[0].Name != "node-1" {
		t.Errorf("esperado 'node-1', obtido '%s'", m.nodes[0].Name)
	}
}

// --- Testes de View ---

// TestModel_View_Pods verifica que a view de pods renderiza corretamente.
func TestModel_View_Pods(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.pods = []monitor.Pod{
		{Name: "nginx-pod", Status: "Running", Namespace: "default", CPU: "10m", Memory: "64Mi"},
	}

	view := model.View()

	if !strings.Contains(view, "nginx-pod") {
		t.Error("view deveria conter o nome do pod 'nginx-pod'")
	}
	if !strings.Contains(view, "Cluster Monitor") {
		t.Error("view deveria conter o título 'Cluster Monitor'")
	}
}

// TestModel_View_Deployments verifica que a view de deployments renderiza corretamente.
func TestModel_View_Deployments(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.activeTab = TabDeployments
	model.deployments = []monitor.Deployment{
		{Name: "api-deploy", Namespace: "backend", Replicas: 3, Ready: 3, Available: 3},
	}

	view := model.View()

	if !strings.Contains(view, "api-deploy") {
		t.Error("view deveria conter o nome do deployment 'api-deploy'")
	}
}

// TestModel_View_Services verifica que a view de services renderiza corretamente.
func TestModel_View_Services(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.activeTab = TabServices
	model.services = []monitor.Service{
		{Name: "web-svc", Namespace: "default", Type: "ClusterIP", ClusterIP: "10.0.0.1", Ports: "80/TCP"},
	}

	view := model.View()

	if !strings.Contains(view, "web-svc") {
		t.Error("view deveria conter o nome do service 'web-svc'")
	}
}

// TestModel_View_Nodes verifica que a view de nodes renderiza corretamente.
func TestModel_View_Nodes(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.activeTab = TabNodes
	model.nodes = []monitor.Node{
		{Name: "worker-1", Status: "Ready", CPUCapacity: "4", MemoryCapacity: "8Gi", Version: "v1.29.0"},
	}

	view := model.View()

	if !strings.Contains(view, "worker-1") {
		t.Error("view deveria conter o nome do node 'worker-1'")
	}
}

// TestModel_View_Error verifica que a view exibe mensagem de erro.
func TestModel_View_Error(t *testing.T) {
	model := NewModel(nil)
	view := model.View()

	if len(view) == 0 {
		t.Error("view está vazia")
	}
	if !strings.Contains(view, "Erro") {
		t.Error("view deveria conter mensagem de erro")
	}
	if !strings.Contains(view, "q") {
		t.Error("view deveria conter instrução para sair")
	}
}

// TestModel_View_Carregando verifica a mensagem de carregamento.
func TestModel_View_Carregando(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	view := model.View()

	if !strings.Contains(view, "Carregando") {
		t.Error("view deveria conter mensagem de carregamento")
	}
}

// --- Teste de WindowSize ---

// TestModel_Update_WindowSize verifica que a mensagem de redimensionamento
// atualiza width e height do modelo.
func TestModel_Update_WindowSize(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := newModel.(Model)

	if m.width != 120 {
		t.Errorf("esperava width 120, obtido %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("esperava height 40, obtido %d", m.height)
	}
}

// --- Teste de Quit ---

// TestModel_Update_Quit verifica que 'q' retorna comando de saída.
func TestModel_Update_Quit(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("esperava comando de saída, obtido nil")
	}
}

// TestModel_Update_ErrorMsg verifica que mensagens de erro são armazenadas.
func TestModel_Update_ErrorMsg(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	errMsg := fmt.Errorf("falha ao conectar")
	newModel, _ := model.Update(errMsg)
	m := newModel.(Model)

	if m.err == nil {
		t.Error("esperava erro armazenado no modelo")
	}
	if m.err.Error() != "falha ao conectar" {
		t.Errorf("mensagem de erro inesperada: %v", m.err)
	}
}

// --- Testes de mensagens dos novos recursos ---

// TestModel_Update_StatefulSetMsg verifica recepção de dados de statefulsets.
func TestModel_Update_StatefulSetMsg(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	sets := []monitor.StatefulSet{
		{Name: "redis-ss", Namespace: "default", Replicas: 3, Ready: 3},
	}

	newModel, _ := model.Update(sets)
	m := newModel.(Model)

	if len(m.statefulsets) != 1 {
		t.Errorf("esperado 1 statefulset, obtido %d", len(m.statefulsets))
	}
	if m.statefulsets[0].Name != "redis-ss" {
		t.Errorf("esperado 'redis-ss', obtido '%s'", m.statefulsets[0].Name)
	}
}

// TestModel_Update_JobMsg verifica recepção de dados de jobs.
func TestModel_Update_JobMsg(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	jobs := []monitor.Job{
		{Name: "backup-job", Namespace: "default", Completions: 1, Succeeded: 1},
	}

	newModel, _ := model.Update(jobs)
	m := newModel.(Model)

	if len(m.jobs) != 1 {
		t.Errorf("esperado 1 job, obtido %d", len(m.jobs))
	}
}

// TestModel_Update_IngressMsg verifica recepção de dados de ingresses.
func TestModel_Update_IngressMsg(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	ings := []monitor.Ingress{
		{Name: "web-ing", Namespace: "default", Class: "nginx", Hosts: "app.example.com", Paths: "/"},
	}

	newModel, _ := model.Update(ings)
	m := newModel.(Model)

	if len(m.ingresses) != 1 {
		t.Errorf("esperado 1 ingress, obtido %d", len(m.ingresses))
	}
}

// TestModel_Update_ConfigMapMsg verifica recepção de dados de configmaps.
func TestModel_Update_ConfigMapMsg(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	cms := []monitor.ConfigMap{
		{Name: "app-config", Namespace: "default", Keys: 5, DataSize: "2.1Ki"},
	}

	newModel, _ := model.Update(cms)
	m := newModel.(Model)

	if len(m.configmaps) != 1 {
		t.Errorf("esperado 1 configmap, obtido %d", len(m.configmaps))
	}
}

// TestModel_Update_EventMsg verifica recepção de dados de eventos.
func TestModel_Update_EventMsg(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)

	events := []monitor.Event{
		{Name: "pod-1", Namespace: "default", Type: "Normal", Reason: "Scheduled", Message: "Pod scheduled", Age: "5m"},
	}

	newModel, _ := model.Update(events)
	m := newModel.(Model)

	if len(m.events) != 1 {
		t.Errorf("esperado 1 evento, obtido %d", len(m.events))
	}
}

// --- Testes de Views dos novos recursos ---

// TestModel_View_StatefulSets verifica que a view de statefulsets renderiza corretamente.
func TestModel_View_StatefulSets(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.activeTab = TabStatefulSets
	model.statefulsets = []monitor.StatefulSet{
		{Name: "redis-ss", Namespace: "default", Replicas: 3, Ready: 3},
	}

	view := model.View()
	if !strings.Contains(view, "redis-ss") {
		t.Error("view deveria conter 'redis-ss'")
	}
}

// TestModel_View_Jobs verifica que a view de jobs renderiza corretamente.
func TestModel_View_Jobs(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.activeTab = TabJobs
	model.jobs = []monitor.Job{
		{Name: "backup-job", Namespace: "default", Completions: 1, Succeeded: 1},
	}

	view := model.View()
	if !strings.Contains(view, "backup-job") {
		t.Error("view deveria conter 'backup-job'")
	}
}

// TestModel_View_Ingresses verifica que a view de ingresses renderiza corretamente.
func TestModel_View_Ingresses(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.activeTab = TabIngresses
	model.ingresses = []monitor.Ingress{
		{Name: "web-ing", Namespace: "default", Class: "nginx", Hosts: "app.example.com", Paths: "/"},
	}

	view := model.View()
	if !strings.Contains(view, "web-ing") {
		t.Error("view deveria conter 'web-ing'")
	}
}

// TestModel_View_ConfigMaps verifica que a view de configmaps renderiza corretamente.
func TestModel_View_ConfigMaps(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.activeTab = TabConfigMaps
	model.configmaps = []monitor.ConfigMap{
		{Name: "app-config", Namespace: "default", Keys: 5, DataSize: "2.1Ki"},
	}

	view := model.View()
	if !strings.Contains(view, "app-config") {
		t.Error("view deveria conter 'app-config'")
	}
}

// TestModel_View_Events verifica que a view de eventos renderiza corretamente.
func TestModel_View_Events(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.activeTab = TabEvents
	model.events = []monitor.Event{
		{Name: "pod-1", Namespace: "default", Type: "Warning", Reason: "BackOff", Message: "Restart", Age: "2m"},
	}

	view := model.View()
	if !strings.Contains(view, "pod-1") {
		t.Error("view deveria conter 'pod-1'")
	}
}

// --- Testes de seleção ---

// TestModel_Selection_HomeEnd verifica que Home e End movem a seleção.
func TestModel_Selection_HomeEnd(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	for i := 0; i < 10; i++ {
		model.pods = append(model.pods, monitor.Pod{
			Name: fmt.Sprintf("pod-%d", i), Status: "Running", Namespace: "default", CPU: "10m", Memory: "64Mi",
		})
	}

	// End vai para o último item
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m := newModel.(Model)
	if m.selectedIndex != 9 {
		t.Errorf("esperava selectedIndex 9 após End, obtido %d", m.selectedIndex)
	}

	// Home volta para o primeiro item
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = newModel.(Model)
	if m.selectedIndex != 0 {
		t.Errorf("esperava selectedIndex 0 após Home, obtido %d", m.selectedIndex)
	}
}

// TestModel_Selection_NoOverflow verifica que j não ultrapassa o último item.
func TestModel_Selection_NoOverflow(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.pods = []monitor.Pod{
		{Name: "pod-0", Status: "Running", Namespace: "default", CPU: "10m", Memory: "64Mi"},
		{Name: "pod-1", Status: "Running", Namespace: "default", CPU: "10m", Memory: "64Mi"},
	}

	// Mover até o fim
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m := newModel.(Model)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newModel.(Model)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newModel.(Model)

	if m.selectedIndex != 1 {
		t.Errorf("esperava selectedIndex 1 (máximo), obtido %d", m.selectedIndex)
	}
}

// TestModel_Selection_AutoScroll verifica scroll automático ao mover seleção.
func TestModel_Selection_AutoScroll(t *testing.T) {
	client := &MockClient{}
	model := NewModel(client)
	model.height = 8 // visibleHeight = 8-5 = 3
	for i := 0; i < 10; i++ {
		model.pods = append(model.pods, monitor.Pod{
			Name: fmt.Sprintf("pod-%d", i), Status: "Running", Namespace: "default", CPU: "10m", Memory: "64Mi",
		})
	}

	// Mover seleção para baixo até sair da viewport
	m := model
	for i := 0; i < 5; i++ {
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = newModel.(Model)
	}

	if m.selectedIndex != 5 {
		t.Errorf("esperava selectedIndex 5, obtido %d", m.selectedIndex)
	}
	// scrollY deve ter sido ajustado para manter o item visível
	if m.scrollY <= 0 {
		t.Errorf("esperava scrollY > 0 após mover seleção para fora da viewport, obtido %d", m.scrollY)
	}
}
