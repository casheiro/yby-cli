package plugin

import (
	"os"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
)

// EnvPedido é a variável com o pedido do core ao plugin interativo.
const EnvPedido = "YBY_PLUGIN_REQUEST"

// EnvPedidoArquivo é a variável com o caminho do arquivo do pedido, usada quando o pedido
// não cabe numa variável de ambiente.
const EnvPedidoArquivo = "YBY_PLUGIN_REQUEST_FILE"

// limitePedidoNoAmbiente é o maior pedido que vai na variável. O Linux recusa a execução
// (E2BIG, "argument list too long") com uma variável acima de 128 KB e o Windows não aceita
// variável com mais de 32.767 caracteres; o contexto de um projeto grande (o blueprint do
// Atlas, os dados do Synapstor) passa disso.
const limitePedidoNoAmbiente = 30 * 1024

// PedidoDoAmbiente devolve o JSON do pedido que o core passou ao plugin interativo: o da
// variável YBY_PLUGIN_REQUEST ou, quando o pedido foi por arquivo, o conteúdo do arquivo de
// YBY_PLUGIN_REQUEST_FILE. Sem nenhum dos dois, devolve nil, e o plugin cai para o stdin.
func PedidoDoAmbiente(getenv func(string) string) ([]byte, error) {
	if env := getenv(EnvPedido); env != "" {
		return []byte(env), nil
	}
	if arquivo := getenv(EnvPedidoArquivo); arquivo != "" {
		dados, err := os.ReadFile(arquivo)
		if err != nil {
			return nil, ybyerrors.Wrap(err, ybyerrors.ErrCodePluginRPC, "falha ao ler o pedido do core").
				WithContext("arquivo", arquivo)
		}
		return dados, nil
	}
	return nil, nil
}

// pedidoNoAmbiente é a entrada do pedido no ambiente do plugin: a própria variável quando o
// pedido cabe nela, senão um arquivo temporário só do usuário com o caminho em
// YBY_PLUGIN_REQUEST_FILE. limpar apaga o arquivo e é chamado depois que o plugin sai.
func pedidoNoAmbiente(req []byte) (entrada string, limpar func(), err error) {
	if len(req) <= limitePedidoNoAmbiente {
		return EnvPedido + "=" + string(req), func() {}, nil
	}
	arquivo, err := os.CreateTemp("", "yby-pedido-*.json")
	if err != nil {
		return "", nil, ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao gravar o pedido do plugin")
	}
	limpar = func() { _ = os.Remove(arquivo.Name()) }
	_, err = arquivo.Write(req)
	if errFechar := arquivo.Close(); err == nil {
		err = errFechar
	}
	if err != nil {
		limpar()
		return "", nil, ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "falha ao gravar o pedido do plugin")
	}
	return EnvPedidoArquivo + "=" + arquivo.Name(), limpar, nil
}
