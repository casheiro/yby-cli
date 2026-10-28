/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/executor"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// bootstrapVpsCmd represents the bootstrap vps command
var bootstrapVpsCmd = &cobra.Command{
	Use:   "vps",
	Short: "Provisiona um VPS com K3s e prepara para GitOps",
	Long: `Conecta via SSH a um servidor VPS (definido no .env) ou executa localmente.
Instala dependências, configura firewall, instala K3s e configura o kubeconfig local.

Pré-requisitos (verificados automaticamente):
* Ubuntu 22.04+ (Recomendado)
* 4GB RAM (Mínimo recomendado para stack completa)
* Acesso root/sudo`,
	Example: `  # Provisionar VPS remoto (requer acesso SSH por chave)
  yby bootstrap vps --host 192.168.1.10 --user ubuntu

  # Provisionar máquina local (laptop/desktop)
  yby bootstrap vps --local`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cmd.OutOrStdout()

		// 0. Detect Mode
		isLocal, _ := cmd.Flags().GetBool("local")
		var avisos []linhaCheck

		// 1. Carregar configuração (Flag > Manifesto > Env (Legacy))
		host, user, port := vpsHost, vpsUser, vpsPort
		if !isLocal {
			// Smart Local Detection:
			// If no host provided, and we are on Linux, ask if user wants to provision THIS machine.
			if host == "" && runtime.GOOS == "linux" {
				confirmLocal, _ := prompter.Confirm("Nenhum host remoto definido. Deseja provisionar ESTA máquina como uma VPS Yby (localhost)?", false)
				isLocal = confirmLocal
			}

			// Legacy .env support (Deprecation Warning)
			if !isLocal && host == "" {
				// Try loading .env only if strictly necessary
				if _, err := os.Stat("../.env"); err == nil {
					_ = godotenv.Load("../.env")
					if val := os.Getenv("VPS_HOST"); val != "" {
						avisos = append(avisos, linhaCheck{ui.CheckAviso, "VPS_HOST lido do ../.env · obsoleto, use --host", "", ""})
						host = val
					}
					if user == "root" && os.Getenv("VPS_USER") != "" {
						user = os.Getenv("VPS_USER")
					}
				}
			}

			if !isLocal && host == "" {
				ui.Title(w, "bootstrap vps", "")
				return errors.New(errors.ErrCodeValidation, "host não definido").
					WithHint("yby bootstrap vps --host IP --user USUÁRIO, ou --local para esta máquina")
			}
		}

		destino := user + "@" + host + ":" + port
		if isLocal {
			// K3s grava o kubeconfig em /etc/rancher/k3s/k3s.yaml; localmente ele aponta
			// para 127.0.0.1 antes do merge com ~/.kube/config.
			host = "127.0.0.1"
			destino = "esta máquina"
		}
		ui.Title(w, "bootstrap vps", destino)
		for _, a := range avisos {
			ui.Check(w, a.tipo, a.nome, a.estado, a.nota)
		}

		// K3s Version Resolution
		// Priority: Flag > cluster-values.yaml > Default
		if !cmd.Flags().Changed("k3s-version") {
			if v := versaoK3sDoClusterValues(); v != "" {
				k3sVersion = v
				ui.Check(w, ui.CheckNeutro, "k3s "+k3sVersion+" · lido de config/cluster-values.yaml", "", "")
			}
		}

		rotuloK3s := "k3s " + k3sVersion
		plano := []string{"requisitos mínimos", "sistema e dependências", "firewall (ufw)", "docker", rotuloK3s, "kubeconfig"}
		if !isLocal {
			plano = append([]string{"conexão ssh"}, plano...)
		}
		prog := novoProgresso(w, plano...)
		parar := animar(prog)
		defer parar()

		// 2. Conexão SSH (só no modo remoto)
		var execClient executor.Executor
		if isLocal {
			execClient = novoExecutorLocal(prog)
		} else {
			prog.Iniciar("conexão ssh")
			var err error
			execClient, err = novoExecutorSSH(user, host, port, prog)
			if err != nil {
				prog.Falhar("")
				return errors.Wrap(err, errors.ErrCodeUnreachable, "falha na conexão SSH").
					WithHint("confira host, usuário e se a chave está no ssh-agent (ssh-add -l)")
			}
			prog.Concluir("")
		}
		defer execClient.Close()

		// 3. Requisitos: a memória é lida aqui (e não num script) para o alerta aparecer
		// antes da espera em que o ctrl+c ainda cancela
		if err := conferirMemoria(execClient, prog); err != nil {
			return err
		}

		// 4. Preparar Servidor
		if err := runEx(execClient, "sistema e dependências", `
			export DEBIAN_FRONTEND=noninteractive
			apt-get update -qq
			if ! command -v curl >/dev/null; then apt-get install -y -qq curl; fi
			apt-get install -y -qq wget git htop nano ca-certificates gnupg lsb-release ufw iptables-persistent
			timedatectl set-timezone America/Sao_Paulo
			swapoff -a
			sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
		`); err != nil {
			return err
		}

		// 4. Firewall
		if err := runEx(execClient, "firewall (ufw)", `
			ufw --force reset
			ufw default deny incoming
			ufw default allow outgoing
			ufw allow 22/tcp
			ufw allow 6443/tcp
			ufw allow 80/tcp
			ufw allow 443/tcp
			ufw allow 8080/tcp
			ufw allow 12000/tcp
			ufw allow 8472/udp
			ufw --force enable
		`); err != nil {
			return err
		}

		// 5. Docker
		if err := runEx(execClient, "docker", `
			if ! command -v docker >/dev/null 2>&1; then
				curl -fsSL https://get.docker.com -o get-docker.sh
				sh get-docker.sh
				systemctl enable docker
				systemctl start docker
			else
				echo "Docker já instalado"
			fi
		`); err != nil {
			return err
		}

		// 6. K3s
		k3sToken := os.Getenv("K3S_TOKEN")
		if k3sToken == "" {
			tokenBytes := make([]byte, 32)
			if _, err := rand.Read(tokenBytes); err != nil {
				return errors.Wrap(err, errors.ErrCodeExec, "falha ao gerar o token do k3s").
					WithHint("defina o token em K3S_TOKEN e rode de novo")
			}
			k3sToken = hex.EncodeToString(tokenBytes)
		}

		// 7. K3s Installation
		// Logic adjustment: for local mode, "host" for tls-san should ideally be the public IP or hostname.
		// Since we default host to 127.0.0.1 for local, we might want to also add the machine's hostname.
		// For MVP, lets keep 127.0.0.1 and maybe detected hostname if we can (via hostname command in script?)
		if err := runEx(execClient, rotuloK3s, fmt.Sprintf(`
			if ! command -v k3s >/dev/null 2>&1; then
				curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="%s" K3S_TOKEN="%s" sh -s - server --cluster-init --write-kubeconfig-mode=644 --tls-san %s
			else
				echo "K3s já instalado"
			fi
		`, k3sVersion, k3sToken, host)); err != nil {
			return err
		}

		// 7. Fetch Kubeconfig
		prog.Iniciar("kubeconfig")
		runner := runnerQuieto{&shared.RealRunner{}}
		if err := configurarKubeconfig(execClient, host, runner); err != nil {
			prog.Falhar("")
			return errors.Wrap(err, errors.ErrCodeConfig, "falha ao configurar o kubeconfig").
				WithHint("confira se o k3s gerou /etc/rancher/k3s/k3s.yaml no servidor")
		}
		prog.Concluir("contexto " + nomeContextoVps())
		parar()

		fmt.Fprintln(w)
		ui.Check(w, ui.CheckOk, "vps pronta", "", "")
		ui.Check(w, ui.CheckNeutro, "yby bootstrap cluster instala a stack GitOps", "", "")
		return nil
	},
}

// novoExecutorLocal cria o executor desta máquina (substituível em testes).
var novoExecutorLocal = func(etapas shared.Etapas) executor.Executor {
	return &executor.LocalExecutor{Etapas: etapas}
}

// novoExecutorSSH conecta ao servidor remoto (substituível em testes).
var novoExecutorSSH = func(user, host, port string, etapas shared.Etapas) (executor.Executor, error) {
	e, err := executor.NewSSHExecutor(user, host, port)
	if err != nil {
		return nil, err
	}
	e.Etapas = etapas
	return e, nil
}

// configurarKubeconfig traz o kubeconfig do k3s para ~/.kube/config (substituível em testes).
var configurarKubeconfig = fetchKubeconfig

// versaoK3sDoClusterValues lê system.k3s.version de config/cluster-values.yaml, ou "".
func versaoK3sDoClusterValues() string {
	data, err := os.ReadFile("config/cluster-values.yaml")
	if err != nil {
		return ""
	}
	var config struct {
		System struct {
			K3s struct {
				Version string `yaml:"version"`
			} `yaml:"k3s"`
		} `yaml:"system"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return ""
	}
	return config.System.K3s.Version
}

// nomeContextoVps é o nome do contexto do kubeconfig do VPS: CLUSTER_NAME ou yby-prod.
func nomeContextoVps() string {
	if nome := os.Getenv("CLUSTER_NAME"); nome != "" {
		return nome
	}
	return "yby-prod"
}

var k3sVersion string
var vpsHost string
var vpsUser string
var vpsPort string
var skipTLSVerify bool

func init() {
	bootstrapCmd.AddCommand(bootstrapVpsCmd)
	bootstrapVpsCmd.Flags().StringVar(&k3sVersion, "k3s-version", "v1.31.2+k3s1", "Versão do K3s a ser instalada")
	bootstrapVpsCmd.Flags().StringVar(&vpsHost, "host", "", "IP ou Hostname do VPS")
	bootstrapVpsCmd.Flags().StringVar(&vpsUser, "user", "root", "Usuário SSH")
	bootstrapVpsCmd.Flags().StringVar(&vpsPort, "port", "22", "Porta SSH")
	bootstrapVpsCmd.Flags().Bool("local", false, "Executa o bootstrap na máquina local (auto-provisionamento)")
	bootstrapVpsCmd.Flags().BoolVar(&skipTLSVerify, "skip-tls-verify", false,
		"Desabilita verificação TLS do certificado do cluster (INSEGURO, usar apenas para debug)")
}

// memoriaMinimaKB é a memória abaixo da qual o bootstrap avisa: 3,8 GB, com folga sobre os
// 4 GB que a stack completa pede.
const memoriaMinimaKB = 3800000

// esperaMemoriaBaixa é o tempo para o ctrl+c depois do alerta de memória baixa
// (substituível em testes).
var esperaMemoriaBaixa = 5 * time.Second

// conferirMemoria é a etapa "requisitos mínimos": lê o MemTotal de /proc/meminfo do
// servidor e, abaixo de memoriaMinimaKB, alerta e espera esperaMemoriaBaixa antes de
// seguir.
func conferirMemoria(e executor.Executor, etapas shared.Etapas) error {
	etapas.Iniciar("requisitos mínimos")
	dados, err := e.FetchFile("/proc/meminfo")
	if err != nil {
		etapas.Falhar("")
		return errors.Wrap(err, errors.ErrCodeExec, "falha ao ler a memória do servidor").
			WithHint("o bootstrap vps pede Linux com /proc/meminfo, como o Ubuntu 22.04+")
	}
	var kb int
	for _, linha := range strings.Split(string(dados), "\n") {
		if campos := strings.Fields(linha); len(campos) >= 2 && campos[0] == "MemTotal:" {
			kb, _ = strconv.Atoi(campos[1])
			break
		}
	}
	if kb <= 0 {
		// sem a leitura, não há como avisar; o resto do bootstrap decide
		etapas.Concluir("memória não identificada")
		return nil
	}
	etapas.Concluir(fmt.Sprintf("%d MB de memória", kb/1024))
	if kb >= memoriaMinimaKB {
		return nil
	}
	etapas.Avisar(fmt.Sprintf("abaixo dos 4 GB da stack · segue em %s, ctrl+c cancela",
		ui.FormatarDuracao(esperaMemoriaBaixa)))
	time.Sleep(esperaMemoriaBaixa)
	return nil
}

func runEx(e executor.Executor, name, script string) error {
	if err := e.Run(name, script); err != nil {
		return errors.Wrap(err, errors.ErrCodeExec, fmt.Sprintf("falha na etapa %s", name)).
			WithHint("veja a saída do script acima, corrija e rode yby bootstrap vps de novo")
	}
	return nil
}

func fetchKubeconfig(e executor.Executor, host string, runner shared.Runner) error {
	ctx := context.Background()

	contentBytes, err := e.FetchFile("/etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return err
	}

	// 1. Preparar conteúdo
	content := string(contentBytes)
	content = strings.ReplaceAll(content, "127.0.0.1", host)
	content = strings.ReplaceAll(content, "localhost", host)

	// Determine Cluster Name
	clusterName := nomeContextoVps()

	// Save to temp file
	tempFile, err := os.CreateTemp("", "k3s-config-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(content); err != nil {
		return err
	}
	tempFile.Close()

	// 2. Renomear Contexto
	os.Setenv("KUBECONFIG", tempFile.Name())
	_ = runner.Run(ctx, "kubectl", "config", "rename-context", "default", clusterName)
	_ = runner.Run(ctx, "kubectl", "config", "set-cluster", "default", "--server=https://"+host+":6443")
	if skipTLSVerify {
		slog.Warn("TLS verify desabilitado — conexão vulnerável a MITM", "host", host)
		_ = runner.Run(ctx, "kubectl", "config", "set-cluster", "default", "--insecure-skip-tls-verify=true")
	}
	_ = runner.Run(ctx, "kubectl", "config", "rename-cluster", "default", clusterName)
	_ = runner.Run(ctx, "kubectl", "config", "rename-user", "default", clusterName+"-admin")
	os.Unsetenv("KUBECONFIG")

	// 3. Merge
	home, _ := os.UserHomeDir()
	kubeDir := filepath.Join(home, ".kube")
	_ = os.MkdirAll(kubeDir, 0755)
	mainConfigPath := filepath.Join(kubeDir, "config")

	// Backup
	if _, err := os.Stat(mainConfigPath); err == nil {
		_ = copyFile(mainConfigPath, mainConfigPath+".bak")
	}

	os.Setenv("KUBECONFIG", fmt.Sprintf("%s:%s", mainConfigPath, tempFile.Name()))
	merged, err := runner.RunCombinedOutput(ctx, "kubectl", "config", "view", "--flatten")
	os.Unsetenv("KUBECONFIG")
	if err != nil {
		return fmt.Errorf("erro no merge do kubeconfig: %v", err)
	}

	if err := os.WriteFile(mainConfigPath, merged, 0600); err != nil {
		return err
	}

	slog.Debug("kubeconfig atualizado", "contexto", clusterName, "arquivo", mainConfigPath)
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}
