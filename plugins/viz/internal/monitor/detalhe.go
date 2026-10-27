package monitor

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// LeitorDeDetalhe lê o que o detalhe do Viz mostra além das listas: os
// eventos de um objeto, o log de um pod e o YAML de um recurso. O K8sClient
// lê do cluster; o RetryClient repassa ao client de dentro.
type LeitorDeDetalhe interface {
	// EventosDoObjeto lista os eventos do objeto, dos mais recentes para os
	// mais antigos; kind é o da API ("Pod"; vazio aceita qualquer um),
	// namespace vazio procura em todos e limite 0 traz todos.
	EventosDoObjeto(kind, nome, namespace string, limite int) ([]Event, error)
	// LogsDoPod traz as últimas linhas do log do contêiner do pod (vazio
	// deixa a API escolher, o que só vale para pod de um contêiner); vazio sem log.
	// anterior avisa que o texto é da execução anterior do contêiner: o log
	// atual estava vazio ou falhou (CrashLoopBackOff).
	LogsDoPod(nome, namespace, conteiner string, linhas int64) (logs string, anterior bool, err error)
	// YAMLDoRecurso traz o manifesto do recurso como está no cluster.
	YAMLDoRecurso(kind, nome, namespace string) (string, error)
}

var (
	_ LeitorDeDetalhe = (*K8sClient)(nil)
	_ LeitorDeDetalhe = (*RetryClient)(nil)
)

// prazoDeLeitura limita cada leitura (listas, detalhe e YAML): sem ele, um
// servidor ou um kubelet que não responde deixaria a aba ou o painel lendo
// para sempre (o rest.Config não tem timeout, para não cortar o watch).
const prazoDeLeitura = 10 * time.Second

// limiteDeBytesDoLog corta a leitura do log: 100 linhas de JSON enorme ou de
// stack trace não podem encher a memória nem a tela.
const limiteDeBytesDoLog int64 = 1 << 20

// lerLog lê as últimas linhas do log de um contêiner (da execução anterior,
// com anterior); o K8sClient usa lerLogs, e os testes trocam.
type lerLog func(ctx context.Context, namespace, pod, conteiner string, linhas int64, anterior bool) (string, error)

// EventosDoObjeto lista os eventos de um objeto pelo field selector do
// involvedObject.
func (c *K8sClient) EventosDoObjeto(kind, nome, namespace string, limite int) ([]Event, error) {
	seletor := fields.Set{"involvedObject.name": nome}
	if kind != "" {
		seletor["involvedObject.kind"] = kind
	}
	opts := metav1.ListOptions{FieldSelector: seletor.String()}
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	list, err := c.clientset.CoreV1().Events(namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar os eventos de %s: %w", nome, err)
	}
	// O filtro vale de novo aqui: o servidor pode ignorar o field selector (o
	// clientset falso dos testes ignora).
	eventos := eventosDeK8s(list.Items, func(e *corev1.Event) bool {
		return e.InvolvedObject.Name == nome && (kind == "" || e.InvolvedObject.Kind == kind)
	})
	if limite > 0 && len(eventos) > limite {
		eventos = eventos[:limite]
	}
	return eventos, nil
}

// LogsDoPod traz as últimas linhas do log do contêiner. Quando ele espera
// para reiniciar (CrashLoopBackOff), o log atual vem vazio ou com erro, e o
// que explica a falha é o da execução anterior.
func (c *K8sClient) LogsDoPod(nome, namespace, conteiner string, linhas int64) (string, bool, error) {
	ler := c.lerLog
	if ler == nil {
		ler = func(ctx context.Context, namespace, pod, conteiner string, linhas int64, anterior bool) (string, error) {
			return lerLogs(ctx, c.clientset, namespace, pod, conteiner, linhas, anterior)
		}
	}
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	logs, err := ler(ctx, namespace, nome, conteiner, linhas, false)
	if err == nil && strings.TrimSpace(logs) != "" {
		return logs, false, nil
	}
	if anterior, errAnterior := ler(ctx, namespace, nome, conteiner, linhas, true); errAnterior == nil && strings.TrimSpace(anterior) != "" {
		return anterior, true, nil
	}
	return logs, false, err
}

// YAMLDoRecurso traz o manifesto do recurso pelo client dinâmico.
func (c *K8sClient) YAMLDoRecurso(kind, nome, namespace string) (string, error) {
	if c.restConfig == nil {
		return "", fmt.Errorf("sem configuração do cluster para ler o YAML")
	}
	return GetResourceYAML(c.restConfig, kind, nome, namespace)
}

// lerLogs lê as últimas linhas do log do contêiner (da execução anterior, com anterior).
func lerLogs(ctx context.Context, clientset kubernetes.Interface, namespace, pod, conteiner string, linhas int64, anterior bool) (string, error) {
	limite := limiteDeBytesDoLog
	opts := &corev1.PodLogOptions{Container: conteiner, TailLines: &linhas, LimitBytes: &limite, Previous: anterior}
	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("falha ao obter logs do pod '%s': %w", pod, err)
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("falha ao ler logs do pod '%s': %w", pod, err)
	}
	return string(data), nil
}

// leitor devolve o leitor de detalhe do client de dentro.
func (r *RetryClient) leitor() (LeitorDeDetalhe, error) {
	if l, ok := r.inner.(LeitorDeDetalhe); ok {
		return l, nil
	}
	return nil, fmt.Errorf("o client não lê eventos, logs nem YAML")
}

// EventosDoObjeto repassa ao client de dentro, sem novas tentativas: o
// detalhe mostra o erro e lê de novo quando a seleção para outra vez.
func (r *RetryClient) EventosDoObjeto(kind, nome, namespace string, limite int) ([]Event, error) {
	l, err := r.leitor()
	if err != nil {
		return nil, err
	}
	return l.EventosDoObjeto(kind, nome, namespace, limite)
}

// LogsDoPod repassa ao client de dentro.
func (r *RetryClient) LogsDoPod(nome, namespace, conteiner string, linhas int64) (string, bool, error) {
	l, err := r.leitor()
	if err != nil {
		return "", false, err
	}
	return l.LogsDoPod(nome, namespace, conteiner, linhas)
}

// YAMLDoRecurso repassa ao client de dentro.
func (r *RetryClient) YAMLDoRecurso(kind, nome, namespace string) (string, error) {
	l, err := r.leitor()
	if err != nil {
		return "", err
	}
	return l.YAMLDoRecurso(kind, nome, namespace)
}

// motivoDoPod explica o estado do pod: o contêiner que terminou com erro (ou,
// esperando para reiniciar, por que a última execução terminou), a mensagem
// de quem espera, por que o pod não foi agendado ou a mensagem do pod.
func motivoDoPod(p *corev1.Pod) string {
	estados := append(append([]corev1.ContainerStatus(nil), p.Status.InitContainerStatuses...), p.Status.ContainerStatuses...)
	for _, c := range estados {
		if t := c.State.Terminated; t != nil && t.ExitCode != 0 {
			return terminou(t)
		}
	}
	for _, c := range estados {
		if t := c.LastTerminationState.Terminated; t != nil && !c.Ready && t.ExitCode != 0 {
			return terminou(t)
		}
	}
	for _, c := range estados {
		if w := c.State.Waiting; w != nil && w.Message != "" {
			return w.Message
		}
	}
	for _, cond := range p.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
			if cond.Message != "" {
				return cond.Message
			}
			if cond.Reason != "" {
				return cond.Reason
			}
		}
	}
	return p.Status.Message
}

// terminou escreve "Error · código de saída 1".
func terminou(t *corev1.ContainerStateTerminated) string {
	motivo := t.Reason
	if motivo == "" {
		motivo = "terminou"
	}
	return fmt.Sprintf("%s · código de saída %d", motivo, t.ExitCode)
}

// sidecars são os contêineres que a malha de serviço põe no pod: o log
// deles não explica o pod.
var sidecars = map[string]bool{"istio-proxy": true, "istio-init": true, "linkerd-proxy": true, "linkerd-init": true}

// conteinerDoLog é o contêiner cujo log o detalhe lê: o que está com problema
// (terminou com erro, espera para reiniciar ou, entre os de init, ainda roda),
// senão o padrão do pod (a anotação que o kubectl usa), senão o primeiro que
// não é sidecar. Sem contêiner, a API recusa o log de pod com mais de um.
func conteinerDoLog(p *corev1.Pod) string {
	for _, c := range p.Status.InitContainerStatuses {
		t := c.State.Terminated
		if c.State.Running != nil || (t != nil && t.ExitCode != 0) || (c.State.Waiting != nil && c.LastTerminationState.Terminated != nil) {
			return c.Name
		}
	}
	for _, c := range p.Status.ContainerStatuses {
		if t := c.State.Terminated; t != nil && t.ExitCode != 0 {
			return c.Name
		}
	}
	for _, c := range p.Status.ContainerStatuses {
		if c.State.Waiting != nil && c.LastTerminationState.Terminated != nil {
			return c.Name
		}
	}
	if nome := p.Annotations["kubectl.kubernetes.io/default-container"]; nome != "" {
		return nome
	}
	for _, c := range p.Spec.Containers {
		if !sidecars[c.Name] {
			return c.Name
		}
	}
	if len(p.Spec.Containers) > 0 {
		return p.Spec.Containers[0].Name
	}
	return ""
}

// donoDoPod é quem controla o pod; o ReplicaSet de um Deployment vira o
// Deployment (o nome do ReplicaSet é o do Deployment mais o pod-template-hash).
func donoDoPod(p *corev1.Pod) string {
	var dono *metav1.OwnerReference
	for i, o := range p.OwnerReferences {
		if dono == nil || (o.Controller != nil && *o.Controller) {
			dono = &p.OwnerReferences[i]
		}
	}
	if dono == nil {
		return ""
	}
	if hash := p.Labels["pod-template-hash"]; dono.Kind == "ReplicaSet" && hash != "" && strings.HasSuffix(dono.Name, "-"+hash) {
		return "Deployment " + strings.TrimSuffix(dono.Name, "-"+hash)
	}
	return dono.Kind + " " + dono.Name
}

// imagens é a imagem de cada contêiner.
func imagens(cs []corev1.Container) []string {
	var r []string
	for _, c := range cs {
		r = append(r, c.Image)
	}
	return r
}

// recursosDe soma os pedidos e limites de CPU e memória dos contêineres.
func recursosDe(cs []corev1.Container) Recursos {
	soma := func(nome corev1.ResourceName, limite bool) string {
		var total resource.Quantity
		achou := false
		for _, c := range cs {
			lista := c.Resources.Requests
			if limite {
				lista = c.Resources.Limits
			}
			if q, ok := lista[nome]; ok {
				total.Add(q)
				achou = true
			}
		}
		if !achou {
			return ""
		}
		return total.String()
	}
	return Recursos{
		CPUPedido: soma(corev1.ResourceCPU, false), CPULimite: soma(corev1.ResourceCPU, true),
		MemPedido: soma(corev1.ResourceMemory, false), MemLimite: soma(corev1.ResourceMemory, true),
	}
}

// estrategiaDe escreve a estratégia do Deployment, com surge e indisponível
// no RollingUpdate.
func estrategiaDe(e appsv1.DeploymentStrategy) string {
	txt := string(e.Type)
	if r := e.RollingUpdate; r != nil {
		if r.MaxSurge != nil {
			txt += " · surge " + r.MaxSurge.String()
		}
		if r.MaxUnavailable != nil {
			txt += " · indisponível " + r.MaxUnavailable.String()
		}
	}
	return txt
}

// seletorDeMapa escreve o seletor do Service como label selector ("app=api").
func seletorDeMapa(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	return labels.Set(m).String()
}

// externosDe junta os IPs externos do Service e os do balanceador.
func externosDe(s *corev1.Service) string {
	ips := append([]string(nil), s.Spec.ExternalIPs...)
	for _, i := range s.Status.LoadBalancer.Ingress {
		if i.IP != "" {
			ips = append(ips, i.IP)
		} else if i.Hostname != "" {
			ips = append(ips, i.Hostname)
		}
	}
	return strings.Join(ips, ", ")
}

// enderecoDe junta os IPs (ou nomes) do balanceador do Ingress.
func enderecoDe(lb []networkingv1.IngressLoadBalancerIngress) string {
	var r []string
	for _, i := range lb {
		if i.IP != "" {
			r = append(r, i.IP)
		} else if i.Hostname != "" {
			r = append(r, i.Hostname)
		}
	}
	return strings.Join(r, ", ")
}

// papeisDe lê os papéis do nó nas labels node-role.kubernetes.io/<papel>.
func papeisDe(ls map[string]string) string {
	const prefixo = "node-role.kubernetes.io/"
	var r []string
	for k := range ls {
		if strings.HasPrefix(k, prefixo) && len(k) > len(prefixo) {
			r = append(r, strings.TrimPrefix(k, prefixo))
		}
	}
	sort.Strings(r)
	return strings.Join(r, ", ")
}

// ipInternoDe é o InternalIP do nó.
func ipInternoDe(enderecos []corev1.NodeAddress) string {
	for _, a := range enderecos {
		if a.Type == corev1.NodeInternalIP {
			return a.Address
		}
	}
	return ""
}

// chavesDe são os nomes das chaves do ConfigMap (texto e binárias), em ordem.
func chavesDe(cm *corev1.ConfigMap) []string {
	var r []string
	for k := range cm.Data {
		r = append(r, k)
	}
	for k := range cm.BinaryData {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
