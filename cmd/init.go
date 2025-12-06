/*
Copyright © 2025 Yby Team

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/casheiro/yby/cli/pkg/config"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Gera a configuração do projeto e contexto local",
	Long: `Inicia um assistente (Wizard) para configurar um novo projeto Yby.
	
Este comando realiza duas ações principais:
1. Define a configuração GitOps do cluster (config/cluster-values.yaml)
2. Cria o Contexto Local seguro (.env.<env>) com seus segredos (Token GitHub)

Ao final, o contexto criado é ativado automaticamente.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🌱 Yby Smart Init - Configuração do Projeto")
		fmt.Println("-----------------------------------------")

		answers := struct {
			Domain      string
			GitRepo     string
			GitBranch   string
			Email       string
			Environment string
			Org         string
			GithubToken string
			Modules     []string
		}{}

		// 1. Ask for Git Repo first to infer Org
		repoQ := []*survey.Question{
			{
				Name: "GitRepo",
				Prompt: &survey.Input{
					Message: "URL do Repositório Git:",
					Default: "https://github.com/casheiro/yby-template",
				},
				Validate: survey.Required,
			},
		}
		survey.Ask(repoQ, &answers)

		// Infer Org from GitRepo
		defaultOrg := "casheiro"
		// Simple logic to extract org from https://github.com/ORG/REPO or git@github.com:ORG/REPO
		if answers.GitRepo != "" {
			// Very basic parsing
			// remove protocol
			clean := answers.GitRepo
			if filepath.Ext(clean) == ".git" {
				clean = clean[:len(clean)-4]
			}
			// parts := filepath.SplitList(clean) // splitlist is for paths, let's use string split
			// try to find the part before repo name
			// ex: github.com/casheiro/yby -> casheiro
			// ex: github.com/casheiro -> casheiro ?
			// let's retry with specific splitting
			// Assume standard github url structure for now
			// Finding last slash
			lastSlash := -1
			msg := clean
			for i := len(msg) - 1; i >= 0; i-- {
				if msg[i] == '/' {
					lastSlash = i
					break
				}
			}
			if lastSlash > 0 {
				// We found repo name at msg[lastSlash+1:]
				// Org is usually before that
				// Search for slash before that
				secondLastSlash := -1
				for i := lastSlash - 1; i >= 0; i-- {
					if msg[i] == '/' || msg[i] == ':' { // handle git@github.com:ORG
						secondLastSlash = i
						break
					}
				}
				if secondLastSlash >= 0 {
					defaultOrg = msg[secondLastSlash+1 : lastSlash]
				}
			}
		}

		qs := []*survey.Question{
			{
				Name: "Domain",
				Prompt: &survey.Input{
					Message: "Domínio Base do Cluster (ex: meudominio.com):",
					Default: "casheiro.com.br",
				},
				Validate: survey.Required,
			},
			{
				Name: "GitBranch",
				Prompt: &survey.Input{
					Message: "Branch Principal:",
					Default: "main",
				},
				Validate: survey.Required,
			},
			{
				Name: "Org",
				Prompt: &survey.Input{
					Message: "Organização GitHub (para Discovery):",
					Default: defaultOrg,
				},
			},
			{
				Name: "Email",
				Prompt: &survey.Input{
					Message: "Email para Let's Encrypt (TLS):",
					Default: "admin@casheiro.com.br",
				},
				Validate: survey.Required,
			},
			{
				Name: "Environment",
				Prompt: &survey.Select{
					Message: "Ambiente (Isso definirá o nome do seu Contexto Local):",
					Options: []string{"prod", "staging", "dev"},
					Default: "prod",
				},
			},
			{
				Name: "GithubToken",
				Prompt: &survey.Password{
					Message: "GitHub Token (PAT) com permissão de repo (salvo apenas localmente):",
				},
				Validate: survey.Required,
			},
			{
				Name: "Modules",
				Prompt: &survey.MultiSelect{
					Message: "Selecione os módulos a serem instalados:",
					Options: []string{
						"Observability (Prometheus/Grafana)",
						"Kepler (Métricas de Energia)",
						"MinIO (Object Storage)",
						"Headlamp (Dashboard UI)",
					},
					Default: []string{
						"Observability (Prometheus/Grafana)",
						"Kepler (Métricas de Energia)",
						"MinIO (Object Storage)",
					},
				},
			},
		}

		err := survey.Ask(qs, &answers)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		// Process modules into a map for easier template access
		modulesMap := make(map[string]bool)
		for _, m := range answers.Modules {
			if m == "Observability (Prometheus/Grafana)" {
				modulesMap["Observability"] = true
			} else if m == "Kepler (Métricas de Energia)" {
				modulesMap["Kepler"] = true
			} else if m == "MinIO (Object Storage)" {
				modulesMap["MinIO"] = true
			} else if m == "Headlamp (Dashboard UI)" {
				modulesMap["Headlamp"] = true
			}
		}

		// Create a composite data object
		data := struct {
			Domain      string
			GitRepo     string
			GitBranch   string
			Email       string
			Environment string
			Org         string
			GithubToken string
			Modules     map[string]bool
		}{
			Domain:      answers.Domain,
			GitRepo:     answers.GitRepo,
			GitBranch:   answers.GitBranch,
			Email:       answers.Email,
			Environment: answers.Environment,
			Org:         answers.Org,
			GithubToken: answers.GithubToken,
			Modules:     modulesMap,
		}

		// 2. Generate Cluster Config (Public/Shared)
		fmt.Println("\n📄 Gerando Configuração do Cluster...")
		err = generateConfig(data)
		if err != nil {
			fmt.Printf("❌ Erro ao gerar configuração: %v\n", err)
			return
		}

		err = generateRootApp(data)
		if err != nil {
			fmt.Printf("❌ Erro ao gerar root-app: %v\n", err)
			return
		}

		// 3. Generate Local Context (Private/Secret)
		fmt.Println("🔒 Gerando Contexto Local Seguro...")
		// Use the new WriteEnvFile logic

		envFileName := fmt.Sprintf(".env.%s", answers.Environment)
		envContent := fmt.Sprintf("# Contexto Local: %s\nGITHUB_TOKEN=%s\nCLUSTER_DOMAIN=%s\nGIT_REPO=%s\n",
			answers.Environment, answers.GithubToken, answers.Domain, answers.GitRepo)

		// Write .env file
		if err := os.WriteFile(envFileName, []byte(envContent), 0600); err != nil {
			fmt.Printf("❌ Erro ao criar arquivo de contexto: %v\n", err)
			return
		}

		// Create/Update .gitignore
		f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			// Check if already ignored? Naive append is fine for now
			if _, err := f.WriteString(fmt.Sprintf("\n%s\n", envFileName)); err != nil {
				fmt.Printf("⚠️  Erro ao atualizar .gitignore: %v\n", err)
			}
		}

		// 4. Activate Context
		fmt.Println("🔄 Ativando Contexto...")
		cfg, err := config.Load()
		if err != nil {
			cfg = &config.Config{}
		}
		cfg.CurrentContext = answers.Environment
		if err := cfg.Save(); err != nil {
			fmt.Printf("⚠️  Erro ao salvar estado local: %v\n", err)
		}

		fmt.Println("")
		fmt.Println("✅ Projeto Inicializado com Sucesso!")
		fmt.Printf("   1. Configuração GitOps definida em config/cluster-values.yaml\n")
		fmt.Printf("   2. Contexto '%s' criado em %s (com Token Seguro)\n", answers.Environment, envFileName)
		fmt.Printf("   3. Contexto '%s' ativado.\n", answers.Environment)
		fmt.Println("\n👉 Próximo passo: Execute 'yby bootstrap' para provisionar seu cluster.")
	},
}

const rootAppTemplate = `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: root-app
  namespace: argocd
  labels:
    project: default
    tier: infrastructure
  finalizers:
  - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  
  source:
    repoURL: {{ .GitRepo }}
    targetRevision: {{ .GitBranch }}
    path: charts/bootstrap

    helm:
      valueFiles:
      - values.yaml
      - ../../config/cluster-values.yaml
    
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
    
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
      allowEmpty: false
    syncOptions:
    - CreateNamespace=true
    - PrunePropagationPolicy=foreground
    - PruneLast=true
    
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
        
  revisionHistoryLimit: 3
`

func generateRootApp(data interface{}) error {
	// Ensure directory exists
	dir := "manifests/argocd"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}

	f, err := os.Create(filepath.Join(dir, "root-app.yaml"))
	if err != nil {
		return err
	}
	defer f.Close()

	t := template.Must(template.New("root-app").Parse(rootAppTemplate))
	return t.Execute(f, data)
}

func init() {
	rootCmd.AddCommand(initCmd)
}

const configTemplate = `# ==============================================
# Yby - Configuração Centralizada (Gerado por yby init)
# ==============================================

# Configurações Globais
global:
  environment: {{ .Environment }}
  domainBase: "{{ .Domain }}"

# Repositório Git do Cluster
git:
  repoURL: {{ .GitRepo }}
  targetRevision: {{ .GitBranch }}
  branch: {{ .GitBranch }}
  repoName: yby

# Argo CD
project: default
argocd:
  namespace: argocd
  destinationServer: https://kubernetes.default.svc
  enabled: true
  url: https://argocd.{{ .Domain }}
  server:
    insecure: true

# Descoberta Automática de Aplicações (Zero-Touch)
discovery:
  enabled: true
  scmProvider: github
  organization: {{ .Org }}
  topic: {{ .Org }}-app
  tokenSecretName: github-token
  tokenSecretKey: token

# Ingress e TLS
ingress:
  enabled: true
  installController: false # Usar Traefik do K3s
  tls:
    enabled: true
    certResolver: letsencrypt
    email: {{ .Email }}

# Argo Events (Webhooks e CI/CD)
events:
  enabled: true

# Storage (MinIO)
storage:
  minio:
    enabled: {{ if .Modules.MinIO }}true{{ else }}false{{ end }}

# Configuração do Subchart MinIO (Oficial)
minio:
  mode: standalone
  replicas: 1
  existingSecret: "minio-creds"
  ingress:
    enabled: true
    ingressClassName: traefik
    hosts:
      - minio.{{ .Domain }}
  consoleIngress:
    enabled: true
    ingressClassName: traefik
    hosts:
      - minio-console.{{ .Domain }}
  persistence:
    enabled: true
    size: 50Gi
  resources:
    requests:
      memory: 256Mi
      cpu: 100m
  buckets:
    - name: {{ .Org }}-assets
      policy: none
      purge: false

# Ecofuturismo & Observabilidade
kepler:
  enabled: {{ if .Modules.Kepler }}true{{ else }}false{{ end }}
  serviceMonitor:
    enabled: {{ if .Modules.Kepler }}true{{ else }}false{{ end }}

# Observabilidade (Prometheus)
observability:
  mode: prometheus
  prometheus:
    enabled: {{ if .Modules.Observability }}true{{ else }}false{{ end }}

# Headlamp (UI)
headlamp:
  enabled: {{ if .Modules.Headlamp }}true{{ else }}false{{ end }}

system:
  k3s:
    upgrade:
      enabled: true
    version: "v1.33.6+k3s1"
    maxPods: 300
`

func generateConfig(data interface{}) error {
	// Ensure directory exists
	configDir := "config"
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		os.MkdirAll(configDir, 0755)
	}

	f, err := os.Create(filepath.Join(configDir, "cluster-values.yaml"))
	if err != nil {
		return err
	}
	defer f.Close()

	t := template.Must(template.New("config").Parse(configTemplate))
	return t.Execute(f, data)
}
func generateState() error {
	// Deprecated in favor of direct activation in Run, but kept for compatibility if needed.
	// Actually we should remove it if no longer used.
	// The new Run implementation handles state saving directly.
	return nil
}
