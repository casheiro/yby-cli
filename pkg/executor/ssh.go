package executor

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshClient define a interface para o cliente SSH (permite mock em testes)
type sshClient interface {
	NewSession() (*ssh.Session, error)
	Close() error
}

// SSHExecutor roda os scripts num servidor remoto por SSH.
type SSHExecutor struct {
	client sshClient
	// Etapas recebe o andamento de cada Run; nil escreve em os.Stdout como ui.Steps.
	Etapas shared.Etapas
}

func (e *SSHExecutor) etapas() shared.Etapas {
	if e.Etapas == nil {
		e.Etapas = etapasPadrao()
	}
	return e.Etapas
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
	return rodarEtapa(e.etapas(), name, func() ([]byte, error) {
		session, err := e.client.NewSession()
		if err != nil {
			return nil, err
		}
		defer session.Close()

		var stdout, stderr bytes.Buffer
		session.Stdout = &stdout
		session.Stderr = &stderr

		err = session.Run(script)
		// o stderr vai por último: as linhas de baixo, mostradas na falha, são as do erro
		return append(stdout.Bytes(), stderr.Bytes()...), err
	})
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
