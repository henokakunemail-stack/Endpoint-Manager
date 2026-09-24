# Windows Agent Installer (NSIS)

Folder ini berisi konfigurasi dan skrip untuk membuat installer GUI Windows (`EndpointAgent-Setup.exe`) menggunakan **NSIS (Nullsoft Scriptable Install System)**.

---

## 1. Fitur Installer

- **Tampilan GUI Modern**: Menggunakan NSIS Modern UI 2 (`MUI2`).
- **Halaman Konfigurasi Server**: Menanyakan URL Server Pusat dan Enrollment Token.
- **Pemasangan Sebagai Windows Service**:
  - Menyalin biner ke `C:\Program Files\EndpointAgent\endpoint-agent.exe`.
  - Mendaftarkan biner ke Windows SCM (`sc.exe create endpoint-agent`).
  - Mengonfigurasi path kredensial sistem `C:\ProgramData\EndpointAgent\creds.json`.
  - Memulai service secara otomatis.
- **Uninstaller Windows Terintegrasi**:
  - Muncul di menu Windows **"Settings > Apps > Installed apps"** (atau Control Panel *Programs and Features*).
  - Menghentikan dan menghapus service `endpoint-agent`.
  - Menghapus biner, log, dan kredensial.

---

## 2. Cara Build Installer

### Prasyarat
1. Pasang compiler NSIS di komputer Windows Anda:
   ```powershell
   winget install NSIS.NSIS
   ```
2. Pastikan Go 1.24+ terpasang untuk mengompilasi biner agen.

### Menjalankan Build
Jalankan skrip PowerShell:
```powershell
cd packaging\windows
.\build.ps1
```

Skrip akan:
1. Mengompilasi `agent-windows-amd64.exe` secara pure Go (`CGO_ENABLED=0`).
2. Menjalankan `makensis.exe agent.nsi`.
3. Menghasilkan biner installer: `packaging\windows\EndpointAgent-Setup.exe`.

---

## 3. Instalasi Senyap / Otomatis (Silent Deployment via GPO / InTune)

Installer NSIS ini juga mendukung parameter baris perintah untuk instalasi massal tanpa dialog GUI:

```cmd
EndpointAgent-Setup.exe /S
```
*(Catatan: `/S` bersifat case-sensitive pada NSIS)*
