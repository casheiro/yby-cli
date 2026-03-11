package scaffold

// EnvironmentOverrides contém os valores específicos por ambiente para geração de values files.
type EnvironmentOverrides struct {
	Environment string
	TLSIssuer   string
	Insecure    bool
	Retention   string
	Resources   ResourceOverrides
}

// ResourceOverrides contém overrides de recursos por ambiente.
type ResourceOverrides struct {
	PrometheusMemory string
}

// GetEnvironmentOverrides retorna os overrides específicos para um dado ambiente.
func GetEnvironmentOverrides(env string) EnvironmentOverrides {
	switch env {
	case "local":
		return EnvironmentOverrides{
			Environment: "local",
			TLSIssuer:   "letsencrypt-staging",
			Insecure:    true,
			Retention:   "2d",
			Resources: ResourceOverrides{
				PrometheusMemory: "256Mi",
			},
		}
	case "dev":
		return EnvironmentOverrides{
			Environment: "dev",
			TLSIssuer:   "letsencrypt-staging",
			Insecure:    true,
			Retention:   "5d",
			Resources: ResourceOverrides{
				PrometheusMemory: "400Mi",
			},
		}
	case "staging":
		return EnvironmentOverrides{
			Environment: "staging",
			TLSIssuer:   "letsencrypt-prod",
			Insecure:    false,
			Retention:   "5d",
			Resources: ResourceOverrides{
				PrometheusMemory: "512Mi",
			},
		}
	case "prod":
		return EnvironmentOverrides{
			Environment: "prod",
			TLSIssuer:   "letsencrypt-prod",
			Insecure:    false,
			Retention:   "15d",
			Resources: ResourceOverrides{
				PrometheusMemory: "1Gi",
			},
		}
	default:
		return EnvironmentOverrides{
			Environment: env,
			TLSIssuer:   "letsencrypt-staging",
			Insecure:    true,
			Retention:   "5d",
			Resources: ResourceOverrides{
				PrometheusMemory: "400Mi",
			},
		}
	}
}

// RenderEnvironmentValues gera o conteúdo YAML de um values file para um ambiente específico.
func RenderEnvironmentValues(ctx *BlueprintContext, env string) string {
	ov := GetEnvironmentOverrides(env)

	insecureStr := "false"
	if ov.Insecure {
		insecureStr = "true"
	}

	extraArgs := ""
	if ov.Insecure {
		extraArgs = `  extraArgs:
    - --insecure`
	}

	return `# ==============================================
# Yby - Configuração do Ambiente: ` + ov.Environment + `
# Gerado automaticamente pelo yby init
# ==============================================

server:
  service:
    type: ClusterIP
  insecure: ` + insecureStr + `
` + extraArgs + `

global:
  environment: ` + ov.Environment + `
  domainBase: "` + ctx.Domain + `"

git:
  repoURL: ` + ctx.GitRepo + `
  branch: ` + ctx.GitBranch + `
  repoName: ` + ctx.ProjectName + `

discovery:
  enabled: ` + boolStr(ctx.GithubDiscovery) + `
  organization: "` + ctx.GithubOrg + `"
  topic: ` + ctx.ProjectName + `-app
  tokenSecretName: github-token

ingress:
  enabled: true
  tls:
    enabled: true
    email: ` + ctx.Email + `
    issuer: ` + ov.TLSIssuer + `

observability:
  mode: prometheus
  prometheus:
    retention: "` + ov.Retention + `"
    resources:
      requests:
        memory: ` + ov.Resources.PrometheusMemory + `

kepler:
  enabled: ` + boolStr(ctx.EnableKepler) + `

keda:
  enabled: ` + boolStr(ctx.EnableKEDA) + `

storage:
  minio:
    enabled: ` + boolStr(ctx.EnableMinio) + `
`
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
