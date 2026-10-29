package cmd

import (
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/scaffold"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

// repoServidorGitLocal é o repositório usado no modo offline (git mirror no cluster).
const repoServidorGitLocal = "http://git-server.yby-system.svc/repo.git"

// modulosInit devolve as opções do multi-select de módulos: o valor é o texto que
// mapModuleSelection reconhece. É uma função porque o huh marca a seleção nas próprias
// opções: uma lista compartilhada levaria a seleção de um init para o outro.
func modulosInit() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Kepler · eficiência energética", "Kepler (Eficiência Energética)"),
		huh.NewOption("MinIO · object storage local", "MinIO (Object Storage Local)"),
		huh.NewOption("KEDA · autoscaling por eventos", "KEDA (Event-Driven Autoscaling)"),
		huh.NewOption("Metrics Server · observabilidade", "Observability Core (Metrics Server)"),
	}
}

// confirmacaoInit cria uma pergunta de sim ou não com os botões alinhados à esquerda,
// sob o título (o huh os centraliza na largura do título).
func confirmacaoInit(chave string) *huh.Confirm {
	return huh.NewConfirm().Key(chave).WithButtonAlignment(lipgloss.Left)
}

// respostasInit guarda os valores ligados aos campos do assistente do init. Os campos são
// exportados porque o huh calcula pelo conteúdo deles quando refazer o resumo da revisão.
type respostasInit struct {
	Destino      string
	Repo         string
	Nome         string
	Sobrescrever bool
	Topologia    string
	Workflow     string
	Dominio      string
	Email        string
	Modulos      []string
	DevContainer bool
	IA           bool
	Provedor     string
	Descricao    string
	Gerar        bool
}

// perguntasInit diz o que o assistente pergunta: só o que as flags não trouxeram.
type perguntasInit struct {
	destino, repo, topologia, workflow, dominio, email, devcontainer, ia, provedor, descricao bool
}

// perguntasPara decide as perguntas com as mesmas regras das perguntas soltas de antes.
func perguntasPara(flags *InitOptions, ctx *scaffold.BlueprintContext) perguntasInit {
	p := perguntasInit{
		destino:      flags.TargetDir == "",
		repo:         ctx.GitRepoURL == "" && !flags.Offline,
		topologia:    ctx.Topology == "",
		workflow:     ctx.WorkflowPattern == "",
		dominio:      ctx.Domain == "yby.local",
		email:        ctx.Email == "admin@yby.local",
		devcontainer: !flags.IncludeDevContainer,
		ia:           !flags.Offline,
	}
	p.provedor = p.ia && flags.AIProvider == ""
	p.descricao = p.ia && flags.Description == ""
	return p
}

// assistenteInit reúne o assistente do init e as respostas ligadas aos campos, já com os
// valores padrão. Nada é gravado aqui: as respostas só vão para as flags e o contexto
// depois da revisão, por aplicar.
type assistenteInit struct {
	flags      *InitOptions
	ctx        *scaffold.BlueprintContext
	perguntas  perguntasInit
	respostas  *respostasInit
	assistente *ui.Assistente
}

func novoAssistenteInit(flags *InitOptions, ctx *scaffold.BlueprintContext) *assistenteInit {
	a := &assistenteInit{flags: flags, ctx: ctx, perguntas: perguntasPara(flags, ctx)}
	r := &respostasInit{
		Destino:      ".",
		Topologia:    "standard",
		Workflow:     "gitflow",
		Dominio:      "yby.local",
		Email:        "admin@yby.local",
		DevContainer: true,
		IA:           true,
		Provedor:     "auto",
		Gerar:        true,
	}
	if ctx.ProjectName != "yby-project" {
		r.Nome = ctx.ProjectName // veio de --project-name ou de --git-repo
	}
	for _, o := range modulosInit() {
		if k, m, ke, ms := mapModuleSelection([]string{o.Value}); (k && ctx.EnableKepler) || (m && ctx.EnableMinio) || (ke && ctx.EnableKEDA) || (ms && ctx.EnableMetricsServer) {
			r.Modulos = append(r.Modulos, o.Value)
		}
	}
	a.respostas = r
	a.assistente = ui.NovoAssistente(0, a.passos()...)
	return a
}

// passos devolve as etapas do assistente, na ordem da rota.
func (a *assistenteInit) passos() []ui.PassoAssistente {
	p, r := a.perguntas, a.respostas
	var projeto, ambientes, cluster, modulos, ia, detalhesIA []huh.Field

	if p.destino {
		projeto = append(projeto, huh.NewInput().Key("destino").
			Title("Onde criar o projeto").Description("caminho relativo ou absoluto").
			Value(&r.Destino))
	}
	if p.repo {
		projeto = append(projeto, huh.NewInput().Key("repositorio").
			Title("Repositório Git").Description("https:// ou git@; vazio para configurar depois").
			Validate(validarCampo(scaffold.ValidateGitRepo)).
			Value(&r.Repo))
	}
	projeto = append(projeto, huh.NewInput().Key("nome").
		Title("Nome do projeto").Description("slug do Kubernetes: minúsculas, números e hífens").
		PlaceholderFunc(a.nomePadrao, &r.Repo).
		Validate(func(string) error {
			nome := a.nomeEfetivo()
			if err := scaffold.ValidateNoYAMLInjection(nome, "nome do projeto"); err != nil {
				return erroDoCampo(err)
			}
			return erroDoCampo(scaffold.ValidateProjectName(nome))
		}).
		Value(&r.Nome))

	sobrescrever := []huh.Field{confirmacaoInit("sobrescrever").
		Title("Sobrescrever o projeto existente?").
		DescriptionFunc(func() string {
			return fmt.Sprintf("o destino %s já tem um projeto Yby · com Não, o init para sem gravar nada; %s volta para trocar o destino",
				a.destinoEfetivo(), ui.RotuloTecla("⇧⇥"))
		}, &r.Destino).
		Affirmative("Sobrescrever").Negative("Não").
		Value(&r.Sobrescrever)}

	if p.topologia {
		ambientes = append(ambientes, huh.NewSelect[string]().Key("topologia").
			Title("Topologia de ambientes").
			Options(
				huh.NewOption("single · só local", "single"),
				huh.NewOption("standard · local e prod", "standard"),
				huh.NewOption("complete · local, dev, staging e prod", "complete"),
			).Value(&r.Topologia))
	}
	if p.workflow {
		ambientes = append(ambientes, huh.NewSelect[string]().Key("workflow").
			Title("Workflow de CI/CD").
			Options(
				huh.NewOption("essential · um pipeline enxuto", "essential"),
				huh.NewOption("gitflow · feature, release e main", "gitflow"),
				huh.NewOption("trunkbased · deploy contínuo da main", "trunkbased"),
			).Value(&r.Workflow))
	}

	if p.dominio {
		cluster = append(cluster, huh.NewInput().Key("dominio").
			Title("Domínio base do cluster").
			Validate(validarOpcional("domínio", scaffold.ValidateDomain)).
			Value(&r.Dominio))
	}
	if p.email {
		cluster = append(cluster, huh.NewInput().Key("email").
			Title("E-mail do admin e dos certificados").
			Validate(validarOpcional("email", scaffold.ValidateEmail)).
			Value(&r.Email))
	}

	modulos = append(modulos, huh.NewMultiSelect[string]().Key("modulos").
		Title("Módulos adicionais").Description(ui.RotuloTecla("␣")+" marca, "+ui.RotuloTecla("↵")+" continua").
		Options(modulosInit()...).Filterable(false).
		Value(&r.Modulos))
	if p.devcontainer {
		modulos = append(modulos, confirmacaoInit("devcontainer").
			Title("Incluir DevContainer?").Description("gera a pasta .devcontainer/").
			Affirmative("Sim").Negative("Não").Value(&r.DevContainer))
	}

	if p.ia {
		ia = append(ia, confirmacaoInit("ia").
			Title("Ativar o assistente de IA (Synapstor e governança)?").
			Affirmative("Sim").Negative("Não").Value(&r.IA))
	}
	if p.provedor {
		detalhesIA = append(detalhesIA, huh.NewSelect[string]().Key("provedor").
			Title("Provedor de IA").
			Options(huh.NewOptions("auto", "ollama", "gemini", "openai")...).
			Value(&r.Provedor))
	}
	if p.descricao {
		detalhesIA = append(detalhesIA, huh.NewInput().Key("descricao").
			Title("Descreva o projeto").Description("em linguagem natural; a IA gera a governança a partir dela").
			Value(&r.Descricao))
	}

	revisao := []huh.Field{confirmacaoInit("gerar").
		Title("Gerar o projeto?").
		DescriptionFunc(a.resumo, r).
		Affirmative("Gerar").Negative("Cancelar").
		Value(&r.Gerar)}

	// com Não no sobrescrever, os passos seguintes somem e o assistente termina ali
	recusou := a.recusouSobrescrever
	return []ui.PassoAssistente{
		{Marco: "projeto", Campos: projeto},
		{Marco: "projeto", Campos: sobrescrever, Ocultar: func() bool { return !a.precisaSobrescrever() }},
		{Marco: "ambientes", Campos: ambientes, Ocultar: recusou},
		{Marco: "cluster", Campos: cluster, Ocultar: recusou},
		{Marco: "módulos", Campos: modulos, Ocultar: recusou},
		{Marco: "ia", Campos: ia, Ocultar: recusou},
		{Marco: "ia", Campos: detalhesIA, Ocultar: func() bool { return recusou() || !r.IA }},
		{Marco: "revisão", Campos: revisao, Ocultar: recusou},
	}
}

// recusouSobrescrever informa se o destino tem um projeto Yby e a resposta ao
// sobrescrever é Não (o padrão, antes de a pergunta aparecer).
func (a *assistenteInit) recusouSobrescrever() bool {
	return a.precisaSobrescrever() && !a.respostas.Sobrescrever
}

// descricaoEfetiva é a descrição que vai para a IA: a do assistente, com a IA ligada, ou a
// do --description. Sem ela, o init não consulta a IA.
func (a *assistenteInit) descricaoEfetiva() string {
	if a.perguntas.descricao {
		if a.perguntas.ia && a.respostas.IA {
			return strings.TrimSpace(a.respostas.Descricao)
		}
		return ""
	}
	return a.flags.Description
}

// ambientesEfetivos são os ambientes que o projeto vai ter, com as regras do buildContext:
// os do --environments ou os da topologia e, com --offline, sempre o local.
func (a *assistenteInit) ambientesEfetivos(topologia string) []string {
	envs := environmentsForTopology(topologia)
	if a.flags.Environments != "" {
		envs = parseEnvironments(a.flags.Environments)
	}
	if a.flags.Offline {
		envs = ensureLocalEnvironment(envs)
	}
	return envs
}

// destinoEfetivo é o diretório onde o projeto vai ser gerado.
func (a *assistenteInit) destinoEfetivo() string {
	if a.perguntas.destino {
		return resolveTargetDir(strings.TrimSpace(a.respostas.Destino))
	}
	return resolveTargetDir(a.flags.TargetDir)
}

// precisaSobrescrever informa se o destino já tem um projeto Yby e o init vai pedir
// para sobrescrever (sem --force nem --update).
func (a *assistenteInit) precisaSobrescrever() bool {
	if a.flags.Force || a.flags.Update {
		return false
	}
	_, err := os.Stat(filepath.Join(a.destinoEfetivo(), ".yby", "blueprint.yaml"))
	return err == nil
}

// repoEfetivo é o repositório Git que vai para o contexto.
func (a *assistenteInit) repoEfetivo() string {
	switch {
	case a.ctx.GitRepoURL != "":
		return a.ctx.GitRepoURL
	case a.flags.Offline:
		return repoServidorGitLocal
	}
	return strings.TrimSpace(a.respostas.Repo)
}

// nomePadrao é o nome sugerido: o das flags ou, sem ele, o derivado do repositório.
func (a *assistenteInit) nomePadrao() string {
	nome := a.ctx.ProjectName
	if repo := a.repoEfetivo(); repo != "" && nome == "yby-project" {
		nome = deriveProjectName(repo)
	}
	return nome
}

// nomeEfetivo é o nome que vai para o contexto, já como slug do Kubernetes.
func (a *assistenteInit) nomeEfetivo() string {
	nome := strings.TrimSpace(a.respostas.Nome)
	if nome == "" {
		nome = a.nomePadrao()
	}
	return scaffold.SanitizeProjectName(nome)
}

// resumo descreve as respostas na etapa de revisão.
func (a *assistenteInit) resumo() string {
	r := a.respostas
	topologia, workflow := a.ctx.Topology, a.ctx.WorkflowPattern
	if a.perguntas.topologia {
		topologia = r.Topologia
	}
	if a.perguntas.workflow {
		workflow = r.Workflow
	}
	dominio, email := a.ctx.Domain, a.ctx.Email
	if a.perguntas.dominio {
		dominio = r.Dominio
	}
	if a.perguntas.email {
		email = r.Email
	}
	repo := a.repoEfetivo()
	if repo == "" {
		repo = "nenhum"
	}
	modulos := "nenhum"
	if len(r.Modulos) > 0 {
		nomes := make([]string, len(r.Modulos))
		for i, m := range r.Modulos {
			nomes[i] = strings.SplitN(m, " ", 2)[0]
		}
		modulos = strings.Join(nomes, " · ")
	}
	devcontainer := a.flags.IncludeDevContainer || r.DevContainer
	// a IA só roda com descrição: ligada sem ela, nada acontece
	ia := "desligada"
	switch {
	case a.descricaoEfetiva() != "":
		ia = "ligada"
	case a.perguntas.ia && r.IA:
		ia = "ligada · sem descrição, a IA não roda"
	}
	linhas := [][2]string{
		{"destino", a.destinoEfetivo()},
		{"nome", a.nomeEfetivo()},
		{"repositório", repo},
		{"ambientes", topologia + " · " + strings.Join(a.ambientesEfetivos(topologia), " · ")},
		{"workflow", workflow},
		{"cluster", dominio + " · " + email},
		{"módulos", modulos},
		{"devcontainer", map[bool]string{true: "sim", false: "não"}[devcontainer]},
		{"ia", ia},
	}
	if a.precisaSobrescrever() {
		// com Não, o assistente termina antes da revisão
		linhas = append(linhas, [2]string{"sobrescrever", "sim"})
	}
	var b strings.Builder
	for i, l := range linhas {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("%-13s %s", l[0], l[1]))
	}
	return b.String()
}

// aplicar leva as respostas para as flags e o contexto, como faziam as perguntas soltas.
func (a *assistenteInit) aplicar() {
	p, r, flags, ctx := a.perguntas, a.respostas, a.flags, a.ctx
	if p.destino {
		flags.TargetDir = strings.TrimSpace(r.Destino)
	}
	if p.topologia {
		ctx.Topology = r.Topologia
	}
	if p.workflow {
		ctx.WorkflowPattern = r.Workflow
	}
	ctx.ProjectName = a.nomeEfetivo()
	ctx.GitRepoURL = a.repoEfetivo()
	// o repositório digitado é o mesmo do --git-repo: vai para os templates (repoURL)
	if repo := strings.TrimSpace(r.Repo); p.repo && repo != "" && ctx.GitRepo == "" {
		ctx.GitRepo = repo
		if org := extractGithubOrg(repo); org != "" {
			ctx.GithubOrg = org
			ctx.GithubDiscovery = true
		}
	}
	if p.dominio {
		ctx.Domain = strings.TrimSpace(r.Dominio)
	}
	if p.email {
		ctx.Email = strings.TrimSpace(r.Email)
	}
	ctx.EnableKepler, ctx.EnableMinio, ctx.EnableKEDA, ctx.EnableMetricsServer = mapModuleSelection(r.Modulos)
	if p.devcontainer {
		ctx.EnableDevContainer = r.DevContainer
	}
	if p.ia && r.IA {
		if p.provedor {
			flags.AIProvider = r.Provedor
			if flags.AIProvider == "auto" {
				flags.AIProvider = ""
			}
		}
		if p.descricao {
			flags.Description = strings.TrimSpace(r.Descricao)
		}
	}
	flags.sobrescritaConfirmada = r.Sobrescrever && a.precisaSobrescrever()
}

// executarAssistente roda o assistente no terminal. Variável para os testes conduzirem o
// modelo sem terminal.
var executarAssistente = func(a *ui.Assistente) error {
	if !stdinTerminal() {
		return erroAssistenteSemTerminal()
	}
	if _, err := tea.NewProgram(a, tea.WithOutput(os.Stderr)).Run(); err != nil {
		return errors.Wrap(err, errors.ErrCodeIO, "o assistente do init falhou").
			WithHint("rode de novo ou use --non-interactive com as flags do projeto")
	}
	return nil
}

// erroAssistenteSemTerminal é devolvido quando o init precisaria perguntar, mas a entrada
// não é um terminal (pipe, CI): o assistente travaria esperando uma resposta.
func erroAssistenteSemTerminal() error {
	return errors.New(errors.ErrCodeValidation, "o assistente do init precisa de um terminal interativo").
		WithHint("use --non-interactive com --topology, --workflow e --project-name (ou --git-repo)")
}

// initCancelado é devolvido quando o usuário sai do assistente, recusa a revisão ou não
// sobrescreve o projeto existente. Não é falha: executarInit escreve uma linha neutra com
// o motivo e termina sem erro.
type initCancelado struct{ motivo string }

func (c initCancelado) Error() string { return "init cancelado: " + c.motivo }

// perguntarInit roda o assistente e aplica as respostas. Só volta sem erro quando o usuário
// chega ao fim e confirma a revisão.
func perguntarInit(flags *InitOptions, ctx *scaffold.BlueprintContext) error {
	a := novoAssistenteInit(flags, ctx)
	if err := executarAssistente(a.assistente); err != nil {
		return err
	}
	switch {
	case !a.assistente.Concluido():
		return initCancelado{"nenhum arquivo foi gravado"}
	case a.recusouSobrescrever():
		return initCancelado{"o projeto existente não foi sobrescrito"}
	case !a.respostas.Gerar:
		return initCancelado{"nenhum arquivo foi gravado"}
	}
	a.aplicar()
	return nil
}

// validarCampo adapta um validador de pkg/scaffold para o huh, que mostra só a mensagem.
func validarCampo(validar func(string) error) func(string) error {
	return func(v string) error { return erroDoCampo(validar(strings.TrimSpace(v))) }
}

// validarOpcional valida o valor só quando ele não está vazio, como o ValidateContext.
func validarOpcional(campo string, validar func(string) error) func(string) error {
	return func(v string) error {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil
		}
		if err := scaffold.ValidateNoYAMLInjection(v, campo); err != nil {
			return erroDoCampo(err)
		}
		return erroDoCampo(validar(v))
	}
}

// erroDoCampo tira o prefixo "[CÓDIGO]" de um YbyError para exibir no campo.
func erroDoCampo(err error) error {
	if err == nil {
		return nil
	}
	return stderrors.New(mensagemDoErro(err))
}
