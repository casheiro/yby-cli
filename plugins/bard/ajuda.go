package main

import "github.com/casheiro/yby-cli/pkg/ui"

// ajudaBard é a ajuda de yby bard (folha SaidasComandos), escrita com ui.EscreverAjuda
// como a dos outros plugins.
func ajudaBard() ui.Ajuda {
	return ui.Ajuda{
		Comando: "yby bard",
		Descricao: "Assistente de IA para infraestrutura Kubernetes. O Bard entende o pedido e executa as " +
			"ferramentas do Yby (sentinel, atlas, kubectl) quando preciso; a IA interpreta os resultados.\n\n" +
			"Na TUI, ↵ envia, esc interrompe a resposta e ctrl+c sai. No Viz, b abre o Bard com a pergunta " +
			"sobre o recurso selecionado já escrita no campo, no cluster e no namespace do Viz; esc volta ao Viz.\n\n" +
			"Fora da TUI, só a resposta vai para o stdout; as etapas (num terminal) e os erros vão para o stderr.",
		Uso: []string{
			"yby bard [flags]",
			`yby bard -p "pergunta"`,
			`echo "pergunta" | yby bard`,
		},
		Exemplos: `
			# conversa na TUI (precisa de um terminal)
			yby bard
			# uma pergunta, resposta no stdout
			yby bard -p "lista os pods"
			# uma pergunta por linha do stdin
			echo "por que o pod api reinicia?" | yby bard
			# a conversa abre com a pergunta escrita; ↵ envia
			yby bard --pergunta "por que o pod api reinicia?" --context k3d-yby-local -n payments`,
		Flags: []ui.Termo{
			{Nome: `-p, --prompt "pergunta"`, Descricao: "responde uma pergunta e sai"},
			{Nome: `    --pergunta "pergunta"`, Descricao: "abre a conversa com a pergunta escrita, sem enviar"},
			{Nome: "    --context nome", Descricao: "contexto do kubeconfig (padrão: o atual)"},
			{Nome: "-n, --namespace nome", Descricao: "namespace da conversa (padrão: o do contexto)"},
			{Nome: "    --voltar instrumento", Descricao: "quem abriu o Bard (o Viz passa Viz): esc fora da resposta volta a ele"},
			{Nome: "-h, --help", Descricao: "mostra esta ajuda"},
		},
		Rodape: "Ferramentas integradas: sentinel scan/investigate, kubectl get/logs/events/describe e atlas " +
			"blueprint. Ferramentas externas em YAML: ~/.yby/tools/*.yaml (do usuário) e .yby/tools/*.yaml (do projeto, depois de yby plugin trust <arquivo>).",
	}
}
