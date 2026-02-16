/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// accessCmd represents the access command
var accessCmd = &cobra.Command{
	Use:   "access",
	Short: "Abre túneis de acesso aos serviços do cluster",
	Long: `Estabelece conexões seguras (port-forward) para os serviços disponíveis:
- Argo CD
- MinIO (se detectado)
- Prometheus (para alimentar Grafana)
- Grafana Local (via Docker)
- Headlamp (Token)

Você pode especificar um contexto (local/prod) com --context.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 Iniciando Acesso Unificado ao Cluster...")

		// Setup context with cancellation
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Setup signal handling for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		targetContext, _ := cmd.Flags().GetString("context")
		if targetContext == "" {
			var err error
			targetContext, err = getKubectlContext()
			if err != nil {
				fmt.Printf("❌ Erro ao detectar contexto atual: %v\n", err)
				return
			}
			fmt.Printf("📍 Contexto: %s (detectado automaticamente)\n", targetContext)
		} else {
			fmt.Printf("📍 Contexto: %s (definido via flag)\n", targetContext)
		}

		// Create errgroup for managing goroutines
		g, gctx := errgroup.WithContext(ctx)

		// 1. Argo CD (Default)
		argoPwd, err := getArgoPassword(targetContext)
		if err != nil {
			fmt.Printf("⚠️  Argo CD: Não foi possível obter senha (talvez não instalado no namespace 'argocd'?): %v\n", err)
		} else {
			fmt.Println("🔌 Conectando Argo CD...")
			killPortForward("8085")
			g.Go(func() error {
				return runPortForwardWithContext(gctx, targetContext, "argocd", "svc/argocd-server", "8085:80")
			})
			fmt.Printf("   -> Argo CD: http://localhost:8085 (admin / %s)\n", argoPwd)
		}

		// 2. MinIO (Dynamic)
		minioSvc, minioNs := findMinioService(targetContext)
		if minioSvc != "" {
			fmt.Printf("🔌 Detectado MinIO (%s/%s)! Conectando...\n", minioNs, minioSvc)
			killPortForward("9000")
			killPortForward("9001")
			g.Go(func() error {
				return runPortForwardWithContext(gctx, targetContext, minioNs, "svc/"+minioSvc, "9000:9000")
			})
			g.Go(func() error {
				return runPortForwardWithContext(gctx, targetContext, minioNs, "svc/"+minioSvc, "9001:9001")
			})

			// Try to get creds (check default candidates first)
			user, pass := getSecretKeys(targetContext, "storage", "minio-secret", "rootUser", "rootPassword")
			if user == "" {
				user, pass = getSecretKeys(targetContext, "default", "minio-creds", "rootUser", "rootPassword")
			}

			// Fallbacks for display
			if user == "" {
				user = "admin (verifique secrets)"
			}
			if pass == "" {
				pass = "***"
			}

			fmt.Printf("   -> MinIO API: http://localhost:9000\n")
			fmt.Printf("   -> MinIO Console: http://localhost:9001 (%s / %s)\n", user, pass)
		} else {
			fmt.Println("ℹ️  MinIO não detectado (ou não instalado).")
		}

		// 3. Prometheus & Grafana (Local First)
		// Check for Prometheus service to feed local Grafana
		promSvc, promNs := findPrometheusService(targetContext)
		if promSvc != "" {
			fmt.Printf("🔌 Detectado Prometheus (%s/%s)! Conectando para Grafana...\n", promNs, promSvc)
			killPortForward("9090")
			g.Go(func() error {
				return runPortForwardWithContext(gctx, targetContext, promNs, "svc/"+promSvc, "9090:9090")
			})

			// Start Local Grafana
			fmt.Println("🐳 Iniciando Grafana Local (Docker)...")
			if err := startLocalGrafanaContainer(); err != nil {
				fmt.Printf("⚠️  Falha ao iniciar Grafana Docker: %v\n", err)
			} else {
				fmt.Println("   -> Grafana: http://localhost:3001 (admin/admin)")
				fmt.Println("      (Dados persistidos no volume 'yby-grafana-data')")
			}
		} else {
			fmt.Println("⚠️  Prometheus não encontrado. Grafana local não será iniciado.")
		}

		// 4. Token Headlamp
		token, err := getHeadlampToken(targetContext)
		if err == nil {
			fmt.Println("")
			fmt.Println("🔑 Token Headlamp (copie abaixo):")
			fmt.Println(token)
		}

		fmt.Println("")
		fmt.Println("ℹ️  Pressione Ctrl+C para encerrar os túneis...")

		// Wait for signal or error
		go func() {
			<-sigChan
			fmt.Println("\n🛑 Encerrando túneis...")
			cancel()
		}()

		// Wait for all goroutines to finish
		if err := g.Wait(); err != nil && err != context.Canceled {
			fmt.Printf("⚠️  Erro nos túneis: %v\n", err)
		}

		fmt.Println("✅ Túneis encerrados.")
	},
}

func init() {
	rootCmd.AddCommand(accessCmd)
	accessCmd.Flags().StringP("context", "c", "", "Nome do contexto Kubernetes")
}

// Helpers

func getKubectlContext() (string, error) {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	return strings.TrimSpace(string(out)), err
}

func hasService(context, namespace, service string) bool {
	// kubectl get svc minio -n storage
	err := exec.Command("kubectl", "--context", context, "-n", namespace, "get", "svc", service).Run()
	return err == nil
}

func findService(context string, candidates []struct{ ns, svc string }) (string, string) {
	for _, c := range candidates {
		if hasService(context, c.ns, c.svc) {
			return c.svc, c.ns
		}
	}
	return "", ""
}

func findMinioService(context string) (string, string) {
	candidates := []struct{ ns, svc string }{
		{"storage", "minio"},
		{"default", "minio"},
		{"default", "cluster-config-minio"},
		{"minio", "minio"},
	}
	return findService(context, candidates)
}

func findPrometheusService(context string) (string, string) {
	candidates := []struct{ ns, svc string }{
		{"kube-system", "system-kube-prometheus-sta-prometheus"},   // Truncated name often seen
		{"kube-system", "system-kube-prometheus-stack-prometheus"}, // Full name
		{"monitoring", "prometheus-kube-prometheus-prometheus"},
		{"monitoring", "prometheus-server"},
		{"default", "prometheus-operated"},
	}
	return findService(context, candidates)
}

func getArgoPassword(context string) (string, error) {
	return getSecretValue(context, "argocd", "argocd-initial-admin-secret", "password")
}

func getSecretKeys(context, ns, secret, keyUser, keyPass string) (string, string) {
	user, _ := getSecretValue(context, ns, secret, keyUser)
	pass, _ := getSecretValue(context, ns, secret, keyPass)
	return user, pass
}

func getSecretValue(context, ns, secret, jsonPathKey string) (string, error) {
	// jsonpath={.data.key}
	cmd := exec.Command("kubectl", "--context", context, "--insecure-skip-tls-verify", "-n", ns, "get", "secret", secret, fmt.Sprintf("-o=jsonpath={.data.%s}", jsonPathKey))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(string(out))
	return string(decoded), err
}

func runPortForwardWithContext(ctx context.Context, kubeContext, namespace, resource, ports string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cmd := exec.CommandContext(ctx, "kubectl", "--context", kubeContext, "--insecure-skip-tls-verify", "-n", namespace, "port-forward", resource, ports)
			cmd.Stdout = nil // Silence stdout
			cmd.Stderr = nil // Silence stderr

			if err := cmd.Run(); err != nil {
				// Check if context was cancelled
				if ctx.Err() != nil {
					return ctx.Err()
				}
				// Otherwise, retry after a delay
				time.Sleep(2 * time.Second)
				continue
			}
			// Clean exit
			return nil
		}
	}
}

func killPortForward(port string) {
	_ = exec.Command("pkill", "-f", fmt.Sprintf("port-forward.*%s", port)).Run()
}

func getHeadlampToken(context string) (string, error) {
	cmd := exec.Command("kubectl", "--context", context, "--insecure-skip-tls-verify", "create", "token", "admin-user", "-n", "kube-system", "--duration=24h")
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func startLocalGrafanaContainer() error {
	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker não encontrado no PATH")
	}

	// host.docker.internal handling for Linux
	// On Linux, we need --add-host=host.docker.internal:host-gateway
	addHost := "--add-host=host.docker.internal:host-gateway"

	// Create volume if not exists
	_ = exec.Command("docker", "volume", "create", "yby-grafana-data").Run()

	// Stop existing
	_ = exec.Command("docker", "rm", "-f", "yby-grafana").Run()

	// Run
	cmd := exec.Command("docker", "run", "-d",
		"--name", "yby-grafana",
		"-p", "3001:3000",
		"-v", "yby-grafana-data:/var/lib/grafana",
		addHost,
		"grafana/grafana:latest")

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", string(out))
	}
	return nil
}
