package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDetectGoFramework verifica detecção de frameworks Go via go.mod.
func TestDetectGoFramework(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLang string
		wantFW   string
	}{
		{
			name:     "gin detectado",
			content:  "module myapp\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.1\n)",
			wantLang: "go",
			wantFW:   "gin",
		},
		{
			name:     "echo detectado",
			content:  "module myapp\n\nrequire github.com/labstack/echo/v4 v4.11.0",
			wantLang: "go",
			wantFW:   "echo",
		},
		{
			name:     "fiber detectado",
			content:  "module myapp\n\nrequire github.com/gofiber/fiber/v2 v2.50.0",
			wantLang: "go",
			wantFW:   "fiber",
		},
		{
			name:     "chi detectado",
			content:  "module myapp\n\nrequire github.com/go-chi/chi/v5 v5.0.10",
			wantLang: "go",
			wantFW:   "chi",
		},
		{
			name:     "gorilla/mux detectado",
			content:  "module myapp\n\nrequire github.com/gorilla/mux v1.8.0",
			wantLang: "go",
			wantFW:   "gorilla/mux",
		},
		{
			name:     "sem framework retorna apenas linguagem",
			content:  "module myapp\n\ngo 1.21",
			wantLang: "go",
			wantFW:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectGoFramework(tt.content)
			if result.Language != tt.wantLang {
				t.Errorf("Language = %q, esperado %q", result.Language, tt.wantLang)
			}
			if result.Framework != tt.wantFW {
				t.Errorf("Framework = %q, esperado %q", result.Framework, tt.wantFW)
			}
		})
	}
}

// TestDetectNodeFramework verifica detecção de frameworks Node.js via package.json.
func TestDetectNodeFramework(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLang string
		wantFW   string
	}{
		{
			name:     "express detectado",
			content:  `{"dependencies": {"express": "^4.18.0"}}`,
			wantLang: "nodejs",
			wantFW:   "express",
		},
		{
			name:     "nestjs detectado",
			content:  `{"dependencies": {"@nestjs/core": "^10.0.0"}}`,
			wantLang: "nodejs",
			wantFW:   "@nestjs/core",
		},
		{
			name:     "next detectado",
			content:  `{"dependencies": {"next": "^14.0.0", "react": "^18.0.0"}}`,
			wantLang: "nodejs",
			wantFW:   "next",
		},
		{
			name:     "react detectado",
			content:  `{"dependencies": {"react": "^18.0.0", "react-dom": "^18.0.0"}}`,
			wantLang: "nodejs",
			wantFW:   "react",
		},
		{
			name:     "vue detectado",
			content:  `{"dependencies": {"vue": "^3.3.0"}}`,
			wantLang: "nodejs",
			wantFW:   "vue",
		},
		{
			name:     "angular detectado",
			content:  `{"dependencies": {"@angular/core": "^17.0.0"}}`,
			wantLang: "nodejs",
			wantFW:   "@angular/core",
		},
		{
			name:     "sem framework retorna apenas linguagem",
			content:  `{"name": "mylib", "version": "1.0.0"}`,
			wantLang: "nodejs",
			wantFW:   "",
		},
		{
			name:     "express sem aspas não deve detectar",
			content:  `dependencies: express sem formatação JSON`,
			wantLang: "nodejs",
			wantFW:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectNodeFramework(tt.content)
			if result.Language != tt.wantLang {
				t.Errorf("Language = %q, esperado %q", result.Language, tt.wantLang)
			}
			if result.Framework != tt.wantFW {
				t.Errorf("Framework = %q, esperado %q", result.Framework, tt.wantFW)
			}
		})
	}
}

// TestDetectPythonFramework verifica detecção de frameworks Python via pyproject.toml.
func TestDetectPythonFramework(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLang string
		wantFW   string
	}{
		{
			name:     "django detectado",
			content:  "[project]\ndependencies = [\n  \"Django>=4.2\",\n]",
			wantLang: "python",
			wantFW:   "django",
		},
		{
			name:     "fastapi detectado",
			content:  "[project]\ndependencies = [\n  \"fastapi>=0.100.0\",\n]",
			wantLang: "python",
			wantFW:   "fastapi",
		},
		{
			name:     "flask detectado",
			content:  "[project]\ndependencies = [\n  \"Flask>=3.0\",\n]",
			wantLang: "python",
			wantFW:   "flask",
		},
		{
			name:     "starlette detectado",
			content:  "[project]\ndependencies = [\n  \"starlette>=0.27.0\",\n]",
			wantLang: "python",
			wantFW:   "starlette",
		},
		{
			name:     "sem framework",
			content:  "[project]\nname = \"mylib\"",
			wantLang: "python",
			wantFW:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectPythonFramework(tt.content)
			if result.Language != tt.wantLang {
				t.Errorf("Language = %q, esperado %q", result.Language, tt.wantLang)
			}
			if result.Framework != tt.wantFW {
				t.Errorf("Framework = %q, esperado %q", result.Framework, tt.wantFW)
			}
		})
	}
}

// TestDetectPythonRequirements verifica detecção de frameworks Python via requirements.txt.
func TestDetectPythonRequirements(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLang string
		wantFW   string
	}{
		{
			name:     "django detectado",
			content:  "Django==4.2.7\ncelery>=5.3.0",
			wantLang: "python",
			wantFW:   "django",
		},
		{
			name:     "fastapi detectado",
			content:  "uvicorn>=0.23.0\nfastapi>=0.100.0",
			wantLang: "python",
			wantFW:   "fastapi",
		},
		{
			name:     "flask detectado",
			content:  "flask>=3.0.0\ngunicorn>=21.2.0",
			wantLang: "python",
			wantFW:   "flask",
		},
		{
			name:     "sem framework",
			content:  "requests>=2.31.0\npydantic>=2.0.0",
			wantLang: "python",
			wantFW:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectPythonRequirements(tt.content)
			if result.Language != tt.wantLang {
				t.Errorf("Language = %q, esperado %q", result.Language, tt.wantLang)
			}
			if result.Framework != tt.wantFW {
				t.Errorf("Framework = %q, esperado %q", result.Framework, tt.wantFW)
			}
		})
	}
}

// TestDetectJavaFrameworkPom verifica detecção de frameworks Java via pom.xml.
func TestDetectJavaFrameworkPom(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLang string
		wantFW   string
	}{
		{
			name:     "spring-boot detectado",
			content:  "<parent><artifactId>spring-boot-starter-parent</artifactId></parent>",
			wantLang: "java",
			wantFW:   "spring-boot",
		},
		{
			name:     "quarkus detectado",
			content:  "<dependency><groupId>io.quarkus</groupId></dependency>",
			wantLang: "java",
			wantFW:   "quarkus",
		},
		{
			name:     "micronaut detectado",
			content:  "<dependency><groupId>io.micronaut</groupId></dependency>",
			wantLang: "java",
			wantFW:   "micronaut",
		},
		{
			name:     "sem framework",
			content:  "<project><modelVersion>4.0.0</modelVersion></project>",
			wantLang: "java",
			wantFW:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectJavaFrameworkPom(tt.content)
			if result.Language != tt.wantLang {
				t.Errorf("Language = %q, esperado %q", result.Language, tt.wantLang)
			}
			if result.Framework != tt.wantFW {
				t.Errorf("Framework = %q, esperado %q", result.Framework, tt.wantFW)
			}
		})
	}
}

// TestDetectJavaFrameworkGradle verifica detecção de frameworks Java via build.gradle.
func TestDetectJavaFrameworkGradle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLang string
		wantFW   string
	}{
		{
			name:     "spring-boot detectado",
			content:  "plugins {\n  id 'org.springframework.boot' version '3.1.0'\n}\napply plugin: 'spring-boot'",
			wantLang: "java",
			wantFW:   "spring-boot",
		},
		{
			name:     "quarkus detectado",
			content:  "plugins {\n  id 'io.quarkus'\n}",
			wantLang: "java",
			wantFW:   "quarkus",
		},
		{
			name:     "sem framework",
			content:  "plugins {\n  id 'java'\n}",
			wantLang: "java",
			wantFW:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectJavaFrameworkGradle(tt.content)
			if result.Language != tt.wantLang {
				t.Errorf("Language = %q, esperado %q", result.Language, tt.wantLang)
			}
			if result.Framework != tt.wantFW {
				t.Errorf("Framework = %q, esperado %q", result.Framework, tt.wantFW)
			}
		})
	}
}

// TestDetectRustFramework verifica detecção de frameworks Rust via Cargo.toml.
func TestDetectRustFramework(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLang string
		wantFW   string
	}{
		{
			name:     "actix-web detectado",
			content:  "[dependencies]\nactix-web = \"4\"",
			wantLang: "rust",
			wantFW:   "actix-web",
		},
		{
			name:     "axum detectado",
			content:  "[dependencies]\naxum = \"0.7\"",
			wantLang: "rust",
			wantFW:   "axum",
		},
		{
			name:     "rocket detectado",
			content:  "[dependencies]\nrocket = \"0.5\"",
			wantLang: "rust",
			wantFW:   "rocket",
		},
		{
			name:     "warp detectado",
			content:  "[dependencies]\nwarp = \"0.3\"",
			wantLang: "rust",
			wantFW:   "warp",
		},
		{
			name:     "sem framework",
			content:  "[package]\nname = \"mylib\"\nversion = \"0.1.0\"",
			wantLang: "rust",
			wantFW:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectRustFramework(tt.content)
			if result.Language != tt.wantLang {
				t.Errorf("Language = %q, esperado %q", result.Language, tt.wantLang)
			}
			if result.Framework != tt.wantFW {
				t.Errorf("Framework = %q, esperado %q", result.Framework, tt.wantFW)
			}
		})
	}
}

// TestDetectCSharpFramework verifica detecção de frameworks C# via *.csproj.
func TestDetectCSharpFramework(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLang string
		wantFW   string
	}{
		{
			name:     "AspNetCore detectado",
			content:  "<PackageReference Include=\"Microsoft.AspNetCore.App\" />",
			wantLang: "csharp",
			wantFW:   "Microsoft.AspNetCore",
		},
		{
			name:     "Blazor detectado",
			content:  "<PackageReference Include=\"Microsoft.AspNetCore.Components.WebAssembly\" />\n<!-- Blazor app -->",
			wantLang: "csharp",
			wantFW:   "Blazor",
		},
		{
			name:     "sem framework",
			content:  "<Project Sdk=\"Microsoft.NET.Sdk\">\n<TargetFramework>net8.0</TargetFramework></Project>",
			wantLang: "csharp",
			wantFW:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectCSharpFramework(tt.content)
			if result.Language != tt.wantLang {
				t.Errorf("Language = %q, esperado %q", result.Language, tt.wantLang)
			}
			if result.Framework != tt.wantFW {
				t.Errorf("Framework = %q, esperado %q", result.Framework, tt.wantFW)
			}
		})
	}
}

// TestDetectDockerCompose verifica que docker-compose retorna resultado vazio.
func TestDetectDockerCompose(t *testing.T) {
	result := detectDockerCompose("version: '3'\nservices:\n  web:\n    image: nginx")
	if result.Language != "" {
		t.Errorf("Language = %q, esperado vazio", result.Language)
	}
	if result.Framework != "" {
		t.Errorf("Framework = %q, esperado vazio", result.Framework)
	}
}

// TestDetectChartYaml verifica que Chart.yaml retorna resultado vazio.
func TestDetectChartYaml(t *testing.T) {
	result := detectChartYaml("apiVersion: v2\nname: mychart\nversion: 0.1.0")
	if result.Language != "" {
		t.Errorf("Language = %q, esperado vazio", result.Language)
	}
	if result.Framework != "" {
		t.Errorf("Framework = %q, esperado vazio", result.Framework)
	}
}

// TestDetectFramework_Integration verifica a função pública DetectFramework com arquivos reais.
func TestDetectFramework_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		wantLang string
		wantFW   string
	}{
		{
			name:     "go.mod com gin",
			filename: "go.mod",
			content:  "module myapp\n\nrequire github.com/gin-gonic/gin v1.9.1",
			wantLang: "go",
			wantFW:   "gin",
		},
		{
			name:     "package.json com express",
			filename: "package.json",
			content:  `{"dependencies": {"express": "^4.18.0"}}`,
			wantLang: "nodejs",
			wantFW:   "express",
		},
		{
			name:     "arquivo desconhecido retorna vazio",
			filename: "README.md",
			content:  "# Projeto",
			wantLang: "",
			wantFW:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("falha ao criar arquivo temporário: %v", err)
			}

			result := DetectFramework(filePath)
			if result.Language != tt.wantLang {
				t.Errorf("Language = %q, esperado %q", result.Language, tt.wantLang)
			}
			if result.Framework != tt.wantFW {
				t.Errorf("Framework = %q, esperado %q", result.Framework, tt.wantFW)
			}
		})
	}
}

// TestDetectFramework_CsprojGlob verifica detecção de C# via arquivos *.csproj.
func TestDetectFramework_CsprojGlob(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "MyApp.csproj")

	content := `<Project Sdk="Microsoft.NET.Sdk.Web">
<PackageReference Include="Microsoft.AspNetCore.App" />
</Project>`

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo temporário: %v", err)
	}

	result := DetectFramework(filePath)
	if result.Language != "csharp" {
		t.Errorf("Language = %q, esperado %q", result.Language, "csharp")
	}
	if result.Framework != "Microsoft.AspNetCore" {
		t.Errorf("Framework = %q, esperado %q", result.Framework, "Microsoft.AspNetCore")
	}
}

// TestDetectFramework_DockerfileGlob verifica que Dockerfile* retorna vazio.
func TestDetectFramework_DockerfileGlob(t *testing.T) {
	tmpDir := t.TempDir()

	filenames := []string{"Dockerfile", "Dockerfile.prod", "Dockerfile.dev"}
	for _, fn := range filenames {
		filePath := filepath.Join(tmpDir, fn)
		if err := os.WriteFile(filePath, []byte("FROM golang:1.21"), 0644); err != nil {
			t.Fatalf("falha ao criar arquivo temporário: %v", err)
		}

		result := DetectFramework(filePath)
		if result.Language != "" {
			t.Errorf("DetectFramework(%q): Language = %q, esperado vazio", fn, result.Language)
		}
		if result.Framework != "" {
			t.Errorf("DetectFramework(%q): Framework = %q, esperado vazio", fn, result.Framework)
		}
	}
}

// TestDetectFramework_ArquivoInexistente verifica comportamento com arquivo que não existe.
func TestDetectFramework_ArquivoInexistente(t *testing.T) {
	result := DetectFramework("/caminho/inexistente/go.mod")
	if result.Language != "" || result.Framework != "" {
		t.Errorf("esperado resultado vazio para arquivo inexistente, obtido Language=%q Framework=%q",
			result.Language, result.Framework)
	}
}
