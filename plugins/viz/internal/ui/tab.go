package ui

import (
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
)

// ResourceTab representa uma aba de recurso
type ResourceTab int

const (
	// TabPods é a aba de pods
	TabPods ResourceTab = iota
	// TabDeployments é a aba de deployments
	TabDeployments
	// TabServices é a aba de services
	TabServices
	// TabNodes é a aba de nodes
	TabNodes
	// tabCount é o sentinela para total de tabs
	tabCount
)

var tabNames = [tabCount]string{"Pods", "Deployments", "Services", "Nodes"}

// String retorna o nome da aba
func (t ResourceTab) String() string {
	if t >= 0 && t < tabCount {
		return tabNames[t]
	}
	return "?"
}

// renderTabBar renderiza a barra de abas com a aba ativa destacada
func renderTabBar(active ResourceTab) string {
	var tabs []string
	for i := ResourceTab(0); i < tabCount; i++ {
		label := fmt.Sprintf(" %d:%s ", i+1, i.String())
		if i == active {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(label))
		}
	}
	return strings.Join(tabs, " ") + "\n"
}

// renderPodTable renderiza a tabela de pods
func renderPodTable(pods []monitor.Pod) string {
	if len(pods) == 0 {
		return "  Nenhum pod encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-30s %-15s %-15s %s", "NOME", "STATUS", "CPU", "NAMESPACE")) + "\n")
	for _, pod := range pods {
		icon := "●"
		style := runningStyle
		if pod.Status != "Running" && pod.Status != "Executando" {
			icon = "✖"
			style = errorStyle
		}
		sb.WriteString(fmt.Sprintf("  %s %-30s %-15s %-15s %s\n",
			style.Render(icon), pod.Name, style.Render(pod.Status), pod.CPU, pod.Namespace))
	}
	return sb.String()
}

// renderDeploymentTable renderiza a tabela de deployments
func renderDeploymentTable(deps []monitor.Deployment) string {
	if len(deps) == 0 {
		return "  Nenhum deployment encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-30s %-12s %-12s %-12s %s", "NOME", "REPLICAS", "PRONTAS", "DISPONÍVEIS", "NAMESPACE")) + "\n")
	for _, d := range deps {
		icon := "●"
		style := runningStyle
		if d.Ready < d.Replicas {
			icon = "✖"
			style = errorStyle
		}
		sb.WriteString(fmt.Sprintf("  %s %-30s %-12s %-12d %-12d %s\n",
			style.Render(icon), d.Name, fmt.Sprintf("%d/%d", d.Ready, d.Replicas), d.Ready, d.Available, d.Namespace))
	}
	return sb.String()
}

// renderServiceTable renderiza a tabela de services
func renderServiceTable(svcs []monitor.Service) string {
	if len(svcs) == 0 {
		return "  Nenhum service encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-30s %-12s %-18s %-20s %s", "NOME", "TIPO", "CLUSTER-IP", "PORTAS", "NAMESPACE")) + "\n")
	for _, s := range svcs {
		sb.WriteString(fmt.Sprintf("  %s %-30s %-12s %-18s %-20s %s\n",
			runningStyle.Render("●"), s.Name, s.Type, s.ClusterIP, s.Ports, s.Namespace))
	}
	return sb.String()
}

// renderNodeTable renderiza a tabela de nodes
func renderNodeTable(nodes []monitor.Node) string {
	if len(nodes) == 0 {
		return "  Nenhum node encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-25s %-10s %-12s %-15s %s", "NOME", "STATUS", "CPU", "MEMÓRIA", "VERSÃO")) + "\n")
	for _, n := range nodes {
		icon := "●"
		style := runningStyle
		if n.Status != "Ready" {
			icon = "✖"
			style = errorStyle
		}
		sb.WriteString(fmt.Sprintf("  %s %-25s %-10s %-12s %-15s %s\n",
			style.Render(icon), n.Name, style.Render(n.Status), n.CPUCapacity, n.MemoryCapacity, n.Version))
	}
	return sb.String()
}
