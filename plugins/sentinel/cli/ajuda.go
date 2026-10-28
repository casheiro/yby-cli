//go:build k8s

package main

import (
	"io"
	"sort"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/profiles"
)

// flagAjuda é a linha do -h nas seções de flags, como no core.
var flagAjuda = ui.Termo{Nome: "-h, --help", Descricao: "mostra esta ajuda"}

// nomesDosPerfis lista os perfis de conformidade em ordem alfabética.
func nomesDosPerfis() []string {
	var nomes []string
	for _, p := range profiles.ListProfiles() {
		nomes = append(nomes, p.Name)
	}
	sort.Strings(nomes)
	return nomes
}

// listaComOu junta itens como "a, b ou c".
func listaComOu(itens []string) string {
	switch len(itens) {
	case 0:
		return ""
	case 1:
		return itens[0]
	}
	return strings.Join(itens[:len(itens)-1], ", ") + " ou " + itens[len(itens)-1]
}

// ajudaSentinel é a ajuda de yby sentinel, sem subcomando.
func ajudaSentinel() ui.Ajuda {
	return ui.Ajuda{
		Comando: "yby sentinel",
		Descricao: "Auditoria de segurança e conformidade do cluster Kubernetes: examina um namespace " +
			"com o Polaris e o OPA (ou com os checks internos, quando eles não respondem) e investiga " +
			"pods com problema usando IA.",
		Uso: []string{"yby sentinel <comando> [flags]"},
		Comandos: []ui.Termo{
			{Nome: "scan", Descricao: "procura problemas de segurança num namespace"},
			{Nome: "investigate <pod>", Descricao: "investiga um pod com problema usando IA"},
		},
		Exemplos: `
			yby sentinel scan -n payments
			yby sentinel scan -n payments --profile cis-l1
			yby sentinel scan -n payments --fix-dry-run
			yby sentinel investigate worker-5d8b6-q7tvn -n payments`,
		Flags:  []ui.Termo{flagAjuda},
		Rodape: "yby sentinel <comando> --help mostra os detalhes de cada um",
	}
}

// ajudaScan é a ajuda de yby sentinel scan.
func ajudaScan() ui.Ajuda {
	return ui.Ajuda{
		Comando: "yby sentinel scan",
		Descricao: "Procura problemas de segurança no namespace: o Polaris confere os pods e as boas " +
			"práticas, e o OPA confere RBAC e NetworkPolicy. Sem nenhum dos dois, rodam os checks internos.",
		Uso: []string{"yby sentinel scan [flags]"},
		Exemplos: `
			yby sentinel scan -n payments
			yby sentinel scan -n payments -o json -f scan.json
			yby sentinel scan -n payments --scope cluster
			yby sentinel scan -n payments --fix-dry-run`,
		Flags: []ui.Termo{
			{Nome: "-n, --namespace texto", Descricao: "namespace a examinar (padrão: o do ambiente pedido ou do contexto do kubeconfig, ou default)"},
			{Nome: "-o, --output formato", Descricao: "terminal, json ou markdown (padrão terminal)"},
			{Nome: "-f, --file arquivo", Descricao: "grava o json ou o markdown no arquivo em vez de na tela"},
			{Nome: "-p, --profile perfil", Descricao: "deixa só os achados do perfil, de todos os motores: " + listaComOu(nomesDosPerfis())},
			{Nome: "    --scope escopo", Descricao: "namespace (padrão: só o RBAC que vale no namespace) ou cluster (o RBAC do cluster todo)"},
			{Nome: "    --fix-dry-run", Descricao: "mostra o plano de correção, conferido pelo servidor sem gravar nada"},
			{Nome: "    --fix", Descricao: "aplica as correções seguras (escalação de privilégios e capabilities) no dono de cada pod, depois de confirmar; recursos do Argo CD ficam de fora"},
			{Nome: "    --yes", Descricao: "com --fix, aplica sem perguntar (fora de um terminal, é obrigatório)"},
			{Nome: "    --no-ai", Descricao: "não pede as recomendações da IA"},
			flagAjuda,
		},
		Rodape: "No formato terminal, o resumo sai na tela e o relatório completo vai para " +
			"~/.yby/reports/sentinel-scan-<namespace>-<contexto>-<data>-<hora>.md.",
	}
}

// ajudaInvestigate é a ajuda de yby sentinel investigate.
func ajudaInvestigate() ui.Ajuda {
	return ui.Ajuda{
		Comando: "yby sentinel investigate",
		Descricao: "Investiga um pod com problema: junta logs, eventos e métricas e, se o pod não " +
			"estiver saudável, pede à IA a causa provável e a correção.",
		Uso: []string{"yby sentinel investigate <pod> [flags]"},
		Exemplos: `
			yby sentinel investigate worker-5d8b6-q7tvn -n payments
			yby sentinel investigate worker-5d8b6-q7tvn -n payments -o markdown -f worker.md`,
		Flags: []ui.Termo{
			{Nome: "-n, --namespace texto", Descricao: "namespace do pod (padrão: o do ambiente pedido ou do contexto do kubeconfig, ou default)"},
			{Nome: "-o, --output formato", Descricao: "terminal, json ou markdown (padrão terminal)"},
			{Nome: "-f, --file arquivo", Descricao: "grava o json ou o markdown no arquivo em vez de na tela"},
			{Nome: "    --no-cache", Descricao: "ignora a análise guardada e pergunta à IA de novo"},
			{Nome: "    --no-ai", Descricao: "só coleta: mostra status, eventos e fim dos logs, sem chamar a IA nem o cache"},
			flagAjuda,
		},
		Rodape: "A análise fica guardada por 1 h em ~/.yby/sentinel/cache e volta enquanto os logs do pod não mudam.",
	}
}

// escreverAjuda escreve em w a ajuda do subcomando (vazio é a do Sentinel).
func escreverAjuda(w io.Writer, subcomando string) {
	switch subcomando {
	case "scan":
		ui.EscreverAjuda(w, ajudaScan())
	case "investigate":
		ui.EscreverAjuda(w, ajudaInvestigate())
	default:
		ui.EscreverAjuda(w, ajudaSentinel())
	}
}

// pedeAjuda informa se args pedem a ajuda.
func pedeAjuda(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}
