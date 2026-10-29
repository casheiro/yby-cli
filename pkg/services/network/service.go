package network

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/casheiro/yby-cli/v5/pkg/retry"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"golang.org/x/sync/errgroup"
)

type DefaultAccessService struct {
	Network   ClusterNetworkManager
	Container LocalContainerManager
	// Saida recebe os endereços, as credenciais e o token para as pessoas; nil usa
	// os.Stdout.
	Saida io.Writer
}

func NewAccessService(network ClusterNetworkManager, container LocalContainerManager) *DefaultAccessService {
	return &DefaultAccessService{
		Network:   network,
		Container: container,
	}
}

// colunasAcesso alinha nome do serviço, endereço e credenciais.
var colunasAcesso = ui.Colunas{Nome: 15, Versao: 24}

func (s *DefaultAccessService) Run(ctx context.Context, opts AccessOptions) error {
	w := s.Saida
	if w == nil {
		w = os.Stdout
	}
	linha := func(tipo ui.TipoCheck, nome, endereco, nota string) {
		ui.CheckCom(w, colunasAcesso, tipo, nome, endereco, nota)
	}

	targetContext := opts.TargetContext
	if targetContext == "" {
		var err error
		targetContext, err = s.Network.GetCurrentContext()
		if err != nil {
			return fmt.Errorf("falha ao detectar o contexto atual: %v", err)
		}
		slog.Debug("Contexto detectado automaticamente", "contexto", targetContext)
	} else {
		slog.Debug("Contexto definido via flag", "contexto", targetContext)
	}
	ui.Section(w, "túneis")
	linha(ui.CheckNeutro, "contexto", targetContext, "")

	g, gctx := errgroup.WithContext(ctx)

	// 1. Argo CD
	argoPwd, err := s.getArgoPassword(gctx, targetContext)
	if err != nil {
		slog.Debug("Argo CD: não foi possível obter senha", "error", err)
		linha(ui.CheckAviso, "argo cd", "sem túnel", "não deu para ler a senha inicial do admin")
	} else {
		slog.Debug("Conectando Argo CD")
		s.Network.KillPortForward("8085")
		g.Go(func() error {
			return retry.DoWithDefault(gctx, func() error {
				return s.Network.PortForward(gctx, targetContext, "argocd", "svc/argocd-server", "8085:80")
			})
		})
		linha(ui.CheckOk, "argo cd", "http://localhost:8085", "admin / "+argoPwd)
	}

	// 2. MinIO
	minioSvc, minioNs := s.findMinioService(gctx, targetContext)
	if minioSvc != "" {
		slog.Debug("MinIO detectado, conectando", "namespace", minioNs, "service", minioSvc)
		s.Network.KillPortForward("9000")
		s.Network.KillPortForward("9001")
		g.Go(func() error {
			return retry.DoWithDefault(gctx, func() error {
				return s.Network.PortForward(gctx, targetContext, minioNs, "svc/"+minioSvc, "9000:9000")
			})
		})
		g.Go(func() error {
			return retry.DoWithDefault(gctx, func() error {
				return s.Network.PortForward(gctx, targetContext, minioNs, "svc/"+minioSvc, "9001:9001")
			})
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

		linha(ui.CheckOk, "minio api", "http://localhost:9000", "")
		linha(ui.CheckOk, "minio console", "http://localhost:9001", user+" / "+pass)
	} else {
		slog.Debug("MinIO não detectado")
		linha(ui.CheckNeutro, "minio", "não encontrado", "")
	}

	// 3. Prometheus & Grafana
	promSvc, promNs := s.findPrometheusService(gctx, targetContext)
	if promSvc != "" {
		slog.Debug("Prometheus detectado, conectando", "namespace", promNs, "service", promSvc)
		s.Network.KillPortForward("9090")
		g.Go(func() error {
			return retry.DoWithDefault(gctx, func() error {
				return s.Network.PortForward(gctx, targetContext, promNs, "svc/"+promSvc, "9090:9090")
			})
		})
		linha(ui.CheckOk, "prometheus", "http://localhost:9090", "")

		slog.Debug("Iniciando Grafana Local via Docker")
		if s.Container.IsAvailable() {
			if err := s.Container.StartGrafana(gctx); err != nil {
				slog.Debug("Falha ao iniciar Grafana Docker", "error", err)
				linha(ui.CheckAviso, "grafana", "não subiu", "o container do Docker falhou · --log-level=debug mostra o erro")
			} else {
				linha(ui.CheckOk, "grafana", "http://localhost:3001", "admin / admin · volume yby-grafana-data")
			}
		} else {
			linha(ui.CheckNeutro, "grafana", "sem docker", "o grafana local precisa do Docker no PATH")
		}
	} else {
		linha(ui.CheckNeutro, "prometheus", "não encontrado", "sem ele o grafana local não sobe")
	}

	// 4. Token Headlamp
	token, err := s.Network.CreateToken(gctx, targetContext, "kube-system", "admin-user", "24h")
	if err == nil {
		ui.Section(w, "token do headlamp")
		fmt.Fprintln(w, "  "+ui.S().Tinta.Render(token))
		slog.Debug("token gerado", "token_preview", maskToken(token))
	}

	fmt.Fprintln(w)
	linha(ui.CheckNeutro, "ctrl+c encerra os túneis", "", "")

	if err := g.Wait(); err != nil && err != context.Canceled {
		return fmt.Errorf("falha nos túneis: %w", err)
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

func maskToken(t string) string {
	if len(t) <= 16 {
		return "***"
	}
	return t[:8] + "***...**" + t[len(t)-8:]
}
