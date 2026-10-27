package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/confianca"
	"gopkg.in/yaml.v3"
)

// usarCasa põe a HOME e o projeto em pastas temporárias, com o projeto como diretório
// atual, e limpa o registry. Nada lê o ~/.yby de quem roda o teste.
func usarCasa(t *testing.T) (home, projeto string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	projeto = t.TempDir()
	t.Chdir(projeto)
	Reset()
	t.Cleanup(Reset)
	return home, projeto
}

// escreverTool grava o YAML em <base>/.yby/tools/<arquivo> e devolve o caminho.
func escreverTool(t *testing.T, base, arquivo, conteudo string) string {
	t.Helper()
	dir := filepath.Join(base, ".yby", "tools")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	caminho := filepath.Join(dir, arquivo)
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
	return caminho
}

// invalidaDe acha a externa inválida pelo arquivo.
func invalidaDe(arquivo string) (Invalida, bool) {
	for _, inv := range Invalidas() {
		if inv.Arquivo == arquivo {
			return inv, true
		}
	}
	return Invalida{}, false
}

// TestAjudanteExterno não é um teste: é o programa que as ferramentas externas dos
// testes rodam (o próprio binário de teste, com YBY_AJUDANTE_EXTERNO). Escreve cada
// argumento depois do "--" numa linha e, se pedido, a variável de ambiente ou uma espera.
func TestAjudanteExterno(t *testing.T) {
	if os.Getenv("YBY_AJUDANTE_EXTERNO") == "" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	for _, a := range args {
		fmt.Println(a)
	}
	if nome := os.Getenv("YBY_AJUDANTE_ENV"); nome != "" {
		fmt.Println(nome + "=" + os.Getenv(nome))
	}
	if marca := os.Getenv("YBY_AJUDANTE_MARCA"); marca != "" {
		_ = os.WriteFile(marca, []byte("rodou"), 0o644)
	}
	if os.Getenv("YBY_AJUDANTE_DORMIR") != "" {
		time.Sleep(20 * time.Second)
	}
	os.Exit(0)
}

// toolAjudante é o YAML de uma ferramenta que roda o ajudante com os argumentos dados.
func toolAjudante(nome, args string) string {
	exe, _ := os.Executable()
	comando := fmt.Sprintf(`"%s" "-test.run=^TestAjudanteExterno$" -- %s`, exe, args)
	return fmt.Sprintf("name: %s\ndescription: ajudante\nintents: [%s]\ncommand: '%s'\nparameters:\n  - name: p\n  - name: q\n", nome, nome, strings.ReplaceAll(comando, "'", "''"))
}

// rodar executa a ferramenta registrada.
func rodar(t *testing.T, nome string, params map[string]string) (string, error) {
	t.Helper()
	tool := Get(nome)
	if tool == nil {
		t.Fatalf("ferramenta %q não registrada; inválidas: %+v", nome, Invalidas())
	}
	return tool.Execute(context.Background(), params)
}

func TestLoadExternalTools_SemPastas(t *testing.T) {
	usarCasa(t)
	LoadExternalTools()
	if n := len(All()); n != 0 {
		t.Errorf("esperava 0 ferramentas externas, obteve %d", n)
	}
}

// Os intents do YAML vão para a ferramenta: sem eles, o classificador nunca a escolhe.
func TestLoadExternalTools_CopiaOsIntents(t *testing.T) {
	home, _ := usarCasa(t)
	escreverTool(t, home, "backup.yaml", "name: backup_db\ndescription: Backup\nintents: [backup, salvar_banco]\ncommand: velero backup create x\n")

	LoadExternalTools()

	for _, intent := range []string{"backup", "salvar_banco"} {
		if tool := FindByIntent(intent); tool == nil || tool.Name != "backup_db" {
			t.Errorf("FindByIntent(%q) = %v, esperava backup_db", intent, tool)
		}
	}
}

// A ferramenta do projeto, confiável, vence a global de mesmo nome.
func TestLoadExternalTools_ProjetoVenceGlobal(t *testing.T) {
	home, projeto := usarCasa(t)
	escreverTool(t, home, "saude.yaml", "name: saude\ndescription: global\ncommand: curl global\n")
	caminho := escreverTool(t, projeto, "saude.yaml", "name: saude\ndescription: projeto\ncommand: curl projeto\n")
	if err := confianca.Confiar(caminho); err != nil {
		t.Fatal(err)
	}

	LoadExternalTools()

	if tool := Get("saude"); tool == nil || tool.Description != "projeto" {
		t.Fatalf("esperava a ferramenta do projeto, obteve %+v", tool)
	}
	if inv := Invalidas(); len(inv) != 0 {
		t.Errorf("a global sombreada não é inválida: %+v", inv)
	}
}

// Sem confiança, a ferramenta do projeto não registra, aparece no /ferramentas com o
// motivo e a global de mesmo nome fica valendo.
func TestLoadExternalTools_ProjetoSemConfianca(t *testing.T) {
	home, projeto := usarCasa(t)
	escreverTool(t, projeto, "x.yaml", "name: so_projeto\ncommand: echo x\n")
	escreverTool(t, projeto, "saude.yaml", "name: saude\ndescription: projeto\ncommand: curl projeto\n")
	escreverTool(t, home, "saude.yaml", "name: saude\ndescription: global\ncommand: curl global\n")

	LoadExternalTools()

	if Get("so_projeto") != nil {
		t.Error("ferramenta do projeto sem confiança não pode registrar")
	}
	if tool := Get("saude"); tool == nil || tool.Description != "global" {
		t.Errorf("sem confiança no projeto, vale a global; obteve %+v", tool)
	}
	inv, ok := invalidaDe(".yby/tools/x.yaml")
	if !ok {
		t.Fatalf("a ferramenta sem confiança deveria aparecer no /ferramentas: %+v", Invalidas())
	}
	for _, trecho := range []string{"sem confiança", "yby plugin trust .yby/tools/x.yaml"} {
		if !strings.Contains(inv.Motivo, trecho) {
			t.Errorf("motivo %q sem %q", inv.Motivo, trecho)
		}
	}
	situacao := false
	for _, s := range Situacoes() {
		if s.Nome == ".yby/tools/x.yaml" && s.Invalida && s.Nota == inv.Motivo {
			situacao = true
		}
	}
	if !situacao {
		t.Error("a situação da ferramenta sem confiança não chegou ao /ferramentas")
	}
}

// O arquivo do projeto mudou depois da confiança: ela cai.
func TestLoadExternalTools_ProjetoMudadoPerdeConfianca(t *testing.T) {
	_, projeto := usarCasa(t)
	caminho := escreverTool(t, projeto, "x.yaml", "name: x_tool\ncommand: echo x\n")
	if err := confianca.Confiar(caminho); err != nil {
		t.Fatal(err)
	}
	escreverTool(t, projeto, "x.yaml", "name: x_tool\ncommand: curl http://evil\n")

	LoadExternalTools()

	if Get("x_tool") != nil {
		t.Error("o conteúdo mudou: a ferramenta não pode registrar")
	}
	if _, ok := invalidaDe(".yby/tools/x.yaml"); !ok {
		t.Error("a ferramenta mudada deveria aparecer como sem confiança")
	}
}

func TestLoadExternalTools_NaoSobrescreveNativa(t *testing.T) {
	home, _ := usarCasa(t)
	Register(&Tool{Name: "kubectl_get", Description: "nativa", Execute: func(context.Context, map[string]string) (string, error) { return "", nil }})
	escreverTool(t, home, "get.yaml", "name: kubectl_get\ncommand: echo externo\n")

	LoadExternalTools()

	if tool := Get("kubectl_get"); tool.Description != "nativa" || tool.Externa {
		t.Error("a externa sobrescreveu a nativa")
	}
	if inv, ok := invalidaDe("~/.yby/tools/get.yaml"); !ok || !strings.Contains(inv.Motivo, "nativa") {
		t.Errorf("esperava a externa inválida pelo nome da nativa: %+v", Invalidas())
	}
}

// Recarregar tira as externas da carga anterior: a que perdeu a confiança não fica.
func TestLoadExternalTools_RecarregarTiraAsAntigas(t *testing.T) {
	home, _ := usarCasa(t)
	caminho := escreverTool(t, home, "x.yaml", "name: x_tool\ncommand: echo x\n")
	LoadExternalTools()
	if Get("x_tool") == nil {
		t.Fatal("a global deveria registrar")
	}
	if err := os.Remove(caminho); err != nil {
		t.Fatal(err)
	}
	LoadExternalTools()
	if Get("x_tool") != nil {
		t.Error("a externa removida continuou registrada")
	}
}

// Comando que precisa de shell não carrega: sem shell os operadores não funcionam, e um
// shell com {{}} interpretaria o valor da IA.
func TestLoadExternalTools_ComandoQuePedeShell(t *testing.T) {
	home, _ := usarCasa(t)
	casos := map[string]string{
		"pipe.yaml":   "name: com_pipe\ncommand: kubectl get pods | grep x\n",
		"e.yaml":      "name: com_e\ncommand: cd /tmp && ls\n",
		"var.yaml":    "name: com_var\ncommand: echo $HOME\n",
		"sh.yaml":     "name: com_sh\ncommand: sh -c 'echo {{p}}'\n",
		"bash.yaml":   "name: com_bash\ncommand: /bin/bash -c \"kubectl get {{p}}\"\n",
		"aspas.yaml":  "name: com_aspas\ncommand: echo 'sem fim\n",
		"pwsh.yaml":   "name: com_pwsh\ncommand: pwsh.exe -Command {{p}}\n",
		"cmd.yaml":    "name: com_cmd\ncommand: cmd /c echo {{p}}\n",
		"shenv.yaml":  "name: sh_com_env\ncommand: sh -c 'kubectl get pods -n \"$YBY_PARAM_NS\" | grep x'\n",
		"aspas2.yaml": "name: pipe_entre_aspas\ncommand: grep -E 'a|b' arquivo\n",
	}
	for arquivo, conteudo := range casos {
		escreverTool(t, home, arquivo, conteudo)
	}

	LoadExternalTools()

	for _, nome := range []string{"com_pipe", "com_e", "com_var", "com_sh", "com_bash", "com_aspas", "com_pwsh", "com_cmd"} {
		if Get(nome) != nil {
			t.Errorf("%s não deveria carregar", nome)
		}
	}
	// shell sem {{}} (os valores vêm por $YBY_PARAM_*) e operador entre aspas valem
	for _, nome := range []string{"sh_com_env", "pipe_entre_aspas"} {
		if Get(nome) == nil {
			t.Errorf("%s deveria carregar; inválidas: %+v", nome, Invalidas())
		}
	}
	if inv, ok := invalidaDe("~/.yby/tools/sh.yaml"); !ok || !strings.Contains(inv.Motivo, "YBY_PARAM_") {
		t.Errorf("o motivo do shell com {{}} deveria apontar o $YBY_PARAM_<NOME>: %+v", inv)
	}
}

// O valor da IA é um argumento inteiro: nada dele vira comando, nem com ";" nem com "$( )".
func TestExecutarExterna_ValorNaoViraComando(t *testing.T) {
	home, projeto := usarCasa(t)
	t.Setenv("YBY_AJUDANTE_EXTERNO", "1")
	escreverTool(t, home, "eco.yaml", toolAjudante("eco", `{{p}} "x {{q}}" fim`))
	LoadExternalTools()

	saida, err := rodar(t, "eco", map[string]string{"p": "; touch MARCA", "q": "$(touch MARCA) `touch MARCA`"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projeto, "MARCA")); err == nil {
		t.Fatal("o valor da IA virou comando: o arquivo MARCA foi criado")
	}
	esperado := "; touch MARCA\nx $(touch MARCA) `touch MARCA`\nfim"
	if saida != esperado {
		t.Errorf("argumentos = %q, esperava %q", saida, esperado)
	}
}

// A IA não transforma uma ferramenta externa genérica em "delete namespace": as guardas
// barram antes de o programa rodar. A mesma ferramenta, com leitura, roda.
func TestExecutarExterna_GuardasAntesDeRodar(t *testing.T) {
	home, projeto := usarCasa(t)
	t.Setenv("YBY_AJUDANTE_EXTERNO", "1")
	marca := filepath.Join(projeto, "MARCA")
	t.Setenv("YBY_AJUDANTE_MARCA", marca)
	escreverTool(t, home, "kube.yaml", toolAjudante("kube", `{{p}} {{q}} prod`))
	LoadExternalTools()

	_, err := rodar(t, "kube", map[string]string{"p": "delete", "q": "namespace"})
	if err == nil || !strings.Contains(err.Error(), "operação bloqueada") {
		t.Fatalf("esperava bloqueio do delete namespace, obteve %v", err)
	}
	if _, err := os.Stat(marca); err == nil {
		t.Fatal("o programa rodou apesar do bloqueio")
	}

	saida, err := rodar(t, "kube", map[string]string{"p": "get", "q": "pods"})
	if err != nil {
		t.Fatalf("leitura não deveria ser bloqueada: %v", err)
	}
	if saida != "get\npods\nprod" {
		t.Errorf("saída = %q", saida)
	}
}

// Como no plano: "echo {{p}}" com p = "; touch MARCA" não cria nada (onde há echo no PATH).
func TestExecutarExterna_EchoComPontoEVirgula(t *testing.T) {
	if _, err := exec.LookPath("echo"); err != nil {
		t.Skip("sem echo no PATH")
	}
	home, projeto := usarCasa(t)
	escreverTool(t, home, "eco.yaml", "name: eco\ncommand: echo {{p}}\nparameters:\n  - name: p\n")
	LoadExternalTools()

	saida, err := rodar(t, "eco", map[string]string{"p": "; touch MARCA"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projeto, "MARCA")); err == nil {
		t.Fatal("o arquivo MARCA foi criado")
	}
	if saida != "; touch MARCA" {
		t.Errorf("saída = %q", saida)
	}
}

// Parâmetro ausente: a palavra que é só o placeholder some; dentro de outra, fica vazio.
func TestExecutarExterna_ParametroAusente(t *testing.T) {
	home, _ := usarCasa(t)
	t.Setenv("YBY_AJUDANTE_EXTERNO", "1")
	escreverTool(t, home, "eco.yaml", toolAjudante("eco", `inicio {{p}} a{{q}}b fim`))
	LoadExternalTools()

	saida, err := rodar(t, "eco", nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if saida != "inicio\nab\nfim" {
		t.Errorf("saída = %q", saida)
	}
}

// Valor que começa com "-" no lugar de um argumento inteiro viraria opção do programa.
func TestExecutarExterna_ValorComHifenNaoViraOpcao(t *testing.T) {
	home, _ := usarCasa(t)
	t.Setenv("YBY_AJUDANTE_EXTERNO", "1")
	escreverTool(t, home, "eco.yaml", toolAjudante("eco", `{{p}} --nome={{q}}`))
	LoadExternalTools()

	if _, err := rodar(t, "eco", map[string]string{"p": "--as=system:admin"}); err == nil || !strings.Contains(err.Error(), "\"-\"") {
		t.Errorf("esperava erro pelo \"-\" no começo, obteve %v", err)
	}
	saida, err := rodar(t, "eco", map[string]string{"p": "pod", "q": "-x"})
	if err != nil {
		t.Fatalf("dentro de outra palavra o \"-\" não vira opção: %v", err)
	}
	if saida != "pod\n--nome=-x" {
		t.Errorf("saída = %q", saida)
	}

	// o placeholder no começo de uma palavra também não pode trazer o "-"
	escreverTool(t, home, "eco.yaml", toolAjudante("eco", `{{p}}.yaml`))
	LoadExternalTools()
	if _, err := rodar(t, "eco", map[string]string{"p": "--raw=/x"}); err == nil || !strings.Contains(err.Error(), "\"-\"") {
		t.Errorf("esperava erro pelo \"-\" vindo do parâmetro, obteve %v", err)
	}
}

// interpretadoPeloCmd: o .bat, o .cmd e o próprio cmd rodam pelo cmd.exe no Windows.
func TestInterpretadoPeloCmd(t *testing.T) {
	casos := map[string]bool{
		`C:\Program Files\Microsoft SDKs\Azure\CLI2\wbin\az.cmd`: true,
		`C:\npm\gcloud.CMD`:             true,
		`x.bat`:                         true,
		`C:\Windows\System32\cmd.exe`:   true,
		`cmd`:                           true,
		`C:\bin\kubectl.exe`:            false,
		`/usr/bin/az`:                   false,
		`C:\bin\cmdtool.exe`:            false,
		`C:\bin\ferramenta.cmd.exe`:     false,
		`C:\Windows\System32\where.exe`: false,
	}
	for caminho, esperado := range casos {
		if obtido := interpretadoPeloCmd(caminho); obtido != esperado {
			t.Errorf("interpretadoPeloCmd(%q) = %v, esperava %v", caminho, obtido, esperado)
		}
	}
}

// valorSeguroNoCmd recusa o que o cmd.exe interpretaria, mesmo entre aspas.
func TestValorSeguroNoCmd(t *testing.T) {
	for _, v := range []string{"meu-cluster", "rg_prod.01", "a b", `C:\dir\x.yaml`, "", "nome:porta/1"} {
		if !valorSeguroNoCmd(v) {
			t.Errorf("%q deveria passar", v)
		}
	}
	for _, v := range []string{`x"&echo X`, "a&calc", "a|b", "a<b", "a>b", "a^b", "%PATH%", "!X!", "a(b", "a)b", "a\nb", "a\rb"} {
		if valorSeguroNoCmd(v) {
			t.Errorf("%q deveria ser recusado", v)
		}
	}
}

// No Windows, um programa .cmd (como o az.cmd e o gcloud.cmd) roda pelo cmd.exe, e o
// escape do Go não protege dele: o valor da IA com & ou " viraria comando. Recusado.
func TestExecutarExterna_CmdNoWindowsRecusaMetacaracteres(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("só no Windows o .cmd roda pelo cmd.exe")
	}
	home, projeto := usarCasa(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "fakeaz.cmd"), []byte("@echo off\r\necho %*\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	escreverTool(t, home, "az.yaml", "name: az_show\ncommand: fakeaz show -n {{p}} --tag=x{{q}}\nparameters:\n  - name: p\n  - name: q\n")
	escreverTool(t, home, "cmdenv.yaml", "name: cmd_env\ncommand: cmd /c echo %YBY_PARAM_P%\nparameters:\n  - name: p\n")
	// Programa relativo sem extensão: o .cmd só aparece no Start (PATHEXT), não no cmd.Path.
	scripts := filepath.Join(projeto, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scripts, "check.cmd"), []byte("@echo off\r\necho %*\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	escreverTool(t, home, "relbarra.yaml", "name: rel_barra\ncommand: 'scripts\\check -n {{p}}'\nparameters:\n  - name: p\n")
	escreverTool(t, home, "relponto.yaml", "name: rel_ponto\ncommand: './scripts/check -n {{p}}'\nparameters:\n  - name: p\n")
	LoadExternalTools()

	marca := filepath.Join(projeto, "MARCA")
	for _, nome := range []string{"rel_barra", "rel_ponto"} {
		if _, err := rodar(t, nome, map[string]string{"p": "a&echo.>MARCA"}); err == nil || !strings.Contains(err.Error(), "check.cmd") {
			t.Errorf("%s: esperava recusa pelo cmd (check.cmd), obteve %v", nome, err)
		}
		if _, err := os.Stat(marca); err == nil {
			t.Fatalf("%s: o valor da IA virou comando no cmd.exe pelo programa relativo sem extensão: MARCA foi criado", nome)
		}
	}
	// Com valor comum, a forma com \ roda (a com ./ o próprio cmd.exe não acha: lê o
	// "/scripts" como opção, e isso não é do yby).
	if saida, err := rodar(t, "rel_barra", map[string]string{"p": "meu-ns"}); err != nil || saida != "-n meu-ns" {
		t.Errorf("rel_barra com valor comum: saída = %q, erro = %v", saida, err)
	}
	ataques := []map[string]string{
		{"p": `x"&echo X>MARCA&"`},
		{"p": "a&echo X>MARCA"},
		{"p": `x\"&echo X>MARCA&rem `},
		{"p": "ok", "q": "&echo X>MARCA"},
		{"p": "%PATH%"},
	}
	for _, params := range ataques {
		if _, err := rodar(t, "az_show", params); err == nil || !strings.Contains(err.Error(), "cmd") {
			t.Errorf("params %q: esperava recusa pelo cmd, obteve %v", params, err)
		}
	}
	if _, err := rodar(t, "cmd_env", map[string]string{"p": "a&echo X>MARCA"}); err == nil {
		t.Error("o cmd com o valor no env também interpretaria o &: esperava recusa")
	}
	if _, err := os.Stat(marca); err == nil {
		t.Fatal("o valor da IA virou comando no cmd.exe: o arquivo MARCA foi criado")
	}

	saida, err := rodar(t, "az_show", map[string]string{"p": "meu-cluster", "q": "1"})
	if err != nil {
		t.Fatalf("valor comum deveria rodar: %v", err)
	}
	if saida != "show -n meu-cluster --tag=x1" {
		t.Errorf("saída = %q", saida)
	}
}

// Os parâmetros declarados vão também por env, para quem chama um shell.
func TestExecutarExterna_ParametrosPorEnv(t *testing.T) {
	home, _ := usarCasa(t)
	t.Setenv("YBY_AJUDANTE_EXTERNO", "1")
	t.Setenv("YBY_AJUDANTE_ENV", "YBY_PARAM_P")
	escreverTool(t, home, "eco.yaml", toolAjudante("eco", `fixo`))
	LoadExternalTools()

	saida, err := rodar(t, "eco", map[string]string{"p": "a b; c"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if saida != "fixo\nYBY_PARAM_P=a b; c" {
		t.Errorf("saída = %q", saida)
	}
}

// O cancelamento (esc no Bard) e o fim do prazo param a ferramenta.
func TestExecutarExterna_RespeitaOContexto(t *testing.T) {
	home, _ := usarCasa(t)
	t.Setenv("YBY_AJUDANTE_EXTERNO", "1")
	t.Setenv("YBY_AJUDANTE_DORMIR", "1")
	escreverTool(t, home, "lenta.yaml", toolAjudante("lenta", `x`))
	LoadExternalTools()
	tool := Get("lenta")
	if tool == nil {
		t.Fatalf("ferramenta não registrada: %+v", Invalidas())
	}

	ctx, cancelar := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelar()
	inicio := time.Now()
	_, err := tool.Execute(ctx, nil)
	if err == nil {
		t.Fatal("esperava erro do cancelamento")
	}
	if d := time.Since(inicio); d > 10*time.Second {
		t.Errorf("a ferramenta não parou com o contexto: %v", d)
	}
}

// TestExternalToolDef_Parsing verifica parsing de YAML para ExternalToolDef.
func TestExternalToolDef_Parsing(t *testing.T) {
	yamlContent := `
name: my_tool
description: "Ferramenta de exemplo"
intents:
  - buscar
  - listar
command: "kubectl get {{resource}} -n {{namespace}}"
parameters:
  - name: resource
    description: "Tipo de recurso K8s"
    required: true
  - name: namespace
    description: "Namespace alvo"
    required: false
`

	var def ExternalToolDef
	err := yaml.Unmarshal([]byte(yamlContent), &def)
	if err != nil {
		t.Fatalf("erro ao fazer parse do YAML: %v", err)
	}

	if def.Name != "my_tool" {
		t.Errorf("nome esperado 'my_tool', obteve %q", def.Name)
	}
	if def.Description != "Ferramenta de exemplo" {
		t.Errorf("descrição inesperada: %q", def.Description)
	}
	if len(def.Intents) != 2 {
		t.Fatalf("esperava 2 intents, obteve %d", len(def.Intents))
	}
	if def.Intents[0] != "buscar" || def.Intents[1] != "listar" {
		t.Errorf("intents inesperados: %v", def.Intents)
	}
	if def.Command != "kubectl get {{resource}} -n {{namespace}}" {
		t.Errorf("comando inesperado: %q", def.Command)
	}
	if len(def.Parameters) != 2 {
		t.Fatalf("esperava 2 parâmetros, obteve %d", len(def.Parameters))
	}
	if !def.Parameters[0].Required {
		t.Error("primeiro parâmetro deveria ser obrigatório")
	}
	if def.Parameters[1].Required {
		t.Error("segundo parâmetro não deveria ser obrigatório")
	}
}

// Os parâmetros do YAML viram o esquema da ferramenta.
func TestLoadExternalTools_Parametros(t *testing.T) {
	home, _ := usarCasa(t)
	escreverTool(t, home, "saude.yaml", "name: custom_health\ndescription: Verifica saúde\ncommand: curl -s http://localhost:{{port}}/health\nparameters:\n  - name: port\n    description: Porta do serviço\n    required: true\n")

	LoadExternalTools()

	tool := Get("custom_health")
	if tool == nil {
		t.Fatalf("ferramenta não registrada: %+v", Invalidas())
	}
	if len(tool.Parameters) != 1 || tool.Parameters[0].Name != "port" || !tool.Parameters[0].Required {
		t.Errorf("parâmetros inesperados: %+v", tool.Parameters)
	}
	if !tool.Externa {
		t.Error("deveria ser marcada como externa")
	}
}
