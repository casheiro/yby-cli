package tools

import (
	"context"
	"strings"
	"testing"
)

func TestParseIntent(t *testing.T) {
	casos := []struct {
		nome    string
		texto   string
		intent  string
		direct  bool
		params  map[string]string
		wantErr bool
	}{
		{"texto antes e número", `Claro! {"intent":"pod_logs","params":{"pod":"a","tail":100}}`, "pod_logs", false, map[string]string{"pod": "a", "tail": "100"}, false},
		{"cercas", "```json\n{\"intent\":\"pod_logs\",\"params\":{\"pod\":\"a\"}}\n```", "pod_logs", false, map[string]string{"pod": "a"}, false},
		{"texto depois", `{"intent":"list_pods","params":{}} espero ter ajudado {`, "list_pods", false, map[string]string{}, false},
		{"booleano e nulo", `{"intent":"x","params":{"follow":true,"ns":null,"obj":{"a":1}}}`, "x", false, map[string]string{"follow": "true"}, false},
		{"direct", `{"intent":"direct","params":{},"direct":true}`, "direct", true, map[string]string{}, false},
		{"objeto sem intent é pulado", `{"nota":1} {"intent":"a","params":{"k":"v"}}`, "a", false, map[string]string{"k": "v"}, false},
		{"sem json", `não sei`, "", false, nil, true},
		{"json quebrado", `{"intent":`, "", false, nil, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r, err := ParseIntent(c.texto)
			if (err != nil) != c.wantErr {
				t.Fatalf("erro = %v, queria erro=%v", err, c.wantErr)
			}
			if err != nil {
				return
			}
			if r.Intent != c.intent || r.Direct != c.direct {
				t.Errorf("intent=%q direct=%v", r.Intent, r.Direct)
			}
			if len(r.Params) != len(c.params) {
				t.Errorf("params = %v, queria %v", r.Params, c.params)
			}
			for k, v := range c.params {
				if r.Params[k] != v {
					t.Errorf("param %s = %q, queria %q", k, r.Params[k], v)
				}
			}
		})
	}
}

func TestDescreverIntents_TodosOsIntentsEOEsquema(t *testing.T) {
	isolarRegistry(t)
	Register(&Tool{Name: "kubectl_logs", Description: "Logs", Intents: []string{"view_logs", "pod_logs"},
		Parameters: []ToolParam{{Name: "pod", Required: true}, {Name: "tail"}}})
	Register(&Tool{Name: "a_tool", Description: "Outra", Intents: []string{"zeta"}})

	primeiro := DescreverIntents()
	for _, esperado := range []string{"- pod_logs: Logs [parâmetros: pod*, tail]", "- view_logs: Logs [parâmetros: pod*, tail]", "- zeta: Outra", "- direct:"} {
		if !strings.Contains(primeiro, esperado) {
			t.Errorf("falta %q em:\n%s", esperado, primeiro)
		}
	}
	if strings.Contains(strings.ToLower(primeiro), "default") {
		t.Errorf("o prompt não deve citar default:\n%s", primeiro)
	}
	if strings.Index(primeiro, "pod_logs") > strings.Index(primeiro, "view_logs") || strings.Index(primeiro, "view_logs") > strings.Index(primeiro, "zeta") {
		t.Errorf("fora de ordem alfabética:\n%s", primeiro)
	}
	for i := 0; i < 20; i++ {
		if DescreverIntents() != primeiro {
			t.Fatal("a lista de intents muda entre chamadas")
		}
	}
}

func TestRegistry_OrdemEstavel(t *testing.T) {
	isolarRegistry(t)
	for _, n := range []string{"c", "a", "d", "b"} {
		Register(&Tool{Name: n, Intents: []string{"i_" + n}})
	}
	prompt := FormatToolsPrompt()
	for i := 0; i < 20; i++ {
		if FormatToolsPrompt() != prompt {
			t.Fatal("FormatToolsPrompt muda entre chamadas")
		}
	}
	var nomes []string
	for _, tl := range All() {
		nomes = append(nomes, tl.Name)
	}
	if strings.Join(nomes, "") != "abcd" {
		t.Errorf("All fora de ordem: %v", nomes)
	}
}

func TestTool_Faltando(t *testing.T) {
	tl := &Tool{Parameters: []ToolParam{{Name: "pod", Required: true}, {Name: "ns"}, {Name: "x", Required: true}},
		Execute: func(context.Context, map[string]string) (string, error) { return "", nil }}
	faltam := tl.Faltando(map[string]string{"pod": "  ", "x": "1"})
	if len(faltam) != 1 || faltam[0] != "pod" {
		t.Errorf("faltando = %v", faltam)
	}
	if f := tl.Faltando(map[string]string{"pod": "a", "x": "1"}); len(f) != 0 {
		t.Errorf("nada deveria faltar: %v", f)
	}
}

// isolarRegistry esvazia o registry no teste e devolve as ferramentas ao fim.
func isolarRegistry(t *testing.T) {
	t.Helper()
	salvas := All()
	Reset()
	t.Cleanup(func() {
		Reset()
		for _, s := range salvas {
			Register(s)
		}
	})
}
