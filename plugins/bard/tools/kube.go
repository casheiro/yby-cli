package tools

import (
	"context"
	"os/exec"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

// kube é o cluster da sessão (yby bard --context, --namespace): o kubectl das ferramentas
// usa esse contexto e, quando a IA não diz o namespace, esse namespace. Vazio, valem o
// contexto atual do kubeconfig e o namespace dele. É definido uma vez, antes da conversa.
var kube struct{ contexto, namespace string }

// DefinirKube define o contexto e o namespace do kubectl das ferramentas.
func DefinirKube(contexto, namespace string) {
	kube.contexto, kube.namespace = contexto, namespace
}

// namespaceDe é o namespace pedido pela IA ou, sem ele, o da sessão.
func namespaceDe(params map[string]string) string {
	if ns := params["namespace"]; ns != "" {
		return ns
	}
	return kube.namespace
}

// argsKubectl põe o --context da sessão na frente dos args.
func argsKubectl(args []string) []string {
	if kube.contexto == "" {
		return args
	}
	return append([]string{"--context", kube.contexto}, args...)
}

// comandoKubectl é o kubectl com os args e o contexto da sessão.
func comandoKubectl(ctx context.Context, args []string) *exec.Cmd {
	return exec.CommandContext(ctx, "kubectl", argsKubectl(args)...)
}

// pedidoAoPlugin é o pedido do hook command a um plugin do Yby (o Sentinel) no cluster da
// sessão: o contexto vai no infra.kube_context e o namespace no infra.namespace do pedido,
// que o SDK do plugin usa no client do Kubernetes. Sem sessão, valem os do kubeconfig.
func pedidoAoPlugin(args []string) plugin.PluginRequest {
	req := plugin.PluginRequest{Hook: "command", Args: args}
	if kube.contexto == "" && kube.namespace == "" {
		return req
	}
	infra := map[string]interface{}{}
	if kube.contexto != "" {
		infra["kube_context"] = kube.contexto
	}
	if kube.namespace != "" {
		infra["namespace"] = kube.namespace
	}
	req.Context = map[string]interface{}{"infra": infra}
	return req
}
