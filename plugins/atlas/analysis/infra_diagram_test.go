package analysis

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

// escreverFixtureInfra monta um projeto com um compose de 5 serviços e 2 charts Helm.
func escreverFixtureInfra(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	escrever := func(rel, conteudo string) {
		caminho := filepath.Join(raiz, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escrever("docker-compose.yml", `services:
  web:
    image: nginx
    depends_on: [api, cache]
    networks: [front, back]
  api:
    image: api
    depends_on: [db, cache]
    networks: [back]
  db:
    image: postgres
    volumes: [dados:/var/lib/postgresql]
  cache:
    image: redis
  worker:
    image: worker
    depends_on: [db, cache]
networks:
  front: {}
  back: {}
volumes:
  dados: {}
`)
	for _, c := range []string{"alfa", "beta"} {
		escrever("charts/"+c+"/Chart.yaml", fmt.Sprintf("apiVersion: v2\nname: %s\nversion: 0.1.0\n", c))
	}
	return raiz
}

// A saída do diagrama não pode variar entre execuções: a ordem dos mapas dos
// analyzers não pode vazar para o .mmd.
func TestGenerateInfraMermaid_Deterministico(t *testing.T) {
	raiz := escreverFixtureInfra(t)
	for _, nivel := range []DetailLevel{DetailOverview, DetailFull} {
		var primeira string
		for i := 0; i < 20; i++ {
			bp, err := discovery.ScanInfra(raiz, nil)
			if err != nil {
				t.Fatal(err)
			}
			saida := GenerateInfraMermaid(bp, nivel)
			if saida == "" {
				t.Fatal("diagrama vazio")
			}
			if i == 0 {
				primeira = saida
				continue
			}
			if saida != primeira {
				t.Fatalf("execução %d (nível %v) diferente da primeira:\n%s\n---\n%s", i, nivel, primeira, saida)
			}
		}
	}
}

// atualizarGolden regrava os goldens: go test ./plugins/atlas/analysis/ -update
var atualizarGolden = flag.Bool("update", false, "regrava os arquivos golden em testdata/")

// conferirGolden compara obtido com testdata/<nome>; com -update, regrava o arquivo.
func conferirGolden(t *testing.T, nome, obtido string) {
	t.Helper()
	caminho := filepath.Join("testdata", nome)
	if *atualizarGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(obtido), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	esperado, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("golden %s ausente (rode com -update): %v", caminho, err)
	}
	if string(esperado) != obtido {
		t.Errorf("saída diferente de %s (rode com -update para regravar):\n%s", caminho, obtido)
	}
}

func TestGenerateInfraMermaid_Golden(t *testing.T) {
	bp, err := discovery.ScanInfra(escreverFixtureInfra(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "infra_overview.mmd", GenerateInfraMermaid(bp, DetailOverview))
	conferirGolden(t, "infra_full.mmd", GenerateInfraMermaid(bp, DetailFull))
}

// copiarFixtureKustomize copia a fixture base+overlays do discovery para um
// diretório temporário, fora do git do repositório.
func copiarFixtureKustomize(t *testing.T) string {
	t.Helper()
	origem := filepath.Join("..", "discovery", "testdata", "kustomize")
	raiz := t.TempDir()
	err := filepath.WalkDir(origem, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(origem, p)
		if err != nil {
			return err
		}
		dados, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		destino := filepath.Join(raiz, rel)
		if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destino, dados, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return raiz
}

func TestGenerateInfraMermaid_GoldenKustomize(t *testing.T) {
	bp, err := discovery.ScanInfra(copiarFixtureKustomize(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "infra_kustomize_overview.mmd", GenerateInfraMermaid(bp, DetailOverview))
	conferirGolden(t, "infra_kustomize_full.mmd", GenerateInfraMermaid(bp, DetailFull))
}
