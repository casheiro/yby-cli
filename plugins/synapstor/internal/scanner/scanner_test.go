package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestScan_DiretorioVazio verifica que escanear um diretório vazio retorna zero resultados.
func TestScan_DiretorioVazio(t *testing.T) {
	tmpDir := t.TempDir()

	results, err := Scan(tmpDir, "")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("esperado 0 resultados em diretório vazio, obtido %d", len(results))
	}
}

// TestScan_EncontraArquivosPorNome verifica que a busca por nome encontra arquivos relevantes.
func TestScan_EncontraArquivosPorNome(t *testing.T) {
	tmpDir := t.TempDir()

	// Criar arquivo com nome que contém a query
	if err := os.WriteFile(filepath.Join(tmpDir, "bootstrap.go"), []byte("package bootstrap\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Criar arquivo que NÃO contém a query
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := Scan(tmpDir, "bootstrap")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("esperado pelo menos 1 resultado para query 'bootstrap'")
	}

	// Verificar que encontrou o arquivo correto
	encontrou := false
	for _, r := range results {
		if r.Path == "bootstrap.go" {
			encontrou = true
			break
		}
	}
	if !encontrou {
		t.Error("esperado encontrar 'bootstrap.go' nos resultados")
	}
}

// TestScan_EncontraArquivosPorConteudo verifica que a busca encontra arquivos
// cujo conteúdo contém a query, mesmo que o nome não contenha.
func TestScan_EncontraArquivosPorConteudo(t *testing.T) {
	tmpDir := t.TempDir()

	// Arquivo cujo conteúdo contém a query
	if err := os.WriteFile(filepath.Join(tmpDir, "config.go"), []byte("// Este arquivo gerencia kubernetes\npackage config\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := Scan(tmpDir, "kubernetes")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("esperado pelo menos 1 resultado para query 'kubernetes' no conteúdo")
	}

	encontrou := false
	for _, r := range results {
		if r.Path == "config.go" {
			encontrou = true
			break
		}
	}
	if !encontrou {
		t.Error("esperado encontrar 'config.go' nos resultados")
	}
}

// TestScan_QueryVazia_RetornaTodos verifica que uma query vazia retorna todos
// os arquivos de texto do diretório.
func TestScan_QueryVazia_RetornaTodos(t *testing.T) {
	tmpDir := t.TempDir()

	arquivos := []string{"a.go", "b.txt", "c.md"}
	for _, nome := range arquivos {
		if err := os.WriteFile(filepath.Join(tmpDir, nome), []byte("conteudo de "+nome), 0644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := Scan(tmpDir, "")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	if len(results) != len(arquivos) {
		t.Errorf("esperado %d resultados, obtido %d", len(arquivos), len(results))
	}
}

// TestScan_IgnoraDiretoriosConhecidos verifica que diretórios como .git,
// node_modules e vendor são ignorados.
func TestScan_IgnoraDiretoriosConhecidos(t *testing.T) {
	tmpDir := t.TempDir()

	dirsIgnorados := []string{".git", "node_modules", "vendor", "dist", ".synapstor"}
	for _, dir := range dirsIgnorados {
		dirPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dirPath, "test.txt"), []byte("deve ser ignorado"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Criar arquivo legítimo
	if err := os.WriteFile(filepath.Join(tmpDir, "real.txt"), []byte("arquivo real"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := Scan(tmpDir, "")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	// Deve encontrar apenas real.txt
	if len(results) != 1 {
		t.Errorf("esperado 1 resultado (apenas real.txt), obtido %d", len(results))
		for _, r := range results {
			t.Logf("  resultado: %s", r.Path)
		}
	}
}

// TestScan_IgnoraArquivosOcultos verifica que arquivos começando com ponto são ignorados.
func TestScan_IgnoraArquivosOcultos(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("arquivo oculto"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "visible.txt"), []byte("arquivo visível"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := Scan(tmpDir, "")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("esperado 1 resultado (apenas visible.txt), obtido %d", len(results))
	}
}

// TestScan_IgnoraArquivosBinarios verifica que arquivos binários (contendo null bytes)
// são filtrados pelo heurístico isText.
func TestScan_IgnoraArquivosBinarios(t *testing.T) {
	tmpDir := t.TempDir()

	// Criar arquivo binário com null bytes
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x0D, 0x0A}
	if err := os.WriteFile(filepath.Join(tmpDir, "image.png"), binaryContent, 0644); err != nil {
		t.Fatal(err)
	}
	// Criar arquivo de texto normal
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("conteudo de texto"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := Scan(tmpDir, "")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	// Apenas o arquivo de texto deve ser retornado
	if len(results) != 1 {
		t.Errorf("esperado 1 resultado (apenas readme.txt), obtido %d", len(results))
	}
	if len(results) > 0 && results[0].Path != "readme.txt" {
		t.Errorf("esperado 'readme.txt', obtido %q", results[0].Path)
	}
}

// TestScan_Subdiretorios verifica que a busca percorre subdiretórios.
func TestScan_Subdiretorios(t *testing.T) {
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "pkg", "services")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "service.go"), []byte("package services\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := Scan(tmpDir, "")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("esperado 1 resultado, obtido %d", len(results))
	}
	if len(results) > 0 {
		esperado := filepath.Join("pkg", "services", "service.go")
		if results[0].Path != esperado {
			t.Errorf("path esperado %q, obtido %q", esperado, results[0].Path)
		}
	}
}

// TestScan_QueryCaseInsensitive verifica que a busca é case-insensitive.
func TestScan_QueryCaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "Config.yaml"), []byte("configuração"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := Scan(tmpDir, "CONFIG")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	if len(results) == 0 {
		t.Error("esperado encontrar 'Config.yaml' com query 'CONFIG' (case-insensitive)")
	}
}

// TestScan_SkipArquivosGrandes verifica que arquivos maiores que 1MB são ignorados.
func TestScan_SkipArquivosGrandes(t *testing.T) {
	tmpDir := t.TempDir()
	// Criar arquivo > 1MB
	bigContent := make([]byte, 2*1024*1024) // 2MB
	for i := range bigContent {
		bigContent[i] = 'a'
	}
	os.WriteFile(filepath.Join(tmpDir, "big.txt"), bigContent, 0644)

	results, err := Scan(tmpDir, "")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	for _, r := range results {
		if r.Path == "big.txt" {
			t.Error("arquivo > 1MB não deveria ser incluído nos resultados")
		}
	}
}

// TestScan_LimiteMaxResultados verifica que no máximo 50 resultados são retornados.
func TestScan_LimiteMaxResultados(t *testing.T) {
	tmpDir := t.TempDir()
	// Criar 60 arquivos
	for i := 0; i < 60; i++ {
		os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i)), []byte("conteúdo"), 0644)
	}

	results, err := Scan(tmpDir, "")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if len(results) > 50 {
		t.Errorf("esperado no máximo 50 resultados, obtido %d", len(results))
	}
}

// TestScan_PriorizaMatchNoNome verifica que resultados com match no nome
// aparecem antes de resultados com match apenas no conteúdo.
func TestScan_PriorizaMatchNoNome(t *testing.T) {
	tmpDir := t.TempDir()
	// Arquivo com match no nome
	os.WriteFile(filepath.Join(tmpDir, "kubernetes.txt"), []byte("irrelevante"), 0644)
	// Arquivo com match apenas no conteúdo
	os.WriteFile(filepath.Join(tmpDir, "outro.txt"), []byte("conteúdo sobre kubernetes"), 0644)

	results, err := Scan(tmpDir, "kubernetes")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("esperado pelo menos 2 resultados, obtido %d", len(results))
	}
	// O primeiro resultado deve ser o match por nome
	if results[0].Path != "kubernetes.txt" {
		t.Errorf("primeiro resultado deveria ser match por nome, obtido %q", results[0].Path)
	}
}

// TestScan_CaminhoInexistente verifica o comportamento com um caminho inválido.
// Nota: O scanner retorna erro via WalkDir quando o caminho raiz não existe.
func TestScan_CaminhoInexistente(t *testing.T) {
	results, err := Scan("/caminho/inexistente/xyz123", "")
	// WalkDir pode retornar erro ou resultados vazios dependendo da implementação
	// O scanner atual ignora erros individuais do callback, mas WalkDir ainda pode
	// retornar erro para o caminho raiz inexistente.
	if err != nil {
		// Comportamento esperado: erro propagado
		return
	}
	// Se não retornou erro, pelo menos não deve ter resultados
	if len(results) != 0 {
		t.Error("esperado 0 resultados para caminho inexistente")
	}
}

// TestScan_ConteudoPreservado verifica que o conteúdo dos arquivos encontrados
// é retornado corretamente no campo Content.
func TestScan_ConteudoPreservado(t *testing.T) {
	tmpDir := t.TempDir()

	conteudo := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(conteudo), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := Scan(tmpDir, "")
	if err != nil {
		t.Fatalf("Scan falhou: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("esperado 1 resultado, obtido %d", len(results))
	}

	if results[0].Content != conteudo {
		t.Errorf("conteúdo não preservado.\nEsperado:\n%s\nObtido:\n%s", conteudo, results[0].Content)
	}
}


// TestIsText_TableDriven verifica o heurístico isText com diversos cenários.
func TestIsText_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "texto ASCII simples",
			data: []byte("hello world"),
			want: true,
		},
		{
			name: "texto UTF-8",
			data: []byte("olá, mundo! Atenção às exceções"),
			want: true,
		},
		{
			name: "dados vazios",
			data: []byte{},
			want: true,
		},
		{
			name: "dados binários com null byte",
			data: []byte{0x48, 0x65, 0x6C, 0x00, 0x6F},
			want: false,
		},
		{
			name: "apenas null bytes",
			data: []byte{0x00, 0x00, 0x00},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isText(tt.data)
			if got != tt.want {
				t.Errorf("isText(%v) = %v, esperado %v", tt.data, got, tt.want)
			}
		})
	}
}
