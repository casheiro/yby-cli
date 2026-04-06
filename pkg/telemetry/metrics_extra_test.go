package telemetry

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFlush_SemEventos cobre o branch de events vazio no Flush (linha 51).
func TestFlush_SemEventos(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	// Flush com lista vazia não deve entrar em pânico
	assert.NotPanics(t, func() {
		Flush()
	})
}

func TestRecord_PreencheCamposDeTracing(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	t.Setenv("YBY_ENV", "staging")
	t.Setenv("YBY_CLUSTER", "cluster-a")

	Record("op-tracing", 50*time.Millisecond, nil)

	mu.Lock()
	require.Len(t, events, 1)
	e := events[0]
	mu.Unlock()

	assert.Equal(t, "staging", e.Environment)
	assert.Equal(t, "cluster-a", e.Cluster)
	assert.NotEmpty(t, e.UserID, "UserID deve ser preenchido com hash anonimizado")
	assert.NotEmpty(t, e.RequestID, "RequestID deve ser UUID gerado automaticamente")
	assert.Len(t, e.RequestID, 36, "RequestID deve ter formato UUID (36 chars)")
}

func TestToJSONEvent_IncluiCamposDeTracing(t *testing.T) {
	e := Event{
		Name:        "op-json",
		Duration:    100 * time.Millisecond,
		Success:     true,
		Timestamp:   time.Now(),
		Environment: "prod",
		Cluster:     "k3d-local",
		UserID:      "abc123",
		RequestID:   "550e8400-e29b-41d4-a716-446655440000",
	}

	je := toJSONEvent(e)
	assert.Equal(t, "prod", je.Environment)
	assert.Equal(t, "k3d-local", je.Cluster)
	assert.Equal(t, "abc123", je.UserID)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", je.RequestID)
}

func TestAnonymizedUserID_Deterministic(t *testing.T) {
	id1 := anonymizedUserID()
	id2 := anonymizedUserID()
	assert.Equal(t, id1, id2, "UserID deve ser determinístico para o mesmo usuário")
	assert.NotEmpty(t, id1)
	// SHA-256 truncado a 8 bytes = 16 chars hex
	assert.Len(t, id1, 16)
}

func TestFlushToFilePath_SemEventos(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")

	err := FlushToFilePath(true, path)
	require.NoError(t, err)

	// Arquivo NÃO deve ter sido criado (sem eventos)
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestFlushToFilePath_CamposTracingNoPersistido(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	t.Setenv("YBY_ENV", "dev")
	t.Setenv("YBY_CLUSTER", "minikube")

	Record("op-persist", 75*time.Millisecond, nil)

	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")

	err := FlushToFilePath(true, path)
	require.NoError(t, err)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	require.True(t, scanner.Scan())

	var je jsonEvent
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &je))
	assert.Equal(t, "dev", je.Environment)
	assert.Equal(t, "minikube", je.Cluster)
	assert.NotEmpty(t, je.UserID)
	assert.NotEmpty(t, je.RequestID)
}

func TestTrack_SemErro(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	func() {
		start := time.Now()
		defer Track("tracked-success", start, nil)
		time.Sleep(5 * time.Millisecond)
	}()

	mu.Lock()
	require.Len(t, events, 1)
	assert.True(t, events[0].Success)
	assert.Equal(t, "tracked-success", events[0].Name)
	mu.Unlock()
}

func TestRotateIfNeeded_ArquivoInexistente(t *testing.T) {
	err := rotateIfNeeded("/caminho/inexistente/arquivo.jsonl", 50)
	assert.NoError(t, err, "arquivo inexistente não deve causar erro")
}

func TestTelemetryFilePath(t *testing.T) {
	path, err := TelemetryFilePath()
	assert.NoError(t, err)
	assert.Contains(t, path, ".yby")
	assert.Contains(t, path, "telemetry.jsonl")
}

func TestFlushToFile_SemEventos(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	err := FlushToFile(true)
	assert.NoError(t, err)
}

func TestFlush_ComEventoSucesso(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	Record("flush-ok", 10*time.Millisecond, nil)
	assert.NotPanics(t, func() {
		Flush()
	})
}

func TestFlush_ComEventoFalha(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	Record("flush-err", 10*time.Millisecond, errors.New("algo deu errado"))
	assert.NotPanics(t, func() {
		Flush()
	})
}
