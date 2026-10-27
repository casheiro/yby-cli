package tools

import (
	"errors"
	"os/exec"
	"sort"
	"strings"
)

// Invalida é um arquivo de ferramenta externa que não carregou.
type Invalida struct {
	Arquivo string // como aparece para o usuário ("~/.yby/tools/backup.yaml")
	Motivo  string // "falta o campo command"
}

// invalidas guarda as externas que não carregaram na última LoadExternalTools (sob mu).
var invalidas []Invalida

func registrarInvalida(arquivo, motivo string) {
	mu.Lock()
	defer mu.Unlock()
	invalidas = append(invalidas, Invalida{Arquivo: arquivo, Motivo: motivo})
}

// Invalidas devolve as ferramentas externas que não carregaram, na ordem em que foram lidas.
func Invalidas() []Invalida {
	mu.RLock()
	defer mu.RUnlock()
	return append([]Invalida(nil), invalidas...)
}

// Situacao diz se uma ferramenta pode rodar agora, para o /ferramentas e o /contexto do
// Bard.
type Situacao struct {
	Nome       string // nome registrado ("kubectl_get"); na externa inválida, o arquivo
	Descricao  string
	Familia    string // primeira parte do nome ("kubectl"), que diz do que ela depende
	Externa    bool
	Invalida   bool // YAML externo que não carregou
	Disponivel bool
	Nota       string // por que não está disponível, ou por que o YAML não carregou
}

// Situacoes lista as ferramentas registradas em ordem de nome, com a disponibilidade de
// cada uma, e no fim as externas que não carregaram. As do kubectl pedem o kubectl no
// PATH; as do sentinel e do atlas, o plugin instalado.
func Situacoes() []Situacao {
	return situacoes(exec.LookPath, discoverPluginBinary)
}

func situacoes(procurar, plugin func(string) (string, error)) []Situacao {
	todas := All()
	sort.Slice(todas, func(i, j int) bool { return todas[i].Name < todas[j].Name })

	notas := map[string]string{} // nota de cada família já verificada ("" disponível)
	verificar := func(familia string) string {
		if nota, ok := notas[familia]; ok {
			return nota
		}
		nota := ""
		switch familia {
		case "kubectl":
			if _, err := procurar("kubectl"); err != nil {
				nota = "kubectl não está no PATH"
			}
		case "sentinel", "atlas":
			_, err := plugin(familia)
			switch {
			case errors.Is(err, errNaoConfiavel):
				nota = "plugin " + familia + " não confiável · rode yby plugin trust " + familia
			case err != nil:
				nota = "plugin " + familia + " não instalado"
			}
		}
		notas[familia] = nota
		return nota
	}

	lista := make([]Situacao, 0, len(todas))
	for _, t := range todas {
		s := Situacao{Nome: t.Name, Descricao: t.Description, Familia: familia(t.Name), Externa: t.Externa}
		if !t.Externa {
			s.Nota = verificar(s.Familia)
		}
		s.Disponivel = s.Nota == ""
		lista = append(lista, s)
	}
	for _, inv := range Invalidas() {
		lista = append(lista, Situacao{Nome: inv.Arquivo, Externa: true, Invalida: true, Nota: inv.Motivo})
	}
	return lista
}

// familia é a primeira parte do nome da ferramenta, antes do sublinhado.
func familia(nome string) string {
	f, _, _ := strings.Cut(nome, "_")
	return f
}
