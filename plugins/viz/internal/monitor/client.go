package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ListFilter contém filtros para listagem de recursos
type ListFilter struct {
	Namespace     string
	LabelSelector string
}

// Client define a interface para acessar recursos Kubernetes
type Client interface {
	GetPods(filter ListFilter) ([]Pod, error)
	GetDeployments(filter ListFilter) ([]Deployment, error)
	GetServices(filter ListFilter) ([]Service, error)
	GetNodes(filter ListFilter) ([]Node, error)
	GetStatefulSets(filter ListFilter) ([]StatefulSet, error)
	GetJobs(filter ListFilter) ([]Job, error)
	GetIngresses(filter ListFilter) ([]Ingress, error)
	GetConfigMaps(filter ListFilter) ([]ConfigMap, error)
	GetEvents(filter ListFilter) ([]Event, error)
}

// K8sClient conecta ao cluster real
type K8sClient struct {
	clientset  kubernetes.Interface
	restConfig *rest.Config
	kubeconfig string // o arquivo pedido no --kubeconfig (vazio: KUBECONFIG ou ~/.kube/config)
	contexto   string // contexto atual do kubeconfig
	namespace  string // namespace do contexto atual (vazio: todos)
	lerLog     lerLog // lê o log de um contêiner; nil usa lerLogs (os testes trocam)
	// intervaloLote é o tempo entre lotes do watch; zero usa intervaloDoLote.
	intervaloLote time.Duration
	// prazo limita cada leitura (listas, eventos e logs do detalhe); zero usa prazoDeLeitura.
	prazo time.Duration
}

// Kubeconfig devolve o arquivo pedido no --kubeconfig (vazio: o padrão do kubectl).
func (c *K8sClient) Kubeconfig() string {
	return c.kubeconfig
}

// Contexto devolve o nome do contexto do kubeconfig em uso (vazio se não há).
func (c *K8sClient) Contexto() string {
	return c.contexto
}

// NamespaceInicial devolve o namespace por onde a TUI começa: o pedido no
// --namespace ou, sem ele, o do contexto (vazio se o contexto não fixa um), como
// o kubectl.
func (c *K8sClient) NamespaceInicial() string {
	return c.namespace
}

// Clientset retorna o clientset Kubernetes para uso em operações avançadas
func (c *K8sClient) Clientset() kubernetes.Interface {
	return c.clientset
}

// RESTConfig retorna a configuração REST para uso com dynamic client
func (c *K8sClient) RESTConfig() *rest.Config {
	return c.restConfig
}

// Conexao escolhe o kubeconfig, o contexto e o namespace inicial do client; vazios,
// valem o kubeconfig padrão, o contexto atual e o namespace dele.
type Conexao struct {
	Kubeconfig string // um arquivo só, no lugar do KUBECONFIG (como o --kubeconfig do kubectl)
	Contexto   string
	Namespace  string
}

// ContextoInexistenteError é o contexto pedido que o kubeconfig não tem.
type ContextoInexistenteError struct {
	Nome        string
	Disponiveis []string // os contextos do kubeconfig, em ordem
}

func (e *ContextoInexistenteError) Error() string {
	return fmt.Sprintf("o contexto %q não existe no kubeconfig", e.Nome)
}

// NewK8sClient cria um client do contexto atual do kubeconfig.
func NewK8sClient() (*K8sClient, error) {
	return NovoK8sClient(Conexao{})
}

// NovoK8sClient cria um client a partir do kubeconfig (KUBECONFIG, que pode ter
// vários arquivos, ou ~/.kube/config), no contexto e no namespace pedidos. Um
// contexto pedido que não existe devolve *ContextoInexistenteError.
func NovoK8sClient(c Conexao) (*K8sClient, error) {
	regras := clientcmd.NewDefaultClientConfigLoadingRules()
	regras.ExplicitPath = c.Kubeconfig
	raw, err := regras.Load()
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar kubeconfig: %w", err)
	}
	contexto := c.Contexto
	if contexto == "" {
		contexto = raw.CurrentContext
	} else if _, ok := raw.Contexts[contexto]; !ok {
		nomes := make([]string, 0, len(raw.Contexts))
		for nome := range raw.Contexts {
			nomes = append(nomes, nome)
		}
		sort.Strings(nomes)
		return nil, &ContextoInexistenteError{Nome: contexto, Disponiveis: nomes}
	}

	config, err := clientcmd.NewNonInteractiveClientConfig(*raw, contexto, &clientcmd.ConfigOverrides{}, regras).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar clientset: %w", err)
	}

	namespace := c.Namespace
	if ctx, ok := raw.Contexts[contexto]; ok && ctx != nil && namespace == "" {
		namespace = ctx.Namespace
	}
	slog.Debug("viz: conectando ao cluster", "kubeconfig", regras.GetLoadingPrecedence(), "contexto", contexto, "namespace", namespace, "servidor", config.Host)

	return &K8sClient{clientset: clientset, restConfig: config, kubeconfig: c.Kubeconfig, contexto: contexto, namespace: namespace}, nil
}

// Sondador confirma que o cluster responde sem listar nada: é o sinal de
// conexão do relógio nas abas cuja lista vem do watch.
type Sondador interface {
	// Sondar pede a versão do servidor; o erro é o da conexão.
	Sondar() error
}

var (
	_ Sondador = (*K8sClient)(nil)
	_ Sondador = (*RetryClient)(nil)
)

// prazoDaSonda limita a espera da sonda: cluster que não responde vira erro.
const prazoDaSonda = 10 * time.Second

// Sondar faz GET /version, a leitura mais barata do servidor. Sem REST (o
// clientset falso dos testes), não há o que sondar.
func (c *K8sClient) Sondar() error {
	rc := c.clientset.Discovery().RESTClient()
	if rc == nil {
		return nil
	}
	ctx, cancelar := context.WithTimeout(context.Background(), prazoDaSonda)
	defer cancelar()
	if _, err := rc.Get().AbsPath("/version").DoRaw(ctx); err != nil {
		return fmt.Errorf("falha ao sondar o cluster: %w", err)
	}
	return nil
}

// contextoDeLeitura é o contexto de uma leitura, com o prazo do client. O
// prazo vai em cada leitura, e não no Timeout do rest.Config, que cortaria
// também o watch dos informers.
func (c *K8sClient) contextoDeLeitura() (context.Context, context.CancelFunc) {
	prazo := c.prazo
	if prazo <= 0 {
		prazo = prazoDeLeitura
	}
	return context.WithTimeout(context.Background(), prazo)
}

// ErroDeCredencial diz se a leitura falhou ao obter a credencial pelo plugin
// exec do kubeconfig (aws, kubelogin, gke-gcloud-auth-plugin): o client-go não
// tipa esse erro, só escreve "getting credentials: …" na mensagem.
func ErroDeCredencial(err error) bool {
	return err != nil && strings.Contains(err.Error(), "getting credentials: ")
}

// GetPods lista pods com filtros opcionais de namespace e label.
func (c *K8sClient) GetPods(filter ListFilter) ([]Pod, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	list, err := c.clientset.CoreV1().Pods(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var pods []Pod
	for i := range list.Items {
		pods = append(pods, podDeK8s(&list.Items[i]))
	}
	return pods, nil
}

// podDeK8s converte o pod da API: estado como o kubectl mostra, contêineres
// prontos, reinícios, nó e IP.
func podDeK8s(p *corev1.Pod) Pod {
	pod := Pod{
		Name:       p.Name,
		Namespace:  p.Namespace,
		UID:        string(p.UID),
		Status:     statusDoPod(p),
		Containers: int32(len(p.Spec.Containers)),
		No:         p.Spec.NodeName,
		IP:         p.Status.PodIP,
		CriadoEm:   p.CreationTimestamp.Time,
		Labels:     p.Labels,
		Motivo:     motivoDoPod(p),
		Imagens:    imagens(p.Spec.Containers),
		Dono:       donoDoPod(p),
		Recursos:   recursosDe(p.Spec.Containers),
		Conteiner:  conteinerDoLog(p),
	}
	for _, c := range p.Status.ContainerStatuses {
		if c.Ready {
			pod.Prontos++
		}
		pod.Reinicios += c.RestartCount
		if t := c.LastTerminationState.Terminated; c.RestartCount > 0 && t != nil && t.FinishedAt.After(pod.UltimoReinicio) {
			pod.UltimoReinicio = t.FinishedAt.Time
		}
	}
	return pod
}

// statusDoPod resume o estado do pod como a coluna STATUS do kubectl: o pod
// saindo é Terminating; um init container pendente vira Init:…; senão vale o
// motivo do contêiner (o primeiro da lista com motivo), e por fim a fase.
func statusDoPod(p *corev1.Pod) string {
	if p.DeletionTimestamp != nil {
		return "Terminating"
	}
	motivo := string(p.Status.Phase)
	if p.Status.Reason != "" {
		motivo = p.Status.Reason
	}
	for i, c := range p.Status.InitContainerStatuses {
		switch {
		case c.State.Terminated != nil && c.State.Terminated.ExitCode == 0:
			continue
		case c.State.Terminated != nil:
			if c.State.Terminated.Reason != "" {
				return "Init:" + c.State.Terminated.Reason
			}
			return fmt.Sprintf("Init:ExitCode:%d", c.State.Terminated.ExitCode)
		case c.State.Waiting != nil && c.State.Waiting.Reason != "" && c.State.Waiting.Reason != "PodInitializing":
			return "Init:" + c.State.Waiting.Reason
		default:
			return fmt.Sprintf("Init:%d/%d", i, len(p.Spec.InitContainers))
		}
	}
	rodando := false
	for i := len(p.Status.ContainerStatuses) - 1; i >= 0; i-- {
		c := p.Status.ContainerStatuses[i]
		switch {
		case c.State.Waiting != nil && c.State.Waiting.Reason != "":
			motivo = c.State.Waiting.Reason
		case c.State.Terminated != nil && c.State.Terminated.Reason != "":
			motivo = c.State.Terminated.Reason
		case c.State.Terminated != nil && c.State.Terminated.ExitCode != 0:
			motivo = fmt.Sprintf("ExitCode:%d", c.State.Terminated.ExitCode)
		case c.State.Running != nil && c.Ready:
			rodando = true
		}
	}
	// Um contêiner que terminou bem ao lado de outro rodando não encerra o pod.
	if motivo == "Completed" && rodando {
		motivo = "Running"
	}
	return motivo
}

// GetDeployments lista deployments com filtros opcionais de namespace e label.
func (c *K8sClient) GetDeployments(filter ListFilter) ([]Deployment, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	list, err := c.clientset.AppsV1().Deployments(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar deployments: %w", err)
	}
	var deps []Deployment
	for i := range list.Items {
		deps = append(deps, deploymentDeK8s(&list.Items[i]))
	}
	return deps, nil
}

// deploymentDeK8s converte o deployment da API.
func deploymentDeK8s(d *appsv1.Deployment) Deployment {
	replicas := int32(0)
	if d.Spec.Replicas != nil {
		replicas = *d.Spec.Replicas
	}
	return Deployment{
		Name:        d.Name,
		Namespace:   d.Namespace,
		UID:         string(d.UID),
		Replicas:    replicas,
		Ready:       d.Status.ReadyReplicas,
		Available:   d.Status.AvailableReplicas,
		Atualizados: d.Status.UpdatedReplicas,
		CriadoEm:    d.CreationTimestamp.Time,
		Labels:      d.Labels,
		Estrategia:  estrategiaDe(d.Spec.Strategy),
		Imagens:     imagens(d.Spec.Template.Spec.Containers),
		Seletor:     metav1.FormatLabelSelector(d.Spec.Selector),
	}
}

// GetServices lista services com filtros opcionais de namespace e label.
func (c *K8sClient) GetServices(filter ListFilter) ([]Service, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	list, err := c.clientset.CoreV1().Services(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar services: %w", err)
	}
	var svcs []Service
	for i := range list.Items {
		svcs = append(svcs, serviceDeK8s(&list.Items[i]))
	}
	return svcs, nil
}

// serviceDeK8s converte o service da API.
func serviceDeK8s(s *corev1.Service) Service {
	ports := ""
	for i, p := range s.Spec.Ports {
		if i > 0 {
			ports += ", "
		}
		ports += fmt.Sprintf("%d/%s", p.Port, p.Protocol)
	}
	return Service{
		Name:      s.Name,
		Namespace: s.Namespace,
		UID:       string(s.UID),
		Type:      string(s.Spec.Type),
		ClusterIP: s.Spec.ClusterIP,
		Ports:     ports,
		CriadoEm:  s.CreationTimestamp.Time,
		Labels:    s.Labels,
		Externos:  externosDe(s),
		Seletor:   seletorDeMapa(s.Spec.Selector),
	}
}

// GetNodes lista nodes com filtros opcionais de label.
func (c *K8sClient) GetNodes(filter ListFilter) ([]Node, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	list, err := c.clientset.CoreV1().Nodes().List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar nodes: %w", err)
	}
	var nodes []Node
	for i := range list.Items {
		nodes = append(nodes, nodeDeK8s(&list.Items[i]))
	}
	return nodes, nil
}

// nodeDeK8s converte o nó da API: Ready pela condição, senão NotReady.
func nodeDeK8s(n *corev1.Node) Node {
	status := "NotReady"
	for _, cond := range n.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			status = "Ready"
			break
		}
	}
	return Node{
		Name:           n.Name,
		Status:         status,
		CPUCapacity:    n.Status.Allocatable.Cpu().String(),
		MemoryCapacity: n.Status.Allocatable.Memory().String(),
		Version:        n.Status.NodeInfo.KubeletVersion,
		CriadoEm:       n.CreationTimestamp.Time,
		Labels:         n.Labels,
		Papeis:         papeisDe(n.Labels),
		IPInterno:      ipInternoDe(n.Status.Addresses),
	}
}

// GetStatefulSets lista statefulsets com filtros opcionais de namespace e label.
func (c *K8sClient) GetStatefulSets(filter ListFilter) ([]StatefulSet, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	list, err := c.clientset.AppsV1().StatefulSets(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar statefulsets: %w", err)
	}
	var result []StatefulSet
	for i := range list.Items {
		result = append(result, statefulSetDeK8s(&list.Items[i]))
	}
	return result, nil
}

// statefulSetDeK8s converte o statefulset da API.
func statefulSetDeK8s(s *appsv1.StatefulSet) StatefulSet {
	replicas := int32(0)
	if s.Spec.Replicas != nil {
		replicas = *s.Spec.Replicas
	}
	return StatefulSet{
		Name:        s.Name,
		Namespace:   s.Namespace,
		UID:         string(s.UID),
		Replicas:    replicas,
		Ready:       s.Status.ReadyReplicas,
		Atualizados: s.Status.UpdatedReplicas,
		CriadoEm:    s.CreationTimestamp.Time,
		Labels:      s.Labels,
		Servico:     s.Spec.ServiceName,
		Imagens:     imagens(s.Spec.Template.Spec.Containers),
		Seletor:     metav1.FormatLabelSelector(s.Spec.Selector),
	}
}

// GetJobs lista jobs com filtros opcionais de namespace e label.
func (c *K8sClient) GetJobs(filter ListFilter) ([]Job, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	list, err := c.clientset.BatchV1().Jobs(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar jobs: %w", err)
	}
	var result []Job
	for i := range list.Items {
		result = append(result, jobDeK8s(&list.Items[i]))
	}
	return result, nil
}

// jobDeK8s converte o job da API, com as condições Complete e Failed.
func jobDeK8s(j *batchv1.Job) Job {
	completions := int32(0)
	if j.Spec.Completions != nil {
		completions = *j.Spec.Completions
	}
	concluido, falhou := false, false
	for _, c := range j.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		switch c.Type {
		case batchv1.JobComplete:
			concluido = true
		case batchv1.JobFailed:
			falhou = true
		}
	}
	return Job{
		Name:        j.Name,
		Namespace:   j.Namespace,
		UID:         string(j.UID),
		Completions: completions,
		Active:      j.Status.Active,
		Succeeded:   j.Status.Succeeded,
		Failed:      j.Status.Failed,
		Concluido:   concluido,
		Falhou:      falhou,
		CriadoEm:    j.CreationTimestamp.Time,
		Labels:      j.Labels,
	}
}

// GetIngresses lista ingresses com filtros opcionais de namespace e label.
func (c *K8sClient) GetIngresses(filter ListFilter) ([]Ingress, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	list, err := c.clientset.NetworkingV1().Ingresses(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar ingresses: %w", err)
	}
	var result []Ingress
	for i := range list.Items {
		result = append(result, ingressDeK8s(&list.Items[i]))
	}
	return result, nil
}

// ingressDeK8s converte o ingress da API: classe, hosts e caminhos das regras.
func ingressDeK8s(ing *networkingv1.Ingress) Ingress {
	class := ""
	if ing.Spec.IngressClassName != nil {
		class = *ing.Spec.IngressClassName
	}
	var hosts []string
	var paths []string
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
		if rule.HTTP != nil {
			for _, p := range rule.HTTP.Paths {
				paths = append(paths, p.Path)
			}
		}
	}
	return Ingress{
		Name:      ing.Name,
		Namespace: ing.Namespace,
		UID:       string(ing.UID),
		Class:     class,
		Hosts:     strings.Join(hosts, ", "),
		Paths:     strings.Join(paths, ", "),
		CriadoEm:  ing.CreationTimestamp.Time,
		Labels:    ing.Labels,
		Endereco:  enderecoDe(ing.Status.LoadBalancer.Ingress),
	}
}

// GetConfigMaps lista configmaps com filtros opcionais de namespace e label.
func (c *K8sClient) GetConfigMaps(filter ListFilter) ([]ConfigMap, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	list, err := c.clientset.CoreV1().ConfigMaps(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar configmaps: %w", err)
	}
	var result []ConfigMap
	for i := range list.Items {
		result = append(result, configMapDeK8s(&list.Items[i]))
	}
	return result, nil
}

// configMapDeK8s converte o configmap da API: chaves e tamanho dos dados.
func configMapDeK8s(cm *corev1.ConfigMap) ConfigMap {
	totalSize := 0
	for _, v := range cm.Data {
		totalSize += len(v)
	}
	for _, v := range cm.BinaryData {
		totalSize += len(v)
	}
	dataSize := fmt.Sprintf("%dB", totalSize)
	if totalSize >= 1024 {
		dataSize = fmt.Sprintf("%.1fKi", float64(totalSize)/1024)
	}
	return ConfigMap{
		Name:      cm.Name,
		Namespace: cm.Namespace,
		UID:       string(cm.UID),
		Keys:      len(cm.Data) + len(cm.BinaryData),
		DataSize:  dataSize,
		CriadoEm:  cm.CreationTimestamp.Time,
		Labels:    cm.Labels,
		Chaves:    chavesDe(cm),
	}
}

// formatAge formata uma duração em formato legível (ex: "5m", "2h", "3d")
func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// Idade formata o tempo de desde até agora como o kubectl ("52m", "3h", "6d");
// sem data devolve vazio.
func Idade(desde, agora time.Time) string {
	if desde.IsZero() {
		return ""
	}
	d := agora.Sub(desde)
	if d < 0 {
		d = 0
	}
	return formatAge(d)
}

// GetEvents lista eventos com filtros opcionais de namespace e label.
func (c *K8sClient) GetEvents(filter ListFilter) ([]Event, error) {
	opts := metav1.ListOptions{}
	if filter.LabelSelector != "" {
		opts.LabelSelector = filter.LabelSelector
	}
	ns := filter.Namespace
	ctx, cancelar := c.contextoDeLeitura()
	defer cancelar()
	list, err := c.clientset.CoreV1().Events(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar eventos: %w", err)
	}

	return eventosDeK8s(list.Items, nil), nil
}

// eventosDeK8s converte os eventos que passam em aceitar (todos, se nil), dos
// mais recentes para os mais antigos.
func eventosDeK8s(itens []corev1.Event, aceitar func(*corev1.Event) bool) []Event {
	var result []Event
	for i := range itens {
		e := &itens[i]
		if aceitar != nil && !aceitar(e) {
			continue
		}
		quando := quandoDoEvento(e)
		age := "N/A"
		if !quando.IsZero() {
			age = formatAge(time.Since(quando))
		}
		quantas := e.Count
		if e.Series != nil && e.Series.Count > quantas {
			quantas = e.Series.Count
		}
		result = append(result, Event{
			ID:        e.Name,
			Name:      e.InvolvedObject.Name,
			Namespace: e.Namespace,
			Type:      e.Type,
			Reason:    e.Reason,
			Message:   e.Message,
			Age:       age,
			Labels:    e.Labels,
			Kind:      e.InvolvedObject.Kind,
			Quantas:   quantas,
			Quando:    quando,
		})
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Quando.After(result[j].Quando) })
	return result
}

// quandoDoEvento é a última ocorrência do evento: o LastTimestamp dos eventos
// antigos, a série ou o EventTime dos novos, o FirstTimestamp ou a criação.
func quandoDoEvento(e *corev1.Event) time.Time {
	switch {
	case !e.LastTimestamp.IsZero():
		return e.LastTimestamp.Time
	case e.Series != nil && !e.Series.LastObservedTime.IsZero():
		return e.Series.LastObservedTime.Time
	case !e.EventTime.IsZero():
		return e.EventTime.Time
	case !e.FirstTimestamp.IsZero():
		return e.FirstTimestamp.Time
	}
	return e.CreationTimestamp.Time
}
