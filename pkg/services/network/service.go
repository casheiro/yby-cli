package network

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

type DefaultAccessService struct {
	Network   ClusterNetworkManager
	Container LocalContainerManager
}

func NewAccessService(network ClusterNetworkManager, container LocalContainerManager) *DefaultAccessService {
	return &DefaultAccessService{
		Network:   network,
		Container: container,
	}
}

func (s *DefaultAccessService) Run(ctx context.Context, opts AccessOptions) error {
	targetContext := opts.TargetContext
	if targetContext == "" {
		var err error
		targetContext, err = s.Network.GetCurrentContext()
		if err != nil {
			return fmt.Errorf("Erro ao detectar contexto atual: %v", err)
		}
		fmt.Printf("📍 Contexto: %s (detectado automaticamente)\n", targetContext)
	} else {
		fmt.Printf("📍 Contexto: %s (definido via flag)\n", targetContext)
	}

	g, gctx := errgroup.WithContext(ctx)

	// 1. Argo CD
	argoPwd, err := s.getArgoPassword(gctx, targetContext)
	if err != nil {
		fmt.Printf("⚠️  Argo CD: Não foi possível obter senha (talvez não instalado no namespace 'argocd'?): %v\n", err)
	} else {
		fmt.Println("🔌 Conectando Argo CD...")
		s.Network.KillPortForward("8085")
		g.Go(func() error {
			return s.Network.PortForward(gctx, targetContext, "argocd", "svc/argocd-server", "8085:80")
		})
		fmt.Printf("   -> Argo CD: http://localhost:8085 (admin / %s)\n", argoPwd)
	}

	// 2. MinIO
	minioSvc, minioNs := s.findMinioService(gctx, targetContext)
	if minioSvc != "" {
		fmt.Printf("🔌 Detectado MinIO (%s/%s)! Conectando...\n", minioNs, minioSvc)
		s.Network.KillPortForward("9000")
		s.Network.KillPortForward("9001")
		g.Go(func() error {
			return s.Network.PortForward(gctx, targetContext, minioNs, "svc/"+minioSvc, "9000:9000")
		})
		g.Go(func() error {
			return s.Network.PortForward(gctx, targetContext, minioNs, "svc/"+minioSvc, "9001:9001")
		})

		user, pass := s.getSecretKeys(gctx, targetContext, "storage", "minio-secret", "rootUser", "rootPassword")
		if user == "" {
			user, pass = s.getSecretKeys(gctx, targetContext, "default", "minio-creds", "rootUser", "rootPassword")
		}

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

	// 3. Prometheus & Grafana
	promSvc, promNs := s.findPrometheusService(gctx, targetContext)
	if promSvc != "" {
		fmt.Printf("🔌 Detectado Prometheus (%s/%s)! Conectando para Grafana...\n", promNs, promSvc)
		s.Network.KillPortForward("9090")
		g.Go(func() error {
			return s.Network.PortForward(gctx, targetContext, promNs, "svc/"+promSvc, "9090:9090")
		})

		fmt.Println("🐳 Iniciando Grafana Local (Docker)...")
		if s.Container.IsAvailable() {
			if err := s.Container.StartGrafana(gctx); err != nil {
				fmt.Printf("⚠️  Falha ao iniciar Grafana Docker: %v\n", err)
			} else {
				fmt.Println("   -> Grafana: http://localhost:3001 (admin/admin)")
				fmt.Println("      (Dados persistidos no volume 'yby-grafana-data')")
			}
		} else {
			fmt.Println("⚠️  Docker não está disponível no PATH. Grafana local não será iniciado.")
		}
	} else {
		fmt.Println("⚠️  Prometheus não encontrado. Grafana local não será iniciado.")
	}

	// 4. Token Headlamp
	token, err := s.Network.CreateToken(gctx, targetContext, "kube-system", "admin-user", "24h")
	if err == nil {
		fmt.Println("\n🔑 Token Headlamp (copie abaixo):")
		fmt.Println(token)
	}

	fmt.Println("\nℹ️  Pressione Ctrl+C para encerrar os túneis...")

	if err := g.Wait(); err != nil && err != context.Canceled {
		return fmt.Errorf("Erro nos túneis: %w", err)
	}

	return nil
}

func (s *DefaultAccessService) getArgoPassword(ctx context.Context, targetContext string) (string, error) {
	return s.Network.GetSecretValue(ctx, targetContext, "argocd", "argocd-initial-admin-secret", "password")
}

func (s *DefaultAccessService) findService(ctx context.Context, targetContext string, candidates []struct{ ns, svc string }) (string, string) {
	for _, c := range candidates {
		if s.Network.HasService(ctx, targetContext, c.ns, c.svc) {
			return c.svc, c.ns
		}
	}
	return "", ""
}

func (s *DefaultAccessService) findMinioService(ctx context.Context, targetContext string) (string, string) {
	candidates := []struct{ ns, svc string }{
		{"storage", "minio"},
		{"default", "minio"},
		{"default", "cluster-config-minio"},
		{"minio", "minio"},
	}
	return s.findService(ctx, targetContext, candidates)
}

func (s *DefaultAccessService) findPrometheusService(ctx context.Context, targetContext string) (string, string) {
	candidates := []struct{ ns, svc string }{
		{"kube-system", "system-kube-prometheus-sta-prometheus"},
		{"kube-system", "system-kube-prometheus-stack-prometheus"},
		{"monitoring", "prometheus-kube-prometheus-prometheus"},
		{"monitoring", "prometheus-server"},
		{"default", "prometheus-operated"},
	}
	return s.findService(ctx, targetContext, candidates)
}

func (s *DefaultAccessService) getSecretKeys(ctx context.Context, targetContext, ns, secret, keyUser, keyPass string) (string, string) {
	user, _ := s.Network.GetSecretValue(ctx, targetContext, ns, secret, keyUser)
	pass, _ := s.Network.GetSecretValue(ctx, targetContext, ns, secret, keyPass)
	return user, pass
}
