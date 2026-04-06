package telemetry

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordAndFlush(t *testing.T) {
	// Reset global state for test
	mu.Lock()
	events = nil
	mu.Unlock()

	// Test success record
	Record("test-success", 100*time.Millisecond, nil)

	// Test failure record
	errFail := errors.New("test failure")
	Record("test-fail", 200*time.Millisecond, errFail)

	mu.Lock()
	assert.Len(t, events, 2)
	assert.Equal(t, "test-success", events[0].Name)
	assert.True(t, events[0].Success)
	assert.Equal(t, "test-fail", events[1].Name)
	assert.False(t, events[1].Success)
	assert.Equal(t, errFail, events[1].Error)
	mu.Unlock()

	// Flush should not panic (output goes to slog)
	assert.NotPanics(t, func() {
		Flush()
	})
}

func TestTrack(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	err := func() (err error) {
		start := time.Now()
		defer Track("tracked-op", start, &err)
		time.Sleep(10 * time.Millisecond)
		return errors.New("tracked error")
	}()

	assert.Error(t, err)

	mu.Lock()
	assert.Len(t, events, 1)
	assert.Equal(t, "tracked-op", events[0].Name)
	assert.False(t, events[0].Success)
	assert.GreaterOrEqual(t, events[0].Duration, 10*time.Millisecond)
	mu.Unlock()
}

func TestFlushToFile_Enabled(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")

	Record("operacao-teste", 150*time.Millisecond, nil)
	Record("operacao-falha", 200*time.Millisecond, errors.New("erro teste"))

	err := FlushToFilePath(true, path)
	require.NoError(t, err)

	// Verificar que o arquivo JSONL foi criado com conteúdo válido
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var parsed []jsonEvent
	for scanner.Scan() {
		var je jsonEvent
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &je))
		parsed = append(parsed, je)
	}
	require.NoError(t, scanner.Err())

	assert.Len(t, parsed, 2)
	assert.Equal(t, "operacao-teste", parsed[0].Name)
	assert.True(t, parsed[0].Success)
	assert.Equal(t, int64(150), parsed[0].DurationMs)
	assert.Empty(t, parsed[0].Error)

	assert.Equal(t, "operacao-falha", parsed[1].Name)
	assert.False(t, parsed[1].Success)
	assert.Equal(t, "erro teste", parsed[1].Error)
}

func TestFlushToFile_Disabled(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")

	Record("operacao", 100*time.Millisecond, nil)

	err := FlushToFilePath(false, path)
	require.NoError(t, err)

	// Arquivo NÃO deve existir
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestFlushToFile_Append(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")

	// Primeira chamada
	Record("op-1", 100*time.Millisecond, nil)
	err := FlushToFilePath(true, path)
	require.NoError(t, err)

	// Segunda chamada (sem limpar events, acumula)
	Record("op-2", 200*time.Millisecond, nil)
	err = FlushToFilePath(true, path)
	require.NoError(t, err)

	// Verificar que ambas as chamadas foram acumuladas
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	count := 0
	for scanner.Scan() {
		count++
	}
	// Primeira chamada: 1 evento, segunda chamada: 2 eventos (1 antigo + 1 novo) = 3 linhas
	assert.Equal(t, 3, count)
}

func TestRotateIfNeeded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")

	// Criar arquivo que excede maxBytes
	bigData := make([]byte, 100)
	require.NoError(t, os.WriteFile(path, bigData, 0644))

	err := rotateIfNeeded(path, 50) // maxBytes = 50, arquivo tem 100
	require.NoError(t, err)

	// Arquivo original deve ter sido renomeado
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))

	// Arquivo .1 deve existir
	_, err = os.Stat(path + ".1")
	assert.NoError(t, err)
}

func TestRotateIfNeeded_ArquivoPequeno(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")

	smallData := make([]byte, 10)
	require.NoError(t, os.WriteFile(path, smallData, 0644))

	err := rotateIfNeeded(path, 50)
	require.NoError(t, err)

	// Arquivo original deve continuar existindo (não rotacionou)
	_, err = os.Stat(path)
	assert.NoError(t, err)

	// Arquivo .1 NÃO deve existir
	_, err = os.Stat(path + ".1")
	assert.True(t, os.IsNotExist(err))
}

func TestExportEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")

	content := `{"name":"op1","duration_ms":100,"success":true,"timestamp":"2025-01-01T00:00:00Z"}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	data, err := ExportEvents(path)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestExportEvents_ArquivoInexistente(t *testing.T) {
	data, err := ExportEvents("/caminho/inexistente/telemetry.jsonl")
	assert.NoError(t, err)
	assert.Nil(t, data)
}
