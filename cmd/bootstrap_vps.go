/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/executor"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// bootstrapVpsCmd represents the bootstrap vps command
var bootstrapVpsCmd = &cobra.Command{
	Use:   "vps",
	Short: "Provisiona um VPS com K3s e prepara para GitOps",
	Long: `Conecta via SSH a um servidor VPS (definido no .env) ou executa localmente.
Instala dependências, configura firewall, instala K3s e configura o kubeconfig local.`,
	Run: func(cmd *cobra.Command, args []string) {
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
			// 1. Carregar .env
			if err := godotenv.Load("../.env"); err != nil {
				fmt.Println(warningStyle.Render("⚠️  Arquivo .env não encontrado ou erro ao carregar. Usando variáveis de ambiente."))
			}

			host = os.Getenv("VPS_HOST")
			user := os.Getenv("VPS_USER")
			if user == "" {
				user = "root"
			}
			port := os.Getenv("VPS_PORT")
			if port == "" {
				port = "22"
			}

			if host == "" {
				fmt.Println(crossStyle.Render("❌ Erro: VPS_HOST não definido no .env"))
				return
			}

			fmt.Printf("%s Conectando a %s@%s:%s...\n", stepStyle.Render("📡"), user, host, port)

			// 2. Conexão SSH
			var err error
			execClient, err = executor.NewSSHExecutor(user, host, port)
			if err != nil {
				fmt.Printf("%s Erro na conexão SSH: %v\n", crossStyle.String(), err)
				return
			}
		}
		defer execClient.Close()

		if !isLocal {
			fmt.Println(checkStyle.Render("✅ Conexão SSH estabelecida!"))
		}

		// 3. Preparar Servidor
		runEx(execClient, "Atualizando sistema e instalando dependências", `
			export DEBIAN_FRONTEND=noninteractive
			apt-get update -qq
			if ! command -v curl >/dev/null; then apt-get install -y -qq curl; fi
			apt-get install -y -qq wget git htop nano ca-certificates gnupg lsb-release ufw iptables-persistent
			timedatectl set-timezone America/Sao_Paulo
			swapoff -a
			sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
		`)

		// 4. Firewall
		runEx(execClient, "Configurando Firewall (UFW)", `
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
		`)

		// 5. Docker
		runEx(execClient, "Instalando Docker", `
			if ! command -v docker >/dev/null 2>&1; then
				curl -fsSL https://get.docker.com -o get-docker.sh
				sh get-docker.sh
				systemctl enable docker
				systemctl start docker
			else
				echo "Docker já instalado"
			fi
		`)

		// 6. K3s
		k3sToken := os.Getenv("K3S_TOKEN")
		if k3sToken == "" {
			k3sToken = fmt.Sprintf("yby-%d", time.Now().Unix())
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

		runEx(execClient, "Instalando K3s", fmt.Sprintf(`
			if ! command -v k3s >/dev/null 2>&1; then
				curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="%s" K3S_TOKEN="%s" sh -s - server --cluster-init --write-kubeconfig-mode=644 --tls-san %s
			else
				echo "K3s já instalado"
			fi
		`, k3sVersion, k3sToken, host))

		// 7. Fetch Kubeconfig
		fmt.Println(stepStyle.Render("🔄 Configurando acesso local (kubeconfig)..."))
		if err := fetchKubeconfig(execClient, host); err != nil {
			fmt.Printf("%s Erro ao configurar kubeconfig: %v\n", crossStyle.String(), err)
			return
		}

		fmt.Println("\n" + checkStyle.Render("🎉 Bootstrap VPS concluído com sucesso!"))
		fmt.Println("👉 Próximo passo: 'yby bootstrap cluster' para instalar a stack GitOps.")
	},
}

var k3sVersion string

func init() {
	bootstrapCmd.AddCommand(bootstrapVpsCmd)
	bootstrapVpsCmd.Flags().StringVar(&k3sVersion, "k3s-version", "v1.31.2+k3s1", "Versão do K3s a ser instalada")
	bootstrapVpsCmd.Flags().Bool("local", false, "Executa o bootstrap na máquina local (auto-provisionamento)")
}

func runEx(e executor.Executor, name, script string) {
	if err := e.Run(name, script); err != nil {
		os.Exit(1)
	}
}

func fetchKubeconfig(e executor.Executor, host string) error {
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
	_ = exec.Command("kubectl", "config", "rename-context", "default", clusterName).Run()
	_ = exec.Command("kubectl", "config", "set-cluster", "default", "--server=https://"+host+":6443").Run()
	_ = exec.Command("kubectl", "config", "set-cluster", "default", "--insecure-skip-tls-verify=true").Run()
	_ = exec.Command("kubectl", "config", "rename-cluster", "default", clusterName).Run()
	_ = exec.Command("kubectl", "config", "rename-user", "default", clusterName+"-admin").Run()
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

	cmd := exec.Command("kubectl", "config", "view", "--flatten")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s:%s", mainConfigPath, tempFile.Name()))

	var merged bytes.Buffer
	cmd.Stdout = &merged
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erro no merge do kubeconfig: %v", err)
	}

	if err := os.WriteFile(mainConfigPath, merged.Bytes(), 0600); err != nil {
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
	return os.WriteFile(dst, data, 0644)
}
