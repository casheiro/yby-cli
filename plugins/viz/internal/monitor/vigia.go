package monitor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// O watch das abas do Viz: um informer por tipo no namespace escolhido (vazio:
// todos), e as mudanças juntadas em lotes, no máximo um por intervalo, para a
// TUI não redesenhar a cada evento num cluster grande.

// TiposObservados são os tipos que o watch acompanha, pelo kind das ações e do
// YAML, na ordem das abas.
var TiposObservados = []string{"pod", "deployment", "statefulset", "service", "node", "job", "ingress", "configmap", "event"}

// Retrato é a lista inteira de um tipo como o cache do watch a vê agora, na
// ordem em que a API lista (namespace e nome).
type Retrato struct {
	Kind   string // "pod", "deployment"… (um de TiposObservados)
	Lista  any    // []Pod, []Deployment…; nil quando há Err
	Err    error  // o watch não conseguiu listar o tipo
	Negado bool   // o Err é falta de permissão (RBAC): o watch do tipo parou
}

// Lote são os retratos dos tipos que mudaram desde o lote anterior.
type Lote struct {
	Namespace string
	Retratos  []Retrato
}

// Observador acompanha os recursos das abas por watch.
type Observador interface {
	// Observar abre o watch dos TiposObservados no namespace (vazio: todos) e
	// manda pelo canal um Lote por intervalo, só com os tipos que mudaram (o
	// primeiro de cada tipo sai quando a lista inicial chega). Tudo para quando
	// ctx acaba, e o canal fecha depois que os watches pararam. Devolve nil se
	// o client não observa.
	Observar(ctx context.Context, namespace string) <-chan Lote
}

var (
	_ Observador = (*K8sClient)(nil)
	_ Observador = (*RetryClient)(nil)
)

// intervaloDoLote é o menor tempo entre dois lotes: até quatro atualizações
// por segundo na tela, por mais eventos que o cluster mande.
const intervaloDoLote = 250 * time.Millisecond

// NewK8sClientDe cria o client sobre um clientset pronto (o fake dos testes),
// sem kubeconfig: sem contexto, sem namespace do contexto e sem REST.
func NewK8sClientDe(cs kubernetes.Interface) *K8sClient {
	return &K8sClient{clientset: cs}
}

// Observar abre o watch dos tipos das abas no cluster.
func (c *K8sClient) Observar(ctx context.Context, namespace string) <-chan Lote {
	intervalo := c.intervaloLote
	if intervalo <= 0 {
		intervalo = intervaloDoLote
	}
	return observar(ctx, c.clientset, namespace, intervalo)
}

// Observar repassa ao client de dentro; nil se ele não observa. O watch não
// passa pelo retry: o informer já tenta de novo sozinho, com espera crescente.
func (r *RetryClient) Observar(ctx context.Context, namespace string) <-chan Lote {
	if o, ok := r.inner.(Observador); ok {
		return o.Observar(ctx, namespace)
	}
	return nil
}

// tipoObservado é o informer de um tipo e o que dele já foi entregue.
type tipoObservado struct {
	kind     string
	informer cache.SharedIndexInformer
	parar    context.CancelFunc // para só este informer (RBAC)
	versao   uint64             // sobe a cada mudança ou erro
	entregue uint64             // a versão do último retrato que saiu
	err      error
	negado   bool
	rvDoErro string // a versão da lista quando o erro chegou: mudou, o informer voltou
}

// vigia junta as mudanças dos informers de um namespace em lotes.
type vigia struct {
	namespace string
	mu        sync.Mutex
	tipos     []*tipoObservado
}

// observar liga um informer por tipo e a entrega dos lotes.
func observar(ctx context.Context, cs kubernetes.Interface, namespace string, intervalo time.Duration) <-chan Lote {
	fabrica := informers.NewSharedInformerFactoryWithOptions(cs, 0, informers.WithNamespace(namespace))
	v := &vigia{namespace: namespace}
	var rodando sync.WaitGroup
	for _, kind := range TiposObservados {
		ctxTipo, parar := context.WithCancel(ctx)
		t := &tipoObservado{kind: kind, informer: informerDe(fabrica, kind), parar: parar, versao: 1}
		// O informer ainda não rodou: as três chamadas não falham.
		_ = t.informer.SetTransform(paraOCache)
		_ = t.informer.SetWatchErrorHandlerWithContext(func(_ context.Context, _ *cache.Reflector, err error) {
			v.falhou(ctx, t, err)
		})
		_, _ = t.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(any) { v.marcar(t) },
			UpdateFunc: func(any, any) { v.marcar(t) },
			DeleteFunc: func(any) { v.marcar(t) },
		})
		v.tipos = append(v.tipos, t)
		rodando.Add(1)
		go func() {
			defer rodando.Done()
			defer parar()
			t.informer.RunWithContext(ctxTipo)
		}()
	}
	saida := make(chan Lote)
	go func() {
		defer close(saida)
		defer rodando.Wait() // o canal só fecha com todos os informers parados
		v.entregar(ctx, saida, intervalo)
	}()
	return saida
}

// informerDe cria o informer do tipo na fábrica do namespace.
func informerDe(f informers.SharedInformerFactory, kind string) cache.SharedIndexInformer {
	switch kind {
	case "pod":
		return f.Core().V1().Pods().Informer()
	case "deployment":
		return f.Apps().V1().Deployments().Informer()
	case "statefulset":
		return f.Apps().V1().StatefulSets().Informer()
	case "service":
		return f.Core().V1().Services().Informer()
	case "node":
		return f.Core().V1().Nodes().Informer() // do cluster: o namespace não vale
	case "job":
		return f.Batch().V1().Jobs().Informer()
	case "ingress":
		return f.Networking().V1().Ingresses().Informer()
	case "configmap":
		return f.Core().V1().ConfigMaps().Informer()
	}
	return f.Core().V1().Events().Informer()
}

// anotacaoTamanho guarda, só no cache do watch, o tamanho em bytes dos dados
// do ConfigMap, que sai do cache sem os valores.
const anotacaoTamanho = "viz.yby/tamanho-dos-dados"

// paraOCache enxuga o objeto antes de ele entrar no cache, deixando só o que
// as conversões das abas leem: sem managedFields nem a annotation
// last-applied (o manifesto inteiro de novo) em todo tipo; o ConfigMap sem os
// valores (as chaves ficam, e o tamanho vai na anotacaoTamanho); o Pod sem
// env, volumes, args e command. O YAML do detalhe vem da API, inteiro. Roda de
// novo sem estragar o objeto já enxuto, como o client-go pede.
func paraOCache(o any) (any, error) {
	if a, ok := o.(metav1.Object); ok {
		a.SetManagedFields(nil)
		if anotacoes := a.GetAnnotations(); anotacoes != nil {
			delete(anotacoes, "kubectl.kubernetes.io/last-applied-configuration")
			if len(anotacoes) == 0 {
				a.SetAnnotations(nil)
			}
		}
	}
	switch v := o.(type) {
	case *corev1.ConfigMap:
		if _, ok := v.Annotations[anotacaoTamanho]; !ok {
			if v.Annotations == nil {
				v.Annotations = map[string]string{}
			}
			v.Annotations[anotacaoTamanho] = strconv.Itoa(tamanhoDosDados(v))
		}
		for k := range v.Data {
			v.Data[k] = ""
		}
		for k := range v.BinaryData {
			v.BinaryData[k] = nil
		}
	case *corev1.Pod:
		v.Spec.Volumes = nil
		for _, cs := range [][]corev1.Container{v.Spec.InitContainers, v.Spec.Containers} {
			for i := range cs {
				cs[i].Env, cs[i].EnvFrom, cs[i].VolumeMounts = nil, nil, nil
				cs[i].Command, cs[i].Args = nil, nil
			}
		}
	}
	return o, nil
}

// marcar registra que o tipo mudou: o próximo lote leva o retrato novo.
func (v *vigia) marcar(t *tipoObservado) {
	v.mu.Lock()
	defer v.mu.Unlock()
	t.err = nil
	t.versao++
}

// falhou registra o erro do watch de um tipo. O fim de um watch (expirado,
// EOF) não é falha: o informer lista de novo sozinho. Sem permissão, o watch
// do tipo para (tentar de novo só encheria o log da API) e a aba fica sem número.
func (v *vigia) falhou(ctx context.Context, t *tipoObservado, err error) {
	switch {
	case ctx.Err() != nil, errors.Is(err, context.Canceled), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF),
		apierrors.IsResourceExpired(err), apierrors.IsGone(err):
		return
	}
	negado := apierrors.IsForbidden(err)
	slog.Debug("viz: watch falhou", "tipo", t.kind, "namespace", v.namespace, "negado", negado, "erro", err)
	v.mu.Lock()
	t.err, t.negado = err, negado
	t.rvDoErro = t.informer.LastSyncResourceVersion()
	t.versao++
	v.mu.Unlock()
	if negado {
		t.parar()
	}
}

// pendente é um tipo que vai no próximo lote, com a versão colhida.
type pendente struct {
	t      *tipoObservado
	versao uint64
	r      Retrato
}

// colher separa os tipos que mudaram desde a última entrega: com erro, o
// retrato do erro; com a lista inicial já lida, a lista do cache. Um tipo
// ainda lendo a primeira lista espera. A conversão roda fora da trava: os
// eventos do informer não esperam por ela.
func (v *vigia) colher() []pendente {
	v.mu.Lock()
	var pendentes []pendente
	for _, t := range v.tipos {
		// O informer que falhou e listou de novo (mesmo uma lista vazia, que não
		// gera evento) voltou: o retrato é a lista nova.
		if t.err != nil && !t.negado && t.informer.HasSynced() && t.informer.LastSyncResourceVersion() != t.rvDoErro {
			t.err = nil
			t.versao++
		}
		if t.versao == t.entregue || (t.err == nil && !t.informer.HasSynced()) {
			continue
		}
		pendentes = append(pendentes, pendente{t: t, versao: t.versao, r: Retrato{Kind: t.kind, Err: t.err, Negado: t.negado}})
	}
	v.mu.Unlock()
	for i := range pendentes {
		if pendentes[i].r.Err == nil {
			pendentes[i].r.Lista = listaDoCache(pendentes[i].t.kind, pendentes[i].t.informer.GetStore().List())
		}
	}
	return pendentes
}

// confirmar marca o que saiu no lote; o tipo que mudou de novo depois da
// colheita segue pendente.
func (v *vigia) confirmar(pendentes []pendente) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, p := range pendentes {
		p.t.entregue = max(p.t.entregue, p.versao)
	}
}

// entregar manda um lote por intervalo enquanto houver mudança. Se a TUI
// ainda não pediu o lote seguinte, os tipos seguem pendentes para o próximo
// intervalo, com a lista mais nova.
func (v *vigia) entregar(ctx context.Context, saida chan<- Lote, intervalo time.Duration) {
	relogio := time.NewTicker(intervalo)
	defer relogio.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-relogio.C:
		}
		pendentes := v.colher()
		if len(pendentes) == 0 {
			continue
		}
		lote := Lote{Namespace: v.namespace, Retratos: make([]Retrato, len(pendentes))}
		for i, p := range pendentes {
			lote.Retratos[i] = p.r
		}
		select {
		case saida <- lote:
			v.confirmar(pendentes)
		case <-ctx.Done():
			return
		default:
		}
	}
}

// comChave é um objeto do cache com a chave namespace/nome dele.
type comChave struct {
	chave string
	obj   any
}

// listaDoCache converte os objetos do cache de um tipo, na ordem da API
// (namespace e nome): a ordem "problemas primeiro" desempata por ela. A chave
// de cada objeto é calculada uma vez, ao lado dele: buscá-la num mapa a cada
// comparação custava mais que a conversão.
func listaDoCache(kind string, objs []any) any {
	ordenados := make([]comChave, len(objs))
	for i, o := range objs {
		k, _ := cache.MetaNamespaceKeyFunc(o)
		ordenados[i] = comChave{chave: k, obj: o}
	}
	slices.SortFunc(ordenados, func(a, b comChave) int { return strings.Compare(a.chave, b.chave) })
	for i := range ordenados {
		objs[i] = ordenados[i].obj
	}
	switch kind {
	case "pod":
		return converter(objs, func(p *corev1.Pod) Pod { return podDeK8s(p) })
	case "deployment":
		return converter(objs, deploymentDeK8s)
	case "statefulset":
		return converter(objs, statefulSetDeK8s)
	case "service":
		return converter(objs, serviceDeK8s)
	case "node":
		return converter(objs, nodeDeK8s)
	case "job":
		return converter(objs, jobDeK8s)
	case "ingress":
		return converter(objs, ingressDeK8s)
	case "configmap":
		return converter(objs, configMapDeK8s)
	}
	return eventosDePonteiros(converter(objs, func(e *corev1.Event) *corev1.Event { return e }), nil)
}

// converter aplica f a cada objeto do tipo O (os outros, que o cache não
// deveria ter, ficam de fora).
func converter[O any, R any](objs []any, f func(*O) R) []R {
	saida := make([]R, 0, len(objs))
	for _, o := range objs {
		if p, ok := o.(*O); ok {
			saida = append(saida, f(p))
		}
	}
	return saida
}
