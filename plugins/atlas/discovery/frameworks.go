package discovery

import (
	"os"
	"path/filepath"
	"strings"
)

// FrameworkResult armazena o resultado da detecção de linguagem e framework.
type FrameworkResult struct {
	Language  string
	Framework string
}

// frameworkDetector mapeia nome de arquivo a uma função que parseia seu conteúdo
// para detectar linguagem e framework.
var frameworkDetectors = map[string]func(content string) FrameworkResult{
	"go.mod":              detectGoFramework,
	"package.json":        detectNodeFramework,
	"pyproject.toml":      detectPythonFramework,
	"requirements.txt":    detectPythonRequirements,
	"pom.xml":             detectJavaFrameworkPom,
	"build.gradle":        detectJavaFrameworkGradle,
	"Cargo.toml":          detectRustFramework,
	"docker-compose.yml":  detectDockerCompose,
	"docker-compose.yaml": detectDockerCompose,
	"Chart.yaml":          detectChartYaml,
}

// DetectFramework lê o arquivo no caminho fornecido e tenta identificar linguagem e framework.
// Para arquivos que correspondem por glob (*.csproj, Dockerfile*), usa detecção especial.
func DetectFramework(filePath string) FrameworkResult {
	filename := filepath.Base(filePath)

	// Verificar detecção por glob: *.csproj
	if strings.HasSuffix(filename, ".csproj") {
		return detectCSharpFramework(readFileContent(filePath))
	}

	// Verificar detecção por glob: Dockerfile*
	if strings.HasPrefix(filename, "Dockerfile") {
		return FrameworkResult{Language: "", Framework: ""}
	}

	detector, ok := frameworkDetectors[filename]
	if !ok {
		return FrameworkResult{}
	}

	content := readFileContent(filePath)
	if content == "" {
		return FrameworkResult{}
	}

	return detector(content)
}

// readFileContent lê o conteúdo de um arquivo e retorna como string.
func readFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// detectGoFramework detecta frameworks Go a partir do conteúdo de go.mod.
func detectGoFramework(content string) FrameworkResult {
	result := FrameworkResult{Language: "go"}

	frameworks := []struct {
		module string
		name   string
	}{
		{"github.com/gin-gonic/gin", "gin"},
		{"github.com/labstack/echo", "echo"},
		{"github.com/gofiber/fiber", "fiber"},
		{"github.com/go-chi/chi", "chi"},
		{"github.com/gorilla/mux", "gorilla/mux"},
	}

	for _, fw := range frameworks {
		if strings.Contains(content, fw.module) {
			result.Framework = fw.name
			return result
		}
	}

	return result
}

// detectNodeFramework detecta frameworks Node.js a partir do conteúdo de package.json.
func detectNodeFramework(content string) FrameworkResult {
	result := FrameworkResult{Language: "nodejs"}

	frameworks := []struct {
		pkg  string
		name string
	}{
		{"@nestjs/core", "@nestjs/core"},
		{"@angular/core", "@angular/core"},
		{"next", "next"},
		{"express", "express"},
		{"react", "react"},
		{"vue", "vue"},
	}

	for _, fw := range frameworks {
		// Buscar "nome" como chave JSON (entre aspas)
		if strings.Contains(content, "\""+fw.pkg+"\"") {
			result.Framework = fw.name
			return result
		}
	}

	return result
}

// detectPythonFramework detecta frameworks Python a partir do conteúdo de pyproject.toml.
func detectPythonFramework(content string) FrameworkResult {
	result := FrameworkResult{Language: "python"}

	frameworks := []struct {
		pkg  string
		name string
	}{
		{"django", "django"},
		{"fastapi", "fastapi"},
		{"flask", "flask"},
		{"starlette", "starlette"},
	}

	lower := strings.ToLower(content)
	for _, fw := range frameworks {
		if strings.Contains(lower, fw.pkg) {
			result.Framework = fw.name
			return result
		}
	}

	return result
}

// detectPythonRequirements detecta frameworks Python a partir de requirements.txt.
func detectPythonRequirements(content string) FrameworkResult {
	result := FrameworkResult{Language: "python"}

	frameworks := []struct {
		pkg  string
		name string
	}{
		{"django", "django"},
		{"fastapi", "fastapi"},
		{"flask", "flask"},
	}

	lower := strings.ToLower(content)
	for _, fw := range frameworks {
		// Verificar se o pacote aparece no início de uma linha
		for _, line := range strings.Split(lower, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, fw.pkg) {
				result.Framework = fw.name
				return result
			}
		}
	}

	return result
}

// detectJavaFrameworkPom detecta frameworks Java a partir do conteúdo de pom.xml.
func detectJavaFrameworkPom(content string) FrameworkResult {
	result := FrameworkResult{Language: "java"}

	frameworks := []struct {
		artifact string
		name     string
	}{
		{"spring-boot", "spring-boot"},
		{"quarkus", "quarkus"},
		{"micronaut", "micronaut"},
	}

	for _, fw := range frameworks {
		if strings.Contains(content, fw.artifact) {
			result.Framework = fw.name
			return result
		}
	}

	return result
}

// detectJavaFrameworkGradle detecta frameworks Java a partir do conteúdo de build.gradle.
func detectJavaFrameworkGradle(content string) FrameworkResult {
	result := FrameworkResult{Language: "java"}

	frameworks := []struct {
		pattern string
		name    string
	}{
		{"spring-boot", "spring-boot"},
		{"quarkus", "quarkus"},
	}

	for _, fw := range frameworks {
		if strings.Contains(content, fw.pattern) {
			result.Framework = fw.name
			return result
		}
	}

	return result
}

// detectRustFramework detecta frameworks Rust a partir do conteúdo de Cargo.toml.
func detectRustFramework(content string) FrameworkResult {
	result := FrameworkResult{Language: "rust"}

	frameworks := []struct {
		crate string
		name  string
	}{
		{"actix-web", "actix-web"},
		{"axum", "axum"},
		{"rocket", "rocket"},
		{"warp", "warp"},
	}

	for _, fw := range frameworks {
		if strings.Contains(content, fw.crate) {
			result.Framework = fw.name
			return result
		}
	}

	return result
}

// detectCSharpFramework detecta frameworks C# a partir do conteúdo de *.csproj.
func detectCSharpFramework(content string) FrameworkResult {
	result := FrameworkResult{Language: "csharp"}

	if strings.Contains(content, "Blazor") {
		result.Framework = "Blazor"
		return result
	}
	if strings.Contains(content, "Microsoft.AspNetCore") {
		result.Framework = "Microsoft.AspNetCore"
		return result
	}

	return result
}

// detectDockerCompose retorna resultado vazio (sem linguagem/framework).
func detectDockerCompose(_ string) FrameworkResult {
	return FrameworkResult{Language: "", Framework: ""}
}

// detectChartYaml retorna resultado vazio (sem linguagem/framework).
func detectChartYaml(_ string) FrameworkResult {
	return FrameworkResult{Language: "", Framework: ""}
}
