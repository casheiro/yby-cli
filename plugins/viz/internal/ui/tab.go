package ui

import (
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
)

// ResourceTab representa uma aba de recurso
type ResourceTab int

const (
	TabPods ResourceTab = iota
	TabDeployments
	TabServices
	TabNodes
	TabStatefulSets
	TabJobs
	TabIngresses
	TabConfigMaps
	TabEvents
	tabCount
)

type tabInfo struct {
	Name  string
	Emoji string
}

var tabData = [tabCount]tabInfo{
	{"Pods", "🟢"},
	{"Deploys", "🚀"},
	{"Services", "🔗"},
	{"Nodes", "🖥"},
	{"StatefulSets", "💾"},
	{"Jobs", "⚡"},
	{"Ingresses", "🌐"},
	{"ConfigMaps", "📋"},
	{"Events", "📡"},
}

// String retorna o nome da aba
func (t ResourceTab) String() string {
	if t >= 0 && t < tabCount {
		return tabData[t].Name
	}
	return "?"
}

// renderTabBar renderiza a barra de abas com emojis e aba ativa destacada
func renderTabBar(active ResourceTab) string {
	var tabs []string
	for i := ResourceTab(0); i < tabCount; i++ {
		info := tabData[i]
		label := fmt.Sprintf(" %s %d:%s ", info.Emoji, i+1, info.Name)
		if i == active {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(label))
		}
	}
	return " " + strings.Join(tabs, "") + "\n"
}

// statusIcon retorna o ícone e estilo baseado no status do recurso
func statusIcon(healthy bool, completed bool) (string, func(...string) string) {
	if completed {
		return "✓", completedStyle.Render
	}
	if healthy {
		return "●", runningStyle.Render
	}
	return "✖", errorStyle.Render
}

// renderPodTable renderiza a tabela de pods
func renderPodTable(pods []monitor.Pod) string {
	if len(pods) == 0 {
		return "  Nenhum pod encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-2s %-35s %-12s %-10s %-10s %s", "", "NOME", "STATUS", "CPU", "MEM", "NAMESPACE")) + "\n")
	for _, pod := range pods {
		isCompleted := pod.Status == "Succeeded" || pod.Status == "Completed"
		isHealthy := pod.Status == "Running" || pod.Status == "Executando"
		icon, render := statusIcon(isHealthy, isCompleted)

		ns := namespaceStyle.Render(pod.Namespace)
		sb.WriteString(fmt.Sprintf("  %s  %-35s %-12s %-10s %-10s %s\n",
			render(icon), pod.Name, render(pod.Status), pod.CPU, pod.Memory, ns))
	}
	return sb.String()
}

// renderDeploymentTable renderiza a tabela de deployments
func renderDeploymentTable(deps []monitor.Deployment) string {
	if len(deps) == 0 {
		return "  Nenhum deployment encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-2s %-35s %-14s %-10s %-10s %s", "", "NOME", "REPLICAS", "PRONTAS", "DISP.", "NAMESPACE")) + "\n")
	for _, d := range deps {
		healthy := d.Ready >= d.Replicas
		icon, render := statusIcon(healthy, false)

		replicas := fmt.Sprintf("%d/%d", d.Ready, d.Replicas)
		ns := namespaceStyle.Render(d.Namespace)
		sb.WriteString(fmt.Sprintf("  %s  %-35s %-14s %-10d %-10d %s\n",
			render(icon), d.Name, replicas, d.Ready, d.Available, ns))
	}
	return sb.String()
}

// renderServiceTable renderiza a tabela de services
func renderServiceTable(svcs []monitor.Service) string {
	if len(svcs) == 0 {
		return "  Nenhum service encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-2s %-30s %-12s %-16s %-20s %s", "", "NOME", "TIPO", "CLUSTER-IP", "PORTAS", "NAMESPACE")) + "\n")
	for _, s := range svcs {
		ns := namespaceStyle.Render(s.Namespace)
		sb.WriteString(fmt.Sprintf("  %s  %-30s %-12s %-16s %-20s %s\n",
			runningStyle.Render("●"), s.Name, s.Type, s.ClusterIP, s.Ports, ns))
	}
	return sb.String()
}

// renderNodeTable renderiza a tabela de nodes
func renderNodeTable(nodes []monitor.Node) string {
	if len(nodes) == 0 {
		return "  Nenhum node encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-2s %-25s %-10s %-8s %-12s %s", "", "NOME", "STATUS", "CPU", "MEMORIA", "VERSAO")) + "\n")
	for _, n := range nodes {
		healthy := n.Status == "Ready"
		icon, render := statusIcon(healthy, false)
		ns := n.Version
		sb.WriteString(fmt.Sprintf("  %s  %-25s %-10s %-8s %-12s %s\n",
			render(icon), n.Name, render(n.Status), n.CPUCapacity, n.MemoryCapacity, ns))
	}
	return sb.String()
}

// renderStatefulSetTable renderiza a tabela de statefulsets
func renderStatefulSetTable(sets []monitor.StatefulSet) string {
	if len(sets) == 0 {
		return "  Nenhum statefulset encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-2s %-35s %-14s %-10s %s", "", "NOME", "REPLICAS", "PRONTAS", "NAMESPACE")) + "\n")
	for _, s := range sets {
		healthy := s.Ready >= s.Replicas
		icon, render := statusIcon(healthy, false)
		replicas := fmt.Sprintf("%d/%d", s.Ready, s.Replicas)
		ns := namespaceStyle.Render(s.Namespace)
		sb.WriteString(fmt.Sprintf("  %s  %-35s %-14s %-10d %s\n",
			render(icon), s.Name, replicas, s.Ready, ns))
	}
	return sb.String()
}

// renderJobTable renderiza a tabela de jobs
func renderJobTable(jobs []monitor.Job) string {
	if len(jobs) == 0 {
		return "  Nenhum job encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-2s %-30s %-12s %-8s %-8s %-8s %s", "", "NOME", "COMPLETIONS", "ATIVO", "OK", "FALHA", "NAMESPACE")) + "\n")
	for _, j := range jobs {
		isCompleted := j.Succeeded >= j.Completions && j.Completions > 0
		hasFailed := j.Failed > 0
		icon, render := statusIcon(!hasFailed, isCompleted)
		completions := fmt.Sprintf("%d/%d", j.Succeeded, j.Completions)
		ns := namespaceStyle.Render(j.Namespace)
		sb.WriteString(fmt.Sprintf("  %s  %-30s %-12s %-8d %-8d %-8d %s\n",
			render(icon), j.Name, completions, j.Active, j.Succeeded, j.Failed, ns))
	}
	return sb.String()
}

// renderIngressTable renderiza a tabela de ingresses
func renderIngressTable(ingresses []monitor.Ingress) string {
	if len(ingresses) == 0 {
		return "  Nenhum ingress encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-2s %-28s %-12s %-25s %-18s %s", "", "NOME", "CLASSE", "HOSTS", "PATHS", "NAMESPACE")) + "\n")
	for _, ing := range ingresses {
		ns := namespaceStyle.Render(ing.Namespace)
		sb.WriteString(fmt.Sprintf("  %s  %-28s %-12s %-25s %-18s %s\n",
			runningStyle.Render("●"), ing.Name, ing.Class, ing.Hosts, ing.Paths, ns))
	}
	return sb.String()
}

// renderConfigMapTable renderiza a tabela de configmaps
func renderConfigMapTable(cms []monitor.ConfigMap) string {
	if len(cms) == 0 {
		return "  Nenhum configmap encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-2s %-35s %-8s %-10s %s", "", "NOME", "CHAVES", "TAMANHO", "NAMESPACE")) + "\n")
	for _, cm := range cms {
		ns := namespaceStyle.Render(cm.Namespace)
		sb.WriteString(fmt.Sprintf("  %s  %-35s %-8d %-10s %s\n",
			runningStyle.Render("●"), cm.Name, cm.Keys, cm.DataSize, ns))
	}
	return sb.String()
}

// renderEventTable renderiza a tabela de eventos
func renderEventTable(events []monitor.Event) string {
	if len(events) == 0 {
		return "  Nenhum evento encontrado.\n"
	}
	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("  %-2s %-25s %-9s %-15s %-6s %-35s %s", "", "NOME", "TIPO", "RAZAO", "IDADE", "MENSAGEM", "NAMESPACE")) + "\n")
	for _, e := range events {
		isWarning := e.Type == "Warning"
		icon := "●"
		render := runningStyle.Render
		if isWarning {
			icon = "⚠"
			render = warningStyle.Render
		}
		msg := e.Message
		if len(msg) > 35 {
			msg = msg[:32] + "..."
		}
		ns := namespaceStyle.Render(e.Namespace)
		sb.WriteString(fmt.Sprintf("  %s  %-25s %-9s %-15s %-6s %-35s %s\n",
			render(icon), e.Name, render(e.Type), e.Reason, e.Age, msg, ns))
	}
	return sb.String()
}
