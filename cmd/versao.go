package cmd

import (
	"runtime/debug"
	"strings"
)

// lerBuildInfo lê as informações que o Go grava no binário (trocada nos testes).
var lerBuildInfo = debug.ReadBuildInfo

// infoVersao descreve o binário em execução.
type infoVersao struct {
	// Versao é a do release ("5.8.3", sem o "v", vinda do GoReleaser), a do módulo
	// gravada pelo go install ("v5.8.4" ou uma pseudo-versão) ou "dev".
	Versao string
	// Commit é o hash do commit ("none" quando não se sabe).
	Commit string
	// Data é a data do build no release ou a do commit no build info ("unknown" quando
	// não se sabe).
	Data string
	// DoBuildInfo diz que a versão saiu do build info, e não dos -ldflags do GoReleaser:
	// nesse caso Data é a do commit, não a do build.
	DoBuildInfo bool
}

// versaoInstalada devolve a versão do yby em execução. O release do GoReleaser traz
// Version, commit e date pelos -ldflags; o go install não passa -ldflags, e aí a versão
// sai do build info: a do módulo (v5.8.4 no go install …@v5.8.4) e, num build feito
// dentro do repositório, o commit (vcs.revision) e a data dele (vcs.time).
func versaoInstalada() infoVersao {
	info := infoVersao{Versao: Version, Commit: commit, Data: date}
	if Version != "dev" {
		return info
	}
	bi, ok := lerBuildInfo()
	if !ok || bi == nil {
		return info
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		info.Versao = v
		info.DoBuildInfo = true
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if info.Commit == "none" && s.Value != "" {
				info.Commit = s.Value
				if len(info.Commit) > 7 {
					info.Commit = info.Commit[:7]
				}
				info.DoBuildInfo = true
			}
		case "vcs.time":
			if info.Data == "unknown" && s.Value != "" {
				info.Data = s.Value
				info.DoBuildInfo = true
			}
		}
	}
	return info
}

// versaoSemV tira o "v" do começo: a tag do GitHub tem ("v5.8.3"), o nome dos assets e o
// Version do GoReleaser não ("5.8.3").
func versaoSemV(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// mesmaVersao compara duas versões sem ligar para o "v" do começo.
func mesmaVersao(a, b string) bool {
	a, b = versaoSemV(a), versaoSemV(b)
	return a != "" && a == b
}
