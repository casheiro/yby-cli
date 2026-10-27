package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/confianca"
	"gopkg.in/yaml.v3"
)

// ExternalToolDef define uma tool customizada carregada de YAML.
type ExternalToolDef struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Intents     []string `yaml:"intents"`
	Command     string   `yaml:"command"`
	Parameters  []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Required    bool   `yaml:"required"`
	} `yaml:"parameters"`
}

// tempoExterna é o prazo de uma ferramenta externa; passou dele, ela é interrompida.
var tempoExterna = 2 * time.Minute

// LoadExternalTools carrega tools de .yby/tools/ (do projeto) e ~/.yby/tools/ (globais).
// As do projeto vencem as globais de mesmo nome, mas só valem com a confiança do usuário
// ("yby plugin trust .yby/tools/<arquivo>"): um repositório clonado pode trazê-las, e elas
// rodam programas com valores escolhidos pela IA. Sem confiança, ou se o arquivo mudou
// depois dela, a do projeto aparece no /ferramentas com o motivo e a global vale.
// Recarregar tira as externas da carga anterior.
func LoadExternalTools() {
	// rotulo é como o diretório aparece no /ferramentas do Bard.
	type pasta struct {
		dir, rotulo string
		projeto     bool
	}
	dirs := []pasta{{filepath.Join(".yby", "tools"), ".yby/tools/", true}}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, pasta{filepath.Join(home, ".yby", "tools"), "~/.yby/tools/", false})
	}

	mu.Lock()
	invalidas = nil
	for nome, t := range registry {
		if t.Externa {
			delete(registry, nome)
		}
	}
	mu.Unlock()

	for _, d := range dirs {
		entries, err := os.ReadDir(d.dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
				continue
			}

			filePath := filepath.Join(d.dir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			arquivo := d.rotulo + entry.Name()

			var def ExternalToolDef
			if err := yaml.Unmarshal(data, &def); err != nil {
				fmt.Fprintf(os.Stderr, "aviso: tool %s invalida: %v\n", entry.Name(), err)
				registrarInvalida(arquivo, "o YAML não é válido")
				continue
			}

			if motivo := camposFaltando(def); motivo != "" {
				registrarInvalida(arquivo, motivo)
				continue
			}

			palavras, err := analisarComando(def.Command)
			if err != nil {
				registrarInvalida(arquivo, err.Error())
				continue
			}

			// A confiança confere o conteúdo lido acima, o mesmo que vai ser registrado.
			if d.projeto && !confianca.Confiavel(filePath, data) {
				registrarInvalida(arquivo, "do projeto, sem confiança (ou mudou depois dela) · rode yby plugin trust "+arquivo)
				continue
			}

			// Não sobrescrever tools built-in; a externa já registrada é a do projeto,
			// que vence a global de mesmo nome.
			if t := Get(def.Name); t != nil {
				if !t.Externa {
					registrarInvalida(arquivo, "o nome "+def.Name+" já é de uma ferramenta nativa")
				}
				continue
			}

			registerExternalTool(def, palavras)
		}
	}
}

// camposFaltando diz quais campos obrigatórios faltam na definição ("falta o campo
// command"); vazio se nenhum.
func camposFaltando(def ExternalToolDef) string {
	switch {
	case def.Name == "" && def.Command == "":
		return "faltam os campos name e command"
	case def.Name == "":
		return "falta o campo name"
	case def.Command == "":
		return "falta o campo command"
	}
	return ""
}

// registerExternalTool converte uma definição YAML, com o comando já separado em
// palavras, em Tool registrada.
func registerExternalTool(def ExternalToolDef, palavras []string) {
	params := make([]ToolParam, len(def.Parameters))
	nomes := make([]string, len(def.Parameters))
	for i, p := range def.Parameters {
		params[i] = ToolParam{
			Name:        p.Name,
			Description: p.Description,
			Required:    p.Required,
		}
		nomes[i] = p.Name
	}

	tool := &Tool{
		Name:        def.Name,
		Description: def.Description,
		Intents:     append([]string(nil), def.Intents...),
		Parameters:  params,
		Execute: func(ctx context.Context, toolParams map[string]string) (string, error) {
			return executeExternalCommand(ctx, palavras, nomes, toolParams)
		},
		Externa: true,
	}

	Register(tool)
}

// placeholder casa o {{nome}} de um parâmetro no comando.
var placeholder = regexp.MustCompile(`\{\{\s*([^{}\s]+)\s*\}\}`)

// shells são os programas que interpretam o argumento como comando: com eles, um {{}}
// faria o valor da IA virar código.
var shells = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "fish": true,
	"cmd": true, "powershell": true, "pwsh": true,
}

// analisarComando separa o comando do YAML em palavras, como um shell faria com as
// aspas, mas sem shell: o programa roda direto, e um parâmetro nunca é reinterpretado
// (é um argumento inteiro ou parte de um). Aspas simples guardam tudo; duplas guardam
// tudo menos \", que vira aspas. Recusa o que só funcionaria num shell (|, &, ;, <, >, $
// e crase fora de aspas), o programa vindo de parâmetro e o {{}} passado a um shell
// (com sh -c, os valores chegam por "$YBY_PARAM_<NOME>").
func analisarComando(comando string) ([]string, error) {
	var (
		palavras []string
		atual    strings.Builder
		temAlgo  bool // a palavra atual existe, mesmo vazia ("")
		aspa     rune // ' ou " abertas, ou 0
	)
	runas := []rune(comando)
	for i := 0; i < len(runas); i++ {
		c := runas[i]
		switch {
		case aspa == '\'':
			if c == '\'' {
				aspa = 0
			} else {
				atual.WriteRune(c)
			}
		case aspa == '"':
			switch {
			case c == '\\' && i+1 < len(runas) && runas[i+1] == '"':
				atual.WriteRune('"')
				i++
			case c == '"':
				aspa = 0
			default:
				atual.WriteRune(c)
			}
		case c == '\'' || c == '"':
			aspa, temAlgo = c, true
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if temAlgo {
				palavras = append(palavras, atual.String())
				atual.Reset()
				temAlgo = false
			}
		case strings.ContainsRune("|&;<>$`", c):
			return nil, fmt.Errorf("o comando usa %q, que só funciona num shell; para isso, chame sh -c '…' com os parâmetros em \"$YBY_PARAM_<NOME>\"", string(c))
		default:
			atual.WriteRune(c)
			temAlgo = true
		}
	}
	if aspa != 0 {
		return nil, fmt.Errorf("o comando tem aspas %c sem fechar", aspa)
	}
	if temAlgo {
		palavras = append(palavras, atual.String())
	}
	if len(palavras) == 0 {
		return nil, errors.New("falta o campo command")
	}
	if strings.Contains(palavras[0], "{{") {
		return nil, errors.New("o programa do comando não pode vir de um parâmetro")
	}
	if programa := nomePrograma(palavras[0]); shells[programa] {
		for _, p := range palavras[1:] {
			if placeholder.MatchString(p) {
				if programa == "cmd" {
					return nil, errors.New("o comando chama o cmd com {{}}: no cmd, use %YBY_PARAM_<NOME>% no lugar de {{nome}} (valores com \" % & | < > ^ ! ( ) são recusados)")
				}
				return nil, fmt.Errorf("o comando chama o %s com {{}}: no shell, use \"$YBY_PARAM_<NOME>\" entre aspas no lugar de {{nome}}", programa)
			}
		}
	}
	return palavras, nil
}

// nomePrograma é o nome do programa sem a pasta e sem o .exe, em minúsculas
// ("C:\Windows\System32\cmd.exe" → "cmd").
func nomePrograma(programa string) string {
	if i := strings.LastIndexAny(programa, `/\`); i >= 0 {
		programa = programa[i+1:]
	}
	return strings.TrimSuffix(strings.ToLower(programa), ".exe")
}

// interpretadoPeloCmd diz se o programa, já resolvido, roda pelo cmd.exe no Windows: o
// próprio cmd e os .bat e .cmd, que o CreateProcess entrega ao cmd.exe (é o caso do
// az.cmd, do gcloud.cmd e dos atalhos do npm).
func interpretadoPeloCmd(caminho string) bool {
	switch strings.ToLower(filepath.Ext(caminho)) {
	case ".bat", ".cmd":
		return true
	}
	return nomePrograma(caminho) == "cmd"
}

// programaResolvido devolve o programa que o Start vai rodar: sem cmd.Dir, ele resolve
// o PATHEXT pelo exec.LookPath, como aqui ("./scripts/check" → "./scripts/check.cmd").
// Se não achar, devolve o caminho como veio (o Start também não vai achar).
func programaResolvido(caminho string) string {
	if lp, err := exec.LookPath(caminho); err == nil {
		return lp
	}
	return caminho
}

// metaCmd são os caracteres que o cmd.exe interpreta num argumento, com ou sem aspas
// (o escape do Go, com \", não vale para ele): com eles, o valor da IA viraria comando
// ou expandiria variáveis de ambiente, segredos inclusive.
const metaCmd = "\"%&|<>^!()\r\n"

// valorSeguroNoCmd diz se o valor pode ir a um programa que roda pelo cmd.exe.
func valorSeguroNoCmd(valor string) bool {
	return !strings.ContainsAny(valor, metaCmd)
}

// envParam é a variável de ambiente que leva o parâmetro ("namespace" → YBY_PARAM_NAMESPACE).
func envParam(nome string) string {
	var b strings.Builder
	for _, c := range strings.ToUpper(nome) {
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	return "YBY_PARAM_" + b.String()
}

// executeExternalCommand roda o programa do comando com os {{param}} trocados pelos
// valores, sem shell: a palavra que é só o placeholder vira um argumento com o valor
// inteiro (some se o valor é vazio, e recusa valor começando com "-", que viraria opção);
// dentro de outra palavra, o valor entra no lugar dele. Os parâmetros declarados vão
// também em YBY_PARAM_<NOME>. No Windows, se o programa roda pelo cmd.exe (cmd, .bat,
// .cmd), recusa valor com caractere que o cmd interpretaria (metaCmd). Roda com o
// contexto do Bard (esc cancela) e com prazo.
func executeExternalCommand(ctx context.Context, palavras, declarados []string, params map[string]string) (string, error) {
	args := make([]string, 0, len(palavras))
	// usados são os parâmetros que chegam ao programa, por argumento ou por env
	usados := append([]string(nil), declarados...)
	for i, p := range palavras {
		if i > 0 {
			for _, m := range placeholder.FindAllStringSubmatch(p, -1) {
				usados = append(usados, m[1])
			}
			if m := placeholder.FindStringSubmatch(p); m != nil && m[0] == p {
				valor := params[m[1]]
				if valor == "" {
					continue
				}
				if strings.HasPrefix(valor, "-") {
					return "", fmt.Errorf("parâmetro '%s' inválido (%q): começa com \"-\" e viraria opção do programa", m[1], valor)
				}
				args = append(args, valor)
				continue
			}
			trocada := placeholder.ReplaceAllStringFunc(p, func(ph string) string {
				return params[placeholder.FindStringSubmatch(ph)[1]]
			})
			// "{{a}}{{b}}" ou "{{p}}.yaml": o "-" do começo também viria do valor
			if strings.HasPrefix(trocada, "-") && !strings.HasPrefix(p, "-") {
				return "", fmt.Errorf("argumento inválido (%q): começa com \"-\" vindo de um parâmetro e viraria opção do programa", trocada)
			}
			p = trocada
		}
		if p == "~" || strings.HasPrefix(p, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				p = home + p[1:]
			}
		}
		args = append(args, p)
	}

	ctxPrazo, cancelar := context.WithTimeout(ctx, tempoExterna)
	defer cancelar()
	cmd := exec.CommandContext(ctxPrazo, args[0], args[1:]...)
	// O nome puro já vem resolvido pelo PATH e pelo PATHEXT ("az" → az.cmd), mas o
	// caminho relativo sem extensão ("./scripts/check", "scripts\check") só ganha o
	// .cmd dentro do Start: confere o programa que de fato vai rodar.
	if programa := programaResolvido(cmd.Path); runtime.GOOS == "windows" && interpretadoPeloCmd(programa) {
		sort.Strings(usados)
		for _, nome := range usados {
			if !valorSeguroNoCmd(params[nome]) {
				return "", fmt.Errorf("parâmetro '%s' recusado (%q): %s roda pelo cmd do Windows, que interpretaria \" %% & | < > ^ ! ( ) ou quebra de linha no valor", nome, params[nome], filepath.Base(programa))
			}
		}
	}
	cmd.Env = os.Environ()
	for _, nome := range declarados {
		cmd.Env = append(cmd.Env, envParam(nome)+"="+params[nome])
	}
	cmd.WaitDelay = 2 * time.Second

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == nil && errors.Is(ctxPrazo.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("a ferramenta passou de %s e foi interrompida\n%s", tempoExterna, string(output))
		}
		return "", fmt.Errorf("comando falhou: %w\n%s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}
