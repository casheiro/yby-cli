package executor

import (
	"log/slog"
	"os"
	"strings"

	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

// linhasDeDetalhe é quantas linhas finais da saída de um script que falhou aparecem
// abaixo da etapa; a saída inteira vai para o log em debug (--log-level=debug).
const linhasDeDetalhe = 12

// etapasPadrao escreve as etapas em os.Stdout, como ui.Steps.
func etapasPadrao() shared.Etapas { return ui.NovoProgresso(os.Stdout) }

// rodarEtapa roda executar como a etapa nome: ◈ enquanto roda, ✓ com a duração ao
// terminar e, na falha, ✕ com as últimas linhas da saída logo abaixo. A saída inteira,
// com sucesso ou falha, vai para o log em debug.
func rodarEtapa(etapas shared.Etapas, nome string, executar func() ([]byte, error)) error {
	etapas.Iniciar(nome)
	saida, err := executar()
	slog.Debug("saída do script", "etapa", nome, "saida", string(saida), "erro", err)
	if err != nil {
		etapas.Falhar("")
		etapas.Detalhe(ultimasLinhas(string(saida), linhasDeDetalhe))
		return err
	}
	etapas.Concluir("")
	return nil
}

// ultimasLinhas devolve as n últimas linhas não vazias de s.
func ultimasLinhas(s string, n int) string {
	var linhas []string
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			linhas = append(linhas, l)
		}
	}
	if len(linhas) > n {
		linhas = linhas[len(linhas)-n:]
	}
	return strings.Join(linhas, "\n")
}
