#!/bin/sh
# Instalador do Yby CLI para Linux e macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/casheiro/yby-cli/main/install.sh | sh -
#
# Variáveis opcionais:
#   YBY_VERSION       versão a instalar (ex.: v5.8.4); sem ela, a última release
#   YBY_INSTALL_DIR   pasta de destino; padrão /usr/local/bin (usa sudo se precisar)
#
# Exemplo: curl -fsSL …/install.sh | YBY_VERSION=v5.8.4 YBY_INSTALL_DIR="$HOME/.local/bin" sh -
#
# No Windows, use o install.ps1.
set -eu

REPO="casheiro/yby-cli"
INSTALL_DIR="${YBY_INSTALL_DIR:-/usr/local/bin}"
# Sem a barra final (/usr/local/bin/), a comparação com o PATH e o destino ficam certos; "/" fica.
[ "$INSTALL_DIR" = "/" ] || INSTALL_DIR="${INSTALL_DIR%/}"

# Cores só num terminal e sem NO_COLOR.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    VERDE=$(printf '\033[0;32m')
    VERMELHO=$(printf '\033[0;31m')
    AMARELO=$(printf '\033[0;33m')
    NORMAL=$(printf '\033[0m')
else
    VERDE=""
    VERMELHO=""
    AMARELO=""
    NORMAL=""
fi

info() { printf '%s\n' "$*"; }
aviso() { printf '%s%s%s\n' "$AMARELO" "aviso: $*" "$NORMAL" >&2; }
falha() {
    printf '%s%s%s\n' "$VERMELHO" "erro: $*" "$NORMAL" >&2
    exit 1
}

baixar() {
    # baixar <url> <arquivo>: -f faz o curl falhar em HTTP 4xx/5xx em vez de salvar a página de erro.
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --retry 3 -o "$2" "$1"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "$2" "$1"
    else
        falha "é preciso o curl ou o wget para baixar o yby"
    fi
}

sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | awk '{print $1}'
    else
        falha "é preciso o sha256sum ou o shasum para conferir o download"
    fi
}

info "${VERDE}Yby CLI: instalação${NORMAL}"

# 1. Sistema e arquitetura
SO=$(uname -s)
case "$SO" in
    Linux) SO="linux" ;;
    Darwin) SO="darwin" ;;
    MINGW* | MSYS* | CYGWIN* | Windows_NT)
        falha "no Windows, instale pelo PowerShell: irm https://raw.githubusercontent.com/$REPO/main/install.ps1 | iex"
        ;;
    *) falha "sistema não suportado: $SO (há binários para Linux, macOS e Windows)" ;;
esac

ARQ=$(uname -m)
case "$ARQ" in
    x86_64 | amd64) ARQ="amd64" ;;
    aarch64 | arm64) ARQ="arm64" ;;
    *) falha "arquitetura não suportada: $ARQ (há binários para amd64 e arm64)" ;;
esac
# Num terminal sob o Rosetta, o macOS diz x86_64 num Mac com Apple Silicon: o binário arm64 é o certo.
if [ "$SO" = "darwin" ] && [ "$ARQ" = "amd64" ] && [ "$(sysctl -n hw.optional.arm64 2>/dev/null || true)" = "1" ]; then
    ARQ="arm64"
fi
info "sistema: $SO/$ARQ"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

# 2. Versão
VERSAO="${YBY_VERSION:-}"
if [ -z "$VERSAO" ]; then
    info "buscando a última versão…"
    baixar "https://api.github.com/repos/$REPO/releases/latest" "$TMP_DIR/release.json" ||
        falha "não deu para consultar a última versão no GitHub; confira a conexão (ou o limite da API) e tente de novo, ou informe a versão em YBY_VERSION"
    VERSAO=$(sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' "$TMP_DIR/release.json" | head -n 1)
    [ -n "$VERSAO" ] || falha "a resposta do GitHub não trouxe a versão; informe-a em YBY_VERSION (ex.: YBY_VERSION=v5.8.4)"
fi
case "$VERSAO" in
    v*) ;;
    *) VERSAO="v$VERSAO" ;;
esac
info "versão: ${VERDE}$VERSAO${NORMAL}"

# 3. Download (o nome do arquivo leva a versão sem o v: yby_5.8.4_linux_amd64.tar.gz)
ARQUIVO="yby_${VERSAO#v}_${SO}_${ARQ}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/$VERSAO"

info "baixando $ARQUIVO…"
baixar "$BASE_URL/$ARQUIVO" "$TMP_DIR/$ARQUIVO" ||
    falha "não deu para baixar $BASE_URL/$ARQUIVO; confira se a versão existe em https://github.com/$REPO/releases"
baixar "$BASE_URL/checksums.txt" "$TMP_DIR/checksums.txt" ||
    falha "não deu para baixar o checksums.txt da $VERSAO"

# 4. SHA-256
ESPERADO=$(awk -v f="$ARQUIVO" '$2 == f || $2 == "*" f {print $1}' "$TMP_DIR/checksums.txt" | head -n 1)
[ -n "$ESPERADO" ] || falha "o checksums.txt da $VERSAO não lista $ARQUIVO"
OBTIDO=$(sha256 "$TMP_DIR/$ARQUIVO")
if [ "$ESPERADO" != "$OBTIDO" ]; then
    falha "o SHA-256 do download não confere (esperado $ESPERADO, obtido $OBTIDO); o arquivo veio corrompido ou alterado, rode de novo"
fi
info "sha256 confere"

# 5. Instalação
tar -xzf "$TMP_DIR/$ARQUIVO" -C "$TMP_DIR" yby || falha "o pacote $ARQUIVO não trouxe o binário yby"

DESTINO="$INSTALL_DIR/yby"
SUDO=""
if [ -d "$INSTALL_DIR" ]; then
    [ -w "$INSTALL_DIR" ] || SUDO="sudo"
else
    PAI=$(dirname "$INSTALL_DIR")
    while [ ! -d "$PAI" ]; do PAI=$(dirname "$PAI"); done
    [ -w "$PAI" ] || SUDO="sudo"
fi
if [ -n "$SUDO" ]; then
    command -v sudo >/dev/null 2>&1 ||
        falha "sem permissão de escrita em $INSTALL_DIR e sem sudo; escolha outra pasta com YBY_INSTALL_DIR=\$HOME/.local/bin"
    info "$INSTALL_DIR pede permissão de administrador (sudo)"
fi
# O tar, como root, mantém o dono gravado no pacote (uid 1001), e um mv mantém o dono de quem
# extraiu. O install cria um arquivo novo, do usuário que roda (root sob sudo), ao lado do
# destino, e o mv troca o binário de uma vez.
NOVO="$INSTALL_DIR/.yby.novo.$$"
if ! { $SUDO mkdir -p "$INSTALL_DIR" &&
    $SUDO install -m 0755 "$TMP_DIR/yby" "$NOVO" &&
    $SUDO mv -f "$NOVO" "$DESTINO"; }; then
    $SUDO rm -f "$NOVO" 2>/dev/null || true
    falha "não deu para gravar em $INSTALL_DIR; sem sudo, escolha outra pasta com YBY_INSTALL_DIR=\$HOME/.local/bin"
fi

info "${VERDE}yby $VERSAO instalado em $DESTINO${NORMAL}"

# 6. PATH: a pasta precisa estar nele, e antes de outro yby (um go install antigo, por exemplo).
case ":$PATH:" in
    *":$INSTALL_DIR:"*)
        ATIVO=$(command -v yby 2>/dev/null || true)
        if [ -n "$ATIVO" ] && [ "$ATIVO" != "$DESTINO" ]; then
            aviso "o yby que o terminal acha primeiro é $ATIVO, não o recém-instalado; apague-o (rm $ATIVO) ou ponha $INSTALL_DIR antes no PATH"
        fi
        ;;
    *)
        aviso "$INSTALL_DIR não está no PATH; acrescente-o no perfil do shell (ex.: export PATH=\"$INSTALL_DIR:\$PATH\")"
        ;;
esac

info "rode 'yby version' para conferir e 'yby doctor' para ver as dependências"
