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
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/executor"
	"github.com/casheiro/yby-cli/pkg/services/shared"
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
		fmt.Println(titleStyle.Render("🚀 Yby Bootstrap - Provisionamento de VPS"))
		fmt.Println("---------------------------------------")

		// 0. Detect Mode
		isLocal, _ := cmd.Flags().GetBool("local")
		var execClient executor.Executor
		var host string

		if isLocal {
			fmt.Println(stepStyle.Render("📡 Modo Local Detectado"))
			execClient = executor.NewLocalExecutor()
			// For kubeconfig setup later, we use "localhost" or detect public IP?
			// For now, let's assume if local, we want kubeconfig to point to localhost or internal IP?
			// K3s writes to /etc/rancher/k3s/k3s.yaml.
			// The fetchKubeconfig logic assumes we want to MERGE with ~/.kube/config.
			host = "127.0.0.1"
		} else {
			// 1. Carregar configuração (Flag > Manifesto > Env (Legacy))
			host = vpsHost
			user := vpsUser
			port := vpsPort

			// Smart Local Detection:
			// If no host provided, and we are on Linux, ask if user wants to provision THIS machine.
			if host == "" && runtime.GOOS == "linux" {
				confirmLocal, _ := prompter.Confirm("Nenhum host remoto definido. Deseja provisionar ESTA máquina como uma VPS Yby (localhost)?", false)

				if confirmLocal {
					isLocal = true
					fmt.Println(stepStyle.Render("🔄 Alternando para modo Auto-Provisionamento (Local)"))
					// Re-initialize as local
					execClient = executor.NewLocalExecutor()
					host = "127.0.0.1"
				}
			}

			// Legacy .env support (Deprecation Warning)
			if host == "" {
				// Try loading .env only if strictly necessary
				if _, err := os.Stat("../.env"); err == nil {
					_ = godotenv.Load("../.env")
					if val := os.Getenv("VPS_HOST"); val != "" {
						fmt.Println(warningStyle.Render("⚠️  Usando VPS_HOST do arquivo .env (Depreciado). Use --host ou manifesto."))
						host = val
					}
					if user == "root" && os.Getenv("VPS_USER") != "" {
						user = os.Getenv("VPS_USER")
					}
				}
			}

			if host == "" {
				return errors.New(errors.ErrCodeValidation, "Host não definido. Use --host.")
			}

			fmt.Printf("%s Conectando a %s@%s:%s...\n", stepStyle.Render("📡"), user, host, port)

			// 2. Conexão SSH (Only if not swapped to local)
			if !isLocal {
				var err error
				execClient, err = executor.NewSSHExecutor(user, host, port)
				if err != nil {
					return errors.Wrap(err, errors.ErrCodeUnreachable, "Erro na conexão SSH")
				}
			}
		}
		defer execClient.Close()

		if !isLocal {
			fmt.Println(checkStyle.Render("✅ Conexão SSH estabelecida!"))
		}

		if err := runEx(execClient, "Verificando Requisitos Mínimos", `
			TOTAL_MEM_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
			# 4GB ~ 4000000 kB. Warning if < 3.8GB to be safe
			if [ "$TOTAL_MEM_KB" -lt 3800000 ]; then
				echo "⚠️  AVISO: Memória disponível ("$((TOTAL_MEM_KB/1024))" MB) é inferior a 4GB."
				echo "   A stack Yby completa requer no mínimo 4GB de RAM para estabilidade."
				echo "   Continuando em 5 segundos... (Ctrl+C para cancelar)"
				sleep 5
			else
				echo "✅ Memória OK: "$((TOTAL_MEM_KB/1024))" MB"
			fi
		`); err != nil {
			return err
		}

		// 4. Preparar Servidor
		if err := runEx(execClient, "Atualizando sistema e instalando dependências", `
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
		if err := runEx(execClient, "Configurando Firewall (UFW)", `
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
		if err := runEx(execClient, "Instalando Docker", `
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
				return errors.Wrap(err, errors.ErrCodeExec, "falha ao gerar token K3s seguro")
			}
			k3sToken = hex.EncodeToString(tokenBytes)
		}

		// 6. K3s Version Resolution
		// Priority: Flag > cluster-values.yaml > Default
		if !cmd.Flags().Changed("k3s-version") {
			if data, err := os.ReadFile("config/cluster-values.yaml"); err == nil {
				var config struct {
					System struct {
						K3s struct {
							Version string `yaml:"version"`
						} `yaml:"k3s"`
					} `yaml:"system"`
				}
				if err := yaml.Unmarshal(data, &config); err == nil && config.System.K3s.Version != "" {
					k3sVersion = config.System.K3s.Version
					fmt.Printf("📄 Usando versão K3s do cluster-values.yaml: %s\n", k3sVersion)
				}
			}
		}

		// 7. K3s Installation
		// Logic adjustment: for local mode, "host" for tls-san should ideally be the public IP or hostname.
		// Since we default host to 127.0.0.1 for local, we might want to also add the machine's hostname.
		// For MVP, lets keep 127.0.0.1 and maybe detected hostname if we can (via hostname command in script?)

		fmt.Printf("📦 Versão K3s alvo: %s\n", k3sVersion)

		if err := runEx(execClient, "Instalando K3s", fmt.Sprintf(`
			if ! command -v k3s >/dev/null 2>&1; then
				curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="%s" K3S_TOKEN="%s" sh -s - server --cluster-init --write-kubeconfig-mode=644 --tls-san %s
			else
				echo "K3s já instalado"
			fi
		`, k3sVersion, k3sToken, host)); err != nil {
			return err
		}

		// 7. Fetch Kubeconfig
		fmt.Println(stepStyle.Render("🔄 Configurando acesso local (kubeconfig)..."))
		runner := &shared.RealRunner{}
		if err := fetchKubeconfig(execClient, host, runner); err != nil {
			return errors.Wrap(err, errors.ErrCodeConfig, "Erro ao configurar kubeconfig")
		}

		fmt.Println("\n" + checkStyle.Render("🎉 Bootstrap VPS concluído com sucesso!"))
		fmt.Println("👉 Próximo passo: 'yby bootstrap cluster' para instalar a stack GitOps.")
		return nil
	},
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

func runEx(e executor.Executor, name, script string) error {
	if err := e.Run(name, script); err != nil {
		return errors.Wrap(err, errors.ErrCodeExec, fmt.Sprintf("Erro executando %s", name))
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
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "yby-prod"
	}

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

	fmt.Printf("%s Kubeconfig atualizado! Contexto: %s\n", checkStyle.String(), clusterName)
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}
