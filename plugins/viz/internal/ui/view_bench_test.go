package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
)

// Medidas do desenho num cluster grande: 10 mil pods e 20 mil eventos em
// todos os namespaces, na janela de 160x45 (painel ao lado). Rodar com:
// cd plugins/viz && go test -run '^$' -bench . -benchmem ./internal/ui/

const (
	podsGrandes    = 10_000
	eventosGrandes = 20_000
)

// podsDoClusterGrande são n pods em 50 namespaces, um em cada 20 com problema.
func podsDoClusterGrande(n int) []monitor.Pod {
	pods := make([]monitor.Pod, n)
	for i := range pods {
		status, prontos := "Running", int32(1)
		if i%20 == 0 {
			status, prontos = "CrashLoopBackOff", 0
		}
		pods[i] = monitor.Pod{
			Name: fmt.Sprintf("app-%05d-x2k2p", i), Namespace: fmt.Sprintf("ns-%02d", i%50), UID: fmt.Sprint(i),
			Status: status, Prontos: prontos, Containers: 1, CriadoEm: horaDaPrancha.Add(-time.Duration(i) * time.Minute),
			No: fmt.Sprintf("ip-10-0-%d-5", i%30), IP: fmt.Sprintf("10.42.%d.%d", i/250, i%250),
			Labels: map[string]string{"app": fmt.Sprintf("app-%d", i%100)},
		}
	}
	return pods
}

// eventosDoClusterGrande são n eventos, um em cada 10 Warning, dos mais recentes aos mais antigos.
func eventosDoClusterGrande(n int) []monitor.Event {
	eventos := make([]monitor.Event, n)
	for i := range eventos {
		tipo := "Normal"
		if i%10 == 0 {
			tipo = "Warning"
		}
		eventos[i] = monitor.Event{
			ID: fmt.Sprintf("evento-%05d", i), Name: fmt.Sprintf("app-%05d-x2k2p", i%podsGrandes), Namespace: fmt.Sprintf("ns-%02d", i%50),
			Type: tipo, Reason: "Pulled", Message: "Container image already present on machine", Age: "3m", Kind: "Pod",
			Quando: horaDaPrancha.Add(-time.Duration(i) * time.Second),
		}
	}
	return eventos
}

// modeloGrande é o Viz com as abas lidas, em todos os namespaces, na aba dada.
func modeloGrande(b *testing.B, tab ResourceTab) Model {
	b.Helper()
	m := NewModel(&MockClient{})
	m.agora = func() time.Time { return horaDaPrancha }
	m.tique = nil
	m.filter.Namespace = ""
	for _, msg := range []tea.Msg{
		tea.WindowSizeMsg{Width: 160, Height: 45},
		podsDoClusterGrande(podsGrandes),
		eventosDoClusterGrande(eventosGrandes),
		[]monitor.Deployment{{Name: "api", Namespace: "ns-00", Replicas: 3, Ready: 3}},
		[]monitor.Service{{Name: "api", Namespace: "ns-00", Type: "ClusterIP"}},
		[]monitor.Node{{Name: "ip-10-0-1-5", Status: "Ready"}},
	} {
		nm, _ := m.Update(msg)
		m = nm.(Model)
	}
	m.activeTab = tab
	return m
}

// BenchmarkView mede um quadro da tela na aba de pods e na de eventos.
func BenchmarkView(b *testing.B) {
	for _, c := range []struct {
		nome string
		tab  ResourceTab
	}{{"pods", TabPods}, {"eventos", TabEvents}} {
		b.Run(c.nome, func(b *testing.B) {
			m := modeloGrande(b, c.tab)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = m.View()
			}
		})
	}
}

// BenchmarkTeclaEView mede o que uma tecla custa: o Update do ↓ e o quadro
// que o Bubble Tea desenha em seguida.
func BenchmarkTeclaEView(b *testing.B) {
	m := modeloGrande(b, TabPods)
	baixo := tea.KeyMsg{Type: tea.KeyDown}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		nm, _ := m.Update(baixo)
		_ = nm.(Model).View()
	}
}

// BenchmarkLoteEView mede a chegada de uma lista nova de pods (o lote do
// watch) e o quadro seguinte; a cada volta chega outra lista.
func BenchmarkLoteEView(b *testing.B) {
	m := modeloGrande(b, TabPods)
	listas := [2][]monitor.Pod{podsDoClusterGrande(podsGrandes), podsDoClusterGrande(podsGrandes)}
	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		nm, _ := m.Update(listas[i%2])
		m = nm.(Model)
		_ = m.View()
		i++
	}
}
