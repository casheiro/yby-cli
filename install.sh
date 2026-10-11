#!/bin/sh
set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "${GREEN}🚀 Yby CLI Installer${NC}"

# 1. Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ] && [ "$OS" != "darwin" ]; then
    echo "${RED}❌ SO não suportado: $OS${NC}"
    exit 1
fi

# 2. Detect Arch
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "${RED}❌ Arquitetura não suportada: $ARCH${NC}"
        exit 1
        ;;
esac

echo "Step 1: Detectado $OS/$ARCH"

# 3. Fetch Latest Version
echo "Step 2: Buscando última versão..."
LATEST_URL="https://api.github.com/repos/casheiro/yby-cli/releases/latest"
VERSION=$(curl -s $LATEST_URL | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
    echo "${RED}❌ Falha ao detectar última versão. Verifique sua conexão ou limite da API do GitHub.${NC}"
    exit 1
fi

echo "Versão encontrada: ${GREEN}$VERSION${NC}"

# 4. Construct URL
# Pattern: yby_0.6.0_linux_amd64.tar.gz
# We need to strip 'v' from version for the filename part if GoReleaser uses semver without v in filename
# Checking goreleaser.yaml: project_name: yby. 
# Usually goreleaser produces: project_version_os_arch.tar.gz
# Let's clean the version string for the filename.
CLEAN_VERSION=$(echo $VERSION | sed 's/^v//')
FILENAME="yby_${CLEAN_VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/casheiro/yby-cli/releases/download/$VERSION/$FILENAME"

echo "Step 3: Baixando de $DOWNLOAD_URL..."

TMP_DIR=$(mktemp -d)
curl -sfL "$DOWNLOAD_URL" -o "$TMP_DIR/$FILENAME"

# 5. Extract and Install
echo "Step 4: Instalando..."
tar -xzf "$TMP_DIR/$FILENAME" -C "$TMP_DIR"

BINARY_PATH="/usr/local/bin/yby"

if [ -w "/usr/local/bin" ]; then
    mv "$TMP_DIR/yby" "$BINARY_PATH"
else
    echo "⚠️  Permissão necessária para escrever em /usr/local/bin"
    sudo mv "$TMP_DIR/yby" "$BINARY_PATH"
fi

chmod +x "$BINARY_PATH"

# Cleanup
rm -rf "$TMP_DIR"

echo "${GREEN}✅ Instalação concluída!${NC}"
echo "Execute 'yby --help' para começar."
