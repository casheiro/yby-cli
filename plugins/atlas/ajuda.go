package main

import (
	"io"

	"github.com/casheiro/yby-cli/pkg/ui"
)

// flagAjuda é a linha do -h nas seções de flags, como no core.
var flagAjuda = ui.Termo{Nome: "-h, --help", Descricao: "mostra esta ajuda"}

// ajudaAtlas é a ajuda de yby atlas, sem subcomando.
func ajudaAtlas() ui.Ajuda {
	return ui.Ajuda{
		Comando: "yby atlas",
		Descricao: "Scanner de topologia da infraestrutura do projeto: lê charts do Helm, manifests do " +
			"Kubernetes, docker-compose, Kustomize e Terraform e desenha as relações entre eles num " +
			"diagrama Mermaid.",
		Uso: []string{"yby atlas <comando> [flags]"},
		Comandos: []ui.Termo{
			{Nome: "diagram", Descricao: "gera o diagrama da topologia em .yby/atlas-diagram.mmd"},
		},
		Exemplos: `
			yby atlas diagram
			yby atlas diagram --detail full
			yby atlas diagram --no-ai`,
		Flags:  []ui.Termo{flagAjuda},
		Rodape: "yby atlas <comando> --help mostra os detalhes de cada um",
	}
}

// ajudaDiagram é a ajuda de yby atlas diagram.
func ajudaDiagram() ui.Ajuda {
	return ui.Ajuda{
		Comando: "yby atlas diagram",
		Descricao: "Gera o diagrama Mermaid da topologia da infraestrutura e o grava em " +
			".yby/atlas-diagram.mmd. Com uma IA disponível, o diagrama é reorganizado e ganha nomes " +
			"legíveis; sem ela, ou com --detail full (que mostra todos os recursos), fica o diagrama " +
			"montado a partir dos arquivos.",
		Uso: []string{"yby atlas diagram [flags]"},
		Exemplos: `
			yby atlas diagram
			yby atlas diagram --detail full --no-ai`,
		Flags: []ui.Termo{
			{Nome: "-d, --detail nível", Descricao: "overview (os principais, até 25 nós) ou full (todos os recursos, com RBAC, CRDs e ConfigMaps); padrão overview"},
			{Nome: "    --no-ai", Descricao: "não refina o diagrama com IA (o full nunca passa por ela)"},
			flagAjuda,
		},
		Rodape: "Para ver o diagrama, abra o arquivo em https://mermaid.live ou na extensão Mermaid do VS Code.",
	}
}

// escreverAjuda escreve em w a ajuda do subcomando (vazio é a do Atlas).
func escreverAjuda(w io.Writer, subcomando string) {
	if subcomando == "diagram" {
		ui.EscreverAjuda(w, ajudaDiagram())
		return
	}
	ui.EscreverAjuda(w, ajudaAtlas())
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
