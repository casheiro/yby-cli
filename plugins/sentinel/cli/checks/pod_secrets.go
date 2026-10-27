//go:build k8s

package checks

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"k8s.io/client-go/kubernetes"
)

// ExposedSecretsCheck verifica se variáveis de ambiente contêm secrets com valores hardcoded.
type ExposedSecretsCheck struct{}

func init() { Register(&ExposedSecretsCheck{}) }

func (c *ExposedSecretsCheck) ID() string         { return "POD_EXPOSED_SECRETS" }
func (c *ExposedSecretsCheck) Name() string       { return "Secrets Expostos" }
func (c *ExposedSecretsCheck) Category() Category { return CategorySecrets }
func (c *ExposedSecretsCheck) Severity() Severity { return SeverityCritical }
func (c *ExposedSecretsCheck) Description() string {
	return "Detecta variáveis de ambiente sensíveis com valores hardcoded"
}

func (c *ExposedSecretsCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := listarPods(ctx, client, namespace)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			for _, env := range container.Env {
				if nomeSensivel(env.Name) && env.ValueFrom == nil {
					findings = append(findings, SecurityFinding{
						CheckID:        c.ID(),
						Severity:       SeverityCritical,
						Category:       CategorySecrets,
						Pod:            pod.Name,
						Container:      container.Name,
						Namespace:      namespace,
						Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
						Message:        fmt.Sprintf("Container '%s' no pod '%s' tem env '%s' com valor hardcoded (use secretKeyRef)", container.Name, pod.Name, env.Name),
						Recommendation: "Use secretKeyRef ou configMapKeyRef para referenciar valores sensíveis",
						Type:           "critical",
						Description:    fmt.Sprintf("Container '%s' no pod '%s' tem env '%s' com valor hardcoded (use secretKeyRef)", container.Name, pod.Name, env.Name),
					})
				}
			}
		}
	}
	return findings, nil
}

// nomeSensivel diz se o nome da variável de ambiente traz uma palavra de segredo
// (PASSWORD, SECRET, TOKEN, KEY). PASSWORD, SECRET e TOKEN casam como trecho (PGPASSWORD,
// JWTSECRET); KEY casa por palavra, separada por "_", "-", "." ou pela troca de minúscula
// para maiúscula: API_KEY e apiKey casam, KEYCLOAK_URL e MONKEYS não.
func nomeSensivel(nome string) bool {
	var palavras []string
	var atual []rune
	runas := []rune(nome)
	for i, r := range runas {
		eLetraOuDigito := unicode.IsLetter(r) || unicode.IsDigit(r)
		viraMaiuscula := i > 0 && unicode.IsUpper(r) && unicode.IsLower(runas[i-1])
		if !eLetraOuDigito || viraMaiuscula {
			if len(atual) > 0 {
				palavras = append(palavras, strings.ToLower(string(atual)))
				atual = nil
			}
		}
		if eLetraOuDigito {
			atual = append(atual, r)
		}
	}
	if len(atual) > 0 {
		palavras = append(palavras, strings.ToLower(string(atual)))
	}
	for _, p := range palavras {
		switch p {
		case "key", "keys", "apikey", "apikeys":
			return true
		}
	}
	// Palavras que não geram falso positivo valem como trecho: PGPASSWORD, JWTSECRET.
	minusculo := strings.ToLower(nome)
	for _, t := range []string{"password", "passwd", "secret", "token"} {
		if strings.Contains(minusculo, t) {
			return true
		}
	}
	return false
}
