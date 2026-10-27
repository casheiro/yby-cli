package tools

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"

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

// Os valores da IA viram argumentos do kubectl e do Sentinel. Um valor começando com "-"
// viraria flag (pod "--as=system:admin", resource "--raw=/api/…/secrets/x", namespace
// "--fix" no Sentinel), e um com espaço ("pods -o yaml") não é nome de nada.
// Por isso os nomes seguem o formato do Kubernetes antes de qualquer execução.
var (
	// nomeKube casa nome de objeto e de namespace (DNS-1123, com maiúsculas e "_" de folga).
	nomeKube = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._\-]{0,252}$`)
	// tipoKube casa o tipo do recurso como o kubectl aceita: pods, deployments.apps,
	// pod/nginx, pods,svc.
	tipoKube = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._\-/,]{0,252}$`)
	// linhasKube casa a quantidade de linhas do --tail.
	linhasKube = regexp.MustCompile(`^[0-9]{1,6}$`)
)

// conferirParams confere, entre os campos dados, os que a IA preencheu: resource como
// tipo do kubectl, tail como número e os demais (pod, name, namespace) como nome.
func conferirParams(params map[string]string, campos ...string) error {
	for _, campo := range campos {
		valor := params[campo]
		if valor == "" {
			continue
		}
		switch campo {
		case "resource":
			if !tipoKube.MatchString(valor) {
				return fmt.Errorf("parâmetro '%s' inválido (%q): use só o tipo do recurso (ex.: pods, deployments), sem espaço e sem começar com \"-\"", campo, valor)
			}
		case "tail":
			if !linhasKube.MatchString(valor) {
				return fmt.Errorf("parâmetro '%s' inválido (%q): use um número de linhas", campo, valor)
			}
		default:
			if !nomeKube.MatchString(valor) {
				return fmt.Errorf("parâmetro '%s' inválido (%q): use só o nome do Kubernetes, sem espaço e sem começar com \"-\"", campo, valor)
			}
		}
	}
	return nil
}
