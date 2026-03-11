package executor

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Styles for consistent output (copying from cmd package or redefining)
// Since styles are private in cmd, we might need to duplicate or export them.
// For now, let's redefine simple ones or pass a logger.
// To keep it simple, we'll just use fmt for now or simple lipgloss if needed.
var (
	stepStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))  // Blue
	checkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // Green
	crossStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
)

// sshClient define a interface para o cliente SSH (permite mock em testes)
type sshClient interface {
	NewSession() (*ssh.Session, error)
	Close() error
}

type SSHExecutor struct {
	client sshClient
}

func NewSSHExecutor(user, host, port string) (*SSHExecutor, error) {
	// Tenta usar SSH Agent primeiro
	socket := os.Getenv("SSH_AUTH_SOCK")
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar ao SSH Agent: %w", err)
	}
	agentClient := agent.NewClient(conn)

	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("falha ao obter diretório home: %w", err)
	}
	knownHostsFile := filepath.Join(homedir, ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar known_hosts; adicione o host com 'ssh-keyscan HOST >> ~/.ssh/known_hosts': %w", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(host, port), config)
	if err != nil {
		return nil, err
	}

	return &SSHExecutor{client: client}, nil
}

func (e *SSHExecutor) Run(name, script string) error {
	fmt.Printf("%s %s... ", stepStyle.Render("⚙️"), name)
	session, err := e.client.NewSession()
	if err != nil {
		fmt.Printf("\n%s Erro ao criar sessão: %v\n", crossStyle.String(), err)
		return err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(script); err != nil {
		fmt.Printf("\n%s Falha!\n%s\n", crossStyle.String(), stderr.String())
		return err
	}
	fmt.Printf("%s\n", checkStyle.String())
	return nil
}

func (e *SSHExecutor) FetchFile(path string) ([]byte, error) {
	session, err := e.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b
	safeCmd := fmt.Sprintf("cat -- '%s'", strings.ReplaceAll(path, "'", "'\\''"))
	if err := session.Run(safeCmd); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func (e *SSHExecutor) Close() error {
	return e.client.Close()
}
