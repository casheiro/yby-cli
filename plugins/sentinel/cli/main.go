//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"github.com/casheiro/yby-cli/pkg/ui"
)

var execCommand = exec.Command

func main() {
	os.Exit(executar())
}

// executar roda o plugin e devolve o código de saída; um erro sai no stderr no formato
// de ui.WriteError, e o core só repassa o código.
func executar() int {
	// O SDK lê o pedido do core (YBY_PLUGIN_REQUEST ou stdin). Sem ele, o hook fica vazio
	// e o plugin não faz nada; o aviso vai para o log, fora do stdout do manifest.
	if err := sdk.Init(); err != nil {
		slog.Warn("não deu para ler o pedido do yby", "erro", err)
	}
	if err := handlePluginRequest(os.Stdout, os.Stderr); err != nil {
		return sairComErro(os.Stderr, err)
	}
	return 0
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
		ye = yerrors.Wrap(err, yerrors.ErrCodePlugin, "o Sentinel falhou")
	}
	ui.DefinirDicaDoLog(dicaDoLogDoPlugin)
	if strings.Contains(ye.GetHint(), "--log-level") {
		ye = ye.WithHint(dicaDoLogDoPlugin + " para ver os detalhes")
	}
	fmt.Fprintln(w)
	ui.WriteError(w, ye)
	return 1
}

// AnalysisResult define a estrutura esperada da resposta da IA
type AnalysisResult struct {
	RootCause       string  `json:"root_cause"`
	TechnicalDetail string  `json:"technical_detail"`
	Confidence      int     `json:"confidence"`
	SuggestedFix    string  `json:"suggested_fix"`
	KubectlPatch    *string `json:"kubectl_patch"`
}

// handlePluginRequest atende o hook pedido pelo core. saida recebe o resultado e erros,
// as etapas quando o resultado é para scripts (json ou markdown no stdout).
func handlePluginRequest(saida, erros io.Writer) error {
	switch sdk.GetHook() {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:        "sentinel",
			Version:     "1.0.0",
			Description: "Auditoria de segurança K8s com scan de vulnerabilidades e investigação por IA",
			Hooks:       []string{"command"},
		})
		return nil
	case "command":
		// Log e modo visual (tema, ASCII, sem cor) antes de qualquer saída.
		configurarLog()
		configurarModoVisual(os.Getenv)
		return comando(context.Background(), sdk.GetArgs(), saida, erros)
	default:
		return nil
	}
}

// comando despacha os subcomandos de yby sentinel.
func comando(ctx context.Context, args []string, saida, erros io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		escreverAjuda(saida, "")
		return nil
	}
	sub, resto := args[0], args[1:]
	switch sub {
	case "scan", "investigate":
	default:
		return yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("o Sentinel não tem o comando %q", sub)).
			WithHint("rode yby sentinel --help para ver os comandos")
	}
	if pedeAjuda(resto) {
		escreverAjuda(saida, sub)
		return nil
	}
	op, err := lerOpcoes(sub, resto)
	if err != nil {
		return err
	}
	if sub == "scan" {
		return scanNamespace(ctx, saida, erros, op)
	}

	// Pod e namespace: flag > contexto do yby > default.
	fullCtx := sdk.GetFullContext()
	if op.pod == "" && fullCtx != nil {
		if p, ok := fullCtx.Data["pod"]; ok {
			op.pod = fmt.Sprintf("%v", p)
		}
	}
	if op.namespace == "" {
		if fullCtx != nil && fullCtx.Infra.Namespace != "" {
			op.namespace = fullCtx.Infra.Namespace
		} else {
			op.namespace = "default"
		}
	}
	if op.pod == "" {
		return yerrors.New(yerrors.ErrCodeValidation, "falta o nome do pod").
			WithHint("use yby sentinel investigate <pod> [-n namespace]")
	}
	return investigate(ctx, saida, erros, op)
}

// opcoes são as flags de scan e investigate.
type opcoes struct {
	pod, namespace, formato, arquivo, perfil string
	semCache, fix, fixDryRun                 bool
}

// lerOpcoes lê as flags de sub, na forma "--flag valor" ou "--flag=valor". Flags que o
// comando não conhece são ignoradas, como sempre foram (o -c do core pode chegar até aqui).
func lerOpcoes(sub string, args []string) (opcoes, error) {
	var op opcoes
	ajuda := "rode yby sentinel " + sub + " --help para ver as flags"
	for i := 0; i < len(args); i++ {
		a, junto, comIgual := args[i], "", false
		if strings.HasPrefix(a, "--") {
			if nome, v, ok := strings.Cut(a, "="); ok {
				a, junto, comIgual = nome, v, true
			}
		}
		var destino *string
		var liga *bool
		switch {
		case a == "-n" || a == "--namespace":
			destino = &op.namespace
		case a == "-o" || a == "--output":
			destino = &op.formato
		case a == "-f" || a == "--file":
			destino = &op.arquivo
		case sub == "scan" && (a == "-p" || a == "--profile"):
			destino = &op.perfil
		case sub == "scan" && a == "--fix":
			liga = &op.fix
		case sub == "scan" && a == "--fix-dry-run":
			liga = &op.fixDryRun
		case sub == "investigate" && a == "--no-cache":
			liga = &op.semCache
		case sub == "investigate" && !strings.HasPrefix(a, "-") && op.pod == "":
			op.pod = a
		}
		switch {
		case liga != nil && comIgual:
			return op, yerrors.New(yerrors.ErrCodeValidation, a+" não recebe valor").WithHint(ajuda)
		case liga != nil:
			*liga = true
		case destino != nil && comIgual:
			if junto == "" {
				return op, yerrors.New(yerrors.ErrCodeValidation, "falta o valor de "+a).WithHint(ajuda)
			}
			*destino = junto
		case destino != nil:
			if i+1 >= len(args) || args[i+1] == "" {
				return op, yerrors.New(yerrors.ErrCodeValidation, "falta o valor de "+a).WithHint(ajuda)
			}
			*destino = args[i+1]
			i++
		}
	}
	switch op.formato {
	case "", "terminal", "json", "markdown":
	default:
		return op, yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("formato de saída desconhecido: %s", op.formato)).
			WithHint("use -o terminal, json ou markdown")
	}
	if op.formato == "terminal" {
		op.formato = ""
	}
	return op, nil
}

// paraScripts informa se o resultado vai para o stdout num formato para scripts (json ou
// markdown sem arquivo): nesse caso o título e as etapas saem do caminho.
func (op opcoes) paraScripts() bool {
	return op.formato != "" && op.arquivo == ""
}

func respond(data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}
