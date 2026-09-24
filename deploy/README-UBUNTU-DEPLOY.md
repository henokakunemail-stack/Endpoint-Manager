# Panduan Deployment Server Fisik Linux Ubuntu
## Enterprise Endpoint Management Platform — `endpoint.esta.co.id`

Dokumen ini memandu langkah-langkah implementasi biner server pada **server fisik Linux Ubuntu** (Ubuntu 20.04 / 22.04 / 24.04 LTS) menggunakan sertifikat SSL Wildcard **`*.esta.co.id`**.

---

### Isi Paket Deployment (`deploy/`)

| Nama File | Fungsi |
|---|---|
| `endpoint-mgmt-server-linux-amd64` | Biner tunggal Go server (Zero CGO, embedded Web Console SPA) |
| `install-ubuntu.sh` | Skrip instalasi otomatis (setup user, direktori, service, nginx, firewall) |
| `endpoint-mgmt.service` | Unit file Systemd daemon dengan proteksi NOFILE=65535 |
| `nginx-endpoint.conf` | Konfigurasi reverse proxy Nginx untuk HTTPS & WebSocket persisten |

---

### Langkah-langkah Implementasi

#### Langkah 1: Transfer File ke Server Ubuntu Fisik
Salin seluruh isi folder `deploy/` ke server Ubuntu fisik Anda menggunakan `scp` atau `rsync`:

```bash
# Dari komputer lokal / workstation Anda:
scp -r ./deploy/* user@ip-server-fisik:/tmp/deploy/
```

#### Langkah 2: Tempatkan Sertifikat Wildcard `*.esta.co.id`
Masuk ke server fisik Anda via SSH, buat folder SSL, dan salin file sertifikat wildcard Anda:

```bash
sudo mkdir -p /etc/ssl/esta

# Salin CRT/PEM (termasuk CA bundle/fullchain) dan Private Key:
sudo cp /path/ke/wildcard.crt /etc/ssl/esta/esta.co.id.fullchain.crt
sudo cp /path/ke/wildcard.key /etc/ssl/esta/esta.co.id.key

# Pastikan permission aman (hanya root yang dapat membaca private key):
sudo chmod 600 /etc/ssl/esta/esta.co.id.key
sudo chmod 644 /etc/ssl/esta/esta.co.id.fullchain.crt
```

#### Langkah 3: Eksekusi Skrip Instalasi Otomatis
Masuk ke direktori `/tmp/deploy/` dan jalankan skrip installer:

```bash
cd /tmp/deploy/
chmod +x install-ubuntu.sh
sudo ./install-ubuntu.sh
```

Skrip installer akan secara otomatis:
1. Menginstal paket `nginx`, `ufw`, dan utilitas keamanan.
2. Membuat user sistem terisolasi `endpointmgmt` (tanpa akses login shell langsung untuk pengerasan keamanan).
3. Menyiapkan direktori aplikasi `/opt/endpoint-mgmt` dan storage SQLite WAL `/opt/endpoint-mgmt/data`.
4. Men-generate `JWT_SECRET` berkekuatan 64-karakter hex secara acak dan password admin default di `/opt/endpoint-mgmt/.env`.
5. Mendaftarkan dan mengaktifkan service background `endpoint-mgmt.service` via Systemd.
6. Memasang konfigurasi Nginx untuk `endpoint.esta.co.id` dengan dukungan WebSocket (`Upgrade: websocket`) dan batas timeout 24 jam.
7. Membuka port 80 & 443 pada firewall Ubuntu (UFW).

#### Langkah 4: Verifikasi Layanan
Pastikan server backend Go dan Nginx berjalan normal:

```bash
# Periksa status service server:
sudo systemctl status endpoint-mgmt

# Periksa status Nginx:
sudo systemctl status nginx

# Periksa log server secara real-time:
sudo journalctl -u endpoint-mgmt -f
```

---

### Konfigurasi DNS & Akses Web Console

1. Di penyedia DNS `esta.co.id`, tambahkan **A Record**:
   ```text
   endpoint.esta.co.id   IN   A   [IP-Publik-Server-Fisik]
   ```
2. Buka browser dan akses Web Console:
   ```text
   https://endpoint.esta.co.id
   ```
   Login menggunakan username: `admin` dan password yang dicetak saat instalasi (atau lihat di `/opt/endpoint-mgmt/.env`).

---

### Cara Pemasangan Agen Klien di Komputer Kantor Cabang

Setelah server aktif, agen endpoint di seluruh kantor cabang dapat langsung dihubungkan tanpa perlu membuka port firewall di kantor cabang (koneksi keluar 100% outbound):

1. Masuk ke Web Console `https://endpoint.esta.co.id`, buka menu **Devices** $\rightarrow$ **Generate Enrollment Token**.
2. Di komputer klien Windows, jalankan installer/biner agen:
   ```cmd
   emagent.exe -server https://endpoint.esta.co.id -enroll <TOKEN_DARI_WEB_CONSOLE>
   ```
3. Komputer klien akan otomatis terdaftar, mengirim inventaris hardware/software pertama, dan tersambung permanen melalui Secure WebSocket `wss://endpoint.esta.co.id/api/agent/connect`.
