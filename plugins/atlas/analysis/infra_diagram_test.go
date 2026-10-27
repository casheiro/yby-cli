package analysis

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
	"github.com/casheiro/yby-cli/plugins/atlas/discovery/analyzers"
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

// copiarFixture copia a fixture testdata/<nome> do discovery para um
// diretório temporário, fora do git do repositório.
func copiarFixture(t *testing.T, nome string) string {
	t.Helper()
	origem := filepath.Join("..", "discovery", "testdata", nome)
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
	bp, err := discovery.ScanInfra(copiarFixture(t, "kustomize"), nil)
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "infra_kustomize_overview.mmd", GenerateInfraMermaid(bp, DetailOverview))
	conferirGolden(t, "infra_kustomize_full.mmd", GenerateInfraMermaid(bp, DetailFull))
}

// nosNoTexto conta as linhas de nó (as de quatro espaços dentro de um subgraph).
func nosNoTexto(mmd string) int {
	n := 0
	for _, linha := range strings.Split(mmd, "\n") {
		if strings.HasPrefix(linha, "    ") && strings.Contains(linha, "[\"") {
			n++
		}
	}
	return n
}

// arestasNoTexto conta as linhas de aresta do diagrama.
func arestasNoTexto(mmd string) int {
	return strings.Count(mmd, " -->|")
}

// conferirStats confere que as estatísticas batem com o que foi desenhado.
func conferirStats(t *testing.T, bp *discovery.InfraBlueprint, nivel DetailLevel, mmd string) InfraStats {
	t.Helper()
	stats := GetInfraStats(bp, nivel)
	if stats.Nodes != nosNoTexto(mmd) {
		t.Errorf("stats.Nodes = %d, mas o diagrama tem %d nós:\n%s", stats.Nodes, nosNoTexto(mmd), mmd)
	}
	if stats.Relations != arestasNoTexto(mmd) {
		t.Errorf("stats.Relations = %d, mas o diagrama tem %d arestas:\n%s", stats.Relations, arestasNoTexto(mmd), mmd)
	}
	return stats
}

// No overview, serviço do compose sem depends_on continua no diagrama; rede
// sem ninguém ligado a ela, não.
func TestGenerateInfraMermaid_OverviewMantemIsolados(t *testing.T) {
	bp := &discovery.InfraBlueprint{Resources: []analyzers.InfraResource{
		{Kind: "ComposeService", Name: "api"},
		{Kind: "ComposeService", Name: "db"},
		{Kind: "ComposeService", Name: "web"},
		{Kind: "ComposeNetwork", Name: "solta"},
	}}
	mmd := GenerateInfraMermaid(bp, DetailOverview)
	for _, nome := range []string{"ComposeService_api", "ComposeService_db", "ComposeService_web"} {
		if !strings.Contains(mmd, nome+"[") {
			t.Errorf("%s sumiu do overview:\n%s", nome, mmd)
		}
	}
	if strings.Contains(mmd, "ComposeNetwork_solta") {
		t.Errorf("rede isolada não deveria aparecer no overview:\n%s", mmd)
	}
	stats := conferirStats(t, bp, DetailOverview, mmd)
	if stats.VisibleResources != 3 || stats.TotalResources != 4 {
		t.Errorf("recursos: %d de %d, esperado 3 de 4", stats.VisibleResources, stats.TotalResources)
	}
}

// Terraform só com recursos soltos não deixa o overview vazio.
func TestGenerateInfraMermaid_OverviewSoComIsoladosSecundarios(t *testing.T) {
	bp := &discovery.InfraBlueprint{Resources: []analyzers.InfraResource{
		{Kind: "TerraformResource", Name: "aws_s3_bucket.logs"},
		{Kind: "TerraformResource", Name: "aws_instance.app"},
	}}
	mmd := GenerateInfraMermaid(bp, DetailOverview)
	if nosNoTexto(mmd) != 2 {
		t.Fatalf("esperados os 2 recursos do Terraform no overview:\n%s", mmd)
	}
	conferirStats(t, bp, DetailOverview, mmd)
}

// blueprintGrande monta 60 Deployments selecionados por um Service.
func blueprintGrande() *discovery.InfraBlueprint {
	bp := &discovery.InfraBlueprint{}
	bp.Resources = append(bp.Resources, analyzers.InfraResource{Kind: "Service", Name: "api"})
	for i := 0; i < 60; i++ {
		d := analyzers.InfraResource{Kind: "Deployment", Name: fmt.Sprintf("app-%02d", i)}
		bp.Resources = append(bp.Resources, d)
		bp.Relations = append(bp.Relations, analyzers.InfraRelation{From: "Service/api", To: d.ID(), Type: "selects"})
	}
	return bp
}

// Acima do teto, o overview agrupa por kind; o full continua com tudo.
func TestGenerateInfraMermaid_OverviewTeto(t *testing.T) {
	bp := blueprintGrande()

	mmd := GenerateInfraMermaid(bp, DetailOverview)
	if n := nosNoTexto(mmd); n > MaxNosOverview {
		t.Fatalf("overview com %d nós, acima do teto de %d:\n%s", n, MaxNosOverview, mmd)
	}
	if !strings.Contains(mmd, "60 recursos") {
		t.Errorf("os 60 Deployments deveriam virar um nó agrupado:\n%s", mmd)
	}
	if arestasNoTexto(mmd) != 1 {
		t.Errorf("as 60 arestas para o grupo deveriam virar uma só:\n%s", mmd)
	}
	conferirGolden(t, "infra_overview_teto.mmd", mmd)
	stats := conferirStats(t, bp, DetailOverview, mmd)
	if stats.VisibleResources != 61 || stats.Agrupados != 60 {
		t.Errorf("recursos %d (agrupados %d), esperado 61 (60)", stats.VisibleResources, stats.Agrupados)
	}

	full := GenerateInfraMermaid(bp, DetailFull)
	if n := nosNoTexto(full); n != 61 {
		t.Errorf("o full não tem teto: esperado 61 nós, obtido %d", n)
	}
	conferirStats(t, bp, DetailFull, full)
}

func TestGenerateInfraMermaid_StatsBatemComOsGoldens(t *testing.T) {
	bp, err := discovery.ScanInfra(escreverFixtureInfra(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, nivel := range []DetailLevel{DetailOverview, DetailFull} {
		conferirStats(t, bp, nivel, GenerateInfraMermaid(bp, nivel))
	}
}

func TestGenerateInfraMermaid_GoldenTerraform(t *testing.T) {
	bp, err := discovery.ScanInfra(copiarFixture(t, "terraform"), nil)
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "infra_terraform_overview.mmd", GenerateInfraMermaid(bp, DetailOverview))
	conferirGolden(t, "infra_terraform_full.mmd", GenerateInfraMermaid(bp, DetailFull))
}
