package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestKubectlTools_Registradas verifica que as 4 tools kubectl estão registradas via init().
func TestKubectlTools_Registradas(t *testing.T) {
	expectedTools := []string{"kubectl_get", "kubectl_logs", "kubectl_events", "kubectl_describe"}

	for _, name := range expectedTools {
		tool := Get(name)
		if tool == nil {
			t.Errorf("ferramenta '%s' não encontrada no registry", name)
			continue
		}
		if tool.Description == "" {
			t.Errorf("ferramenta '%s' sem descrição", name)
		}
		if tool.Execute == nil {
			t.Errorf("ferramenta '%s' sem função Execute", name)
		}
	}
}

// TestKubectlGet_SemResource verifica que kubectl_get exige o parâmetro resource.
func TestKubectlGet_SemResource(t *testing.T) {
	tool := Get("kubectl_get")
	if tool == nil {
		t.Fatal("ferramenta kubectl_get não encontrada")
	}

	_, err := tool.Execute(context.Background(), map[string]string{})
	if err == nil {
		t.Error("esperava erro quando resource está vazio")
	}
}

// TestKubectlGet_Parametros verifica que kubectl_get tem os parâmetros corretos.
func TestKubectlGet_Parametros(t *testing.T) {
	tool := Get("kubectl_get")
	if tool == nil {
		t.Fatal("ferramenta kubectl_get não encontrada")
	}

	if len(tool.Parameters) != 3 {
		t.Fatalf("esperava 3 parâmetros, obteve %d", len(tool.Parameters))
	}

	// resource deve ser obrigatório
	if !tool.Parameters[0].Required {
		t.Error("parâmetro 'resource' deveria ser obrigatório")
	}
	// namespace e output_format opcionais
	if tool.Parameters[1].Required {
		t.Error("parâmetro 'namespace' deveria ser opcional")
	}
	if tool.Parameters[2].Required {
		t.Error("parâmetro 'output_format' deveria ser opcional")
	}
}

// TestKubectlLogs_SemPod verifica que kubectl_logs exige o parâmetro pod.
func TestKubectlLogs_SemPod(t *testing.T) {
	tool := Get("kubectl_logs")
	if tool == nil {
		t.Fatal("ferramenta kubectl_logs não encontrada")
	}

	_, err := tool.Execute(context.Background(), map[string]string{})
	if err == nil {
		t.Error("esperava erro quando pod está vazio")
	}
}

// TestKubectlLogs_Parametros verifica parâmetros da tool logs.
func TestKubectlLogs_Parametros(t *testing.T) {
	tool := Get("kubectl_logs")
	if tool == nil {
		t.Fatal("ferramenta kubectl_logs não encontrada")
	}

	if len(tool.Parameters) != 5 {
		t.Fatalf("esperava 5 parâmetros, obteve %d", len(tool.Parameters))
	}

	if !tool.Parameters[0].Required {
		t.Error("parâmetro 'pod' deveria ser obrigatório")
	}
}

// TestKubectlEvents_Parametros verifica parâmetros da tool events.
func TestKubectlEvents_Parametros(t *testing.T) {
	tool := Get("kubectl_events")
	if tool == nil {
		t.Fatal("ferramenta kubectl_events não encontrada")
	}

	if len(tool.Parameters) != 2 {
		t.Fatalf("esperava 2 parâmetros, obteve %d", len(tool.Parameters))
	}

	if tool.Parameters[0].Required {
		t.Error("parâmetro 'namespace' deveria ser opcional")
	}
}

// O kubectl_events ordena do mais antigo para o mais novo: ficam o cabeçalho e os últimos
// eventos, com a contagem dos que saíram.
func TestUltimosEventos(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("LAST SEEN   TYPE      REASON   OBJECT   MESSAGE\n")
	for i := 1; i <= 300; i++ {
		fmt.Fprintf(&sb, "%ds   Warning   BackOff   pod/worker   evento %03d\n", i, i)
	}
	texto := ultimosEventos(sb.String())

	if !strings.HasPrefix(texto, "LAST SEEN") {
		t.Errorf("o cabeçalho deveria ficar: %q", texto[:40])
	}
	if strings.Count(texto, "evento ") != maxEventos {
		t.Errorf("esperava os %d últimos eventos:\n%s", maxEventos, texto)
	}
	if !strings.Contains(texto, "evento 300") || strings.Contains(texto, "evento 001") {
		t.Errorf("deveriam ficar os mais recentes:\n%s", texto)
	}
	if !strings.Contains(texto, fmt.Sprintf("… %d eventos mais antigos omitidos", 300-maxEventos)) {
		t.Errorf("faltou a contagem dos omitidos:\n%s", texto)
	}

	poucos := "LAST SEEN   TYPE\n1s   Normal\n"
	if got := ultimosEventos(poucos); got != poucos {
		t.Errorf("com poucos eventos a saída fica igual: %q", got)
	}
}

// O tail do kubectl_logs tem teto: a IA pedindo 100000 linhas leva maxLinhasLogs.
func TestArgsKubectlLogs_TetoDoTail(t *testing.T) {
	DefinirKube("", "")
	casos := map[string]string{
		"":       "50",
		"30":     "30",
		"100000": fmt.Sprint(maxLinhasLogs),
	}
	for tail, esperado := range casos {
		args := argsKubectlLogs("worker-1", map[string]string{"tail": tail})
		if got := strings.Join(args, " "); got != "logs worker-1 --tail "+esperado {
			t.Errorf("tail %q: args %q", tail, got)
		}
	}
}

// TestKubectlDescribe_SemResource verifica que kubectl_describe exige resource.
func TestKubectlDescribe_SemResource(t *testing.T) {
	tool := Get("kubectl_describe")
	if tool == nil {
		t.Fatal("ferramenta kubectl_describe não encontrada")
	}

	_, err := tool.Execute(context.Background(), map[string]string{"name": "nginx"})
	if err == nil {
		t.Error("esperava erro quando resource está vazio")
	}
}

// TestKubectlDescribe_SemName verifica que kubectl_describe exige name.
func TestKubectlDescribe_SemName(t *testing.T) {
	tool := Get("kubectl_describe")
	if tool == nil {
		t.Fatal("ferramenta kubectl_describe não encontrada")
	}

	_, err := tool.Execute(context.Background(), map[string]string{"resource": "pod"})
	if err == nil {
		t.Error("esperava erro quando name está vazio")
	}
}

// TestKubectlDescribe_Parametros verifica parâmetros da tool describe.
func TestKubectlDescribe_Parametros(t *testing.T) {
	tool := Get("kubectl_describe")
	if tool == nil {
		t.Fatal("ferramenta kubectl_describe não encontrada")
	}

	if len(tool.Parameters) != 3 {
		t.Fatalf("esperava 3 parâmetros, obteve %d", len(tool.Parameters))
	}

	// resource e name obrigatórios
	if !tool.Parameters[0].Required {
		t.Error("parâmetro 'resource' deveria ser obrigatório")
	}
	if !tool.Parameters[1].Required {
		t.Error("parâmetro 'name' deveria ser obrigatório")
	}
	if tool.Parameters[2].Required {
		t.Error("parâmetro 'namespace' deveria ser opcional")
	}
}

// semKubectl tira o kubectl do PATH e o Sentinel do HOME: o teste nunca chega a um
// cluster de verdade, e a validação, se faltar, aparece como "não encontrado" em vez do
// erro esperado.
func semKubectl(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	DefinirKube("", "")
}

// O conteúdo dos recursos (o data dos Secrets, em qualquer grafia) não vai à IA: o
// kubectl_get só lista em wide ou name.
func TestKubectlGet_FormatoQueMostraConteudoRecusado(t *testing.T) {
	semKubectl(t)
	for _, recurso := range []string{"secrets", "secret", "sec", "secret/db", "configmaps", "pods"} {
		for _, formato := range []string{"yaml", "json", "jsonpath={.data}", "go-template={{.data}}", "custom-columns=D:.data"} {
			_, err := executeKubectlGet(context.Background(), map[string]string{"resource": recurso, "output_format": formato})
			if err == nil || !strings.Contains(err.Error(), "formato") {
				t.Errorf("get %s -o %s deveria ser recusado pelo formato, veio: %v", recurso, formato, err)
			}
		}
	}
}

// Valor da IA começando com "-" (ou com espaço) viraria flag do kubectl ou do Sentinel.
func TestFerramentas_ValorDaIAQueViraFlag(t *testing.T) {
	semKubectl(t)
	casos := []struct {
		ferramenta string
		params     map[string]string
	}{
		{"kubectl_get", map[string]string{"resource": "--raw=/api/v1/namespaces/x/secrets/y"}},
		{"kubectl_get", map[string]string{"resource": "pods", "namespace": "--as=system:admin"}},
		{"kubectl_get", map[string]string{"resource": "pods -o yaml"}},
		{"kubectl_logs", map[string]string{"pod": "--as=system:admin"}},
		{"kubectl_logs", map[string]string{"pod": "api", "tail": "--as=system:admin"}},
		{"kubectl_logs", map[string]string{"pod": "api", "namespace": "-A"}},
		{"kubectl_describe", map[string]string{"resource": "-f", "name": "x"}},
		{"kubectl_describe", map[string]string{"resource": "pod", "name": "--as=system:admin"}},
		{"kubectl_events", map[string]string{"namespace": "--as=system:admin"}},
		{"sentinel_investigate", map[string]string{"pod": "--fix"}},
		{"sentinel_investigate", map[string]string{"pod": "api", "namespace": "--fix"}},
		{"sentinel_scan", map[string]string{"namespace": "--fix"}},
	}
	for _, c := range casos {
		_, err := Get(c.ferramenta).Execute(context.Background(), c.params)
		if err == nil || !strings.Contains(err.Error(), "inválido") {
			t.Errorf("%s %v deveria ser recusado como inválido, veio: %v", c.ferramenta, c.params, err)
		}
	}
}

// Os nomes que o kubectl aceita continuam passando, e wide e name continuam valendo.
func TestConferirParams_NomesValidos(t *testing.T) {
	validos := []map[string]string{
		{"resource": "pods", "namespace": "kube-system"},
		{"resource": "deployments.apps"},
		{"resource": "pod/nginx-7c9d"},
		{"resource": "pods,svc"},
		{"resource": "Deployment", "name": "api-v2.1"},
		{"pod": "worker-1", "tail": "100"},
	}
	for _, p := range validos {
		if err := conferirParams(p, "resource", "name", "pod", "namespace", "tail"); err != nil {
			t.Errorf("%v deveria passar: %v", p, err)
		}
	}
	semKubectl(t)
	for _, formato := range []string{"", "wide", "name", "WIDE"} {
		_, err := executeKubectlGet(context.Background(), map[string]string{"resource": "secrets", "output_format": formato})
		if err == nil || strings.Contains(err.Error(), "formato") {
			t.Errorf("-o %q deveria chegar ao kubectl (ausente no teste), veio: %v", formato, err)
		}
	}
}

// envKubectlFalso faz o binário de teste rodar como o kubectl (TestMain): anota os args de
// cada chamada, uma por linha, no arquivo da variável. O "logs" falha com a saída de
// envKubectlFalsoLogs; o "get" devolve os nomes de contêineres de nomesDoPodFalso.
const (
	envKubectlFalso     = "YBY_BARD_TOOLS_KUBECTL_FALSO"
	envKubectlFalsoLogs = "YBY_BARD_TOOLS_KUBECTL_FALSO_LOGS"
	nomesDoPodFalso     = "app sidecar init-db debugger"
)

// rodarKubectlFalso é o kubectl falso; devolve o código de saída.
func rodarKubectlFalso(arquivo string) int {
	args := os.Args[1:]
	f, err := os.OpenFile(arquivo, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 2
	}
	_, _ = fmt.Fprintln(f, strings.Join(args, " "))
	_ = f.Close()
	if len(args) > 0 && args[0] == "logs" {
		fmt.Print(os.Getenv(envKubectlFalsoLogs))
		return 1
	}
	fmt.Print(nomesDoPodFalso)
	return 0
}

// usarKubectlFalso troca o kubectl das ferramentas pelo falso, com saidaLogs como a saída
// do "logs", e devolve o arquivo das chamadas.
func usarKubectlFalso(t *testing.T, saidaLogs string) string {
	t.Helper()
	DefinirKube("", "")
	arquivo := filepath.Join(t.TempDir(), "chamadas.txt")
	t.Setenv(envKubectlFalso, arquivo)
	t.Setenv(envKubectlFalsoLogs, saidaLogs)
	anterior := binarioKubectl
	binarioKubectl = os.Args[0]
	t.Cleanup(func() { binarioKubectl = anterior })
	return arquivo
}

// chamadasKubectl lê as chamadas anotadas pelo kubectl falso (nenhuma, sem o arquivo).
func chamadasKubectl(t *testing.T, arquivo string) []string {
	t.Helper()
	dados, err := os.ReadFile(arquivo)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(dados), "\n"), "\n")
}

// kubectl_events com pod filtra pelo objeto; sem pod, o namespace inteiro.
func TestKubectlEvents_FiltraPeloPod(t *testing.T) {
	args := strings.Join(argsKubectlEvents(map[string]string{"pod": "api-1", "namespace": "prod"}), " ")
	if !strings.Contains(args, "--field-selector=involvedObject.name=api-1") || !strings.Contains(args, "-n prod") {
		t.Errorf("args sem o filtro do pod: %s", args)
	}
	if strings.Contains(strings.Join(argsKubectlEvents(map[string]string{}), " "), "field-selector") {
		t.Error("sem pod não deve filtrar")
	}
	// Pod começando com '-' não vira flag: a validação recusa antes do kubectl.
	arquivo := usarKubectlFalso(t, "")
	_, err := executeKubectlEvents(context.Background(), map[string]string{"pod": "-A"})
	if err == nil || !strings.Contains(err.Error(), "parâmetro 'pod' inválido") {
		t.Errorf("o pod -A deveria ser recusado pela validação, veio: %v", err)
	}
	if chamadas := chamadasKubectl(t, arquivo); len(chamadas) != 0 {
		t.Errorf("o kubectl não deveria ter sido chamado: %q", chamadas)
	}
}

// kubectl_logs aceita container e previous, e recusa valor com cara de flag.
func TestKubectlLogs_ContainerEPrevious(t *testing.T) {
	args := strings.Join(argsKubectlLogs("a", map[string]string{"container": "c", "previous": "true"}), " ")
	if !strings.Contains(args, "-c c") || !strings.Contains(args, "--previous") {
		t.Errorf("faltou -c ou --previous: %s", args)
	}
	args = strings.Join(argsKubectlLogs("a", map[string]string{"previous": "false"}), " ")
	if strings.Contains(args, "--previous") || strings.Contains(args, "-c ") {
		t.Errorf("não pedido, não vai: %s", args)
	}
	arquivo := usarKubectlFalso(t, "")
	casos := []struct {
		params map[string]string
		campo  string
	}{
		{map[string]string{"pod": "a", "container": "--all-containers"}, "container"},
		{map[string]string{"pod": "a", "previous": "-x"}, "previous"},
	}
	for _, c := range casos {
		_, err := executeKubectlLogs(context.Background(), c.params)
		if err == nil || !strings.Contains(err.Error(), "parâmetro '"+c.campo+"' inválido") {
			t.Errorf("%v deveria ser recusado pela validação, veio: %v", c.params, err)
		}
	}
	if chamadas := chamadasKubectl(t, arquivo); len(chamadas) != 0 {
		t.Errorf("o kubectl não deveria ter sido chamado: %q", chamadas)
	}
}

// No erro de contêiner que o kubectl não lista (nome inválido), a ferramenta consulta os
// contêineres do pod, inclusive os de init e os efêmeros, que o kubectl logs -c também
// aceita. No "choose one of" (o kubectl já lista) e nos outros erros, não consulta.
func TestKubectlLogs_ConteineresDoPodNoErro(t *testing.T) {
	t.Run("contêiner inválido", func(t *testing.T) {
		arquivo := usarKubectlFalso(t, "error: container foo is not valid for pod a\n")
		_, err := executeKubectlLogs(context.Background(), map[string]string{"pod": "a", "container": "foo"})
		if err == nil || !strings.Contains(err.Error(), "Contêineres do pod: app, sidecar, init-db, debugger") {
			t.Errorf("o erro deveria listar os contêineres do pod: %v", err)
		}
		chamadas := chamadasKubectl(t, arquivo)
		if len(chamadas) != 2 {
			t.Fatalf("esperava o logs e o get, vieram %q", chamadas)
		}
		for _, campo := range []string{".spec.containers[*].name", ".spec.initContainers[*].name", ".spec.ephemeralContainers[*].name"} {
			if !strings.Contains(chamadas[1], campo) {
				t.Errorf("a consulta dos contêineres deveria pedir %s: %s", campo, chamadas[1])
			}
		}
	})
	for nome, saida := range map[string]string{
		"o kubectl já lista": "error: a container name must be specified for pod a, choose one of: [app sidecar]\n",
		"outro erro":         "Error from server (NotFound): pods \"a\" not found\n",
	} {
		t.Run(nome, func(t *testing.T) {
			arquivo := usarKubectlFalso(t, saida)
			_, err := executeKubectlLogs(context.Background(), map[string]string{"pod": "a"})
			if err == nil || strings.Contains(err.Error(), "Contêineres do pod") {
				t.Errorf("o erro não deveria trazer a lista consultada: %v", err)
			}
			if chamadas := chamadasKubectl(t, arquivo); len(chamadas) != 1 {
				t.Errorf("só o logs deveria rodar, vieram %q", chamadas)
			}
		})
	}
}
