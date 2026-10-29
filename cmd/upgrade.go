/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
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
			codigo := errors.ErrCodeNetworkTimeout
			if strings.Contains(err.Error(), "HTTP 404") {
				// o servidor respondeu: não é falha de rede, é release ou tag inexistente
				codigo = errors.ErrCodeValidation
				// sem --version, o 404 é da busca do último release, não de uma tag
				dica = "nenhum release publicado foi encontrado; veja github.com/casheiro/yby-cli/releases"
				if targetVersion != "" {
					dica = "o release " + targetVersion + " não foi encontrado; confira a tag em --version"
				}
			}
			return errors.Wrap(err, codigo, "falha ao buscar informações do release").
				WithHint(dica)
		}

		// a tag vem com "v" (v5.8.3) e o Version do GoReleaser sem (5.8.3)
		instalada := versaoInstalada().Versao
		emDia := mesmaVersao(release.TagName, instalada)
		ui.Check(w, ui.CheckNeutro, "instalada", instalada, "")
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
		assetName := nomeDoAsset(release.TagName, sistemaUpgrade, arquiteturaUpgrade)
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
				WithHint("este sistema pode não ter binário nesse release; veja github.com/casheiro/yby-cli/releases")
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
				WithHint("o release está incompleto; baixe o binário à mão em github.com/casheiro/yby-cli/releases")
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
		binaryData, err := extrairBinario(assetData, sistemaUpgrade)
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

// pacoteDoUpgrade é o pacote que o upgrade baixa: o da variante em execução, para o
// yby-cloud não virar o yby padrão (trocada nos testes).
var pacoteDoUpgrade = varianteDoPacote

// nomeDoAsset devolve o nome do pacote do yby no release: o GoReleaser tira o "v" da tag
// (yby_5.8.3_linux_amd64.tar.gz, yby-cloud_5.8.3_linux_amd64.tar.gz) e empacota o
// Windows em zip.
func nomeDoAsset(tag, sistema, arquitetura string) string {
	extensao := "tar.gz"
	if sistema == "windows" {
		extensao = "zip"
	}
	return fmt.Sprintf("%s_%s_%s_%s.%s", pacoteDoUpgrade, versaoSemV(tag), sistema, arquitetura, extensao)
}

// extrairBinario tira o executável do pacote do release: yby.exe (ou yby-cloud.exe) do
// zip no Windows, yby (ou yby-cloud) do tar.gz nos outros sistemas.
func extrairBinario(pacote []byte, sistema string) ([]byte, error) {
	if sistema == "windows" {
		return extractBinaryFromZip(pacote, pacoteDoUpgrade+".exe")
	}
	return extractBinaryFromTarGz(pacote)
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
	return execPath, trocarExecutavel(execPath, binario)
}

// trocarExecutavel põe binario no lugar de execPath. O atual vira execPath+".bak" (o
// Windows deixa renomear o executável em uso, não apagar nem sobrescrever) e volta se a
// escrita falhar. No fim o .bak é apagado; no Windows ele é o executável em uso e só sai
// depois que o yby fechar (removerDepois). Um .bak que tenha ficado de um upgrade anterior
// é apagado antes.
func trocarExecutavel(execPath string, binario []byte) error {
	backupPath := execPath + ".bak"

	if err := osRemove(backupPath); err != nil && !stderrors.Is(err, fs.ErrNotExist) {
		slog.Debug("backup antigo do yby não foi apagado", "caminho", backupPath, "erro", err)
	}

	// Renomear binário atual para .bak
	if err := osRename(execPath, backupPath); err != nil {
		return errors.Wrap(err, errors.ErrCodeIO, "falha ao criar backup do binário atual").
			WithHint(dicaRemocao(err, dicaPermissaoUpgrade()))
	}

	// Escrever novo binário
	if err := osWriteFile(execPath, binario, 0755); err != nil {
		// Rollback: restaurar backup
		_ = osRename(backupPath, execPath)
		return errors.Wrap(err, errors.ErrCodeIO, "falha ao instalar novo binário").
			WithHint("o binário anterior foi restaurado; rode yby upgrade de novo")
	}

	// Remover backup: no Windows ele é o executável em uso
	if err := osRemove(backupPath); err != nil && !stderrors.Is(err, fs.ErrNotExist) {
		if err := removerDepois(backupPath); err != nil {
			slog.Debug("remoção do backup do yby não foi agendada", "caminho", backupPath, "erro", err)
		}
	}
	return nil
}

// osWriteFile escreve o binário novo (trocada nos testes).
var osWriteFile = os.WriteFile

// dicaPermissaoUpgrade é a dica quando falta permissão na pasta do executável.
func dicaPermissaoUpgrade() string {
	if sistemaUpgrade == "windows" {
		return "abra o terminal como administrador e rode yby upgrade de novo"
	}
	return "sem permissão na pasta do executável: rode com sudo yby upgrade"
}

// fetchLatestRelease busca o último release do GitHub
func fetchLatestRelease() (*releaseInfo, error) {
	return fetchReleaseFromURL("https://api.github.com/repos/casheiro/yby-cli/releases/latest")
}

// fetchRelease busca um release específico por tag. A tag do GitHub tem o "v" (v5.8.3);
// --version 5.8.3 também vale, como no yby plugin install.
func fetchRelease(tag string) (*releaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/casheiro/yby-cli/releases/tags/%s", tagDoRelease(tag))
	return fetchReleaseFromURL(url)
}

// tagDoRelease põe o "v" da tag do GitHub numa versão que começa por dígito.
func tagDoRelease(versao string) string {
	versao = strings.TrimSpace(versao)
	if versao != "" && versao[0] >= '0' && versao[0] <= '9' {
		return "v" + versao
	}
	return versao
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
		if baseName == pacoteDoUpgrade && header.Typeflag == tar.TypeReg {
			return io.ReadAll(tarReader)
		}
	}
	return nil, fmt.Errorf("binário '%s' não encontrado no arquivo", pacoteDoUpgrade)
}

// extractBinaryFromZip extrai o arquivo nome (em qualquer pasta) de um zip, o formato dos
// pacotes do Windows no release.
func extractBinaryFromZip(data []byte, nome string) ([]byte, error) {
	leitor, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir zip: %w", err)
	}
	for _, f := range leitor.File {
		// os nomes no zip usam "/" em todo sistema
		if f.FileInfo().IsDir() || path.Base(f.Name) != nome {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("falha ao ler %s do zip: %w", nome, err)
		}
		defer r.Close()
		return io.ReadAll(r)
	}
	return nil, fmt.Errorf("binário '%s' não encontrado no arquivo", nome)
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().Bool("check", false, "Apenas verificar se há atualização disponível")
	upgradeCmd.Flags().Bool("force", false, "Forçar atualização mesmo se já estiver na versão mais recente")
	upgradeCmd.Flags().String("version", "", "Versão específica para instalar (ex: v0.6.0)")
}
