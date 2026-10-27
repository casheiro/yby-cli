package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/indexer"
)

func main() {
	os.Exit(executar())
}

// executar roda o plugin e devolve o código de saída; um erro sai no stderr no formato
// de ui.WriteError, e o core só repassa o código.
func executar() int {
	if err := handlePluginRequest(lerPedido(), os.Stdout, os.Stderr); err != nil {
		return sairComErro(os.Stderr, err)
	}
	return 0
}

// lerPedido lê o pedido do core (YBY_PLUGIN_REQUEST ou stdin) pelo SDK. Sem pedido, o
// binário roda sozinho, com o subcomando e as flags na linha de comando (sem nenhum, é a
// ajuda).
func lerPedido() plugin.PluginRequest {
	if err := sdk.Init(); err != nil {
		slog.Debug("sem pedido do yby; usando a linha de comando", "erro", err)
	} else if hook := sdk.GetHook(); hook != "" {
		return plugin.PluginRequest{Hook: hook, Args: sdk.GetArgs()}
	}
	return plugin.PluginRequest{Hook: "command", Args: os.Args[1:]}
}

// dicaDoLogDoPlugin é o jeito de ver o log no plugin: o --log-level do core não chega a
// este processo, e o YBY_LOG_LEVEL chega (configurarLog o aplica).
const dicaDoLogDoPlugin = "rode de novo com YBY_LOG_LEVEL=debug"

// sairComErro escreve err em w como ui.WriteError (um erro comum vira ERR_PLUGIN),
// depois de uma linha em branco que o separa do que já saiu, e devolve o código de saída.
// As dicas de depuração que mandam usar o --log-level viram a do YBY_LOG_LEVEL.
func sairComErro(w io.Writer, err error) int {
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) {
		ye = yerrors.Wrap(err, yerrors.ErrCodePlugin, "o Synapstor falhou")
	}
	ui.DefinirDicaDoLog(dicaDoLogDoPlugin)
	if strings.Contains(ye.GetHint(), "--log-level") {
		ye = ye.WithHint(dicaDoLogDoPlugin + " para ver os detalhes")
	}
	fmt.Fprintln(w)
	ui.WriteError(w, ye)
	return 1
}

// handlePluginRequest atende o hook pedido pelo core. Os hooks manifest e context
// respondem JSON no stdout; o command escreve para pessoas em saida, com o log em erros.
func handlePluginRequest(req plugin.PluginRequest, saida, erros io.Writer) error {
	switch req.Hook {
	case "manifest":
		respond(saida, plugin.PluginManifest{
			Name:        "synapstor",
			Version:     "1.0.0",
			Description: "Gestão de conhecimento do projeto: UKIs, busca semântica, nota de qualidade e exportação",
			Hooks:       []string{"command", "context"},
		})
		return nil
	case "context":
		handleContextHook(saida)
		return nil
	case "command":
		// Log e modo visual (tema, ASCII, sem cor) antes de qualquer saída.
		configurarLog()
		configurarModoVisual(os.Getenv)
		cwd, err := os.Getwd()
		if err != nil {
			return yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para ler a pasta atual")
		}
		return comando(context.Background(), cwd, req.Args, saida, erros)
	}
	return nil
}

// comando despacha os subcomandos de yby synapstor; raiz é a pasta do projeto.
func comando(ctx context.Context, raiz string, args []string, saida, erros io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		escreverAjuda(saida, "")
		return nil
	}
	sub, resto := args[0], args[1:]
	if _, ok := ajudas[sub]; !ok {
		return yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("o Synapstor não tem o comando %q", sub)).
			WithHint("rode yby synapstor --help para ver os comandos")
	}
	if pedeAjuda(resto) {
		escreverAjuda(saida, sub)
		return nil
	}
	switch sub {
	case "capture":
		return runCapture(ctx, raiz, resto, saida, erros)
	case "study":
		return runStudy(ctx, raiz, resto, saida, erros)
	case "search":
		return runSearch(ctx, raiz, resto, saida, erros)
	case "index":
		return runIndex(ctx, raiz, resto, saida, erros)
	case "quality":
		return runQuality(raiz, saida)
	case "decay":
		return runDecay(raiz, saida)
	default: // export
		return runExport(raiz, resto, saida, erros)
	}
}

// handleContextHook retorna dados de contexto do Synapstor para o sistema de plugins, com
// um só embrulho (o de respond): {"data": {"synapstor_…": …}}.
func handleContextHook(w io.Writer) {
	cwd, _ := os.Getwd()
	manifestPath := filepath.Join(cwd, ".synapstor", ".index_manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// Manifest não existe — retornar status "not_indexed"
		respond(w, map[string]interface{}{
			"synapstor_indexed_files": 0,
			"synapstor_last_indexed":  "",
			"synapstor_status":        "not_indexed",
		})
		return
	}

	var manifest indexer.IndexManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		respond(w, map[string]interface{}{
			"synapstor_indexed_files": 0,
			"synapstor_last_indexed":  "",
			"synapstor_status":        "not_indexed",
		})
		return
	}

	total := len(manifest.Files)
	var lastDate time.Time
	for _, f := range manifest.Files {
		if f.IndexedAt.After(lastDate) {
			lastDate = f.IndexedAt
		}
	}

	lastDateStr := ""
	if !lastDate.IsZero() {
		lastDateStr = lastDate.Format(time.RFC3339)
	}

	respond(w, map[string]interface{}{
		"synapstor_indexed_files": total,
		"synapstor_last_indexed":  lastDateStr,
		"synapstor_status":        "active",
	})
}

func respond(w io.Writer, data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	_ = json.NewEncoder(w).Encode(resp)
}
