/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"fmt"
	"io"
	"time"

	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
)

var cloudStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Exibe o status de credenciais dos provedores cloud",
	Example: `  yby cloud status`,
	RunE:    runCloudStatus,
}

func init() {
	cloudCmd.AddCommand(cloudStatusCmd)
}

// statusProviderCloud é o que cloud status levantou de um provider.
type statusProviderCloud struct {
	nome, versao string
	status       *cloud.CredentialStatus
	err          error
}

func runCloudStatus(cmd *cobra.Command, args []string) error {
	runner := &shared.RealRunner{}
	providers := detectarProvidersCloud(gocontext.Background(), runner)

	var levantados []statusProviderCloud
	for _, p := range providers {
		item := statusProviderCloud{nome: p.Name()}
		if version, verErr := p.CLIVersion(gocontext.Background()); verErr == nil {
			item.versao = version
		}

		// Validar credenciais com timeout de 10s
		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 10*time.Second)
		item.status, item.err = p.ValidateCredentials(ctx)
		cancel()
		levantados = append(levantados, item)
	}

	renderCloudStatus(cmd.OutOrStdout(), levantados, agora())
	return nil
}

// renderCloudStatus escreve uma seção por provider com o CLI, as credenciais, o método e
// a validade do token.
func renderCloudStatus(w io.Writer, providers []statusProviderCloud, quando time.Time) {
	ui.Title(w, "cloud", "status")
	if len(providers) == 0 {
		fmt.Fprintln(w)
		ui.Check(w, ui.CheckNeutro, "nenhum CLI de cloud detectado · instale aws, az ou gcloud", "", "")
		return
	}

	// as colunas valem para todas as seções, como em renderChecks
	secoes := make([][]linhaCheck, len(providers))
	var todas []linhaCheck
	for i, p := range providers {
		secoes[i] = linhasStatusProvider(p, quando)
		todas = append(todas, secoes[i]...)
	}
	col := colunasDe(todas...)
	for i, p := range providers {
		ui.Section(w, p.nome)
		checksCom(w, col, secoes[i]...)
	}
}

// linhasStatusProvider monta as linhas da seção de um provider em cloud status.
func linhasStatusProvider(p statusProviderCloud, quando time.Time) []linhaCheck {
	var linhas []linhaCheck
	if p.versao != "" {
		linhas = append(linhas, linhaCheck{ui.CheckNeutro, "cli", p.versao, ""})
	}
	if p.err != nil || p.status == nil || !p.status.Authenticated {
		nota := cloud.DicaDeLogin(p.nome)
		if p.err != nil {
			nota = p.err.Error()
		}
		return append(linhas, linhaCheck{ui.CheckErro, "credenciais", "sem login", nota})
	}

	linhas = append(linhas, linhaCheck{ui.CheckOk, "credenciais", "válidas", p.status.Identity})
	if p.status.Method != "" {
		linhas = append(linhas, linhaCheck{ui.CheckNeutro, "método", p.status.Method, ""})
	}
	if exp := p.status.ExpiresAt; exp != nil {
		validade := exp.In(quando.Location()).Format("02/01/2006 15:04")
		if restante := exp.Sub(quando); restante > 0 {
			linhas = append(linhas, linhaCheck{ui.CheckOk, "token", "válido", "até " + validade + " · faltam " + formatDuration(restante)})
		} else {
			linhas = append(linhas, linhaCheck{ui.CheckAviso, "token", "expirado", "desde " + validade + " · yby cloud refresh renova"})
		}
	}
	return linhas
}

// formatDuration formata uma duração em formato legível.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, minutes)
}
