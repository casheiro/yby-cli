package scaffold

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
)

var (
	// RFC 1123 label: lowercase, alfanumérico, hífens, max 63 chars
	rfc1123Regex = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?$`)

	// Domínio válido: sem espaços, com pelo menos um ponto ou ".local"
	domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$`)

	// Email básico: contém @
	emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

	// Git URL: https:// ou git@
	gitURLRegex = regexp.MustCompile(`^(https?://|git@)`)

	validTopologies = map[string]bool{
		"single":   true,
		"standard": true,
		"complete": true,
	}

	validWorkflows = map[string]bool{
		"essential":  true,
		"gitflow":    true,
		"trunkbased": true,
	}

	validSecretsStrategies = map[string]bool{
		"sealed-secrets":   true,
		"external-secrets": true,
		"sops":             true,
	}
)

// ValidateProjectName valida o nome do projeto conforme RFC 1123.
func ValidateProjectName(name string) error {
	if name == "" {
		return errors.New(errors.ErrCodeValidation, "nome do projeto não pode ser vazio")
	}
	if len(name) > 63 {
		return errors.New(errors.ErrCodeValidation,
			fmt.Sprintf("nome do projeto deve ter no máximo 63 caracteres, recebeu %d", len(name)))
	}
	if !rfc1123Regex.MatchString(name) {
		return errors.New(errors.ErrCodeValidation,
			"nome do projeto deve seguir RFC 1123: apenas letras minúsculas, números e hífens, começando e terminando com alfanumérico")
	}
	return nil
}

// ValidateDomain valida o domínio.
func ValidateDomain(domain string) error {
	if domain == "" {
		return errors.New(errors.ErrCodeValidation, "domínio não pode ser vazio")
	}
	if strings.Contains(domain, " ") {
		return errors.New(errors.ErrCodeValidation, "domínio não pode conter espaços")
	}
	if !domainRegex.MatchString(domain) {
		return errors.New(errors.ErrCodeValidation,
			"domínio inválido: deve ser um domínio válido (ex: app.example.com, yby.local)")
	}
	return nil
}

// ValidateEmail valida o formato básico de email.
func ValidateEmail(email string) error {
	if email == "" {
		return errors.New(errors.ErrCodeValidation, "email não pode ser vazio")
	}
	if !emailRegex.MatchString(email) {
		return errors.New(errors.ErrCodeValidation,
			"email inválido: deve conter @ e domínio (ex: admin@example.com)")
	}
	return nil
}

// ValidateGitRepo valida a URL do repositório Git.
func ValidateGitRepo(repo string) error {
	if repo == "" {
		return nil // git-repo é opcional
	}
	if !gitURLRegex.MatchString(repo) {
		return errors.New(errors.ErrCodeValidation,
			"URL do repositório Git inválida: deve começar com https:// ou git@ (ex: https://github.com/org/repo.git)")
	}
	return nil
}

// ValidateTopology valida o valor da topologia.
func ValidateTopology(topology string) error {
	if topology == "" {
		return nil // pode ser vazio se interativo
	}
	if !validTopologies[topology] {
		return errors.New(errors.ErrCodeValidation,
			fmt.Sprintf("topologia inválida: '%s'. Valores válidos: single, standard, complete", topology))
	}
	return nil
}

// ValidateWorkflow valida o valor do workflow.
func ValidateWorkflow(workflow string) error {
	if workflow == "" {
		return nil // pode ser vazio se interativo
	}
	if !validWorkflows[workflow] {
		return errors.New(errors.ErrCodeValidation,
			fmt.Sprintf("workflow inválido: '%s'. Valores válidos: essential, gitflow, trunkbased", workflow))
	}
	return nil
}

// SanitizeProjectName converte um nome de projeto para formato RFC 1123.
func SanitizeProjectName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	// Remove caracteres inválidos
	reg := regexp.MustCompile(`[^a-z0-9\-]`)
	name = reg.ReplaceAllString(name, "")
	// Remove hífens consecutivos
	reg = regexp.MustCompile(`-+`)
	name = reg.ReplaceAllString(name, "-")
	// Remove hífens no início e fim
	name = strings.Trim(name, "-")
	// Truncar a 63 caracteres
	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "-")
	}
	if name == "" {
		return "yby-project"
	}
	return name
}

// ValidateSecretsStrategy valida o valor da estratégia de secrets.
func ValidateSecretsStrategy(strategy string) error {
	if strategy == "" {
		return nil
	}
	if !validSecretsStrategies[strategy] {
		return errors.New(errors.ErrCodeValidation,
			fmt.Sprintf("estratégia de secrets inválida: '%s'. Valores válidos: sealed-secrets, external-secrets, sops", strategy))
	}
	return nil
}

// ValidateEnvironmentName valida o nome de um ambiente conforme RFC 1123.
func ValidateEnvironmentName(name string) error {
	if name == "" {
		return errors.New(errors.ErrCodeValidation, "nome do ambiente não pode ser vazio")
	}
	if len(name) > 63 {
		return errors.New(errors.ErrCodeValidation,
			fmt.Sprintf("nome do ambiente deve ter no máximo 63 caracteres, recebeu %d", len(name)))
	}
	if !rfc1123Regex.MatchString(name) {
		return errors.New(errors.ErrCodeValidation,
			fmt.Sprintf("nome do ambiente '%s' deve seguir RFC 1123: apenas letras minúsculas, números e hífens, começando e terminando com alfanumérico", name))
	}
	return nil
}

// ValidateEnvironmentNames valida uma lista de nomes de ambientes.
func ValidateEnvironmentNames(names []string) error {
	if len(names) == 0 {
		return errors.New(errors.ErrCodeValidation, "lista de ambientes não pode ser vazia")
	}
	seen := make(map[string]bool)
	for _, name := range names {
		if err := ValidateEnvironmentName(name); err != nil {
			return err
		}
		if seen[name] {
			return errors.New(errors.ErrCodeValidation,
				fmt.Sprintf("nome do ambiente '%s' está duplicado", name))
		}
		seen[name] = true
	}
	return nil
}

// ValidateNoYAMLInjection rejeita valores que contêm caracteres perigosos para YAML injection.
// Bloqueia newlines, caracteres de controle e sequências multiline.
func ValidateNoYAMLInjection(value, fieldName string) error {
	for _, r := range value {
		if r == '\n' || r == '\r' || r == '\x00' || (r < 0x20 && r != ' ' && r != '\t') {
			return errors.New(errors.ErrCodeValidation,
				fmt.Sprintf("%s contém caracteres de controle não permitidos (possível YAML injection)", fieldName))
		}
	}

	// Rejeitar sequências multiline YAML (indicadores de bloco no início)
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 0 && (trimmed[0] == '|' || trimmed[0] == '>') {
		return errors.New(errors.ErrCodeValidation,
			fmt.Sprintf("%s contém indicador de bloco YAML não permitido (possível YAML injection)", fieldName))
	}

	return nil
}

// ValidateContext valida todos os campos do BlueprintContext.
// Retorna o primeiro erro encontrado.
func ValidateContext(ctx *BlueprintContext) error {
	// Validação contra YAML injection nos campos de entrada do usuário
	for _, field := range []struct {
		valor string
		nome  string
	}{
		{ctx.ProjectName, "nome do projeto"},
		{ctx.Domain, "domínio"},
		{ctx.Email, "email"},
	} {
		if field.valor != "" {
			if err := ValidateNoYAMLInjection(field.valor, field.nome); err != nil {
				return err
			}
		}
	}

	if ctx.ProjectName != "" {
		if err := ValidateProjectName(ctx.ProjectName); err != nil {
			return err
		}
	}
	if ctx.Domain != "" {
		if err := ValidateDomain(ctx.Domain); err != nil {
			return err
		}
	}
	if ctx.Email != "" {
		if err := ValidateEmail(ctx.Email); err != nil {
			return err
		}
	}
	if err := ValidateGitRepo(ctx.GitRepoURL); err != nil {
		return err
	}
	if err := ValidateTopology(ctx.Topology); err != nil {
		return err
	}
	if err := ValidateWorkflow(ctx.WorkflowPattern); err != nil {
		return err
	}
	if err := ValidateSecretsStrategy(ctx.SecretsStrategy); err != nil {
		return err
	}
	return nil
}
