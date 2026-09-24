#!/usr/bin/env bash
# ==============================================================================
# Enterprise Endpoint Management — Server installer for Ubuntu / Debian
#
# The public hostname is a parameter, not a constant. Run as:
#
#   sudo ./install-ubuntu.sh --domain mgmt.example.com
#
# Optional flags:
#   --email you@example.com   contact address used by certbot (if requested)
#   --no-tls                 skip nginx TLS setup entirely (behind another LB)
# ==============================================================================
set -euo pipefail

# ---------------------------------------------------------------- parameters --
DOMAIN=""
EMAIL=""
SETUP_TLS=1
INSTALL_DIR="/opt/endpoint-mgmt"
SERVICE_USER="endpointmgmt"
SERVICE_NAME="endpoint-mgmt"

usage() {
  sed -n '2,12p' "$0"
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --domain)   DOMAIN="${2:-}"; shift 2 ;;
    --email)    EMAIL="${2:-}"; shift 2 ;;
    --no-tls)   SETUP_TLS=0; shift ;;
    -h|--help)  usage 0 ;;
    *) echo "unknown option: $1" >&2; usage 1 ;;
  esac
done

if [ "$SETUP_TLS" -eq 1 ] && [ -z "$DOMAIN" ]; then
  echo "[-] --domain is required when TLS setup is enabled." >&2
  echo "    Example: sudo ./install-ubuntu.sh --domain mgmt.example.com" >&2
  exit 1
fi

if [ "$EUID" -ne 0 ]; then
  echo "[-] This script must run as root (use sudo)." >&2
  exit 1
fi

DATA_DIR="$INSTALL_DIR/data"
SSL_DIR="/etc/ssl/endpoint-mgmt"

echo "=================================================================="
echo "  ENDPOINT MANAGEMENT SERVER INSTALLER (UBUNTU / DEBIAN)"
echo "  Public hostname: ${DOMAIN:-<TLS disabled>}"
echo "=================================================================="

# 1. Base packages
echo "[1/8] Installing base packages (nginx, ufw, openssl, curl)..."
apt-get update -y
apt-get install -y nginx ufw openssl curl

# 2. Dedicated system user
echo "[2/8] Preparing system user '$SERVICE_USER'..."
if ! id "$SERVICE_USER" &>/dev/null; then
  useradd --system --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin "$SERVICE_USER"
  echo "  [+] System user '$SERVICE_USER' created."
fi

# 3. Directory layout
echo "[3/8] Creating directories under $INSTALL_DIR..."
mkdir -p "$DATA_DIR/packages" "$DATA_DIR/agent-releases"
if [ "$SETUP_TLS" -eq 1 ]; then
  mkdir -p "$SSL_DIR"
fi

# 4. Install the server binary
echo "[4/8] Installing server binary..."
BIN_SRC=""
for candidate in ./endpoint-mgmt-server-linux-amd64 ./endpoint-mgmt-server ./server; do
  if [ -f "$candidate" ]; then BIN_SRC="$candidate"; break; fi
done
if [ -z "$BIN_SRC" ]; then
  echo "[-] Server binary not found. Expected ./endpoint-mgmt-server-linux-amd64" >&2
  echo "    Build it with: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o endpoint-mgmt-server-linux-amd64 ./server/cmd/server" >&2
  exit 1
fi
cp "$BIN_SRC" "$INSTALL_DIR/endpoint-mgmt-server"
chmod 0755 "$INSTALL_DIR/endpoint-mgmt-server"

# 5. Environment file with generated secrets
echo "[5/8] Writing environment file..."
if [ ! -f "$INSTALL_DIR/.env" ]; then
  GENERATED_JWT="$(openssl rand -hex 32)"
  GENERATED_PASS="$(openssl rand -base64 12 | tr -d '/+=')"

  # The WebSocket console is same-host by default. List extra browser origins
  # here only if you serve the console from a different hostname, e.g.
  # ALLOWED_ORIGIN_DOMAINS=console.example.com,*.example.com
  cat <<EOF > "$INSTALL_DIR/.env"
# Endpoint Management server runtime configuration
HTTP_ADDR=127.0.0.1:8443
DB_PATH=$DATA_DIR/endpoint-mgmt.db
JWT_SECRET=$GENERATED_JWT
LOG_LEVEL=info
ADMIN_PASSWORD=$GENERATED_PASS
ACCESS_TOKEN_TTL=30m
REFRESH_TOKEN_TTL=168h
AGENT_OFFLINE_AFTER=90s
ENROLLMENT_TTL=30m
BACKUP_INTERVAL=1h
BACKUP_RETAIN=24
EOF
  chmod 600 "$INSTALL_DIR/.env"
  echo "  [+] Environment file created."
  echo "  [!] INITIAL ADMIN PASSWORD: $GENERATED_PASS"
  echo "      Record it now and change it after the first login."
else
  echo "  [*] Environment file already exists; leaving it untouched."
fi

chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"

# 6. systemd unit
echo "[6/8] Installing systemd unit..."
cp ./endpoint-mgmt.service "/etc/systemd/system/${SERVICE_NAME}.service"
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}.service"

# 7. nginx reverse proxy
echo "[7/8] Configuring nginx reverse proxy..."
if [ "$SETUP_TLS" -eq 1 ]; then
  sed -e "s/__DOMAIN__/${DOMAIN}/g" \
      -e "s#__SSL_DIR__#${SSL_DIR}#g" \
      ./nginx-endpoint.conf.template > "/etc/nginx/sites-available/${DOMAIN}"
  ln -sf "/etc/nginx/sites-available/${DOMAIN}" "/etc/nginx/sites-enabled/${DOMAIN}"
  rm -f /etc/nginx/sites-enabled/default

  # cert/key are named after the domain so wildcard and per-host certs both fit.
  CERT="$SSL_DIR/${DOMAIN}.fullchain.crt"
  KEY="$SSL_DIR/${DOMAIN}.key"
  if [ -f "$CERT" ] && [ -f "$KEY" ]; then
    echo "  [+] Certificate found for ${DOMAIN}."
    nginx -t && systemctl reload nginx
  else
    echo "  [!] Certificate not found. Place it at:"
    echo "        $CERT"
    echo "        $KEY"
    echo "      (A wildcard cert may be copied under this name.)"
    echo "      Then run: nginx -t && systemctl reload nginx"
    if [ -n "$EMAIL" ]; then
      echo "      Or issue one automatically with:"
      echo "        certbot --nginx -d ${DOMAIN} --email ${EMAIL}"
    fi
  fi
else
  echo "  [*] TLS setup skipped (--no-tls). Terminate TLS upstream instead."
fi

# 8. Firewall
echo "[8/8] Configuring firewall (ufw)..."
ufw allow 22/tcp || true
ufw allow 80/tcp  || true
ufw allow 443/tcp || true

systemctl restart "${SERVICE_NAME}.service"

echo ""
echo "=================================================================="
echo "  INSTALLATION COMPLETE"
echo "  Service status : systemctl status ${SERVICE_NAME}"
echo "  Live logs      : journalctl -u ${SERVICE_NAME} -f"
if [ "$SETUP_TLS" -eq 1 ]; then
  echo "  Console URL    : https://${DOMAIN}"
fi
echo "  Health check   : curl -s http://127.0.0.1:8443/healthz"
echo "=================================================================="
