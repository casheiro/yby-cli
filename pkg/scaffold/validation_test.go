package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		nome      string
		entrada   string
		esperaErr bool
		msgContem string
	}{
		{
			nome:      "vazio deve retornar erro",
			entrada:   "",
			esperaErr: true,
			msgContem: "não pode ser vazio",
		},
		{
			nome:      "nome válido simples",
			entrada:   "meu-projeto",
			esperaErr: false,
		},
		{
			nome:      "nome válido com números",
			entrada:   "app123",
			esperaErr: false,
		},
		{
			nome:      "nome válido de um caractere",
			entrada:   "a",
			esperaErr: false,
		},
		{
			nome:      "nome válido com hífen no meio",
			entrada:   "a-b",
			esperaErr: false,
		},
		{
			nome:      "com espaços deve retornar erro",
			entrada:   "meu projeto",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "com letras maiúsculas deve retornar erro",
			entrada:   "MeuProjeto",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "com caracteres especiais deve retornar erro",
			entrada:   "meu_projeto!",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "com underscore deve retornar erro",
			entrada:   "meu_projeto",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "começando com hífen deve retornar erro",
			entrada:   "-projeto",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "terminando com hífen deve retornar erro",
			entrada:   "projeto-",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "mais de 63 caracteres deve retornar erro",
			entrada:   strings.Repeat("a", 64),
			esperaErr: true,
			msgContem: "máximo 63 caracteres",
		},
		{
			nome:      "exatamente 63 caracteres é válido",
			entrada:   strings.Repeat("a", 63),
			esperaErr: false,
		},
		{
			nome:      "com ponto deve retornar erro",
			entrada:   "meu.projeto",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateProjectName(tt.entrada)
			if tt.esperaErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.msgContem)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		nome      string
		entrada   string
		esperaErr bool
		msgContem string
	}{
		{
			nome:      "vazio deve retornar erro",
			entrada:   "",
			esperaErr: true,
			msgContem: "não pode ser vazio",
		},
		{
			nome:      "domínio válido com subdomínio",
			entrada:   "app.example.com",
			esperaErr: false,
		},
		{
			nome:      "domínio válido simples",
			entrada:   "example",
			esperaErr: false,
		},
		{
			nome:      "domínio .local válido",
			entrada:   "yby.local",
			esperaErr: false,
		},
		{
			nome:      "subdomínio com múltiplos níveis",
			entrada:   "api.v2.staging.example.com",
			esperaErr: false,
		},
		{
			nome:      "com espaços deve retornar erro",
			entrada:   "meu dominio.com",
			esperaErr: true,
			msgContem: "não pode conter espaços",
		},
		{
			nome:      "not a domain com espaços",
			entrada:   "not a domain",
			esperaErr: true,
			msgContem: "não pode conter espaços",
		},
		{
			nome:      "começando com ponto deve retornar erro",
			entrada:   ".local",
			esperaErr: true,
			msgContem: "domínio inválido",
		},
		{
			nome:      "começando com hífen deve retornar erro",
			entrada:   "-example.com",
			esperaErr: true,
			msgContem: "domínio inválido",
		},
		{
			nome:      "terminando com hífen deve retornar erro",
			entrada:   "example-.com",
			esperaErr: true,
			msgContem: "domínio inválido",
		},
		{
			nome:      "domínio com hífen válido",
			entrada:   "meu-app.example.com",
			esperaErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateDomain(tt.entrada)
			if tt.esperaErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.msgContem)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		nome      string
		entrada   string
		esperaErr bool
		msgContem string
	}{
		{
			nome:      "vazio deve retornar erro",
			entrada:   "",
			esperaErr: true,
			msgContem: "não pode ser vazio",
		},
		{
			nome:      "email válido",
			entrada:   "admin@example.com",
			esperaErr: false,
		},
		{
			nome:      "email válido com subdomínio",
			entrada:   "user@mail.example.com",
			esperaErr: false,
		},
		{
			nome:      "sem @ deve retornar erro",
			entrada:   "adminexample.com",
			esperaErr: true,
			msgContem: "email inválido",
		},
		{
			nome:      "sem domínio após @ deve retornar erro",
			entrada:   "admin@",
			esperaErr: true,
			msgContem: "email inválido",
		},
		{
			nome:      "sem parte local antes de @ deve retornar erro",
			entrada:   "@example.com",
			esperaErr: true,
			msgContem: "email inválido",
		},
		{
			nome:      "com espaços deve retornar erro",
			entrada:   "admin @example.com",
			esperaErr: true,
			msgContem: "email inválido",
		},
		{
			nome:      "sem ponto no domínio deve retornar erro",
			entrada:   "admin@example",
			esperaErr: true,
			msgContem: "email inválido",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateEmail(tt.entrada)
			if tt.esperaErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.msgContem)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGitRepo(t *testing.T) {
	tests := []struct {
		nome      string
		entrada   string
		esperaErr bool
		msgContem string
	}{
		{
			nome:      "vazio é válido (opcional)",
			entrada:   "",
			esperaErr: false,
		},
		{
			nome:      "URL https válida",
			entrada:   "https://github.com/org/repo.git",
			esperaErr: false,
		},
		{
			nome:      "URL http válida",
			entrada:   "http://github.com/org/repo.git",
			esperaErr: false,
		},
		{
			nome:      "URL git@ válida",
			entrada:   "git@github.com:org/repo.git",
			esperaErr: false,
		},
		{
			nome:      "URL inválida sem protocolo",
			entrada:   "github.com/org/repo.git",
			esperaErr: true,
			msgContem: "URL do repositório Git inválida",
		},
		{
			nome:      "URL inválida texto qualquer",
			entrada:   "nao-eh-uma-url",
			esperaErr: true,
			msgContem: "URL do repositório Git inválida",
		},
		{
			nome:      "URL inválida com ftp",
			entrada:   "ftp://github.com/repo.git",
			esperaErr: true,
			msgContem: "URL do repositório Git inválida",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateGitRepo(tt.entrada)
			if tt.esperaErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.msgContem)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTopology(t *testing.T) {
	tests := []struct {
		nome      string
		entrada   string
		esperaErr bool
		msgContem string
	}{
		{
			nome:      "vazio é válido (opcional)",
			entrada:   "",
			esperaErr: false,
		},
		{
			nome:      "single é válido",
			entrada:   "single",
			esperaErr: false,
		},
		{
			nome:      "standard é válido",
			entrada:   "standard",
			esperaErr: false,
		},
		{
			nome:      "complete é válido",
			entrada:   "complete",
			esperaErr: false,
		},
		{
			nome:      "valor inválido deve retornar erro",
			entrada:   "invalido",
			esperaErr: true,
			msgContem: "topologia inválida",
		},
		{
			nome:      "maiúsculas devem retornar erro",
			entrada:   "Single",
			esperaErr: true,
			msgContem: "topologia inválida",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateTopology(tt.entrada)
			if tt.esperaErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.msgContem)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWorkflow(t *testing.T) {
	tests := []struct {
		nome      string
		entrada   string
		esperaErr bool
		msgContem string
	}{
		{
			nome:      "vazio é válido (opcional)",
			entrada:   "",
			esperaErr: false,
		},
		{
			nome:      "essential é válido",
			entrada:   "essential",
			esperaErr: false,
		},
		{
			nome:      "gitflow é válido",
			entrada:   "gitflow",
			esperaErr: false,
		},
		{
			nome:      "trunkbased é válido",
			entrada:   "trunkbased",
			esperaErr: false,
		},
		{
			nome:      "valor inválido deve retornar erro",
			entrada:   "invalido",
			esperaErr: true,
			msgContem: "workflow inválido",
		},
		{
			nome:      "maiúsculas devem retornar erro",
			entrada:   "Gitflow",
			esperaErr: true,
			msgContem: "workflow inválido",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateWorkflow(tt.entrada)
			if tt.esperaErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.msgContem)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitizeProjectName(t *testing.T) {
	tests := []struct {
		nome     string
		entrada  string
		esperado string
	}{
		{
			nome:     "com espaços converte para hífens",
			entrada:  "meu projeto legal",
			esperado: "meu-projeto-legal",
		},
		{
			nome:     "maiúsculas converte para minúsculas",
			entrada:  "MeuProjeto",
			esperado: "meuprojeto",
		},
		{
			nome:     "caracteres especiais são removidos",
			entrada:  "meu@projeto!#2024",
			esperado: "meuprojeto2024",
		},
		{
			nome:     "underscores são removidos",
			entrada:  "meu_projeto_legal",
			esperado: "meuprojetolegal",
		},
		{
			nome:     "hífens consecutivos são unificados",
			entrada:  "meu---projeto",
			esperado: "meu-projeto",
		},
		{
			nome:     "hífens no início e fim são removidos",
			entrada:  "-meu-projeto-",
			esperado: "meu-projeto",
		},
		{
			nome:     "nome muito longo é truncado a 63 caracteres",
			entrada:  strings.Repeat("a", 100),
			esperado: strings.Repeat("a", 63),
		},
		{
			nome:     "truncamento remove hífen final resultante",
			entrada:  strings.Repeat("a", 62) + "-" + strings.Repeat("b", 5),
			esperado: strings.Repeat("a", 62),
		},
		{
			nome:     "vazio retorna valor padrão",
			entrada:  "",
			esperado: "yby-project",
		},
		{
			nome:     "somente caracteres especiais retorna valor padrão",
			entrada:  "!@#$%",
			esperado: "yby-project",
		},
		{
			nome:     "espaços e maiúsculas combinados",
			entrada:  "Meu Projeto Legal",
			esperado: "meu-projeto-legal",
		},
		{
			nome:     "caracteres acentuados são removidos",
			entrada:  "projeção",
			esperado: "projeo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			resultado := SanitizeProjectName(tt.entrada)
			assert.Equal(t, tt.esperado, resultado)
		})
	}
}

func TestValidateContext(t *testing.T) {
	tests := []struct {
		nome      string
		ctx       *BlueprintContext
		esperaErr bool
		msgContem string
	}{
		{
			nome: "contexto válido completo",
			ctx: &BlueprintContext{
				ProjectName:     "meu-projeto",
				Domain:          "app.example.com",
				Email:           "admin@example.com",
				GitRepoURL:      "https://github.com/org/repo.git",
				Topology:        "standard",
				WorkflowPattern: "gitflow",
			},
			esperaErr: false,
		},
		{
			nome:      "contexto vazio é válido (campos opcionais)",
			ctx:       &BlueprintContext{},
			esperaErr: false,
		},
		{
			nome: "contexto com apenas git repo vazio é válido",
			ctx: &BlueprintContext{
				ProjectName: "app",
				Domain:      "example.com",
				Email:       "a@b.com",
			},
			esperaErr: false,
		},
		{
			nome: "nome do projeto inválido retorna erro",
			ctx: &BlueprintContext{
				ProjectName: "INVALIDO!",
			},
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome: "domínio inválido retorna erro",
			ctx: &BlueprintContext{
				ProjectName: "app",
				Domain:      ".invalido",
			},
			esperaErr: true,
			msgContem: "domínio inválido",
		},
		{
			nome: "email inválido retorna erro",
			ctx: &BlueprintContext{
				ProjectName: "app",
				Domain:      "example.com",
				Email:       "sem-arroba",
			},
			esperaErr: true,
			msgContem: "email inválido",
		},
		{
			nome: "git repo inválido retorna erro",
			ctx: &BlueprintContext{
				GitRepoURL: "nao-eh-url",
			},
			esperaErr: true,
			msgContem: "URL do repositório Git inválida",
		},
		{
			nome: "topologia inválida retorna erro",
			ctx: &BlueprintContext{
				Topology: "invalida",
			},
			esperaErr: true,
			msgContem: "topologia inválida",
		},
		{
			nome: "workflow inválido retorna erro",
			ctx: &BlueprintContext{
				WorkflowPattern: "invalido",
			},
			esperaErr: true,
			msgContem: "workflow inválido",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateContext(tt.ctx)
			if tt.esperaErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.msgContem)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// ValidateSecretsStrategy
// ═══════════════════════════════════════════════════════════════════════════════

func TestValidateSecretsStrategy(t *testing.T) {
	tests := []struct {
		nome     string
		strategy string
		esperaOK bool
	}{
		{"vazio é válido", "", true},
		{"sealed-secrets válido", "sealed-secrets", true},
		{"external-secrets válido", "external-secrets", true},
		{"sops válido", "sops", true},
		{"inválido", "vault", false},
		{"inválido com espaço", "sealed secrets", false},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateSecretsStrategy(tt.strategy)
			if tt.esperaOK {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "estratégia de secrets inválida")
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// ValidateNoYAMLInjection
// ═══════════════════════════════════════════════════════════════════════════════

func TestValidateNoYAMLInjection(t *testing.T) {
	tests := []struct {
		nome      string
		valor     string
		esperaErr bool
		msgContem string
	}{
		{
			nome:      "valor simples é válido",
			valor:     "meu-projeto",
			esperaErr: false,
		},
		{
			nome:      "valor com espaço é válido",
			valor:     "meu projeto",
			esperaErr: false,
		},
		{
			nome:      "valor com newline deve retornar erro",
			valor:     "valor\nmalicioso",
			esperaErr: true,
			msgContem: "caracteres de controle",
		},
		{
			nome:      "valor com carriage return deve retornar erro",
			valor:     "valor\rmalicioso",
			esperaErr: true,
			msgContem: "caracteres de controle",
		},
		{
			nome:      "valor com null byte deve retornar erro",
			valor:     "valor\x00malicioso",
			esperaErr: true,
			msgContem: "caracteres de controle",
		},
		{
			nome:      "valor com caractere de controle deve retornar erro",
			valor:     "valor\x01malicioso",
			esperaErr: true,
			msgContem: "caracteres de controle",
		},
		{
			nome:      "indicador de bloco pipe deve retornar erro",
			valor:     "| echo malicioso",
			esperaErr: true,
			msgContem: "indicador de bloco YAML",
		},
		{
			nome:      "indicador de bloco > deve retornar erro",
			valor:     "> texto multiline",
			esperaErr: true,
			msgContem: "indicador de bloco YAML",
		},
		{
			nome:      "indicador de bloco com espaço antes deve retornar erro",
			valor:     "  | echo",
			esperaErr: true,
			msgContem: "indicador de bloco YAML",
		},
		{
			nome:      "valor com dois pontos é válido (não é controle)",
			valor:     "chave:valor",
			esperaErr: false,
		},
		{
			nome:      "valor com hash é válido (não é controle)",
			valor:     "valor # comentário",
			esperaErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateNoYAMLInjection(tt.valor, "campo")
			if tt.esperaErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.msgContem)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateContext_YAMLInjection(t *testing.T) {
	tests := []struct {
		nome      string
		ctx       *BlueprintContext
		esperaErr bool
		msgContem string
	}{
		{
			nome: "ProjectName com newline deve retornar erro",
			ctx: &BlueprintContext{
				ProjectName: "app\nmalicioso",
			},
			esperaErr: true,
			msgContem: "caracteres de controle",
		},
		{
			nome: "Domain com indicador de bloco deve retornar erro",
			ctx: &BlueprintContext{
				Domain: "| echo hack",
			},
			esperaErr: true,
			msgContem: "indicador de bloco YAML",
		},
		{
			nome: "Email com carriage return deve retornar erro",
			ctx: &BlueprintContext{
				Email: "user\r@example.com",
			},
			esperaErr: true,
			msgContem: "caracteres de controle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateContext(tt.ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.msgContem)
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// ValidateEnvironmentName
// ═══════════════════════════════════════════════════════════════════════════════

func TestValidateEnvironmentName(t *testing.T) {
	tests := []struct {
		nome      string
		entrada   string
		esperaErr bool
		msgContem string
	}{
		{
			nome:      "vazio deve retornar erro",
			entrada:   "",
			esperaErr: true,
			msgContem: "não pode ser vazio",
		},
		{
			nome:      "nome válido simples",
			entrada:   "dev",
			esperaErr: false,
		},
		{
			nome:      "nome válido com hífen",
			entrada:   "hom-01",
			esperaErr: false,
		},
		{
			nome:      "nome válido longo",
			entrada:   "qa",
			esperaErr: false,
		},
		{
			nome:      "nome válido uat",
			entrada:   "uat",
			esperaErr: false,
		},
		{
			nome:      "maiúsculas deve retornar erro",
			entrada:   "DEV",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "com espaço deve retornar erro",
			entrada:   "dev env",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "com underscore deve retornar erro",
			entrada:   "dev_env",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "começando com hífen deve retornar erro",
			entrada:   "-dev",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "terminando com hífen deve retornar erro",
			entrada:   "dev-",
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "mais de 63 caracteres deve retornar erro",
			entrada:   strings.Repeat("a", 64),
			esperaErr: true,
			msgContem: "no máximo 63 caracteres",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateEnvironmentName(tt.entrada)
			if tt.esperaErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.msgContem)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// ValidateEnvironmentNames
// ═══════════════════════════════════════════════════════════════════════════════

func TestValidateEnvironmentNames(t *testing.T) {
	tests := []struct {
		nome      string
		entrada   []string
		esperaErr bool
		msgContem string
	}{
		{
			nome:      "lista válida",
			entrada:   []string{"local", "dev", "hom", "prod"},
			esperaErr: false,
		},
		{
			nome:      "lista vazia deve retornar erro",
			entrada:   []string{},
			esperaErr: true,
			msgContem: "não pode ser vazia",
		},
		{
			nome:      "nome inválido na lista deve retornar erro",
			entrada:   []string{"local", "DEV"},
			esperaErr: true,
			msgContem: "RFC 1123",
		},
		{
			nome:      "duplicata deve retornar erro",
			entrada:   []string{"local", "dev", "local"},
			esperaErr: true,
			msgContem: "duplicado",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			err := ValidateEnvironmentNames(tt.entrada)
			if tt.esperaErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.msgContem)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
