package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

func init() {
	Register(&Tool{
		Name:        "atlas_blueprint",
		Description: "Obtém o blueprint do projeto via plugin Atlas (componentes, charts, workloads e relações)",
		Intents:     []string{"infra_topology", "project_architecture", "blueprint"},
		Parameters:  []ToolParam{},
		Execute:     executeAtlasBlueprint,
	})
}

// pedidoDoBlueprint é o pedido ao Atlas: o hook context com o argumento infra, que traz a
// topologia da infraestrutura (charts, workloads, services) ao lado do blueprint dos
// componentes. O contrato está em plugins/atlas/testdata/contrato-blueprint*.json.
var pedidoDoBlueprint = plugin.PluginRequest{Hook: "context", Args: []string{"infra"}}

func executeAtlasBlueprint(ctx context.Context, params map[string]string) (string, error) {
	binaryPath, err := discoverPluginBinary("atlas")
	if err != nil {
		return "", fmt.Errorf("atlas não disponível: %w", err)
	}

	resp, err := invokePlugin(ctx, binaryPath, pedidoDoBlueprint)
	if err != nil {
		return "", err
	}

	return formatBlueprintResults(resp.Data), nil
}

// respostaDoAtlas é o Data do hook context do Atlas: {"blueprint": …, "infra": …}. Os
// caminhos vêm relativos à raiz do projeto e com '/'; nas relações do blueprint, from e to
// são caminhos de componentes (ou o repositório remoto de um chart), e nas da infra, IDs
// Kind/Name ou Kind/Namespace/Name.
type respostaDoAtlas struct {
	Blueprint *struct {
		Components []struct {
			Name      string `json:"name"`
			Type      string `json:"type"`
			Path      string `json:"path"`
			Language  string `json:"language"`
			Framework string `json:"framework"`
		} `json:"components"`
		Relations []relacaoDoAtlas `json:"relations"`
	} `json:"blueprint"`
	Infra *struct {
		Resources []struct {
			Kind      string            `json:"kind"`
			Name      string            `json:"name"`
			Namespace string            `json:"namespace"`
			Path      string            `json:"path"`
			Metadata  map[string]string `json:"metadata"`
		} `json:"resources"`
		Relations []relacaoDoAtlas `json:"relations"`
	} `json:"infra"`
}

type relacaoDoAtlas struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// kindsDeWorkload são os recursos que rodam alguma coisa; os outros (services, configmaps,
// charts…) entram agrupados por kind.
var kindsDeWorkload = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true, "ReplicaSet": true,
	"Job": true, "CronJob": true, "Pod": true, "ComposeService": true,
}

// Limites do resumo: quantos itens cada lista mostra antes de virar "e mais N", e o
// tamanho total que vai à IA.
const (
	maxItensPorLista  = 40
	maxNomesPorKind   = 20
	limiteDoBlueprint = 8 * 1024
)

// formatBlueprintResults converte a resposta do hook context do Atlas num resumo para a
// IA, com os nomes: componentes, charts Helm, workloads, os outros recursos por kind e as
// relações. O resumo passa de no máximo limiteDoBlueprint bytes.
func formatBlueprintResults(data interface{}) string {
	if data == nil {
		return "Nenhum blueprint disponível."
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	var r respostaDoAtlas
	if err := json.Unmarshal(dataBytes, &r); err != nil || (r.Blueprint == nil && r.Infra == nil) {
		return "Nenhum blueprint disponível."
	}

	var sb strings.Builder
	sb.WriteString("## Blueprint do Projeto\n")

	if bp := r.Blueprint; bp != nil {
		// Nas relações, o caminho do componente vira o nome dele.
		nomePorCaminho := make(map[string]string)
		var componentes, charts []string
		for _, c := range bp.Components {
			if _, ok := nomePorCaminho[c.Path]; !ok {
				nomePorCaminho[c.Path] = c.Name
			}
			if c.Type == "helm" {
				charts = append(charts, fmt.Sprintf("%s em %s", c.Name, c.Path))
				continue
			}
			detalhes := []string{c.Type}
			for _, d := range []string{c.Language, c.Framework} {
				if d != "" {
					detalhes = append(detalhes, d)
				}
			}
			componentes = append(componentes, fmt.Sprintf("%s (%s) em %s", c.Name, strings.Join(detalhes, ", "), c.Path))
		}
		escreverLista(&sb, "Componentes", componentes)
		escreverLista(&sb, "Charts Helm", charts)

		var relacoes []string
		for _, rel := range bp.Relations {
			de, para := rel.From, rel.To
			if nome, ok := nomePorCaminho[de]; ok {
				de = nome
			}
			if nome, ok := nomePorCaminho[para]; ok {
				para = nome
			}
			relacoes = append(relacoes, fmt.Sprintf("%s → %s (%s)", de, para, rel.Type))
		}
		escreverLista(&sb, "Relações entre componentes", relacoes)
	}

	if infra := r.Infra; infra != nil {
		var workloads []string
		outros := make(map[string][]string)
		for _, rec := range infra.Resources {
			nome := rec.Name
			if rec.Namespace != "" {
				nome = rec.Namespace + "/" + rec.Name
			}
			if !kindsDeWorkload[rec.Kind] {
				outros[rec.Kind] = append(outros[rec.Kind], nome)
				continue
			}
			linha := fmt.Sprintf("%s %s em %s", rec.Kind, nome, rec.Path)
			if img := rec.Metadata["image"]; img != "" {
				linha += " · imagem " + img
			}
			workloads = append(workloads, linha)
		}
		escreverLista(&sb, "Workloads", workloads)

		kinds := make([]string, 0, len(outros))
		for k := range outros {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		var porKind []string
		total := 0
		for _, k := range kinds {
			nomes := outros[k]
			total += len(nomes)
			mais := ""
			if len(nomes) > maxNomesPorKind {
				mais = fmt.Sprintf(" e mais %d", len(nomes)-maxNomesPorKind)
				nomes = nomes[:maxNomesPorKind]
			}
			porKind = append(porKind, fmt.Sprintf("%s: %s%s", k, strings.Join(nomes, ", "), mais))
		}
		if total > 0 {
			fmt.Fprintf(&sb, "\n### Outros recursos de infraestrutura (%d)\n", total)
			for _, l := range porKind {
				sb.WriteString("- " + l + "\n")
			}
		}

		var relacoes []string
		for _, rel := range infra.Relations {
			relacoes = append(relacoes, fmt.Sprintf("%s → %s (%s)", rel.From, rel.To, rel.Type))
		}
		escreverLista(&sb, "Relações da infraestrutura", relacoes)
	}

	return cortarResumo(sb.String(), limiteDoBlueprint)
}

// escreverLista escreve a seção "### titulo (N)" com um item por linha, até
// maxItensPorLista; o resto vira "e mais N". Lista vazia não escreve nada.
func escreverLista(sb *strings.Builder, titulo string, itens []string) {
	if len(itens) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n### %s (%d)\n", titulo, len(itens))
	for i, item := range itens {
		if i == maxItensPorLista {
			fmt.Fprintf(sb, "- … e mais %d\n", len(itens)-maxItensPorLista)
			break
		}
		sb.WriteString("- " + item + "\n")
	}
}

// cortarResumo devolve texto com no máximo limite bytes: corta na última linha inteira que
// cabe e avisa que o resumo foi cortado.
func cortarResumo(texto string, limite int) string {
	if len(texto) <= limite {
		return texto
	}
	aviso := fmt.Sprintf("\n… resumo cortado no limite de %d bytes\n", limite)
	corte := limite - len(aviso)
	for corte > 0 && !utf8.RuneStart(texto[corte]) {
		corte--
	}
	if i := strings.LastIndexByte(texto[:corte], '\n'); i >= 0 {
		corte = i
	}
	return texto[:corte] + aviso
}
