package cmd

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/spf13/pflag"

	"github.com/casheiro/yby-cli/pkg/scaffold"
)

// casosArvoreInit são as combinações de flags do init --non-interactive cuja árvore gerada
// fica guardada em testdata/init_arvore_<nome>.golden. A linha de base foi capturada com o
// init de antes do assistente (commit a18740d), com os templates em LF como estão no
// repositório: os arquivos têm de continuar iguais byte a byte. Roda sem rede, sem IA, sem
// helm e sem os plugins do usuário (isolarInit).
var casosArvoreInit = []struct {
	nome string
	args []string
}{
	{"single_offline", []string{"--topology", "single", "--workflow", "essential", "--project-name", "app-single", "--offline"}},
	{"standard_gitflow_devcontainer", []string{"--topology", "standard", "--workflow", "gitflow", "--git-repo", "https://github.com/acme/loja.git", "--include-devcontainer"}},
	{"complete_trunk_modulos", []string{"--topology", "complete", "--workflow", "trunkbased", "--git-repo", "git@github.com:acme/pay.git", "--include-ci=false", "--enable-kepler", "--enable-minio", "--enable-keda", "--enable-metrics-server", "--secrets-strategy", "sealed-secrets"}},
	{"standard_essential_ambientes", []string{"--topology", "standard", "--workflow", "essential", "--project-name", "dados", "--environments", "local,dev,hom,prod", "--domain", "exemplo.com.br", "--email", "ops@exemplo.com.br", "--env", "hom", "--secrets-strategy", "sops"}},
	{"complete_gitflow_targetdir", []string{"--topology", "complete", "--workflow", "gitflow", "--target-dir", "infra", "--project-name", "api-gate", "--offline"}},
	{"single_trunk_env_ajustado", []string{"--topology", "single", "--workflow", "trunkbased", "--project-name", "svc-x", "--env", "prod", "--git-branch", "develop"}},
	{"standard_trunk_offline_gitlab", []string{"--topology", "standard", "--workflow", "trunkbased", "--git-repo", "https://gitlab.com/acme/estoque.git", "--offline", "--env", "prod", "--include-devcontainer", "--include-ci=false"}},
}

// reCriadoEm acha a data de criação do manifest, que muda a cada execução.
var reCriadoEm = regexp.MustCompile(`(?m)^(\s*createdAt: ).*$`)

// arvoreGerada lista os diretórios ("d caminho/") e os arquivos ("f sha256 caminho") sob
// raiz, em ordem de caminho. A data de criação dos .yby/project.yaml é trocada por <data>
// antes do hash.
func arvoreGerada(t *testing.T, raiz string) string {
	t.Helper()
	type entrada struct{ caminho, linha string }
	var entradas []entrada
	err := filepath.WalkDir(raiz, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(raiz, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			entradas = append(entradas, entrada{rel + "/", "d " + rel + "/"})
			return nil
		}
		dados, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if rel == ".yby/project.yaml" || strings.HasSuffix(rel, "/.yby/project.yaml") {
			dados = reCriadoEm.ReplaceAll(dados, []byte("${1}<data>"))
		}
		entradas = append(entradas, entrada{rel, fmt.Sprintf("f %x %s", sha256.Sum256(dados), rel)})
		return nil
	})
	if err != nil {
		t.Fatalf("lendo a árvore gerada: %v", err)
	}
	sort.Slice(entradas, func(i, j int) bool { return entradas[i].caminho < entradas[j].caminho })
	var b strings.Builder
	for _, e := range entradas {
		b.WriteString(e.linha + "\n")
	}
	return b.String()
}

// reDefaultSimples acha nos templates o `{{ .Campo | default "valor" }}`: o FuncMap do
// scaffold não tem a função default, e o scaffold para no primeiro template que a usa.
var reDefaultSimples = regexp.MustCompile(`\{\{\s*(\.[A-Za-z]+)\s*\|\s*default\s+"([^"]*)"\s*\}\}`)

// templatesSemDefault copia os assets de fsys para a memória trocando, nos .tmpl, o
// `{{ .Campo | default "valor" }}` por `{{ if .Campo }}{{ .Campo }}{{ else }}valor{{ end }}`,
// que dá o mesmo texto sem a função que falta.
func templatesSemDefault(t *testing.T, fsys fs.FS) fs.FS {
	t.Helper()
	m := fstest.MapFS{}
	err := fs.WalkDir(fsys, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			m[p] = &fstest.MapFile{Mode: fs.ModeDir | 0o755}
			return nil
		}
		dados, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		if strings.HasSuffix(p, ".tmpl") {
			dados = reDefaultSimples.ReplaceAll(dados, []byte(`{{ if $1 }}{{ $1 }}{{ else }}$2{{ end }}`))
		}
		m[p] = &fstest.MapFile{Data: dados, Mode: 0o644}
		return nil
	})
	if err != nil {
		t.Fatalf("copiando os templates: %v", err)
	}
	return m
}

// usarScaffoldCompleto faz o scaffold do init chegar ao fim: os templates passam por
// templatesSemDefault e o aviso da validação pós-render (campos críticos vazios, que o
// scaffold devolve como erro) não interrompe o init. Assim o teste cobre também o que o
// init faz depois do scaffold: values de cada ambiente, manifest e validação dos charts.
// Sai quando o template for corrigido: aí o init roda com scaffold.Apply puro.
func usarScaffoldCompleto(t *testing.T) {
	t.Helper()
	anterior := aplicarScaffoldInit
	t.Cleanup(func() { aplicarScaffoldInit = anterior })
	aplicarScaffoldInit = func(dir string, ctx *scaffold.BlueprintContext, fsys fs.FS) error {
		err := scaffold.Apply(dir, ctx, templatesSemDefault(t, fsys))
		if err != nil && strings.Contains(err.Error(), "validação pós-render falhou") {
			return nil
		}
		return err
	}
}

// rodarInitComFlags roda o init como a linha de comando: volta as flags ao padrão, lê args
// e chama o RunE. Roda dentro de dir (fora de qualquer repositório git) e devolve o erro.
func rodarInitComFlags(t *testing.T, dir string, args []string) error {
	t.Helper()
	f := initCmd.Flags()
	f.VisitAll(func(fl *pflag.Flag) {
		_ = fl.Value.Set(fl.DefValue)
		fl.Changed = false
	})
	t.Cleanup(func() {
		f.VisitAll(func(fl *pflag.Flag) {
			_ = fl.Value.Set(fl.DefValue)
			fl.Changed = false
		})
	})
	if err := f.Parse(append([]string{"--non-interactive"}, args...)); err != nil {
		t.Fatalf("flags: %v", err)
	}
	saidaDoComando(t, initCmd)

	anterior, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(anterior) }()
	return initCmd.RunE(initCmd, nil)
}

// TestInitNaoInterativo_ArvoreIgualALinhaDeBase confere a árvore completa do
// --non-interactive (scaffold, values de cada ambiente e .yby/project.yaml) contra a do
// init de antes, rodado com os mesmos templates sem default e o mesmo tratamento do aviso
// pós-render.
func TestInitNaoInterativo_ArvoreIgualALinhaDeBase(t *testing.T) {
	for _, c := range casosArvoreInit {
		t.Run(c.nome, func(t *testing.T) {
			isolarInit(t)
			usarScaffoldCompleto(t)
			dir := t.TempDir()

			resultado := "resultado: ok\n"
			if err := rodarInitComFlags(t, dir, c.args); err != nil {
				resultado = "resultado: " + err.Error() + "\n"
			}
			conferirGolden(t, "init_arvore_"+c.nome, resultado+arvoreGerada(t, dir))
		})
	}
}

// TestInitNaoInterativo_ArvoreComTemplatesAtuais confere a árvore que o init deixa com os
// templates de hoje, que param o scaffold em charts/bootstrap/values.yaml.tmpl: igual à do
// init de antes. Quando o template for corrigido, o init chega ao fim, este teste falha e
// deixa de ser necessário (o de cima passa a rodar com scaffold.Apply puro).
func TestInitNaoInterativo_ArvoreComTemplatesAtuais(t *testing.T) {
	isolarInit(t)
	dir := t.TempDir()

	resultado := "resultado: ok\n"
	if err := rodarInitComFlags(t, dir, casosArvoreInit[0].args); err != nil {
		resultado = "resultado: o scaffold parou\n"
	}
	conferirGolden(t, "init_arvore_templates_atuais", resultado+arvoreGerada(t, dir))
}
