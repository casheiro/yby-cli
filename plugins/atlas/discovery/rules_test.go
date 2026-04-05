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
		{
			name:     "Chart.yaml deve retornar tipo helm",
			filename: "Chart.yaml",
			want:     "helm",
		},
		{
			name:     "kustomization.yaml deve retornar tipo kustomize",
			filename: "kustomization.yaml",
			want:     "kustomize",
		},
		{
			name:     "pyproject.toml deve retornar tipo app",
			filename: "pyproject.toml",
			want:     "app",
		},
		{
			name:     "requirements.txt deve retornar tipo app",
			filename: "requirements.txt",
			want:     "app",
		},
		{
			name:     "pom.xml deve retornar tipo app",
			filename: "pom.xml",
			want:     "app",
		},
		{
			name:     "build.gradle deve retornar tipo app",
			filename: "build.gradle",
			want:     "app",
		},
		{
			name:     "Cargo.toml deve retornar tipo app",
			filename: "Cargo.toml",
			want:     "app",
		},
		{
			name:     "MyApp.csproj deve retornar tipo app (glob *.csproj)",
			filename: "MyApp.csproj",
			want:     "app",
		},
		{
			name:     "docker-compose.yml deve retornar tipo infra",
			filename: "docker-compose.yml",
			want:     "infra",
		},
		{
			name:     "docker-compose.yaml deve retornar tipo infra",
			filename: "docker-compose.yaml",
			want:     "infra",
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

	// Verificar que todas as regras têm pelo menos um critério de correspondência
	for i, rule := range DefaultRules {
		if rule.MatchFile == "" && rule.MatchGlob == "" {
			t.Errorf("regra [%d] não tem MatchFile nem MatchGlob", i)
		}
		if rule.Type == "" {
			t.Errorf("regra [%d] tem Type vazio", i)
		}
	}

	// Verificar tipos permitidos
	tiposValidos := map[string]bool{
		"app": true, "lib": true, "infra": true,
		"config": true, "helm": true, "kustomize": true,
	}
	for _, rule := range DefaultRules {
		label := rule.MatchFile
		if label == "" {
			label = rule.MatchGlob
		}
		if !tiposValidos[rule.Type] {
			t.Errorf("regra %q tem tipo inválido: %q", label, rule.Type)
		}
	}
}

// TestMergeRules_CustomOverride verifica que regras customizadas são adicionadas
// no início da lista, tendo precedência sobre as regras padrão.
func TestMergeRules_CustomOverride(t *testing.T) {
	custom := []RuleConfig{
		{MatchFile: "custom.yaml", Type: "config"},
		{MatchGlob: "*.special", Type: "lib"},
	}

	merged := MergeRules(custom)

	// Regras customizadas devem estar no início
	if len(merged) != len(custom)+len(DefaultRules) {
		t.Fatalf("esperado %d regras, obtido %d", len(custom)+len(DefaultRules), len(merged))
	}

	if merged[0].MatchFile != "custom.yaml" {
		t.Errorf("primeira regra deveria ser custom.yaml, obtido %q", merged[0].MatchFile)
	}
	if merged[1].MatchGlob != "*.special" {
		t.Errorf("segunda regra deveria ter glob *.special, obtido %q", merged[1].MatchGlob)
	}

	// Verificar que a correspondência customizada tem precedência
	result := MatchWithRules("custom.yaml", merged)
	if result != "config" {
		t.Errorf("esperado tipo 'config' para custom.yaml, obtido %q", result)
	}
}

// TestMergeRules_Empty verifica que uma lista vazia de regras customizadas
// retorna as regras padrão.
func TestMergeRules_Empty(t *testing.T) {
	merged := MergeRules(nil)

	if len(merged) != len(DefaultRules) {
		t.Fatalf("esperado %d regras (padrão), obtido %d", len(DefaultRules), len(merged))
	}

	// Verificar que é a mesma referência (otimização)
	for i := range DefaultRules {
		if merged[i] != DefaultRules[i] {
			t.Errorf("regra [%d] não corresponde à regra padrão", i)
		}
	}
}

// TestMatchWithRules_GlobPattern verifica que padrões glob funcionam corretamente.
func TestMatchWithRules_GlobPattern(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "Dockerfile.prod deve corresponder ao glob Dockerfile*",
			filename: "Dockerfile.prod",
			want:     "infra",
		},
		{
			name:     "Dockerfile.dev deve corresponder ao glob Dockerfile*",
			filename: "Dockerfile.dev",
			want:     "infra",
		},
		{
			name:     "Dockerfile simples deve corresponder (exato antes do glob)",
			filename: "Dockerfile",
			want:     "infra",
		},
		{
			name:     "arquivo sem correspondência não deve retornar tipo",
			filename: "random.txt",
			want:     "",
		},
	}

	rules := []Rule{
		{MatchGlob: "Dockerfile*", Type: "infra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchWithRules(tt.filename, rules)
			if got != tt.want {
				t.Errorf("MatchWithRules(%q) = %q, esperado %q", tt.filename, got, tt.want)
			}
		})
	}
}

// TestMatchWithRules_HelmDetection verifica que Chart.yaml é detectado como helm.
func TestMatchWithRules_HelmDetection(t *testing.T) {
	result := MatchWithRules("Chart.yaml", DefaultRules)
	if result != "helm" {
		t.Errorf("esperado tipo 'helm' para Chart.yaml, obtido %q", result)
	}
}

// TestMatchWithRules_KustomizeDetection verifica que kustomization.yaml é detectado como kustomize.
func TestMatchWithRules_KustomizeDetection(t *testing.T) {
	result := MatchWithRules("kustomization.yaml", DefaultRules)
	if result != "kustomize" {
		t.Errorf("esperado tipo 'kustomize' para kustomization.yaml, obtido %q", result)
	}
}
