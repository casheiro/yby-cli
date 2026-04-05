package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockHTTPResponse cria um http.Response com o body e status fornecidos
func mockHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestUpgradeCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "upgrade", upgradeCmd.Use)
	assert.NotEmpty(t, upgradeCmd.Short)
	assert.NotEmpty(t, upgradeCmd.Long)
	assert.NotEmpty(t, upgradeCmd.Example)
	assert.NotNil(t, upgradeCmd.RunE)

	// Verificar flags
	f := upgradeCmd.Flags()
	assert.NotNil(t, f.Lookup("check"))
	assert.NotNil(t, f.Lookup("force"))
	assert.NotNil(t, f.Lookup("version"))
}

func TestUpgradeCmd_CheckOnly_SameVersion(t *testing.T) {
	origHTTP := httpGet
	defer func() { httpGet = origHTTP }()

	origVersion := Version
	defer func() { Version = origVersion }()
	Version = "v1.0.0"

	release := releaseInfo{
		TagName: "v1.0.0",
		Assets:  []releaseAsset{},
	}
	releaseJSON, _ := json.Marshal(release)

	httpGet = func(url string) (*http.Response, error) {
		return mockHTTPResponse(200, string(releaseJSON)), nil
	}

	upgradeCmd.Flags().Set("check", "true")
	defer upgradeCmd.Flags().Set("check", "false")

	err := upgradeCmd.RunE(upgradeCmd, []string{})
	assert.NoError(t, err)
}

func TestUpgradeCmd_CheckOnly_NewVersion(t *testing.T) {
	origHTTP := httpGet
	defer func() { httpGet = origHTTP }()

	origVersion := Version
	defer func() { Version = origVersion }()
	Version = "v0.9.0"

	release := releaseInfo{
		TagName: "v1.0.0",
		Assets:  []releaseAsset{},
	}
	releaseJSON, _ := json.Marshal(release)

	httpGet = func(url string) (*http.Response, error) {
		return mockHTTPResponse(200, string(releaseJSON)), nil
	}

	upgradeCmd.Flags().Set("check", "true")
	defer upgradeCmd.Flags().Set("check", "false")

	err := upgradeCmd.RunE(upgradeCmd, []string{})
	assert.NoError(t, err)
}

func TestUpgradeCmd_FetchError(t *testing.T) {
	origHTTP := httpGet
	defer func() { httpGet = origHTTP }()

	httpGet = func(url string) (*http.Response, error) {
		return nil, fmt.Errorf("rede indisponível")
	}

	err := upgradeCmd.RunE(upgradeCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao buscar informações do release")
}

func TestUpgradeCmd_AssetNotFound(t *testing.T) {
	origHTTP := httpGet
	defer func() { httpGet = origHTTP }()

	origVersion := Version
	defer func() { Version = origVersion }()
	Version = "v0.9.0"

	release := releaseInfo{
		TagName: "v1.0.0",
		Assets:  []releaseAsset{}, // sem assets
	}
	releaseJSON, _ := json.Marshal(release)

	httpGet = func(url string) (*http.Response, error) {
		return mockHTTPResponse(200, string(releaseJSON)), nil
	}

	upgradeCmd.Flags().Set("check", "false")

	err := upgradeCmd.RunE(upgradeCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrado no release")
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	// Criar um tar.gz com um binário "yby" falso
	binaryContent := []byte("#!/bin/sh\necho hello")
	tarGzData := createTestTarGz(t, "yby", binaryContent)

	result, err := extractBinaryFromTarGz(tarGzData)
	assert.NoError(t, err)
	assert.Equal(t, binaryContent, result)
}

func TestExtractBinaryFromTarGz_NotFound(t *testing.T) {
	// Criar um tar.gz sem o binário "yby"
	tarGzData := createTestTarGz(t, "outro-binario", []byte("data"))

	_, err := extractBinaryFromTarGz(tarGzData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrado")
}

func TestFetchReleaseFromURL_BadStatus(t *testing.T) {
	origHTTP := httpGet
	defer func() { httpGet = origHTTP }()

	httpGet = func(url string) (*http.Response, error) {
		return mockHTTPResponse(404, "not found"), nil
	}

	_, err := fetchReleaseFromURL("http://example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status HTTP 404")
}

func TestFetchChecksums(t *testing.T) {
	origHTTP := httpGet
	defer func() { httpGet = origHTTP }()

	checksumContent := "abc123  yby_v1.0.0_linux_amd64.tar.gz\ndef456  yby_v1.0.0_darwin_amd64.tar.gz\n"

	httpGet = func(url string) (*http.Response, error) {
		return mockHTTPResponse(200, checksumContent), nil
	}

	release := &releaseInfo{
		Assets: []releaseAsset{
			{Name: "checksums.txt", BrowserDownloadURL: "http://example.com/checksums.txt"},
		},
	}

	checksums, err := fetchChecksums(release)
	assert.NoError(t, err)
	assert.Equal(t, "abc123", checksums["yby_v1.0.0_linux_amd64.tar.gz"])
	assert.Equal(t, "def456", checksums["yby_v1.0.0_darwin_amd64.tar.gz"])
}

func TestFetchChecksums_NotFound(t *testing.T) {
	release := &releaseInfo{
		Assets: []releaseAsset{}, // sem checksums.txt
	}

	_, err := fetchChecksums(release)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksums.txt não encontrado")
}

// createTestTarGz cria um arquivo tar.gz em memória para testes
func createTestTarGz(t *testing.T, filename string, content []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	header := &tar.Header{
		Name: filename,
		Size: int64(len(content)),
		Mode: 0755,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("falha ao escrever header tar: %v", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatalf("falha ao escrever conteúdo tar: %v", err)
	}

	tarWriter.Close()
	gzWriter.Close()
	return buf.Bytes()
}

// Usar _ para evitar warning de import não utilizado
var _ = sha256.Sum256
var _ = hex.EncodeToString
