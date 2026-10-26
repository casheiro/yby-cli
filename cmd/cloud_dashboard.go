/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/cloud"
	ybycontext "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var cloudDashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Short:   "Painel dos clusters cloud, com a validade dos tokens",
	Example: `  yby cloud dashboard`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Sem terminal na entrada (pipe, CI) a TUI não teria de onde ler as teclas; com a
		// saída num pipe ou arquivo, a tela iria parar lá, com as sequências do terminal.
		if !stdinTerminal() || !stdoutTerminal() {
			return errors.New(errors.ErrCodeValidation, "o painel do cloud precisa de um terminal interativo").
				WithHint("para uma lista simples, use yby cloud list")
		}
		m := novoPainelCloud(fontesReais(&shared.RealRunner{}), time.Now)
		if _, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "o painel do cloud falhou").
				WithHint("rode de novo ou use yby cloud list")
		}
		return nil
	},
}

func init() {
	cloudCmd.AddCommand(cloudDashboardCmd)
}

// stdoutTerminal informa se a saída padrão é um terminal. Variável para os testes.
var stdoutTerminal = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// janelaAuditoria é até onde o painel olha para trás no audit log.
const janelaAuditoria = 30 * 24 * time.Hour

// fontesPainel são as leituras e a ação do painel, injetadas para os testes.
type fontesPainel struct {
	// ler busca os clusters (com as credenciais de cada provider) e a auditoria recente.
	ler func(ctx gocontext.Context) leituraPainel
	// renovar renova o token do cluster.
	renovar func(ctx gocontext.Context, c clusterPainel) error
	// auditoria relê só o audit log (depois de um t, deu certo ou não); nil: não relê.
	auditoria func() []cloud.CloudAuditEvent
	// contextos lê os nomes dos contextos do kubeconfig dado (vazio: o padrão).
	contextos func(kubeconfig string) (map[string]bool, error)
	// abrirViz roda o Viz com os args dados e espera ele fechar.
	abrirViz func(args []string) error
	// executar solta o terminal para o comando (nil: tea.Exec); os testes rodam o comando.
	executar func(tea.ExecCommand, tea.ExecCallback) tea.Cmd
}

// leituraPainel é o resultado de uma leitura das clouds.
type leituraPainel struct {
	provedores int // providers detectados no kubeconfig com o CLI instalado
	clusters   []clusterPainel
	auditoria  []cloud.CloudAuditEvent
	falhas     map[string]error // provider → erro do ListClusters (os outros responderam)
	err        error            // nenhum provider respondeu
}

// clusterPainel é uma linha do painel: o cluster e o que se sabe da credencial dele.
type clusterPainel struct {
	info        cloud.ClusterInfo // Provider sempre preenchido
	autenticado bool
	identidade  string
	conta       string // conta AWS, assinatura Azure ou projeto GCP (sem máscara)
	papel       string
	usuario     string // a identidade quando ela é um usuário e não um papel (o e-mail no Azure)
	metodo      string // já no texto da tela ("SSO" + sep + "Identity Center")
	sso         bool
	ambiente    string // nome em .yby/environments.yaml, quando houver
	// kubeconfig, kubeContexto e namespace são os do ambiente, por onde o ↵ abre o Viz.
	kubeconfig   string
	kubeContexto string
	namespace    string
	expiraEm     time.Time // zero: validade desconhecida
	// desatualizado: o provider não respondeu nesta leitura e a linha é da anterior.
	desatualizado bool
	lidoEm        time.Time // a leitura de onde vêm os dados da linha
}

// chave identifica o cluster entre leituras.
func (c clusterPainel) chave() string {
	return c.info.Provider + "/" + c.info.Region + "/" + c.info.Name
}

// fontesReais lê as clouds pelos CLIs (aws, az, gcloud) e o audit log de ~/.yby.
func fontesReais(runner shared.Runner) fontesPainel {
	return fontesPainel{
		ler: func(ctx gocontext.Context) leituraPainel {
			return lerClouds(ctx, runner, ambientesCloud(), auditoriaCloud(), time.Now())
		},
		renovar: func(ctx gocontext.Context, c clusterPainel) error {
			p := obterProviderCloud(runner, c.info.Provider)
			if p == nil {
				return erroProviderDesconhecido(c.info.Provider)
			}
			err := p.RefreshToken(ctx, c.info)
			registrarRenovacao(auditoriaCloud(), c, time.Now(), err)
			return err
		},
		auditoria: func() []cloud.CloudAuditEvent {
			return lerAuditoria(auditoriaCloud(), time.Now())
		},
		contextos: contextosDoKubeconfig,
		abrirViz:  abrirVizPeloManager,
	}
}

// registrarRenovacao grava o t do painel no audit log, com o método da credencial.
func registrarRenovacao(log *cloud.AuditLogger, c clusterPainel, quando time.Time, err error) {
	e := cloud.CloudAuditEvent{
		Timestamp: quando,
		Action:    "refresh",
		Provider:  c.info.Provider,
		Identity:  c.identidade,
		Cluster:   c.info.Name,
		Region:    c.info.Region,
		Method:    metodoCurto(c),
		Success:   err == nil,
	}
	if err != nil {
		e.Error = err.Error()
	}
	if errLog := log.Log(e); errLog != nil {
		slog.Debug("cloud.dashboard.auditoria", "erro", errLog)
	}
}

// metodoCurto é o método para o audit log: "SSO", "IAM", "CLI do Azure".
func metodoCurto(c clusterPainel) string {
	if c.sso {
		return "SSO"
	}
	return strings.SplitN(c.metodo, " "+ui.G().Ponto+" ", 2)[0]
}

// lerClouds lista os clusters de cada provider detectado e lê a auditoria recente. O
// provider que falha fica em falhas; err só quando nenhum respondeu.
func lerClouds(ctx gocontext.Context, runner shared.Runner, ambientes map[string]ambienteCloud, auditoria *cloud.AuditLogger, agora time.Time) leituraPainel {
	providers := detectarProvidersCloud(ctx, runner)
	l := leituraPainel{provedores: len(providers)}
	var falha error
	for _, p := range providers {
		clusters, err := p.ListClusters(ctx, cloud.ListOptions{})
		if err != nil {
			slog.Debug("cloud.dashboard.listar", "provider", p.Name(), "erro", err)
			if l.falhas == nil {
				l.falhas = map[string]error{}
			}
			l.falhas[p.Name()] = err
			falha = err
			continue
		}
		cred, err := p.ValidateCredentials(ctx)
		if err != nil {
			slog.Debug("cloud.dashboard.credenciais", "provider", p.Name(), "erro", err)
		}
		for _, c := range clusters {
			l.clusters = append(l.clusters, novoClusterPainel(p.Name(), c, cred, ambientes))
		}
	}
	if len(providers) > 0 && len(l.falhas) == len(providers) {
		l.err = falha
	}
	l.auditoria = lerAuditoria(auditoria, agora)
	return l
}

// lerAuditoria lê os registros dos últimos 30 dias do audit log.
func lerAuditoria(auditoria *cloud.AuditLogger, agora time.Time) []cloud.CloudAuditEvent {
	eventos, err := auditoria.ReadEvents(agora.Add(-janelaAuditoria))
	if err != nil {
		slog.Debug("cloud.dashboard.auditoria", "erro", err)
	}
	return eventos
}

// ambienteCloud é um ambiente de .yby/environments.yaml ligado a um cluster cloud.
type ambienteCloud struct {
	nome         string
	cfg          *ybycontext.CloudConfig
	kubeconfig   string // kube_config do ambiente (vazio: o kubeconfig padrão)
	kubeContexto string // kube_context do ambiente
	namespace    string
}

// ambientesCloud lê os ambientes cloud do projeto, indexados por provider/cluster. Fora
// de um projeto, nenhum.
func ambientesCloud() map[string]ambienteCloud {
	raiz, err := FindInfraRoot()
	if err != nil {
		raiz = "."
	}
	manifesto, err := ybycontext.NewManager(raiz).LoadManifest()
	if err != nil {
		return nil
	}
	saida := map[string]ambienteCloud{}
	nomes := make([]string, 0, len(manifesto.Environments))
	for nome := range manifesto.Environments {
		nomes = append(nomes, nome)
	}
	sort.Strings(nomes) // dois ambientes no mesmo cluster: vale o primeiro em ordem alfabética
	for _, nome := range nomes {
		amb := manifesto.Environments[nome]
		cfg := amb.Cloud
		if cfg == nil || cfg.Cluster == "" {
			continue
		}
		k := strings.ToLower(cfg.Provider) + "/" + cfg.Cluster
		if _, existe := saida[k]; !existe {
			saida[k] = ambienteCloud{nome: nome, cfg: cfg, kubeconfig: amb.KubeConfig, kubeContexto: amb.KubeContext, namespace: amb.Namespace}
		}
	}
	return saida
}

// novoClusterPainel junta o cluster, a credencial do provider e o ambiente do projeto.
func novoClusterPainel(provider string, info cloud.ClusterInfo, cred *cloud.CredentialStatus, ambientes map[string]ambienteCloud) clusterPainel {
	info.Provider = provider
	c := clusterPainel{info: info}
	contaDaCred := ""
	if cred != nil {
		c.autenticado = cred.Authenticated
		c.identidade = cred.Identity
		contaDaCred = cred.Account
		if cred.ExpiresAt != nil {
			c.expiraEm = *cred.ExpiresAt
		}
	}
	amb, temAmbiente := ambientes[provider+"/"+info.Name]
	if temAmbiente {
		c.ambiente, c.kubeconfig, c.kubeContexto, c.namespace = amb.nome, amb.kubeconfig, amb.kubeContexto, amb.namespace
	}
	c.conta, c.papel, c.usuario = contaEPapel(provider, c.identidade, contaDaCred, info, amb.cfg)
	metodoCred := ""
	if cred != nil {
		metodoCred = cred.Method
	}
	c.metodo, c.sso = metodoDeAcesso(provider, metodoCred, c.papel, amb.cfg)
	return c
}

// contaEPapel tira a conta, o papel e o usuário da credencial: o ARN na AWS; no Azure, a
// assinatura ativa e o usuário (o az não diz o papel); no GCP, o projeto e a conta de
// serviço. O ambiente do projeto completa o que a credencial não diz.
func contaEPapel(provider, identidade, contaDaCred string, info cloud.ClusterInfo, cfg *ybycontext.CloudConfig) (conta, papel, usuario string) {
	switch provider {
	case "aws":
		// arn:aws:sts::482100009310:assumed-role/yby-staging-admin/sessao
		if partes := strings.SplitN(identidade, ":", 6); len(partes) == 6 {
			conta, papel = partes[4], partes[5]
			if recurso := strings.Split(papel, "/"); len(recurso) >= 2 && recurso[0] == "assumed-role" {
				papel = recurso[1]
			}
		}
		if conta == "" {
			conta = contaDaCred
		}
		if cfg != nil && cfg.RoleARN != "" {
			papel = cfg.RoleARN[strings.LastIndex(cfg.RoleARN, "/")+1:]
		}
	case "azure":
		// o az lista os clusters da assinatura ativa, que é a da credencial
		usuario, conta = identidade, contaDaCred
		if cfg != nil && conta == "" {
			conta = cfg.Subscription
		}
	case "gcp":
		conta, papel = info.ProjectID, identidade
		if cfg != nil && conta == "" {
			conta = cfg.ProjectID
		}
		if cfg != nil && cfg.ServiceAccount != "" {
			papel = cfg.ServiceAccount
		}
	default:
		papel = identidade
	}
	return conta, papel, usuario
}

// metodoDeAcesso põe o método de autenticação no texto da tela e diz se é SSO.
func metodoDeAcesso(provider, metodoCred, papel string, cfg *ybycontext.CloudConfig) (string, bool) {
	sep := " " + ui.G().Ponto + " "
	if cfg != nil && cfg.Auth != nil && cfg.Auth.Method != "" {
		switch cfg.Auth.Method {
		case "sso":
			return "SSO" + sep + "Identity Center", true
		case "web-identity":
			return "IRSA" + sep + "web identity", false
		case "mfa":
			return "MFA" + sep + "assume-role", false
		}
		return cfg.Auth.Method, false
	}
	switch {
	case provider == "aws" && strings.HasPrefix(papel, "AWSReservedSSO_"):
		return "SSO" + sep + "Identity Center", true
	case metodoCred == "iam" && cfg != nil && cfg.Profile != "":
		return "IAM" + sep + "perfil " + cfg.Profile, false
	case metodoCred == "iam":
		return "IAM", false
	case metodoCred == "az-cli":
		return "CLI do Azure", false
	case metodoCred == "gcloud-cli":
		return "CLI do gcloud", false
	}
	return metodoCred, false
}
