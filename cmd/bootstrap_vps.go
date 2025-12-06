/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"gopkg.in/yaml.v3"
)

// bootstrapVpsCmd represents the bootstrap vps command
var bootstrapVpsCmd = &cobra.Command{
	Use:   "vps",
	Short: "Provisiona um VPS com K3s e prepara para GitOps",
	Long: `Conecta via SSH a um servidor VPS (definido no .env), instala dependências,
configura firewall, instala K3s e configura o kubeconfig local.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🚀 Yby Bootstrap - Provisionamento de VPS"))
		fmt.Println("---------------------------------------")

		// 1. Carregar .env
		if err := godotenv.Load("../.env"); err != nil {
			fmt.Println(warningStyle.Render("⚠️  Arquivo .env não encontrado ou erro ao carregar. Usando variáveis de ambiente."))
		}

		host := os.Getenv("VPS_HOST")
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
		client, err := connectSSH(user, host, port)
		if err != nil {
			fmt.Printf("%s Erro na conexão SSH: %v\n", crossStyle.String(), err)
			return
		}
		defer client.Close()
		fmt.Println(checkStyle.Render("✅ Conexão SSH estabelecida!"))

		// 3. Preparar Servidor
		runStep(client, "Atualizando sistema e instalando dependências", `
			export DEBIAN_FRONTEND=noninteractive
			apt-get update -qq
			apt-get install -y -qq curl wget git htop nano ca-certificates gnupg lsb-release ufw iptables-persistent
			timedatectl set-timezone America/Sao_Paulo
			swapoff -a
			sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
		`)

		// 4. Firewall
		runStep(client, "Configurando Firewall (UFW)", `
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
		runStep(client, "Instalando Docker", `
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

		// Use flag value
		fmt.Printf("📦 Versão K3s alvo: %s\n", k3sVersion)

		runStep(client, "Instalando K3s", fmt.Sprintf(`
			if ! command -v k3s >/dev/null 2>&1; then
				curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="%s" K3S_TOKEN="%s" sh -s - server --cluster-init --write-kubeconfig-mode=644 --tls-san %s
			else
				echo "K3s já instalado"
			fi
		`, k3sVersion, k3sToken, host))

		// 7. Fetch Kubeconfig
		fmt.Println(stepStyle.Render("🔄 Configurando acesso local (kubeconfig)..."))
		if err := fetchKubeconfig(client, host); err != nil {
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
}

func connectSSH(user, host, port string) (*ssh.Client, error) {
	// Tenta usar SSH Agent primeiro
	socket := os.Getenv("SSH_AUTH_SOCK")
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar ao SSH Agent: %w", err)
	}
	agentClient := agent.NewClient(conn)

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Simplificação MVP
		Timeout:         10 * time.Second,
	}

	return ssh.Dial("tcp", net.JoinHostPort(host, port), config)
}

func runStep(client *ssh.Client, name, script string) {
	fmt.Printf("%s %s... ", stepStyle.Render("⚙️"), name)
	session, err := client.NewSession()
	if err != nil {
		fmt.Printf("\n%s Erro ao criar sessão: %v\n", crossStyle.String(), err)
		os.Exit(1)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(script); err != nil {
		fmt.Printf("\n%s Falha!\n%s\n", crossStyle.String(), stderr.String())
		os.Exit(1)
	}
	fmt.Printf("%s\n", checkStyle.String())
}

func fetchKubeconfig(client *ssh.Client, host string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run("cat /etc/rancher/k3s/k3s.yaml"); err != nil {
		return err
	}

	// 1. Preparar conteúdo
	content := b.String()
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
	exec.Command("kubectl", "config", "rename-context", "default", clusterName).Run()
	exec.Command("kubectl", "config", "set-cluster", "default", "--server=https://"+host+":6443").Run()
	exec.Command("kubectl", "config", "set-cluster", "default", "--insecure-skip-tls-verify=true").Run()
	exec.Command("kubectl", "config", "rename-cluster", "default", clusterName).Run()
	exec.Command("kubectl", "config", "rename-user", "default", clusterName+"-admin").Run()
	os.Unsetenv("KUBECONFIG")

	// 3. Merge
	home, _ := os.UserHomeDir()
	kubeDir := filepath.Join(home, ".kube")
	os.MkdirAll(kubeDir, 0755)
	mainConfigPath := filepath.Join(kubeDir, "config")

	// Backup
	if _, err := os.Stat(mainConfigPath); err == nil {
		copyFile(mainConfigPath, mainConfigPath+".bak")
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
