package cloud

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAuditLogger_Log(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewAuditLoggerWithPath(path)

	event := CloudAuditEvent{
		Timestamp: time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
		Action:    "authenticate",
		Provider:  "aws",
		Identity:  "arn:aws:iam::123:user/admin",
		Method:    "sso",
		Success:   true,
	}

	if err := logger.Log(event); err != nil {
		t.Fatalf("erro ao registrar evento: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("erro ao ler arquivo: %v", err)
	}

	var parsed CloudAuditEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("erro ao decodificar JSON: %v", err)
	}

	if parsed.Action != "authenticate" {
		t.Errorf("action esperado 'authenticate', obtido '%s'", parsed.Action)
	}
	if parsed.Provider != "aws" {
		t.Errorf("provider esperado 'aws', obtido '%s'", parsed.Provider)
	}
	if !parsed.Success {
		t.Error("success esperado true, obtido false")
	}
}

func TestAuditLogger_LogAuthentication(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewAuditLoggerWithPath(path)

	err := logger.LogAuthentication("azure", "user@contoso.com", "sso", true, nil)
	if err != nil {
		t.Fatalf("erro ao registrar autenticação: %v", err)
	}

	events, err := logger.ReadEvents(time.Time{})
	if err != nil {
		t.Fatalf("erro ao ler eventos: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("esperado 1 evento, obtido %d", len(events))
	}

	e := events[0]
	if e.Action != "authenticate" {
		t.Errorf("action esperado 'authenticate', obtido '%s'", e.Action)
	}
	if e.Provider != "azure" {
		t.Errorf("provider esperado 'azure', obtido '%s'", e.Provider)
	}
	if e.Identity != "user@contoso.com" {
		t.Errorf("identity esperado 'user@contoso.com', obtido '%s'", e.Identity)
	}
	if e.Error != "" {
		t.Errorf("error esperado vazio, obtido '%s'", e.Error)
	}

	// Testar com erro
	testErr := fmt.Errorf("credenciais inválidas")
	err = logger.LogAuthentication("aws", "user", "default", false, testErr)
	if err != nil {
		t.Fatalf("erro ao registrar autenticação com falha: %v", err)
	}

	events, _ = logger.ReadEvents(time.Time{})
	if len(events) != 2 {
		t.Fatalf("esperado 2 eventos, obtido %d", len(events))
	}
	if events[1].Error != "credenciais inválidas" {
		t.Errorf("error esperado 'credenciais inválidas', obtido '%s'", events[1].Error)
	}
}

func TestAuditLogger_ReadEvents_WithSince(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewAuditLoggerWithPath(path)

	// Registrar eventos em datas diferentes
	old := CloudAuditEvent{
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Action:    "authenticate",
		Provider:  "aws",
		Identity:  "old-user",
		Success:   true,
	}
	recent := CloudAuditEvent{
		Timestamp: time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
		Action:    "refresh",
		Provider:  "gcp",
		Identity:  "recent-user",
		Success:   true,
	}

	_ = logger.Log(old)
	_ = logger.Log(recent)

	// Filtrar desde março
	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	events, err := logger.ReadEvents(since)
	if err != nil {
		t.Fatalf("erro ao ler eventos: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("esperado 1 evento filtrado, obtido %d", len(events))
	}
	if events[0].Identity != "recent-user" {
		t.Errorf("esperado 'recent-user', obtido '%s'", events[0].Identity)
	}

	// Sem filtro: retorna todos
	all, _ := logger.ReadEvents(time.Time{})
	if len(all) != 2 {
		t.Errorf("esperado 2 eventos sem filtro, obtido %d", len(all))
	}
}

func TestAuditLogger_ReadEvents_FileNotFound(t *testing.T) {
	logger := NewAuditLoggerWithPath("/tmp/audit-inexistente-test.log")
	events, err := logger.ReadEvents(time.Time{})
	if err != nil {
		t.Fatalf("erro inesperado para arquivo inexistente: %v", err)
	}
	if events != nil {
		t.Errorf("esperado nil para arquivo inexistente, obtido %v", events)
	}
}

func TestAuditLogger_Rotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := &AuditLogger{
		filePath:    path,
		maxFileSize: 500, // Tamanho pequeno para forçar rotação
	}

	// Escrever eventos até exceder o limite
	for i := 0; i < 20; i++ {
		event := CloudAuditEvent{
			Timestamp: time.Now(),
			Action:    "authenticate",
			Provider:  "aws",
			Identity:  fmt.Sprintf("arn:aws:iam::%d:user/admin", i),
			Success:   true,
		}
		if err := logger.Log(event); err != nil {
			t.Fatalf("erro ao registrar evento %d: %v", i, err)
		}
	}

	// Verificar que o arquivo .1 foi criado
	rotatedPath := path + ".1"
	if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
		t.Error("arquivo rotacionado (.1) não foi criado")
	}

	// Arquivo principal deve existir e ser menor que o rotacionado
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("arquivo principal não existe após rotação: %v", err)
	}
	if info.Size() == 0 {
		t.Error("arquivo principal está vazio após rotação")
	}
}

func TestAuditLogger_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewAuditLoggerWithPath(path)

	event := CloudAuditEvent{
		Timestamp: time.Now(),
		Action:    "authenticate",
		Provider:  "aws",
		Identity:  "test",
		Success:   true,
	}
	_ = logger.Log(event)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("erro ao verificar arquivo: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("permissão esperada 0600, obtida %04o", perm)
	}
}

func TestAuditLogger_Export_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewAuditLoggerWithPath(path)

	event := CloudAuditEvent{
		Timestamp: time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
		Action:    "authenticate",
		Provider:  "aws",
		Identity:  "test-user",
		Success:   true,
	}
	_ = logger.Log(event)

	var buf bytes.Buffer
	if err := logger.Export("json", time.Time{}, &buf); err != nil {
		t.Fatalf("erro ao exportar JSON: %v", err)
	}

	var exported []CloudAuditEvent
	if err := json.Unmarshal(buf.Bytes(), &exported); err != nil {
		t.Fatalf("erro ao decodificar JSON exportado: %v", err)
	}

	if len(exported) != 1 {
		t.Fatalf("esperado 1 evento exportado, obtido %d", len(exported))
	}
	if exported[0].Identity != "test-user" {
		t.Errorf("identity esperado 'test-user', obtido '%s'", exported[0].Identity)
	}
}

func TestAuditLogger_Export_CSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewAuditLoggerWithPath(path)

	event := CloudAuditEvent{
		Timestamp: time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
		Action:    "assume-role",
		Provider:  "aws",
		Identity:  "test-user",
		Role:      "admin-role",
		Success:   true,
	}
	_ = logger.Log(event)

	var buf bytes.Buffer
	if err := logger.Export("csv", time.Time{}, &buf); err != nil {
		t.Fatalf("erro ao exportar CSV: %v", err)
	}

	reader := csv.NewReader(strings.NewReader(buf.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("erro ao ler CSV: %v", err)
	}

	// Header + 1 registro
	if len(records) != 2 {
		t.Fatalf("esperado 2 linhas (header + evento), obtido %d", len(records))
	}

	header := records[0]
	if header[0] != "timestamp" || header[1] != "action" {
		t.Errorf("header inesperado: %v", header)
	}

	row := records[1]
	if row[1] != "assume-role" {
		t.Errorf("action esperado 'assume-role', obtido '%s'", row[1])
	}
	if row[4] != "admin-role" {
		t.Errorf("role esperado 'admin-role', obtido '%s'", row[4])
	}
}

func TestAuditLogger_Export_InvalidFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewAuditLoggerWithPath(path)

	_ = logger.Log(CloudAuditEvent{
		Timestamp: time.Now(),
		Action:    "authenticate",
		Provider:  "aws",
		Identity:  "test",
		Success:   true,
	})

	var buf bytes.Buffer
	err := logger.Export("xml", time.Time{}, &buf)
	if err == nil {
		t.Error("esperado erro para formato inválido")
	}
}

func TestAuditLogger_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewAuditLoggerWithPath(path)

	var wg sync.WaitGroup
	errCh := make(chan error, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := CloudAuditEvent{
				Timestamp: time.Now(),
				Action:    "authenticate",
				Provider:  "aws",
				Identity:  fmt.Sprintf("user-%d", idx),
				Success:   true,
			}
			if err := logger.Log(event); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("erro em escrita concorrente: %v", err)
	}

	events, err := logger.ReadEvents(time.Time{})
	if err != nil {
		t.Fatalf("erro ao ler eventos: %v", err)
	}
	if len(events) != 50 {
		t.Errorf("esperado 50 eventos, obtido %d", len(events))
	}
}
