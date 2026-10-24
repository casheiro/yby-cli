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

	"github.com/spf13/pflag"
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

// rodarInitComFlags roda o init como a linha de comando: volta as flags ao padrão, lê args
// e chama o RunE. Roda dentro de dir (fora de qualquer repositório git) e devolve o erro.
func rodarInitComFlags(t *testing.T, dir string, args []string) error {
	_, err := rodarInitComSaida(t, dir, args)
	return err
}

// rodarInitComSaida é o rodarInitComFlags que devolve também o que o init escreveu.
func rodarInitComSaida(t *testing.T, dir string, args []string) (string, error) {
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
	saida := saidaDoComando(t, initCmd)

	anterior, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(anterior) }()
	err = initCmd.RunE(initCmd, nil)
	return saida.String(), err
}

// TestInitNaoInterativo_ArvoreIgualALinhaDeBase confere a árvore completa do
// --non-interactive (scaffold, values de cada ambiente e .yby/project.yaml) contra a do
// init de antes. A linha de base foi gerada com o `| default` dos templates trocado por
// if/else e sem parar nos campos críticos vazios, que é o que o scaffold faz agora.
func TestInitNaoInterativo_ArvoreIgualALinhaDeBase(t *testing.T) {
	for _, c := range casosArvoreInit {
		t.Run(c.nome, func(t *testing.T) {
			isolarInit(t)
			dir := t.TempDir()

			resultado := "resultado: ok\n"
			if err := rodarInitComFlags(t, dir, c.args); err != nil {
				resultado = "resultado: " + err.Error() + "\n"
			}
			conferirGolden(t, "init_arvore_"+c.nome, resultado+arvoreGerada(t, dir))
		})
	}
}

// Sem --git-repo, o init gera o projeto e avisa o repoURL vazio; no --offline, não avisa
// (o bootstrap aponta para o git-server local).
func TestInitNaoInterativo_AvisaRepoVazio(t *testing.T) {
	isolarInit(t)
	saida, err := rodarInitComSaida(t, t.TempDir(), casosArvoreInit[5].args)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(saida, "repoURL") || !strings.Contains(saida, "use --git-repo ou preencha antes do yby bootstrap") {
		t.Errorf("esperava o aviso do repoURL vazio:\n%s", saida)
	}

	isolarInit(t)
	saida, err = rodarInitComSaida(t, t.TempDir(), casosArvoreInit[0].args)
	if err != nil {
		t.Fatalf("init --offline: %v", err)
	}
	if strings.Contains(saida, "repoURL") {
		t.Errorf("no --offline o repoURL vazio não é aviso:\n%s", saida)
	}
}

// Sem helm na máquina, a validação vira um aviso só, sem repetir a falha por chart.
func TestInitNaoInterativo_SemHelm(t *testing.T) {
	isolarInit(t)
	lookPath = func(string) (string, error) { return "", fs.ErrNotExist }
	saida, err := rodarInitComSaida(t, t.TempDir(), casosArvoreInit[5].args)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(saida, "helm não encontrado · instale e rode yby validate") {
		t.Errorf("esperava o aviso do helm:%s", saida)
	}
	if strings.Contains(saida, "com problema") {
		t.Errorf("a falha do helm não deveria repetir por chart:%s", saida)
	}
}
