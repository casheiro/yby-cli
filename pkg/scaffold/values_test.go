package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- TestGetEnvironmentOverrides ---

func TestGetEnvironmentOverrides(t *testing.T) {
	tests := []struct {
		nome            string
		env             string
		wantEnvironment string
		wantTLSIssuer   string
		wantInsecure    bool
		wantRetention   string
		wantPromMemory  string
	}{
		{
			nome:            "ambiente local retorna valores de desenvolvimento leve",
			env:             "local",
			wantEnvironment: "local",
			wantTLSIssuer:   "letsencrypt-staging",
			wantInsecure:    true,
			wantRetention:   "2d",
			wantPromMemory:  "256Mi",
		},
		{
			nome:            "ambiente dev retorna valores intermediarios",
			env:             "dev",
			wantEnvironment: "dev",
			wantTLSIssuer:   "letsencrypt-staging",
			wantInsecure:    true,
			wantRetention:   "5d",
			wantPromMemory:  "400Mi",
		},
		{
			nome:            "ambiente staging retorna valores de producao com retencao menor",
			env:             "staging",
			wantEnvironment: "staging",
			wantTLSIssuer:   "letsencrypt-prod",
			wantInsecure:    false,
			wantRetention:   "5d",
			wantPromMemory:  "512Mi",
		},
		{
			nome:            "ambiente prod retorna valores de producao completos",
			env:             "prod",
			wantEnvironment: "prod",
			wantTLSIssuer:   "letsencrypt-prod",
			wantInsecure:    false,
			wantRetention:   "15d",
			wantPromMemory:  "1Gi",
		},
		{
			nome:            "ambiente desconhecido usa defaults de dev",
			env:             "qualquer-coisa",
			wantEnvironment: "qualquer-coisa",
			wantTLSIssuer:   "letsencrypt-staging",
			wantInsecure:    true,
			wantRetention:   "5d",
			wantPromMemory:  "400Mi",
		},
		{
			nome:            "string vazia usa defaults de dev",
			env:             "",
			wantEnvironment: "",
			wantTLSIssuer:   "letsencrypt-staging",
			wantInsecure:    true,
			wantRetention:   "5d",
			wantPromMemory:  "400Mi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			ov := GetEnvironmentOverrides(tt.env)

			assert.Equal(t, tt.wantEnvironment, ov.Environment, "Environment incorreto")
			assert.Equal(t, tt.wantTLSIssuer, ov.TLSIssuer, "TLSIssuer incorreto")
			assert.Equal(t, tt.wantInsecure, ov.Insecure, "Insecure incorreto")
			assert.Equal(t, tt.wantRetention, ov.Retention, "Retention incorreto")
			assert.Equal(t, tt.wantPromMemory, ov.Resources.PrometheusMemory, "PrometheusMemory incorreto")
		})
	}
}

// --- TestBoolStr ---

func TestBoolStr(t *testing.T) {
	assert.Equal(t, "true", boolStr(true), "boolStr(true) deveria retornar 'true'")
	assert.Equal(t, "false", boolStr(false), "boolStr(false) deveria retornar 'false'")
}

// --- TestRenderEnvironmentValues ---

func TestRenderEnvironmentValues(t *testing.T) {
	// Contexto completo usado em vários sub-testes
	ctxCompleto := &BlueprintContext{
		ProjectName:     "meu-projeto",
		Domain:          "meu.dominio.com",
		Email:           "admin@meu.dominio.com",
		GitRepo:         "https://github.com/org/meu-projeto",
		GitBranch:       "main",
		GithubDiscovery: true,
		GithubOrg:       "minha-org",
		EnableKEDA:      true,
		EnableKepler:    true,
		EnableMinio:     false,
	}

	t.Run("ambiente prod NAO contem --insecure", func(t *testing.T) {
		resultado := RenderEnvironmentValues(ctxCompleto, "prod")

		assert.NotContains(t, resultado, "--insecure",
			"Ambiente prod nao deve conter flag --insecure")
		assert.Contains(t, resultado, "insecure: false",
			"Ambiente prod deve ter insecure: false")
	})

	t.Run("ambiente local contem insecure true e extraArgs", func(t *testing.T) {
		resultado := RenderEnvironmentValues(ctxCompleto, "local")

		assert.Contains(t, resultado, "insecure: true",
			"Ambiente local deve ter insecure: true")
		assert.Contains(t, resultado, "--insecure",
			"Ambiente local deve ter extraArgs com --insecure")
	})

	t.Run("environment correto aparece no YAML", func(t *testing.T) {
		ambientes := []string{"local", "dev", "staging", "prod"}
		for _, env := range ambientes {
			resultado := RenderEnvironmentValues(ctxCompleto, env)
			assert.Contains(t, resultado, "environment: "+env,
				"YAML deve conter o environment: %s", env)
			assert.Contains(t, resultado, "Ambiente: "+env,
				"Cabecalho do YAML deve indicar o ambiente: %s", env)
		}
	})

	t.Run("valores do ctx sao propagados no YAML", func(t *testing.T) {
		resultado := RenderEnvironmentValues(ctxCompleto, "dev")

		assert.Contains(t, resultado, ctxCompleto.Domain,
			"YAML deve conter o dominio do contexto")
		assert.Contains(t, resultado, ctxCompleto.GitRepo,
			"YAML deve conter a URL do repositorio")
		assert.Contains(t, resultado, ctxCompleto.GitBranch,
			"YAML deve conter o branch do git")
		assert.Contains(t, resultado, ctxCompleto.ProjectName,
			"YAML deve conter o nome do projeto")
		assert.Contains(t, resultado, ctxCompleto.Email,
			"YAML deve conter o email")
		assert.Contains(t, resultado, ctxCompleto.GithubOrg,
			"YAML deve conter a organizacao do GitHub")
	})

	t.Run("feature flags sao renderizadas corretamente", func(t *testing.T) {
		resultado := RenderEnvironmentValues(ctxCompleto, "dev")

		// EnableKEDA = true
		assert.Contains(t, resultado, "enabled: true",
			"YAML deve conter features habilitadas")

		// Verifica que as secoes de keda, kepler e minio existem
		assert.Contains(t, resultado, "keda:", "YAML deve conter secao keda")
		assert.Contains(t, resultado, "kepler:", "YAML deve conter secao kepler")
		assert.Contains(t, resultado, "minio:", "YAML deve conter secao minio")
	})

	t.Run("ctx com todas features desabilitadas", func(t *testing.T) {
		ctxMinimo := &BlueprintContext{
			ProjectName:     "simples",
			Domain:          "simples.local",
			Email:           "a@b.com",
			GitRepo:         "https://github.com/org/simples",
			GitBranch:       "develop",
			GithubDiscovery: false,
			GithubOrg:       "",
			EnableKEDA:      false,
			EnableKepler:    false,
			EnableMinio:     false,
		}
		resultado := RenderEnvironmentValues(ctxMinimo, "local")

		// Todas as features devem estar como false
		// keda, kepler e minio devem ter enabled: false
		linhas := strings.Split(resultado, "\n")
		kedaIdx := -1
		keplerIdx := -1
		minioIdx := -1
		for i, l := range linhas {
			trimmed := strings.TrimSpace(l)
			if trimmed == "keda:" {
				kedaIdx = i
			}
			if trimmed == "kepler:" {
				keplerIdx = i
			}
			if trimmed == "minio:" {
				minioIdx = i
			}
		}

		require.NotEqual(t, -1, kedaIdx, "Secao keda nao encontrada")
		require.NotEqual(t, -1, keplerIdx, "Secao kepler nao encontrada")
		require.NotEqual(t, -1, minioIdx, "Secao minio nao encontrada")

		// A linha seguinte de cada secao deve conter "enabled: false"
		assert.Contains(t, linhas[kedaIdx+1], "enabled: false",
			"KEDA deve estar desabilitado")
		assert.Contains(t, linhas[keplerIdx+1], "enabled: false",
			"Kepler deve estar desabilitado")
		assert.Contains(t, linhas[minioIdx+1], "enabled: false",
			"Minio deve estar desabilitado")

		// Discovery deve estar false
		assert.Contains(t, resultado, "enabled: false",
			"Discovery deve estar desabilitada quando GithubDiscovery=false")
	})

	t.Run("YAML contem secoes obrigatorias", func(t *testing.T) {
		resultado := RenderEnvironmentValues(ctxCompleto, "staging")

		secoesObrigatorias := []string{
			"server:", "global:", "git:", "discovery:",
			"ingress:", "observability:", "kepler:", "keda:", "storage:",
		}
		for _, secao := range secoesObrigatorias {
			assert.Contains(t, resultado, secao,
				"YAML deve conter a secao %s", secao)
		}
	})

	t.Run("valores de TLS issuer variam por ambiente", func(t *testing.T) {
		localYAML := RenderEnvironmentValues(ctxCompleto, "local")
		prodYAML := RenderEnvironmentValues(ctxCompleto, "prod")

		assert.Contains(t, localYAML, "letsencrypt-staging",
			"Local deve usar letsencrypt-staging")
		assert.Contains(t, prodYAML, "letsencrypt-prod",
			"Prod deve usar letsencrypt-prod")
	})

	t.Run("retencao e memoria do prometheus variam por ambiente", func(t *testing.T) {
		localYAML := RenderEnvironmentValues(ctxCompleto, "local")
		prodYAML := RenderEnvironmentValues(ctxCompleto, "prod")

		assert.Contains(t, localYAML, `retention: "2d"`,
			"Local deve ter retencao de 2d")
		assert.Contains(t, prodYAML, `retention: "15d"`,
			"Prod deve ter retencao de 15d")
		assert.Contains(t, localYAML, "memory: 256Mi",
			"Local deve ter 256Mi de memoria para Prometheus")
		assert.Contains(t, prodYAML, "memory: 1Gi",
			"Prod deve ter 1Gi de memoria para Prometheus")
	})
}

// --- Testes de diferenciacao entre ambientes ---

func TestAmbientesStagingProdDiferemDeLocal(t *testing.T) {
	local := GetEnvironmentOverrides("local")
	staging := GetEnvironmentOverrides("staging")
	prod := GetEnvironmentOverrides("prod")

	t.Run("staging difere de local em TLSIssuer", func(t *testing.T) {
		assert.NotEqual(t, local.TLSIssuer, staging.TLSIssuer,
			"Staging deve usar TLS issuer diferente de local")
	})

	t.Run("staging difere de local em Insecure", func(t *testing.T) {
		assert.NotEqual(t, local.Insecure, staging.Insecure,
			"Staging nao deve ser insecure como local")
	})

	t.Run("prod difere de local em retention", func(t *testing.T) {
		assert.NotEqual(t, local.Retention, prod.Retention,
			"Prod deve ter retencao diferente de local")
	})

	t.Run("prod difere de local em PrometheusMemory", func(t *testing.T) {
		assert.NotEqual(t, local.Resources.PrometheusMemory, prod.Resources.PrometheusMemory,
			"Prod deve ter memoria do Prometheus diferente de local")
	})

	t.Run("staging difere de local em PrometheusMemory", func(t *testing.T) {
		assert.NotEqual(t, local.Resources.PrometheusMemory, staging.Resources.PrometheusMemory,
			"Staging deve ter memoria do Prometheus diferente de local")
	})

	t.Run("prod difere de staging em retention", func(t *testing.T) {
		assert.NotEqual(t, staging.Retention, prod.Retention,
			"Prod deve ter retencao diferente de staging")
	})
}
