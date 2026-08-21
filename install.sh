#!/bin/sh
set -e
umask 077

REPO="MX2Tech/cloudflare-ddns-agent"
BIN_PATH="/usr/local/bin/cloudflare-ddns-agent"
CONFIG_DIR="/etc/cloudflare-ddns-agent"
CONFIG_PATH="$CONFIG_DIR/config.yaml"

if [ "$(id -u)" != "0" ]; then
  echo "Este instalador precisa rodar como root (use sudo)." >&2
  exit 1
fi

ARCH=$(uname -m)
case "$ARCH" in
  x86_64) GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *)
    echo "Arquitetura não suportada: $ARCH" >&2
    exit 1
    ;;
esac

echo "Baixando cloudflare-ddns-agent (linux/$GOARCH)..."
DOWNLOAD_URL="https://github.com/$REPO/releases/latest/download/cloudflare-ddns-agent_linux_${GOARCH}"
curl -fsSL "$DOWNLOAD_URL" -o "${BIN_PATH}.tmp"
chmod 755 "${BIN_PATH}.tmp"
mv "${BIN_PATH}.tmp" "$BIN_PATH"

mkdir -p "$CONFIG_DIR"

if [ ! -f "$CONFIG_PATH" ]; then
  echo ""
  echo "Configuração do cloudflare-ddns-agent"
  echo "--------------------------------------"
  printf "Cloudflare API Token: "
  read -r CF_TOKEN < /dev/tty
  printf "Zona (ex: tecnologiadsl.com.br): "
  read -r CF_ZONE < /dev/tty
  printf "Hostname a atualizar (ex: hub.tecnologiadsl.com.br): "
  read -r CF_HOSTNAME < /dev/tty
  printf "Intervalo de checagem em segundos [30]: "
  read -r CF_INTERVAL < /dev/tty
  CF_INTERVAL=${CF_INTERVAL:-30}

  cat > "$CONFIG_PATH" <<EOF
cloudflare:
  api_token: "$CF_TOKEN"
check_interval: ${CF_INTERVAL}s
records:
  - zone: $CF_ZONE
    hostname: $CF_HOSTNAME
EOF
  chmod 600 "$CONFIG_PATH"
else
  echo ""
  echo "Configuração existente encontrada em $CONFIG_PATH -- mantendo como está."
fi

echo ""
echo "Testando a configuração..."
if ! "$BIN_PATH" update; then
  echo ""
  echo "A checagem inicial falhou -- confira o token/zona/hostname acima e rode '$BIN_PATH update' de novo depois de corrigir $CONFIG_PATH." >&2
  exit 1
fi

echo ""
echo "Instalando o serviço systemd..."
"$BIN_PATH" install

echo ""
echo "Pronto! O agente está instalado e ativo."
echo "Ver logs: journalctl -u cloudflare-ddns-agent -f"
