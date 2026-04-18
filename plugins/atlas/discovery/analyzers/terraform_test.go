package analyzers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTerraformAnalyzer_Name(t *testing.T) {
	a := &TerraformAnalyzer{}
	if a.Name() != "terraform" {
		t.Errorf("esperado 'terraform', obteve %q", a.Name())
	}
}

func TestTerraformAnalyzer_Analyze_Resources(t *testing.T) {
	dir := t.TempDir()
	content := `
resource "aws_instance" "web" {
  ami           = "ami-12345"
  instance_type = "t3.micro"
}

resource "aws_security_group" "allow_http" {
  name = "allow_http"
}
`
	filePath := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &TerraformAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	if result.Type != "terraform" {
		t.Errorf("esperado tipo 'terraform', obteve %q", result.Type)
	}

	resCount := 0
	for _, r := range result.Resources {
		if r.Kind == "TerraformResource" {
			resCount++
			if r.Metadata["type"] == "" || r.Metadata["name"] == "" {
				t.Errorf("metadata incompleta para recurso %s", r.Name)
			}
		}
	}
	if resCount != 2 {
		t.Errorf("esperado 2 TerraformResource, obteve %d", resCount)
	}

	// Verifica nomes compostos
	names := map[string]bool{}
	for _, r := range result.Resources {
		names[r.Name] = true
	}
	if !names["aws_instance.web"] {
		t.Error("esperado recurso 'aws_instance.web'")
	}
	if !names["aws_security_group.allow_http"] {
		t.Error("esperado recurso 'aws_security_group.allow_http'")
	}
}

func TestTerraformAnalyzer_Analyze_Modules(t *testing.T) {
	dir := t.TempDir()
	content := `
module "vpc" {
  source = "./modules/vpc"
  cidr   = "10.0.0.0/16"
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "19.0"
}
`
	filePath := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &TerraformAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	modCount := 0
	for _, r := range result.Resources {
		if r.Kind == "TerraformModule" {
			modCount++
		}
	}
	if modCount != 2 {
		t.Errorf("esperado 2 TerraformModule, obteve %d", modCount)
	}

	// Apenas o módulo local (./modules/vpc) deve gerar relação includes
	includesCount := 0
	for _, rel := range result.Relations {
		if rel.Type == "includes" {
			includesCount++
		}
	}
	if includesCount != 1 {
		t.Errorf("esperado 1 relação includes (módulo local), obteve %d", includesCount)
	}
}

func TestTerraformAnalyzer_Analyze_DataSources(t *testing.T) {
	dir := t.TempDir()
	content := `
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]
}

data "aws_vpc" "default" {
  default = true
}
`
	filePath := filepath.Join(dir, "data.tf")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &TerraformAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	dataCount := 0
	for _, r := range result.Resources {
		if r.Kind == "TerraformData" {
			dataCount++
		}
	}
	if dataCount != 2 {
		t.Errorf("esperado 2 TerraformData, obteve %d", dataCount)
	}

	names := map[string]bool{}
	for _, r := range result.Resources {
		names[r.Name] = true
	}
	if !names["aws_ami.ubuntu"] {
		t.Error("esperado data source 'aws_ami.ubuntu'")
	}
	if !names["aws_vpc.default"] {
		t.Error("esperado data source 'aws_vpc.default'")
	}
}

func TestTerraformAnalyzer_Analyze_MultipleFilesInDir(t *testing.T) {
	dir := t.TempDir()

	mainContent := `
resource "aws_instance" "web" {
  ami = "ami-123"
}
`
	varsContent := `
variable "region" {
  default = "us-east-1"
}
`
	outputContent := `
data "aws_caller_identity" "current" {}
`

	for name, content := range map[string]string{
		"main.tf":    mainContent,
		"vars.tf":    varsContent,
		"outputs.tf": outputContent,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	files := []string{
		filepath.Join(dir, "main.tf"),
		filepath.Join(dir, "vars.tf"),
		filepath.Join(dir, "outputs.tf"),
	}

	a := &TerraformAnalyzer{}
	result, err := a.Analyze(dir, files)
	if err != nil {
		t.Fatal(err)
	}

	// 1 resource + 1 data = 2 (variable não é capturado)
	if len(result.Resources) != 2 {
		t.Errorf("esperado 2 recursos, obteve %d", len(result.Resources))
		for _, r := range result.Resources {
			t.Logf("  recurso: %s/%s", r.Kind, r.Name)
		}
	}
}

func TestTerraformAnalyzer_Analyze_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.tf")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	a := &TerraformAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos para arquivo vazio, obteve %d", len(result.Resources))
	}
}

func TestTerraformAnalyzer_Analyze_NoMatchingFiles(t *testing.T) {
	a := &TerraformAnalyzer{}
	result, err := a.Analyze("/tmp", []string{"/tmp/main.go", "/tmp/Dockerfile"})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("esperado 0 recursos, obteve %d", len(result.Resources))
	}
}

func TestTerraformAnalyzer_Analyze_ModuleRelativeSource(t *testing.T) {
	dir := t.TempDir()
	content := `
module "network" {
  source = "../modules/network"
}
`
	filePath := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &TerraformAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	includesCount := 0
	for _, rel := range result.Relations {
		if rel.Type == "includes" {
			includesCount++
			// O alvo deve ser o nome base do diretório
			expected := "TerraformModule/network"
			if rel.To != expected {
				t.Errorf("esperado relação para %q, obteve %q", expected, rel.To)
			}
		}
	}
	if includesCount != 1 {
		t.Errorf("esperado 1 relação includes, obteve %d", includesCount)
	}
}

func TestTerraformAnalyzer_Analyze_MixedContent(t *testing.T) {
	dir := t.TempDir()
	content := `
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

data "aws_availability_zones" "available" {}

module "subnets" {
  source = "./modules/subnets"
  vpc_id = aws_vpc.main.id
}

resource "aws_internet_gateway" "gw" {
  vpc_id = aws_vpc.main.id
}
`
	filePath := filepath.Join(dir, "infra.tf")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &TerraformAnalyzer{}
	result, err := a.Analyze(dir, []string{filePath})
	if err != nil {
		t.Fatal(err)
	}

	kinds := map[string]int{}
	for _, r := range result.Resources {
		kinds[r.Kind]++
	}

	if kinds["TerraformResource"] != 2 {
		t.Errorf("esperado 2 TerraformResource, obteve %d", kinds["TerraformResource"])
	}
	if kinds["TerraformData"] != 1 {
		t.Errorf("esperado 1 TerraformData, obteve %d", kinds["TerraformData"])
	}
	if kinds["TerraformModule"] != 1 {
		t.Errorf("esperado 1 TerraformModule, obteve %d", kinds["TerraformModule"])
	}
}
