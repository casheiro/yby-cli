package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

// UnmarshalJSON aceita parâmetros que a IA manda como número ou booleano
// ("tail": 100) e os guarda como texto; nulo, objeto e lista são descartados.
func (r *IntentResult) UnmarshalJSON(data []byte) error {
	var bruto struct {
		Intent string                     `json:"intent"`
		Params map[string]json.RawMessage `json:"params"`
		Direct bool                       `json:"direct"`
	}
	if err := json.Unmarshal(data, &bruto); err != nil {
		return err
	}
	r.Intent, r.Direct = bruto.Intent, bruto.Direct
	r.Params = make(map[string]string, len(bruto.Params))
	for nome, valor := range bruto.Params {
		valor = bytes.TrimSpace(valor)
		if len(valor) == 0 {
			continue
		}
		switch valor[0] {
		case '"':
			var s string
			if err := json.Unmarshal(valor, &s); err == nil {
				r.Params[nome] = s
			}
		case '{', '[', 'n':
			slog.Debug("parâmetro da intenção ignorado", "parametro", nome)
		default: // número ou booleano
			r.Params[nome] = string(valor)
		}
	}
	return nil
}

// ParseIntent lê a resposta do classificador. Tolera cercas de código e texto
// antes ou depois do JSON: vale o primeiro objeto que traz "intent".
func ParseIntent(texto string) (*IntentResult, error) {
	for i := 0; i < len(texto); i++ {
		if texto[i] != '{' {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(texto[i:]))
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			continue
		}
		var r IntentResult
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		if r.Intent != "" || r.Direct {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("resposta do classificador sem JSON de intenção")
}

// Faltando devolve os nomes dos parâmetros obrigatórios que params não traz
// (ausentes ou só com espaços), na ordem declarada.
func (t *Tool) Faltando(params map[string]string) []string {
	var faltam []string
	for _, p := range t.Parameters {
		if p.Required && strings.TrimSpace(params[p.Name]) == "" {
			faltam = append(faltam, p.Name)
		}
	}
	return faltam
}

// DescreverIntents lista, para o classificador, cada intent de cada ferramenta
// com a descrição e o esquema de parâmetros ("*" marca os obrigatórios). A
// ordem é estável: alfabética pelo nome do intent.
func DescreverIntents() string {
	linhas := map[string]string{}
	for _, t := range All() {
		var params []string
		for _, p := range t.Parameters {
			nome := p.Name
			if p.Required {
				nome += "*"
			}
			params = append(params, nome)
		}
		esquema := ""
		if len(params) > 0 {
			esquema = " [parâmetros: " + strings.Join(params, ", ") + "]"
		}
		for _, in := range t.Intents {
			if _, ja := linhas[in]; !ja {
				linhas[in] = fmt.Sprintf("- %s: %s%s\n", in, t.Description, esquema)
			}
		}
	}
	nomes := make([]string, 0, len(linhas))
	for n := range linhas {
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)
	var sb strings.Builder
	for _, n := range nomes {
		sb.WriteString(linhas[n])
	}
	sb.WriteString("- direct: responder diretamente sem executar ferramenta\n")
	return sb.String()
}

// ordenadas devolve as ferramentas por nome. Quem chama segura o mu.
func ordenadas() []*Tool {
	lista := make([]*Tool, 0, len(registry))
	for _, t := range registry {
		lista = append(lista, t)
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].Name < lista[j].Name })
	return lista
}
