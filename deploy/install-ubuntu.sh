#!/usr/bin/env bash
# ==============================================================================
# Enterprise Endpoint Management - Automated Ubuntu Server Installer
# Target Domain: endpoint.esta.co.id (*.esta.co.id)
# ==============================================================================
set -euo pipefail

# Ensure script is run as root
if [ "$EUID" -ne 0 ]; then
  echo "[-] Skrip ini harus dijalankan sebagai root (gunakan sudo)."
  exit 1
fi

echo "=================================================================="
echo "  MEMULAI INSTALASI ENDPOINT MANAGEMENT SERVER (UBUNTU LINUX)    "
echo "=================================================================="

INSTALL_DIR="/opt/endpoint-mgmt"
DATA_DIR="$INSTALL_DIR/data"
SSL_DIR="/etc/ssl/esta"
SERVICE_USER="endpointmgmt"

# 1. Update paket dan install dependensi dasar
echo "[1/7] Memperbarui paket sistem dan menginstal dependensi (nginx, ufw, openssl)..."
apt-get update -y
apt-get install -y nginx ufw openssl curl

# 2. Buat user sistem terisolasi (tanpa login shell)
echo "[2/7] Menyiapkan user sistem $SERVICE_USER..."
if ! id "$SERVICE_USER" &>/dev/null; then
  useradd --system --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin "$SERVICE_USER"
  echo "  [+] User $SERVICE_USER berhasil dibuat."
fi

# 3. Siapkan struktur direktori
echo "[3/7] Membuat direktori aplikasi di $INSTALL_DIR..."
mkdir -p "$DATA_DIR"
mkdir -p "$DATA_DIR/packages"
mkdir -p "$DATA_DIR/agent-releases"
mkdir -p "$SSL_DIR"

# 4. Salin biner server ke /opt/endpoint-mgmt
if [ -f "./endpoint-mgmt-server-linux-amd64" ]; then
  cp ./endpoint-mgmt-server-linux-amd64 "$INSTALL_DIR/endpoint-mgmt-server"
elif [ -f "./endpoint-mgmt-server" ]; then
  cp ./endpoint-mgmt-server "$INSTALL_DIR/endpoint-mgmt-server"
else
  echo "[-] Biner server tidak ditemukan di direktori lokal. Pastikan file 'endpoint-mgmt-server-linux-amd64' ada."
  exit 1
fi
chmod +x "$INSTALL_DIR/endpoint-mgmt-server"

# 5. Buat konfigurasi .env jika belum ada
echo "[4/7] Menyiapkan file konfigurasi environment .env..."
if [ ! -f "$INSTALL_DIR/.env" ]; then
  GENERATED_JWT=$(openssl rand -hex 32)
  GENERATED_PASS=$(openssl rand -base64 12)

  cat <<EOF > "$INSTALL_DIR/.env"
# Runtime Server Configuration for endpoint.esta.co.id
HTTP_ADDR=127.0.0.1:8443
DB_PATH=$DATA_DIR/endpoint-mgmt.db
JWT_SECRET=$GENERATED_JWT
LOG_LEVEL=info
ADMIN_PASSWORD=$GENERATED_PASS
ACCESS_TOKEN_TTL=30m
REFRESH_TOKEN_TTL=168h
AGENT_OFFLINE_AFTER=90s
ENROLLMENT_TTL=30m
EOF
  chmod 600 "$INSTALL_DIR/.env"
  echo "  [+] File .env berhasil dibuat."
  echo "  [!] Password Admin Default: $GENERATED_PASS (Harap catat password ini!)"
else
  echo "  [*] File .env sudah ada, mempertahankan konfigurasi yang ada."
fi

# Set kepemilikan direktori
chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"

# 6. Pasang service systemd
echo "[5/7] Mengonfigurasi systemd service..."
cp ./endpoint-mgmt.service /etc/systemd/system/endpoint-mgmt.service
systemctl daemon-reload
systemctl enable endpoint-mgmt.service

# 7. Konfigurasi Nginx Reverse Proxy
echo "[6/7] Menyiapkan Nginx reverse proxy untuk endpoint.esta.co.id..."
cp ./nginx-endpoint.conf /etc/nginx/sites-available/endpoint.esta.co.id
ln -sf /etc/nginx/sites-available/endpoint.esta.co.id /etc/nginx/sites-enabled/endpoint.esta.co.id
rm -f /etc/nginx/sites-enabled/default

# Periksa sertifikat SSL
if [ -f "$SSL_DIR/esta.co.id.fullchain.crt" ] && [ -f "$SSL_DIR/esta.co.id.key" ]; then
  echo "  [+] Sertifikat wildcard *.esta.co.id ditemukan."
  nginx -t && systemctl reload nginx
else
  echo "  [!] PERINGATAN: Sertifikat belum ditemukan di $SSL_DIR/"
  echo "      Harap salin file sertifikat wildcard Anda ke:"
  echo "      - $SSL_DIR/esta.co.id.fullchain.crt"
  echo "      - $SSL_DIR/esta.co.id.key"
  echo "      Setelah disalin, jalankan: systemctl reload nginx"
fi

# 8. Firewall setup (UFW)
echo "[7/7] Menyesuaikan aturan firewall (UFW)..."
ufw allow 22/tcp || true
ufw allow 80/tcp || true
ufw allow 443/tcp || true

# Jalankan service server
systemctl restart endpoint-mgmt.service

echo ""
echo "=================================================================="
echo "  INSTALASI SERVER ENDPOINT MANAGEMENT BERHASIL!                 "
echo "=================================================================="
echo "  Status Service : systemctl status endpoint-mgmt"
echo "  Log Real-time  : journalctl -u endpoint-mgmt -f"
echo "  Alamat Web     : https://endpoint.esta.co.id"
echo "=================================================================="
