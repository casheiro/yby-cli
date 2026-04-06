package mirror

import (
	"context"
	"fmt"
	"net"
	"time"
)

// HealthChecker verifica se uma conexão está ativa
type HealthChecker interface {
	Check(ctx context.Context, localPort int) error
}

type tcpHealthChecker struct {
	timeout time.Duration
}

// NewTCPHealthChecker cria um verificador de saúde via TCP
func NewTCPHealthChecker(timeout time.Duration) HealthChecker {
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	return &tcpHealthChecker{timeout: timeout}
}

func (h *tcpHealthChecker) Check(ctx context.Context, localPort int) error {
	addr := fmt.Sprintf("localhost:%d", localPort)
	conn, err := net.DialTimeout("tcp", addr, h.timeout)
	if err != nil {
		return fmt.Errorf("health check falhou em %s: %w", addr, err)
	}
	conn.Close()
	return nil
}
