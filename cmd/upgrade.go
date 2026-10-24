/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
)

// newUpgradeHTTPClient cria um http.Client com timeout e suporte a CA customizado via YBY_CA_BUNDLE.
var newUpgradeHTTPClient = func() (*http.Client, error) {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if caPath := os.Getenv("YBY_CA_BUNDLE"); caPath != "" {
		caCert, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("falha ao ler CA bundle '%s': %w", caPath, err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("falha ao adicionar certificados de '%s' ao pool de CAs", caPath)
		}
		tlsCfg.RootCAs = pool
	}

	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}, nil
}

// httpGet permite substituição em testes
var httpGet = func(url string) (*http.Response, error) {
	client, err := newUpgradeHTTPClient()
	if err != nil {
		return nil, err
	}
	return client.Get(url)
}

// sistemaUpgrade e arquiteturaUpgrade escolhem o asset do release (substituíveis em
// testes, para o golden ser o mesmo em toda plataforma).
var sistemaUpgrade, arquiteturaUpgrade = runtime.GOOS, runtime.GOARCH

// releaseInfo representa os dados relevantes de um release do GitHub
type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

// releaseAsset representa um asset de release do GitHub
type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// upgradeCmd representa o comando de atualização automática
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Atualiza o Yby CLI para a versão mais recente",
	Long: `Verifica e instala a versão mais recente do Yby CLI.

Busca o último release no GitHub, verifica o checksum SHA256
e substitui o binário atual.`,
	Example: `  yby upgrade --check
  yby upgrade
  yby upgrade --version v0.6.0`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cmd.OutOrStdout()
		checkOnly, _ := cmd.Flags().GetBool("check")
		force, _ := cmd.Flags().GetBool("force")
		targetVersion, _ := cmd.Flags().GetString("version")

		ui.Title(w, "upgrade", sistemaUpgrade+"/"+arquiteturaUpgrade)

		// Buscar informações do release
		var release *releaseInfo
		var err error

		if targetVersion != "" {
			release, err = fetchRelease(targetVersion)
		} else {
			release, err = fetchLatestRelease()
		}
		if err != nil {
			dica := "confira a conexão com a internet; atrás de proxy com CA própria, use YBY_CA_BUNDLE"
			if strings.Contains(err.Error(), "HTTP 404") {
				// sem --version, o 404 é da busca do último release, não de uma tag
				dica = "nenhum release publicado foi encontrado; veja github.com/casheiro-org/yby-cli/releases"
				if targetVersion != "" {
					dica = "o release " + targetVersion + " não foi encontrado; confira a tag em --version"
				}
			}
			return errors.Wrap(err, errors.ErrCodeNetworkTimeout, "falha ao buscar informações do release").
				WithHint(dica)
		}

		emDia := release.TagName == Version
		ui.Check(w, ui.CheckNeutro, "instalada", Version, "")
		switch {
		case emDia:
			ui.Check(w, ui.CheckOk, "disponível", release.TagName, "já é a mais recente")
		case checkOnly:
			ui.Check(w, ui.CheckAviso, "disponível", release.TagName, "yby upgrade atualiza")
		default:
			ui.Check(w, ui.CheckAviso, "disponível", release.TagName, "")
		}

		if checkOnly || (!force && emDia) {
			return nil
		}

		// Determinar o asset correto
		assetName := fmt.Sprintf("yby_%s_%s_%s.tar.gz", release.TagName, sistemaUpgrade, arquiteturaUpgrade)
		var assetURL string
		for _, a := range release.Assets {
			if a.Name == assetName {
				assetURL = a.BrowserDownloadURL
				break
			}
		}
		if assetURL == "" {
			return errors.New(errors.ErrCodeValidation,
				fmt.Sprintf("asset '%s' não encontrado no release %s", assetName, release.TagName)).
				WithHint("este sistema pode não ter binário nesse release; veja github.com/casheiro-org/yby-cli/releases")
		}

		// Buscar checksums
		checksums, err := fetchChecksums(release)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeNetworkTimeout, "falha ao buscar checksums").
				WithHint("tente de novo em instantes; o release pode estar sendo publicado")
		}

		fmt.Fprintln(w)
		prog := novoProgresso(w, "download", "checksum", "instalação")
		parar := animar(prog)
		defer parar()

		// Download do asset
		prog.Iniciar("download")
		assetData, err := downloadAsset(assetURL)
		if err != nil {
			prog.Falhar("")
			return errors.Wrap(err, errors.ErrCodeNetworkTimeout, "falha ao baixar asset").
				WithHint("confira a conexão e rode yby upgrade de novo")
		}
		prog.Concluir(assetName)

		// Verificar checksum
		prog.Iniciar("checksum")
		expectedHash, ok := checksums[assetName]
		if !ok {
			prog.Falhar("")
			return errors.New(errors.ErrCodeValidation, "checksum não encontrado para o asset").
				WithHint("o release está incompleto; baixe o binário à mão em github.com/casheiro-org/yby-cli/releases")
		}

		actualHash := sha256.Sum256(assetData)
		actualHashStr := hex.EncodeToString(actualHash[:])
		if actualHashStr != expectedHash {
			prog.Falhar("")
			// os hashes vão no contexto: no título, quebrariam a linha do erro
			return errors.New(errors.ErrCodeValidation, "checksum do download não confere").
				WithContext("esperado", expectedHash).
				WithContext("obtido", actualHashStr).
				WithHint("o download veio corrompido ou alterado; rode yby upgrade de novo")
		}
		prog.Concluir("sha256 confere")

		// Extrair binário do tar.gz e trocar o executável
		prog.Iniciar("instalação")
		binaryData, err := extractBinaryFromTarGz(assetData)
		if err != nil {
			prog.Falhar("")
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao extrair o binário do pacote").
				WithHint("rode yby upgrade de novo; o pacote pode ter vindo incompleto")
		}
		execPath, err := instalarBinario(binaryData)
		if err != nil {
			prog.Falhar("")
			return err
		}
		prog.Concluir(execPath)
		parar()

		fmt.Fprintln(w)
		ui.Check(w, ui.CheckOk, "atualizado para "+release.TagName, "", "")
		return nil
	},
}

// instalarBinario troca o executável atual por binario, com backup e rollback, e devolve
// o caminho instalado. Variável para os testes não tocarem no binário de verdade.
var instalarBinario = func(binario []byte) (string, error) {
	// Self-replace com rollback
	execPath, err := os.Executable()
	if err != nil {
		return "", errors.Wrap(err, errors.ErrCodeIO, "falha ao localizar o binário atual").
			WithHint("baixe a versão nova da página de releases e troque o binário à mão")
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", errors.Wrap(err, errors.ErrCodeIO, "falha ao seguir o link do binário atual").
			WithHint("baixe a versão nova da página de releases e troque o binário à mão")
	}

	backupPath := execPath + ".bak"

	// Renomear binário atual para .bak
	if err := os.Rename(execPath, backupPath); err != nil {
		return "", errors.Wrap(err, errors.ErrCodeIO, "falha ao criar backup do binário atual").
			WithHint("sem permissão na pasta do executável? rode com sudo ou reinstale pelo instalador")
	}

	// Escrever novo binário
	if err := os.WriteFile(execPath, binario, 0755); err != nil {
		// Rollback: restaurar backup
		_ = os.Rename(backupPath, execPath)
		return "", errors.Wrap(err, errors.ErrCodeIO, "falha ao instalar novo binário").
			WithHint("o binário anterior foi restaurado; rode yby upgrade de novo")
	}

	// Remover backup
	_ = os.Remove(backupPath)
	return execPath, nil
}

// fetchLatestRelease busca o último release do GitHub
func fetchLatestRelease() (*releaseInfo, error) {
	return fetchReleaseFromURL("https://api.github.com/repos/casheiro-org/yby-cli/releases/latest")
}

// fetchRelease busca um release específico por tag
func fetchRelease(tag string) (*releaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/casheiro-org/yby-cli/releases/tags/%s", tag)
	return fetchReleaseFromURL(url)
}

// fetchReleaseFromURL busca informações de release de uma URL
func fetchReleaseFromURL(url string) (*releaseInfo, error) {
	resp, err := httpGet(url)
	if err != nil {
		return nil, fmt.Errorf("falha na requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status HTTP %d ao buscar release", resp.StatusCode)
	}

	var release releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta: %w", err)
	}
	return &release, nil
}

// fetchChecksums busca e parseia o arquivo checksums.txt do release
func fetchChecksums(release *releaseInfo) (map[string]string, error) {
	var checksumURL string
	for _, a := range release.Assets {
		if a.Name == "checksums.txt" {
			checksumURL = a.BrowserDownloadURL
			break
		}
	}
	if checksumURL == "" {
		return nil, fmt.Errorf("arquivo checksums.txt não encontrado no release")
	}

	resp, err := httpGet(checksumURL)
	if err != nil {
		return nil, fmt.Errorf("falha ao baixar checksums: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler checksums: %w", err)
	}

	checksums := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			checksums[parts[1]] = parts[0]
		}
	}
	return checksums, nil
}

// downloadAsset baixa um asset do GitHub
func downloadAsset(url string) ([]byte, error) {
	resp, err := httpGet(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status HTTP %d ao baixar asset", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// extractBinaryFromTarGz extrai o binário "yby" de um arquivo tar.gz
func extractBinaryFromTarGz(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir gzip: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("falha ao ler tar: %w", err)
		}

		baseName := filepath.Base(header.Name)
		if baseName == "yby" && header.Typeflag == tar.TypeReg {
			return io.ReadAll(tarReader)
		}
	}
	return nil, fmt.Errorf("binário 'yby' não encontrado no arquivo")
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().Bool("check", false, "Apenas verificar se há atualização disponível")
	upgradeCmd.Flags().Bool("force", false, "Forçar atualização mesmo se já estiver na versão mais recente")
	upgradeCmd.Flags().String("version", "", "Versão específica para instalar (ex: v0.6.0)")
}
