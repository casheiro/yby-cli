package discovery

import "testing"

// TestMatch_TableDriven verifica a função Match com diversos cenários de correspondência.
func TestMatch_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		// Correspondências positivas para cada regra padrão
		{
			name:     "go.mod deve retornar tipo app",
			filename: "go.mod",
			want:     "app",
		},
		{
			name:     "package.json deve retornar tipo app",
			filename: "package.json",
			want:     "app",
		},
		{
			name:     "Dockerfile deve retornar tipo infra",
			filename: "Dockerfile",
			want:     "infra",
		},
		{
			name:     "Taskfile.yml deve retornar tipo config",
			filename: "Taskfile.yml",
			want:     "config",
		},
		{
			name:     "Makefile deve retornar tipo config",
			filename: "Makefile",
			want:     "config",
		},

		// Correspondências negativas — arquivos desconhecidos
		{
			name:     "arquivo Go comum não deve corresponder",
			filename: "main.go",
			want:     "",
		},
		{
			name:     "README.md não deve corresponder",
			filename: "README.md",
			want:     "",
		},
		{
			name:     "string vazia não deve corresponder",
			filename: "",
			want:     "",
		},
		{
			name:     "dockerfile minúsculo não deve corresponder (case-sensitive)",
			filename: "dockerfile",
			want:     "",
		},
		{
			name:     "go.sum não deve corresponder",
			filename: "go.sum",
			want:     "",
		},
		{
			name:     "docker-compose.yml não deve corresponder",
			filename: "docker-compose.yml",
			want:     "",
		},
		{
			name:     "Taskfile.yaml (extensão diferente) não deve corresponder",
			filename: "Taskfile.yaml",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.filename)
			if got != tt.want {
				t.Errorf("Match(%q) = %q, esperado %q", tt.filename, got, tt.want)
			}
		})
	}
}

// TestDefaultRules_Integridade verifica que DefaultRules contém as regras esperadas.
func TestDefaultRules_Integridade(t *testing.T) {
	if len(DefaultRules) == 0 {
		t.Fatal("DefaultRules não deve estar vazio")
	}

	// Verificar que todas as regras têm campos preenchidos
	for i, rule := range DefaultRules {
		if rule.MatchFile == "" {
			t.Errorf("regra [%d] tem MatchFile vazio", i)
		}
		if rule.Type == "" {
			t.Errorf("regra [%d] (%s) tem Type vazio", i, rule.MatchFile)
		}
	}

	// Verificar tipos permitidos
	tiposValidos := map[string]bool{"app": true, "lib": true, "infra": true, "config": true}
	for _, rule := range DefaultRules {
		if !tiposValidos[rule.Type] {
			t.Errorf("regra %q tem tipo inválido: %q", rule.MatchFile, rule.Type)
		}
	}
}
