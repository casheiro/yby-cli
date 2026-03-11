package cmd

import (
	"testing"

	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/stretchr/testify/assert"
)

// ========================================================
// resolveTargetDir
// ========================================================

func TestResolveTargetDir(t *testing.T) {
	tests := []struct {
		name      string
		targetDir string
		expected  string
	}{
		{
			name:      "Diretório vazio retorna ponto",
			targetDir: "",
			expected:  ".",
		},
		{
			name:      "Diretório especificado é retornado diretamente",
			targetDir: "infra",
			expected:  "infra",
		},
		{
			name:      "Caminho absoluto é mantido",
			targetDir: "/home/user/project",
			expected:  "/home/user/project",
		},
		{
			name:      "Caminho relativo com subdiretórios",
			targetDir: "src/infra/k8s",
			expected:  "src/infra/k8s",
		},
		{
			name:      "Ponto explícito é mantido",
			targetDir: ".",
			expected:  ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveTargetDir(tt.targetDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========================================================
// environmentsForTopology
// ========================================================

func TestEnvironmentsForTopology(t *testing.T) {
	tests := []struct {
		name     string
		topology string
		expected []string
	}{
		{
			name:     "Topologia single retorna apenas local",
			topology: "single",
			expected: []string{"local"},
		},
		{
			name:     "Topologia standard retorna local e prod",
			topology: "standard",
			expected: []string{"local", "prod"},
		},
		{
			name:     "Topologia complete retorna todos os ambientes",
			topology: "complete",
			expected: []string{"local", "dev", "staging", "prod"},
		},
		{
			name:     "Topologia vazia cai no default",
			topology: "",
			expected: []string{"local"},
		},
		{
			name:     "Topologia desconhecida cai no default",
			topology: "custom-topology",
			expected: []string{"local"},
		},
		{
			name:     "Topologia com letras maiúsculas não é reconhecida (case-sensitive)",
			topology: "Standard",
			expected: []string{"local"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := environmentsForTopology(tt.topology)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========================================================
// ensureLocalEnvironment
// ========================================================

func TestEnsureLocalEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		envs     []string
		expected []string
	}{
		{
			name:     "Adiciona local quando não existe",
			envs:     []string{"prod"},
			expected: []string{"local", "prod"},
		},
		{
			name:     "Não duplica local quando já existe",
			envs:     []string{"local", "prod"},
			expected: []string{"local", "prod"},
		},
		{
			name:     "Adiciona local em lista vazia",
			envs:     []string{},
			expected: []string{"local"},
		},
		{
			name:     "Local no meio da lista não duplica",
			envs:     []string{"dev", "local", "staging", "prod"},
			expected: []string{"dev", "local", "staging", "prod"},
		},
		{
			name:     "Lista com apenas local permanece igual",
			envs:     []string{"local"},
			expected: []string{"local"},
		},
		{
			name:     "Múltiplos ambientes sem local recebem local no início",
			envs:     []string{"dev", "staging", "prod"},
			expected: []string{"local", "dev", "staging", "prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensureLocalEnvironment(tt.envs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========================================================
// validateEnvironment
// ========================================================

func TestValidateEnvironment(t *testing.T) {
	tests := []struct {
		name          string
		env           string
		envs          []string
		expectedEnv   string
		expectedValid bool
	}{
		{
			name:          "Ambiente válido é aceito",
			env:           "prod",
			envs:          []string{"local", "prod"},
			expectedEnv:   "prod",
			expectedValid: true,
		},
		{
			name:          "Ambiente inválido retorna o primeiro da lista",
			env:           "dev",
			envs:          []string{"local", "prod"},
			expectedEnv:   "local",
			expectedValid: false,
		},
		{
			name:          "Lista vazia retorna o ambiente original",
			env:           "dev",
			envs:          []string{},
			expectedEnv:   "dev",
			expectedValid: false,
		},
		{
			name:          "Primeiro ambiente da lista como fallback",
			env:           "staging",
			envs:          []string{"prod"},
			expectedEnv:   "prod",
			expectedValid: false,
		},
		{
			name:          "Ambiente local é válido na topologia standard",
			env:           "local",
			envs:          []string{"local", "prod"},
			expectedEnv:   "local",
			expectedValid: true,
		},
		{
			name:          "Ambiente staging é válido na topologia complete",
			env:           "staging",
			envs:          []string{"local", "dev", "staging", "prod"},
			expectedEnv:   "staging",
			expectedValid: true,
		},
		{
			name:          "Ambiente vazio não é válido",
			env:           "",
			envs:          []string{"local", "prod"},
			expectedEnv:   "local",
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultEnv, valid := validateEnvironment(tt.env, tt.envs)
			assert.Equal(t, tt.expectedEnv, resultEnv)
			assert.Equal(t, tt.expectedValid, valid)
		})
	}
}

// ========================================================
// mapModuleSelection
// ========================================================

func TestMapModuleSelection(t *testing.T) {
	tests := []struct {
		name                  string
		selectedModules       []string
		expectedKepler        bool
		expectedMinio         bool
		expectedKEDA          bool
		expectedMetricsServer bool
	}{
		{
			name:                  "Nenhum módulo selecionado",
			selectedModules:       []string{},
			expectedKepler:        false,
			expectedMinio:         false,
			expectedKEDA:          false,
			expectedMetricsServer: false,
		},
		{
			name:                  "Todos os módulos selecionados",
			selectedModules:       []string{"Kepler (Eficiência Energética)", "MinIO (Object Storage Local)", "KEDA (Event-Driven Autoscaling)", "Observability Core (Metrics Server)"},
			expectedKepler:        true,
			expectedMinio:         true,
			expectedKEDA:          true,
			expectedMetricsServer: true,
		},
		{
			name:                  "Apenas Kepler selecionado",
			selectedModules:       []string{"Kepler (Eficiência Energética)"},
			expectedKepler:        true,
			expectedMinio:         false,
			expectedKEDA:          false,
			expectedMetricsServer: false,
		},
		{
			name:                  "Apenas MinIO selecionado",
			selectedModules:       []string{"MinIO (Object Storage Local)"},
			expectedKepler:        false,
			expectedMinio:         true,
			expectedKEDA:          false,
			expectedMetricsServer: false,
		},
		{
			name:                  "Apenas KEDA selecionado",
			selectedModules:       []string{"KEDA (Event-Driven Autoscaling)"},
			expectedKepler:        false,
			expectedMinio:         false,
			expectedKEDA:          true,
			expectedMetricsServer: false,
		},
		{
			name:                  "Apenas Observability selecionado",
			selectedModules:       []string{"Observability Core (Metrics Server)"},
			expectedKepler:        false,
			expectedMinio:         false,
			expectedKEDA:          false,
			expectedMetricsServer: true,
		},
		{
			name:                  "Combinação parcial: Kepler e KEDA",
			selectedModules:       []string{"Kepler (Eficiência Energética)", "KEDA (Event-Driven Autoscaling)"},
			expectedKepler:        true,
			expectedMinio:         false,
			expectedKEDA:          true,
			expectedMetricsServer: false,
		},
		{
			name:                  "Lista nil retorna tudo falso",
			selectedModules:       nil,
			expectedKepler:        false,
			expectedMinio:         false,
			expectedKEDA:          false,
			expectedMetricsServer: false,
		},
		{
			name:                  "String que contém substring parcial de módulo",
			selectedModules:       []string{"Algo com Kepler no nome"},
			expectedKepler:        true,
			expectedMinio:         false,
			expectedKEDA:          false,
			expectedMetricsServer: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kepler, minio, keda, metricsServer := mapModuleSelection(tt.selectedModules)
			assert.Equal(t, tt.expectedKepler, kepler, "Kepler")
			assert.Equal(t, tt.expectedMinio, minio, "MinIO")
			assert.Equal(t, tt.expectedKEDA, keda, "KEDA")
			assert.Equal(t, tt.expectedMetricsServer, metricsServer, "MetricsServer")
		})
	}
}

// ========================================================
// validateNonInteractiveFlags
// ========================================================

func TestValidateNonInteractiveFlags(t *testing.T) {
	tests := []struct {
		name            string
		ctx             *scaffold.BlueprintContext
		flags           *InitOptions
		expectedMissing []string
	}{
		{
			name: "Todos os campos preenchidos, sem flags faltando",
			ctx: &scaffold.BlueprintContext{
				Topology:        "standard",
				WorkflowPattern: "gitflow",
				GitRepoURL:      "https://github.com/org/repo.git",
				ProjectName:     "my-project",
			},
			flags:           &InitOptions{ProjectName: "my-project"},
			expectedMissing: []string{},
		},
		{
			name: "Topology vazia",
			ctx: &scaffold.BlueprintContext{
				Topology:        "",
				WorkflowPattern: "gitflow",
				GitRepoURL:      "https://github.com/org/repo.git",
			},
			flags:           &InitOptions{},
			expectedMissing: []string{"--topology"},
		},
		{
			name: "Workflow vazio",
			ctx: &scaffold.BlueprintContext{
				Topology:        "standard",
				WorkflowPattern: "",
				GitRepoURL:      "https://github.com/org/repo.git",
			},
			flags:           &InitOptions{},
			expectedMissing: []string{"--workflow"},
		},
		{
			name: "Topology e Workflow vazios",
			ctx: &scaffold.BlueprintContext{
				Topology:        "",
				WorkflowPattern: "",
				GitRepoURL:      "https://github.com/org/repo.git",
			},
			flags:           &InitOptions{},
			expectedMissing: []string{"--topology", "--workflow"},
		},
		{
			name: "Sem GitRepo e sem ProjectName (não offline)",
			ctx: &scaffold.BlueprintContext{
				Topology:        "standard",
				WorkflowPattern: "gitflow",
				GitRepoURL:      "",
				ProjectName:     "yby-project",
			},
			flags:           &InitOptions{ProjectName: "", Offline: false},
			expectedMissing: []string{"--project-name OR --git-repo"},
		},
		{
			name: "Sem GitRepo mas com ProjectName explícito",
			ctx: &scaffold.BlueprintContext{
				Topology:        "standard",
				WorkflowPattern: "gitflow",
				GitRepoURL:      "",
				ProjectName:     "meu-projeto",
			},
			flags:           &InitOptions{ProjectName: "meu-projeto"},
			expectedMissing: []string{},
		},
		{
			name: "Sem GitRepo, sem ProjectName, mas modo offline",
			ctx: &scaffold.BlueprintContext{
				Topology:        "standard",
				WorkflowPattern: "gitflow",
				GitRepoURL:      "",
				ProjectName:     "yby-project",
			},
			flags:           &InitOptions{ProjectName: "", Offline: true},
			expectedMissing: []string{},
		},
		{
			name: "Tudo faltando (pior caso)",
			ctx: &scaffold.BlueprintContext{
				Topology:        "",
				WorkflowPattern: "",
				GitRepoURL:      "",
				ProjectName:     "yby-project",
			},
			flags:           &InitOptions{ProjectName: "", Offline: false},
			expectedMissing: []string{"--topology", "--workflow", "--project-name OR --git-repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing := validateNonInteractiveFlags(tt.ctx, tt.flags)
			assert.Equal(t, tt.expectedMissing, missing)
		})
	}
}

// ========================================================
// resolveAIFilePath
// ========================================================

func TestResolveAIFilePath(t *testing.T) {
	tests := []struct {
		name      string
		filePath  string
		targetDir string
		gitRoot   string
		expected  string
	}{
		{
			name:      "Arquivo normal é colocado dentro do targetDir",
			filePath:  "config/policy.yaml",
			targetDir: "infra",
			gitRoot:   "",
			expected:  "infra/config/policy.yaml",
		},
		{
			name:      "Arquivo .github com gitRoot vai para a raiz do repo",
			filePath:  ".github/workflows/ci.yaml",
			targetDir: "infra",
			gitRoot:   "/home/user/project",
			expected:  "/home/user/project/.github/workflows/ci.yaml",
		},
		{
			name:      "Arquivo .github sem gitRoot e com targetDir diferente de ponto",
			filePath:  ".github/CODEOWNERS",
			targetDir: "infra",
			gitRoot:   "",
			expected:  ".github/CODEOWNERS",
		},
		{
			name:      "Arquivo .github sem gitRoot e com targetDir ponto",
			filePath:  ".github/workflows/deploy.yaml",
			targetDir: ".",
			gitRoot:   "",
			expected:  ".github/workflows/deploy.yaml",
		},
		{
			name:      "Arquivo normal com targetDir ponto",
			filePath:  "docs/README.md",
			targetDir: ".",
			gitRoot:   "",
			expected:  "docs/README.md",
		},
		{
			name:      "Arquivo .github com gitRoot e targetDir ponto",
			filePath:  ".github/workflows/test.yaml",
			targetDir: ".",
			gitRoot:   "/repo",
			expected:  "/repo/.github/workflows/test.yaml",
		},
		{
			name:      "Arquivo .github sem gitRoot e targetDir vazio",
			filePath:  ".github/dependabot.yml",
			targetDir: "",
			gitRoot:   "",
			expected:  ".github/dependabot.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveAIFilePath(tt.filePath, tt.targetDir, tt.gitRoot)
			assert.Equal(t, tt.expected, result)
		})
	}
}
