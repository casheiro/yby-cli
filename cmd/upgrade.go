/*
Copyright © 2025 Yby Team
*/
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
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// httpGet permite substituição em testes
var httpGet = http.Get

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
		checkOnly, _ := cmd.Flags().GetBool("check")
		force, _ := cmd.Flags().GetBool("force")
		targetVersion, _ := cmd.Flags().GetString("version")

		// Buscar informações do release
		var release *releaseInfo
		var err error

		if targetVersion != "" {
			release, err = fetchRelease(targetVersion)
		} else {
			release, err = fetchLatestRelease()
		}
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeNetworkTimeout, "falha ao buscar informações do release")
		}

		fmt.Printf("Versão atual: %s\n", Version)
		fmt.Printf("Versão disponível: %s\n", release.TagName)

		if !force && release.TagName == Version {
			fmt.Println("Você já está na versão mais recente.")
			return nil
		}

		if checkOnly {
			if release.TagName != Version {
				fmt.Println("Execute 'yby upgrade' para atualizar.")
			}
			return nil
		}

		// Determinar o asset correto
		assetName := fmt.Sprintf("yby_%s_%s_%s.tar.gz", release.TagName, runtime.GOOS, runtime.GOARCH)
		var assetURL string
		for _, a := range release.Assets {
			if a.Name == assetName {
				assetURL = a.BrowserDownloadURL
				break
			}
		}
		if assetURL == "" {
			return errors.New(errors.ErrCodeValidation,
				fmt.Sprintf("asset '%s' não encontrado no release %s", assetName, release.TagName))
		}

		// Buscar checksums
		checksums, err := fetchChecksums(release)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeNetworkTimeout, "falha ao buscar checksums")
		}

		// Download do asset
		fmt.Printf("Baixando %s...", assetName)
		assetData, err := downloadAsset(assetURL)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeNetworkTimeout, "falha ao baixar asset")
		}
		fmt.Println(" OK")

		// Verificar checksum
		fmt.Print("Verificando checksum...")
		expectedHash, ok := checksums[assetName]
		if !ok {
			return errors.New(errors.ErrCodeValidation, "checksum não encontrado para o asset")
		}

		actualHash := sha256.Sum256(assetData)
		actualHashStr := hex.EncodeToString(actualHash[:])
		if actualHashStr != expectedHash {
			return errors.New(errors.ErrCodeValidation,
				fmt.Sprintf("checksum inválido: esperado %s, obtido %s", expectedHash, actualHashStr))
		}
		fmt.Println(" OK")

		// Extrair binário do tar.gz
		binaryData, err := extractBinaryFromTarGz(assetData)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao extrair binário do arquivo")
		}

		// Self-replace com rollback
		execPath, err := os.Executable()
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao obter caminho do executável")
		}
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao resolver symlinks")
		}

		backupPath := execPath + ".bak"

		// Renomear binário atual para .bak
		if err := os.Rename(execPath, backupPath); err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao criar backup do binário atual")
		}

		// Escrever novo binário
		if err := os.WriteFile(execPath, binaryData, 0755); err != nil {
			// Rollback: restaurar backup
			_ = os.Rename(backupPath, execPath)
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao instalar novo binário")
		}

		// Remover backup
		_ = os.Remove(backupPath)

		fmt.Printf("Atualizado para %s!\n", release.TagName)
		return nil
	},
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
