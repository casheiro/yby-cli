package analysis

import (
	"strings"
	"testing"
)

// rascunhoDeTeste é um rascunho no formato do GenerateInfraMermaid.
const rascunhoDeTeste = `flowchart TD
  subgraph Helm_Charts["Helm Charts"]
    HelmChart_api["api\n(HelmChart)"]
  end
  subgraph Workloads["Workloads"]
    Deployment_api_server["api-server\n(Deployment)"]
    grupo_Job["3 recursos\n(Job)"]
  end
  HelmChart_api -->|deploys| Deployment_api_server
`

func TestValidarMermaid_Aceita(t *testing.T) {
	casos := map[string]string{
		"refinado com subgraphs":  "flowchart TD\n  subgraph app[\"Aplicação\"]\n    HelmChart_api[\"API\"] -->|implanta| Deployment_api_server[\"api-server\"]\n  end\n",
		"graph e comentário":      "%% feito pela IA\ngraph LR\n  HelmChart_api[API]\n  HelmChart_api --> grupo_Job{{Jobs & CronJobs}}",
		"declarado depois":        "flowchart TD\n  HelmChart_api --> Deployment_api_server\n  HelmChart_api[API]\n  Deployment_api_server(api-server)",
		"aresta para subgraph":    "flowchart TD\n  subgraph cargas[Cargas]\n    grupo_Job[[Jobs]]\n  end\n  HelmChart_api[API] -.->|agenda| cargas",
		"vários com &":            "flowchart TD\n  HelmChart_api[API] --> Deployment_api_server[api] & grupo_Job[jobs]",
		"classe e seta no rótulo": "flowchart TD\n  HelmChart_api[\"API --> v2\"]:::destaque -- implanta --> Deployment_api_server[\"api\"];\n  classDef destaque fill:#f96",
		"subgraph aninhado":       "flowchart TD\n  subgraph a[A]\n    direction LR\n    subgraph b[B]\n      HelmChart_api[API]\n    end\n  end",
		"rascunho como veio":      rascunhoDeTeste,
	}
	for nome, texto := range casos {
		if err := ValidarMermaid(texto, rascunhoDeTeste); err != nil {
			t.Errorf("%s: esperado válido, obtido %v", nome, err)
		}
	}
}

func TestValidarMermaid_Recusa(t *testing.T) {
	casos := []struct {
		nome, texto, motivo string
	}{
		{"prosa antes", "Aqui está o flowchart:\nHelmChart_api-->Deployment_api_server", "flowchart"},
		{"vazio", "", "flowchart"},
		{"só o cabeçalho", "flowchart TD\n", "nenhum nó"},
		{"subgraph sem end", "flowchart TD\n  subgraph x\n    HelmChart_api[API] --> Deployment_api_server[api]", "end"},
		{"end sobrando", "flowchart TD\n  HelmChart_api[API]\n  end", "end"},
		{"nó fora do rascunho", "flowchart TD\n  api[API] --> redis[(Redis)]", "api"},
		{"aresta sem declarar", "flowchart TD\n  HelmChart_api[API] --> Deployment_api_server", "Deployment_api_server"},
		{"prosa depois", "flowchart TD\n  HelmChart_api[API]\nEste diagrama mostra a API.", "Este"},
		{"subgraph não é nó", "flowchart TD\n  Workloads[\"Workloads\"] --> HelmChart_api[API]", "Workloads"},
	}
	for _, c := range casos {
		err := ValidarMermaid(c.texto, rascunhoDeTeste)
		if err == nil {
			t.Errorf("%s: esperado inválido", c.nome)
			continue
		}
		if !strings.Contains(err.Error(), c.motivo) {
			t.Errorf("%s: o motivo cita %q, obtido %q", c.nome, c.motivo, err)
		}
	}
}
