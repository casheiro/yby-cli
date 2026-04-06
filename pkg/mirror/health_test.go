package mirror

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTCPHealthChecker(t *testing.T) {
	hc := NewTCPHealthChecker(5 * time.Second)
	require.NotNil(t, hc, "deve criar instância não nula")
}

func TestTCPHealthChecker_TimeoutDefault(t *testing.T) {
	// Ao passar timeout 0, deve usar 3s como padrão
	hc := NewTCPHealthChecker(0)
	require.NotNil(t, hc)

	// Verifica que o timeout interno é 3s
	checker, ok := hc.(*tcpHealthChecker)
	require.True(t, ok, "deve ser do tipo *tcpHealthChecker")
	assert.Equal(t, 3*time.Second, checker.timeout, "timeout padrão deve ser 3s")
}

func TestTCPHealthChecker_PortaAberta(t *testing.T) {
	// Abre um listener TCP em porta aleatória
	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "deve conseguir abrir listener")
	defer ln.Close()

	// Extrai a porta alocada
	addr := ln.Addr().(*net.TCPAddr)
	port := addr.Port

	hc := NewTCPHealthChecker(2 * time.Second)
	ctx := context.Background()

	err = hc.Check(ctx, port)
	assert.NoError(t, err, "health check deve passar com porta aberta")
}

func TestTCPHealthChecker_PortaFechada(t *testing.T) {
	// Usa uma porta alta que provavelmente não tem listener
	// Abre e fecha imediatamente para garantir que está livre
	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	addr := ln.Addr().(*net.TCPAddr)
	port := addr.Port
	ln.Close() // fecha imediatamente — porta fica sem listener

	hc := NewTCPHealthChecker(500 * time.Millisecond)
	ctx := context.Background()

	err = hc.Check(ctx, port)
	assert.Error(t, err, "health check deve falhar com porta fechada")
	assert.Contains(t, err.Error(), "health check falhou")
}
