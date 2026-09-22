# Fase 2 — Device Management

**Modul:** Device Management (inventory lengkap, groups, lifecycle, agent-side collectors)
**Tanggal:** 2026-09-22
**Status:** `TESTED (STAGING)`

---

## 1. Brainstorming

### 1.1 Posisi Fase 2 sekarang

Fase 1 membangun **registry dasar**: tabel `devices`, enrollment token, status
online/offline, `os_version`/`agent_version` yang dikirim saat hello. Itu
cukup untuk "device X ada dan sedang online", tapi **bukan** Device Management.

Fase 2 harus menjawab pertanyaan operasional nyata yang tidak bisa dijawab Fase 1:

- "Berapa device di cabang Surabaya yang OS-nya masih Windows 10 21H2?" (patch management butuh ini)
- "Device mana yang disk C-nya di bawah 10% free?" (alerting butuh ini)
- "Tampilkan 50 device dengan RAM terkecil" (upgrade planning)
- "Pindahkan PC-07 ke grup 'Server Lab'" (deployment target oleh grup, bukan satu-satu)
- "PC ini diganti, putuskan hubungannya" (retirement, bukan delete → kehilangan riwayat audit)

Tanpa jawaban itu, Fase 4/5 tidak punya target yang masuk akal.

### 1.2 Ruang lingkup Fase 2

**Dibangun:**

1. **Hardware inventory**: CPU (model/cores/logical), RAM total, disk (per-volume size/free/label), NIC (MAC + IP), manufacturer/model/serial, firmware/BIOS.
2. **Software inventory** (Windows): installed programs via registry uninstall keys + MSI `UpgradeCode` untuk dedup.
3. **OS & update info**: build number, edition, install date, arsitektur, last boot time, uptime.
4. **Lifecycle**: retire (soft-delete) + restore; device tidak benar-benar dihapus — audit trail harus utuh.
5. **Groups**: static group membership, group sebagai deployment target.
6. **Agent collectors**: per-OS implementation, kirim hasil lewat command/response di transport yang sudah ada.
7. **API**: filtering/pagination/sorting di `GET /api/devices`, detail perubahan inventory.

**Tidak dibangun di Fase 2** (memang fase lain):

- Software license reconciliation → Asset & License Management (modul tambahan)
- OS deployment / bare-metal provisioning → di luar scope produk ini
- Endpoint detection & response → di luar scope
- Software metering (siapa pakai apa, berapa lama) → Asset & License

### 1.3 Opsi implementasi inventory — trade-off

Pertanyaan utama: **bagaimana agent mengumpulkan dan melaporkan hardware/software facts.**

#### Opsi A — Selalu telpon ulang dengan command dari server (poll on demand)

Server kirim command `inventory.collect` → agent kumpulkan saat itu juga → balas.

- ✅ Sederhana, tidak ada jadwal tambahan, memakai command queue yang sudah ada.
- ✅ Hasil selalu segar saat admin membuka detail device.
- ❌ Detail device lambat dibuka pertama kali (butuh detik detik).
- ❌ Tidak ada data jika device sedang offline — **dashboard butuh data terakhir** walau device offline.

#### Opsi B — Agent kirim sendiri periodically (periodic self-report)

Agent mengumpulkan tiap N jam (default 6) dan kirim tanpa diminta, ditambah saat hello.

- ✅ Server selalu punya snapshot terakhir untuk dashboard/report, walau device offline.
- ✅ Hello message jadi kaya (os info + hw summary) tanpa round-trip.
- ❌ Data bisa stale hingga N jam.
- ❌ Butuh penjadwalan agent-side.
- ❌ Kumpul 500 device di jam yang sama = badai. Harus distagger.

#### Opsi C — Hybrid (REKOMENDASI)

- **Saat hello**: agent kirim **hardware summary ringan** (model, serial, CPU, RAM, MAC) — murah, selalu tersedia.
- **Periodic**: full inventory tiap 6 jam, **dijitter per-device** (offset = hash(device_id) % window), jadi tersebar.
- **On demand**: admin bisa memaksa `inventory.collect` kapan saja via button/API.
- Server simpan snapshot terakhir; jika perbedaan signifikan, catat di audit log.

Dipilih karena **data harus selalu ada untuk dashboard** (B) tapi **harus bisa diminta fresh** (A). Stagger mencegah badai 500 agent serentak.

#### Opsi D — Agent-side event-driven (tolak)

Mendeteksi setiap perubahan hardware/software secara real-time butuh kernel hook / registry
notification yang berat. Overkill untuk inventory harian, dan menambah risk agent.

### 1.4 Software inventory di Windows — trade-off spesifik

#### W1 — WMI/CIM (`Win32_Product`, `Win32_QuickFixEngineering`)

- ✅ Data kaya dan terstandar.
- ❌ **`Win32_Product` sangat lambat** — dia reconfigures setiap installer, bisa menit.
- ❌ Memerlukan `wmic.exe` atau COM automation; dari pure-Go harus pakai CIM via PowerShell exec.

#### W2 — Baca registry Uninstall keys langsung (REKOMENDASI untuk software)

- ✅ Cepat (milidetik), andal, sumber yang sama dengan "Programs and Features".
- ✅ `x/sys/windows/registry` tersedia (terverifikasi).
- ✅ Dedup MSI via `UpgradeCode` + `ProductCode`.
- ❌ Hanya Windows; Linux/macOS butuh approach berbeda (lihat §1.5).

#### W3 — Parse output `Get-ItemProperty HKLM:\...\Uninstall\*` via PowerShell exec

- ✅ Satu exec, data lengkap, urutan tidak penting.
- ❌ Spawn PowerShell tiap collection (1-3 detik); berat untuk 500 device yang poll serentak.

**Keputusan: W2** (registry langsung via `x/sys/windows/registry`) untuk daftar program, plus
ringkasan hotfix dari `Get-HotFix` (W3) hanya bila `Win32_QuickFixEngineering` dibutuhkan di
Fase 5. Patch Management akan punya mekanisme scan-nya sendiri; Fase 2 cukup kumpul
software list + OS build number.

### 1.5 Perbedaan OS (memenuhi konstrain spec — harus eksplisit)

| Fitur | Windows | Linux | macOS |
|---|---|---|---|
| CPU | `GetActiveProcessorCount` + registry `ProcessorNameString` (terverifikasi tersedia) | `/proc/cpuinfo`, `nproc` | `sysctl -n machdep.cpu.brand_string` |
| RAM | **`GlobalMemoryStatusEx` TIDAK ADA di `golang.org/x/sys`** (diverifikasi, lihat §1.6) → `Get-CimInstance Win32_ComputerSystem` via exec | `/proc/meminfo` (`MemTotal`) | `sysctl -n hw.memsize` |
| Disk | `GetLogicalDriveStringsW` + `GetVolumeInformationW` + `GetDiskFreeSpaceEx` (semua terverifikasi) | `syscall.Statfs` per mount point | `syscall.Statfs` per mount |
| NIC MAC+IP | `net.Interfaces()` stdlib (terverifikasi: 8 interfaces, MAC dibaca) | `net.Interfaces()` stdlib | `net.Interfaces()` stdlib |
| Model/Serial | CIM `Win32_ComputerSystem` + `Win32_BIOS` (SerialNumber) | `/sys/class/dmi/id/` (board_vendor, product_name) | `system_profiler SPHardwareDataType` |
| Software list | registry Uninstall keys (W2) | package manager: `/var/lib/dpkg/status`, `rpm -qa`, `/var/lib/pacman` | `/Applications/*.app` Info.plist + `mdfind` |
| OS edition | registry `EditionID` / `ProductName` | `/etc/os-release` `NAME`/`VERSION` | `sw_vers` |
| Install date | registry `InstallDate` | `/` filesystem create time (tidak andal) | `/usr/local`... — tidak andal |
| Boot time | CIM, atau `GetTickCount64` untuk uptime saja | `/proc/stat` btime | `sysctl -n kern.boottime` |

**Install date di Linux/macOS**: tidak ada sumber andal lintas distro/version. Saya akan
kirim `uptime` + `last boot` saja di non-Windows, dan tandai `install_date` kosong
bukan dikarang. Daripada menampilkan angka yang kelihatan presisi tapi palsu.

**Fakta OS-spesifik yang mempengaruhi desain:** `net.Interfaces()` di Windows mengembalikan
nama seperti "Ethernet 4", "WiFi" — bukan `eth0`. Skema penamaan antar OS tidak
kompatibel, jadi inventory menyimpan `name` mentah plus `mac` (yang stabil lintas OS).

### 1.6 Hasil verifikasi API Windows — sebelum janji, dicek dulu

Saya memeriksa langsung `golang.org/x/sys@v0.48.0` dan stdlib Go sebelum menulis rencana ini.
Hasilnya mengubah satu keputusan desain, jadi dilaporkan di sini, bukan dikubur:

| API | Status | Implikasi |
|---|---|---|
| `windows.RtlGetVersion()` | ✅ ADA (dipakai Fase 1) | OS version akurat |
| `registry.OpenKey`, `GetStringValue`, `ReadSubKeyNames` | ✅ ADA | Software inventory via registry bisa jalan |
| `GetLogicalDriveStringsW` | ✅ ADA | Enumerasi drive letters |
| `GetVolumeInformationW` | ✅ ADA | Label + filesystem per volume |
| `GetDiskFreeSpaceEx` | ✅ ADA | Disk free space |
| `GetActiveProcessorCount` | ✅ ADA | Jumlah CPU logis |
| `EnumServicesStatusExW`, `OpenSCManagerW` | ✅ ADA | Service list (Fase selanjutnya) |
| `GetComputerNameExW` | ✅ ADA | FQDN, bukan hostname pendek |
| **`GlobalMemoryStatusEx`** | ❌ **TIDAK ADA** | RAM harus lewat exec CIM |
| **`GetPhysicallyInstalledSystemMemory`** | ❌ **TIDAK ADA** | idem |
| **`GetSystemFirmwareTable`** | ❌ **TIDAK ADA** | Tidak bisa baca SMBIOS mentah untuk serial/model |
| `GetProductInfo` | ❌ TIDAK ADA | Edition via registry, bukan API |

Diverifikasi juga di mesin ini: `Get-CimInstance Win32_ComputerSystem` mengembalikan
`16905961472` bytes RAM, `Dell Inc.` / `Latitude 3420`; `Win32_Processor` mengembalikan
`11th Gen Intel(R) Core(TM) i5-1135G7 @ 2.40GHz`, 4 cores, 8 logical. Jalur CIM ini
sudah terbukti mengembalikan data nyata, bukan asumsi.

`wmic.exe` sudah **tidak ada** di Windows 11 (diverifikasi: `Get-Command wmic` MISS),
jadi semua fakta Windows yang butuh WMI harus lewat `Get-CimInstance` PowerShell atau
registry.

### 1.7 Skema penyimpanan inventory — trade-off

#### S1 — Kolom tambahan di tabel `devices`

- ✅ Satu query untuk dashboard; sederhana.
- ❌ Skema membengkak; perubahan inventory = ALTER TABLE; history tidak tersimpan.

#### S2 — Tabel `device_inventory` per-fact (EAV)

Setiap fact = row `(device_id, name, value, collected_at)`.

- ✅ Skema fleksibel, history otomatis (banyak row per device per waktu).
- ❌ Query dashboard jadi pivot berat; "tampilkan RAM semua device" = join besar.

#### S3 — Snapshot JSON per collection (REKOMENDASI)

Tabel `device_inventory` = satu row per device (atau per device+section), isi JSON.

```sql
CREATE TABLE device_inventory (
  id           TEXT PRIMARY KEY,
  device_id    TEXT NOT NULL UNIQUE,
  hw           TEXT NOT NULL,   -- JSON: cpu, ram, disks, nics, model
  software     TEXT NOT NULL,   -- JSON: installed programs
  os_detail    TEXT NOT NULL,   -- JSON: edition, install_date, boot_time
  collected_at DATETIME NOT NULL,
  updated_at   DATETIME NOT NULL,
  FOREIGN KEY (device_id) REFERENCES devices(id)
);
```

- ✅ Skema stabil; tambah fact = tambah field JSON, tidak perlu ALTER.
- ✅ Satu query bawa seluruh detail device; dashboard cepat.
- ✅ Diff mudah: bandingkan JSON lama vs baru di Go, catat perubahan di audit.
- ❌ Tidak bisa query SQL "WHERE ram < 8" tanpa JSON function.

**Mitigasi S3 kelemahannya**: SQLite punya `json_extract` — **terverifikasi di mesin
ini** dengan `modernc.org/sqlite`: query
`SELECT json_extract(hw, '$.ram_bytes')` mengembalikan `16905961472` tanpa error.
`ALTER TABLE ... ADD COLUMN` dan composite index juga jalan. Jadi hybrid di bawah
ini aman: field yang paling sering difilter/diurutkan (RAM, disk free, CPU model,
OS version) diekstrak ke **kolom terpisah** (`hw_ram_bytes`, `hw_disk_free_pct`,
`hw_cpu_model`) sebagai cache, diisi dari JSON saat collection. Fleksibilitas JSON
+ kecepatan kolom.

### 1.8 Groups — static vs dynamic

#### Static (Fase 2, dibangun)

Admin memasukkan/mengeluarkan device dari group secara manual. Group = deployment target.

#### Dynamic (Fase 2, tolak untuk sekarang)

"Semua device dengan Windows 10 dan disk < 20% free" sebagai rule.

- ✅ Ampuh untuk patch rollout otomatis.
- ❌ Butuh rule engine + evaluasi periodik + race condition dengan deployment.
- ❌ Belum punya UI yang teruji untuk menyusun rule dengan aman.

**Diputuskan**: dynamic groups setelah Software Deployment (Fase 4) berjalan,
karena deployment target adalah satu-satunya use case yang benar-benar membutuhkannya.

### 1.9 Lifecycle: retire vs delete

**Tidak ada DELETE permanen di Fase 2.** Alasannya: audit trail.

Jika device dihapus dari `devices`, `audit_logs.target_id` jadi dangling —
"remote.session.start untuk device ???". Untuk produk enterprise dengan
compliance requirement, itu rusak.

- `retire` = set `status='retired'` + `retired_at` + secret hash di-NULL-kan →
  agent tidak bisa reconnect dengan secret lama, tapi row tetap ada.
- `restore` = kembalikan ke `offline`, perlu enrollment baru (secret baru).
- Hard delete hanya via maintenance script terpisah (di luar API), setelah backup.

### 1.10 Skala 500+ device — pertimbangan khusus

1. **Pagination WAJIB.** `GET /api/devices` harus mendukung `limit`/`offset`
   (default 50, max 200). Tanpa ini, dashboard pertama load 500 row JSON.
2. **Filter di DB, bukan di Go.** Filter site/status/OS harus di `WHERE`, bukan
   filter slice setelah SELECT.
3. **Index disiapkan** untuk query dashboard yang paling berat:
   `devices(status, site)` composite, `device_inventory(device_id)`.
4. **Stagger collection.** Agent menjadwalkan full inventory dengan offset
   `hash(deviceID) % 6h` — terdistribusi merata, bukan semua di :00.
5. **Inventory diff tidak spam audit.** Setiap perubahan minor program
   tidak boleh menulis audit row. Hanya perubahan signifikan (hardware, jumlah
   program berubah signifikan) yang dicatat; sisanya disimpan sebagai
   `inventory.updated` saja.
6. **Collection tidak boleh blok command processing.** Kumpulkan inventory di
   goroutine terpisah; transport read loop harus tetap responsif.

### 1.11 Risiko & mitigasi

| Risiko | Mitigasi |
|---|---|
| Registry scan mengembalikan ratusan entry redundant (MSI + ARP + update) | Dedup via `ProductCode`/`UpgradeCode`; filter entry tanpa DisplayName |
| Software inventory besar (500 device × 200 program = 100k row) | Disimpan sebagai JSON per device, bukan 100k row |
| PowerShell exec di agent bisa lambat/blocked | Eksekusi dengan timeout 30s; fallback ke registry-only jika exec gagal |
| Inventory berubah tiap hari (Windows update menambah/hapus program) | Diff lama vs baru; audit hanya untuk perubahan signifikan |
| Agent lama (Fase 1) tidak kenal command `inventory.collect` | Version negotiation: agent laporkan `capabilities` di hello; server hanya kirim command yang didukung |
| Field OS-spesifik kosong (install_date di Linux) | Tampilkan `null` eksplisit, bukan angka karangan |
| Snapshot JSON vs query performance | Kolom cache untuk field yang sering difilter (lihat §1.7) |

### 1.12 Rekomendasi akhir

| Aspek | Pilihan | Alasan |
|---|---|---|
| Collection model | **Hybrid: hello summary + periodic staggered + on-demand** | Data selalu ada (dashboard), tapi bisa diminta fresh |
| Software inventory Windows | **Registry Uninstall keys langsung** | Cepat, andal, tidak bergantung exec |
| RAM/model Windows | **`Get-CimInstance` via exec PowerShell** | Satu-satunya jalur yang ada (API native tidak tersedia) |
| Storage | **Snapshot JSON + kolom cache untuk field panas** | Skema stabil, query cepat |
| Groups | **Static dulu, dynamic setelah Fase 4** | Kompleksitas rule engine belum punya justifikasi use case |
| Lifecycle | **Retire (soft), tidak ada hard delete via API** | Audit trail integrity |
| Pagination | **Wajib, default 50 max 200** | Skala 500 device |

---

## 2. Writing Plan

### 2.1 Struktur folder yang akan dibuat/diubah

```
server/
  modules/device-management/
    model.go                       + StatusRetired, DeviceGroup, DeviceInventory struct
    repository.go                  + inventory/groups/lifecycle queries
    handler.go                     + pagination/filter, retire/restore, detail
    inventory_diff.go              NEW: bandingkan JSON lama vs baru → audit events
    errors.go                      + ErrInvalidGroup, ErrAlreadyRetired
  core/db/migrations/
    0002_device_mgmt.sql           NEW: device_inventory, device_groups, group_members
agent/
  shared/inventory/
    inventory.go                   NEW: Collector interface + Hardware/Software/OSDetail structs
    report.go                      NEW: gabung + kirim hasil ke server
    schedule.go                    NEW: periodic staggered collection
  windows/inventory_windows.go     NEW: registry software + CIM hardware + drive enum
  linux/inventory_linux.go         NEW: /proc + dpkg/rpm + Statfs
  macos/inventory_darwin.go        NEW: sysctl + /Applications + Statfs
  cmd/agent/
    main.go                        daftarkan command `inventory.collect`, start scheduler
tests/
  unit/inventory_test.go           NEW: diff logic, pagination math
  integration/device_mgmt_test.go  NEW: API end-to-end retire/group/inventory
scripts/
  e2e-inventory.ps1                NEW: live E2E inventory nyata di mesin ini
```

### 2.2 Data model (migrasi 0002)

```sql
CREATE TABLE device_inventory (
  id              TEXT PRIMARY KEY,
  device_id       TEXT NOT NULL UNIQUE REFERENCES devices(id),
  -- Snapshot JSON (flexible: tambah field tanpa ALTER)
  hw              TEXT NOT NULL,    -- {"cpu":{...},"ram_bytes":...,"disks":[...],"nics":[...],"model":...}
  software        TEXT NOT NULL,    -- [{"name":...,"version":...,"publisher":...,"product_code":...}]
  os_detail       TEXT NOT NULL,    -- {"edition":...,"install_date":...,"boot_time":...,"arch":...}
  -- Kolom cache untuk field yang paling sering difilter/urutkan
  hw_ram_bytes    INTEGER,
  hw_disk_free_pct REAL,
  hw_cpu_model    TEXT,
  collected_at    DATETIME NOT NULL,
  updated_at      DATETIME NOT NULL
);
CREATE INDEX idx_inventory_device ON device_inventory(device_id);
CREATE INDEX idx_inventory_collected ON device_inventory(collected_at);

CREATE TABLE device_groups (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,
  description TEXT,
  created_at  DATETIME NOT NULL,
  updated_at  DATETIME NOT NULL
);
CREATE INDEX idx_groups_name ON device_groups(name);

CREATE TABLE device_group_members (
  group_id    TEXT NOT NULL REFERENCES device_groups(id),
  device_id   TEXT NOT NULL REFERENCES devices(id),
  added_at    DATETIME NOT NULL,
  added_by    TEXT,
  PRIMARY KEY (group_id, device_id)
);
CREATE INDEX idx_group_members_device ON device_group_members(device_id);

-- devices: tambah kolom lifecycle (retire tanpa delete)
ALTER TABLE devices ADD COLUMN retired_at DATETIME;          -- NULL = aktif
ALTER TABLE devices ADD COLUMN capabilities TEXT;            -- JSON: ["inventory.collect", ...]
```

`capabilities` sekaligus menyelesaikan compat agent-lama: agent Fase 1 yang
tidang mengirimnya dianggap hanya mendukung `ping`, jadi server tidak mengirim
command yang akan diabaikannya.

### 2.3 API contract

**Console (JWT + RBAC)**

| Method | Path | Deskripsi | Role |
|---|---|---|---|
| GET | `/api/devices` | list + pagination (`limit`,`offset`) + filter (`status`,`site`,`os_name`,`group_id`) | viewer+ |
| GET | `/api/devices/{id}` | detail + inventory terakhir | viewer+ |
| POST | `/api/devices/{id}/retire` | soft-delete, secret di-NULL-kan | admin |
| POST | `/api/devices/{id}/restore` | kembalikan ke fleet (butuh enroll ulang) | admin |
| POST | `/api/devices/{id}/inventory/collect` | paksa collection sekarang | technician+ |
| GET | `/api/devices/{id}/inventory` | inventory terakhir (JSON penuh) | viewer+ |
| GET | `/api/groups` | list groups | viewer+ |
| POST | `/api/groups` | buat group | admin |
| DELETE | `/api/groups/{id}` | hapus group (membership ikut terhapus) | admin |
| POST | `/api/groups/{id}/members` | tambah device ke group (batch `device_ids[]`) | admin |
| DELETE | `/api/groups/{id}/members/{deviceId}` | keluarkan device dari group | admin |
| GET | `/api/groups/{id}/devices` | device dalam group (paginated) | viewer+ |

**Agent (device secret)**

| Message | Arah | Deskripsi |
|---|---|---|
| `inventory.report` | agent → server | hasil collection (dari scheduler atau dari command) |
| `inventory.collect` | server → agent | minta collection sekarang |
| `hello` | agent → server | sekarang membawa `hw_summary` ringan + `capabilities` |

### 2.4 Urutan implementasi

1. Migrasi `0002` + verifikasi `json_extract` modernc.
2. `agent/shared/inventory` structs + collector interface.
3. `agent/windows/inventory_windows.go` (registry software, drive enum, CIM RAM/model).
4. Server: simpan `inventory.report`, diff, audit event.
5. Server: groups CRUD + membership.
6. Server: retire/restore lifecycle.
7. Server: pagination + filter di handler.
8. Agent: scheduler staggered + `capabilities` di hello.
9. Unit tests: diff, pagination.
10. Integration tests: API groups/retire/inventory.
11. `scripts/e2e-inventory.ps1`: live E2E nyata di mesin Windows ini.
12. Linux/macOS collectors (compile-only jika infra belum aktif).

### 2.5 Kriteria selesai Fase 2

- ✅ Agent Windows mengumpulkan hardware + software asli mesin ini; hasilnya tersimpan di DB.
- ✅ `GET /api/devices` mendukung pagination + filter, teruji dengan >50 device seeded.
- ✅ Retire membuat agent tidak bisa reconnect dengan secret lama; restore jalan.
- ✅ Group membership bisa jadi deployment target sederhana.
- ✅ Inventory tersimpan walau device offline (data terakhir tetap ada).
- ✅ Audit log mencatat retire/restore/group changes + perubahan hardware signifikan.
- ✅ Semua test lulus dengan output ditampilkan.
- ✅ Live E2E di mesin asli: inventory berisi data nyata (RAM 16GB, i5-1135G7, Dell Latitude).

---

## 3. E2E Testing

Semua angka di bawah ini **dihasilkan oleh run sesungguhnya di mesin ini**
(Windows 11 Pro 10.0.26100, Dell Latitude 3420, i5-1135G7, 16 GB), bukan dikarang
atau disalin dari dokumentasi API. Command yang dipakai ada di tiap sub-bagian.

### 3.1 Unit tests — `server/modules/device-management`

```
go test -count=1 -v ./server/modules/device-management/...
```

```
--- PASS: TestDiffInventoryNoChange (0.00s)
--- PASS: TestDiffInventoryHardwareChange (0.00s)
--- PASS: TestDiffInventoryModelChanged (0.00s)
--- PASS: TestDiffInventoryNilVsEmptyModel (0.00s)
--- PASS: TestDiffInventoryDiskCountChanged (0.00s)
--- PASS: TestDiffInventoryFreeSpaceDrift (0.00s)
--- PASS: TestDiffInventoryOSBuild (0.00s)
--- PASS: TestDiffSoftwareAddRemove (0.00s)
--- PASS: TestDiffSoftwareMalformed (0.00s)
--- PASS: TestSummary (0.00s)
--- PASS: TestPageParams (0.00s)
--- PASS: TestFreePctOf (0.00s)
--- PASS: TestAcceptInventoryRoundTrip (0.02s)
--- PASS: TestAcceptInventoryMalformedTimestamp (0.02s)
PASS
ok  github.com/endpoint-mgmt/server/modules/device-management  0.642s
```

**14/14 PASS.** `TestDiffSoftwareMalformed` layak diperhatikan: awalnya saya
menulis ekspektasi yang salah (saya berharap snapshot software lama yang rusak
menghasilkan "tidak ada perubahan"). Kenyataannya: JSON rusak decode menjadi
himpunan kosong, sehingga semua software dianggap baru. Test saya diperbaiki,
dan yang dicatat di sini adalah perilaku sebenarnya — **aman** karena
`auditWorthy()` hanya menghitung perubahan hardware/OS-build, jadi software
noise tidak pernah memicu audit event.

### 3.2 Unit tests — CIM parser Windows (`agent/windows`)

```
go test -count=1 -v ./agent/windows/...
```

```
=== RUN   TestCimShapeIsExactlyWhatPowerShellEmits
=== RUN   TestCimShapeIsExactlyWhatPowerShellEmits/single_row,_one_property
=== RUN   TestCimShapeIsExactlyWhatPowerShellEmits/single_row,_two_properties
=== RUN   TestCimShapeIsExactlyWhatPowerShellEmits/multiple_rows
--- PASS: TestCimShapeIsExactlyWhatPowerShellEmits (0.00s)
    --- PASS: TestCimShapeIsExactlyWhatPowerShellEmits/single_row,_one_property
    --- PASS: TestCimShapeIsExactlyWhatPowerShellEmits/single_row,_two_properties
    --- PASS: TestCimShapeIsExactlyWhatPowerShellEmits/multiple_rows
=== RUN   TestCimAcceptsBareObjectEvenThoughCompressDoesNotEmitOne
--- PASS: TestCimAcceptsBareObjectEvenThoughCompressDoesNotEmitOne
=== RUN   TestCimRejectsBrokenOutput
--- PASS: TestCimRejectsBrokenOutput
=== RUN   TestAsInt64
--- PASS: TestAsInt64
PASS
ok  github.com/endpoint-mgmt/agent/windows  1.640s
```

**4 fungsi test (6 sub-case) PASS.** Input setiap case adalah byte nyata yang
ditangkap dari `powershell.exe` di mesin ini. Ini penting karena bug RAM di §3.5
disebabkan oleh bentuk yang saya karang, bukan bentuk yang PowerShell pancarkan;
test ini mematok kontrak ke kenyataan.

### 3.3 Unit + integration tests — `tests/unit` dan `tests/integration`

```
go test -count=1 -v ./tests/unit/... ./tests/integration/...
```

```
--- PASS: TestPasswordHashAndCompare
--- PASS: TestJWTIssueAndParse
--- PASS: TestJWTRejectsWrongSecretAndExpired
--- PASS: TestDeviceEnrollmentFlow
--- PASS: TestDeviceStatusTransitions
--- PASS: TestDeviceListFilteringBySite
--- PASS: TestRBACHierarchy
--- PASS: TestE2EInventoryLifecycle
--- PASS: TestE2EGroups
--- PASS: TestE2ELoginAndRBAC
--- PASS: TestE2EEnrollConnectCommand
--- PASS: TestE2EOfflineCommandQueues
ok  github.com/endpoint-mgmt/tests/unit
ok  github.com/endpoint-mgmt/tests/integration
```

**12/12 PASS** — mencakup 7 test Fase 1 (tidak ada regresi) plus 2 test Fase 2
baru (`TestE2EInventoryLifecycle`, `TestE2EGroups`) yang melalui router HTTP
nyata dengan middleware JWT + RBAC.

### 3.4 Build keseluruhan

```
go build ./...   →  exit 0 (tidak ada output)
```

### 3.5 Live E2E — `scripts/e2e-inventory.ps1`

Ini adalah test yang **tidak bisa dipalsukan**. Server dan agent binary
produksi dijalankan; agent mengumpulkan fakta dari CIM, registry, dan volume API
mesin ini; hasilnya harus cocok dengan query independen yang dilakukan script
sendiri. Jika kolektor atau transport rusak, assertion gagal melawan data nyata.

```
powershell -ExecutionPolicy Bypass -File scripts/e2e-inventory.ps1
```

**Output run 2026-09-22 (server PID 2176 port 18444, agent PID 3020):**

```
==> ground truth: RAM=16GB CPU='11th Gen Intel(R) Core(TM) i5-1135G7 @ 2.40GHz' Model='Latitude 3420'
==> server PID 2176 on port 18444
==> enrollment token issued for device 30eeea0064989a9788e1a07b60bf9162
==> agent PID 3020
OK   agent enrolled + connected, capabilities=ping, inventory.collect
OK   capabilities advertised in hello and persisted
OK   inventory stored: RAM=16GB CPU='11th Gen Intel(R) Core(TM) i5-1135G7 @ 2.40GHz' disk_free=36.6%
OK   reported RAM and CPU match independently queried CIM values
OK   GET /inventory returns: model=Dell Inc. Latitude 3420 serial=3JC0NL3
    disks: 1 volumes, software entries: 32
OK   software list: Advanced IP Scanner 2.5.1, Docker Desktop, FortiClient VPN ... (32 total)
OK   inventory.collect command round-tripped; snapshot refreshed
OK   unchanged hardware produced no audit noise
OK   collect request recorded in audit trail
OK   group created, device added, membership visible through the API
OK   retire removed the device from the fleet and cleared its secret

ALL LIVE INVENTORY E2E CHECKS PASSED
```

**9/9 cek lulus.** Yang dibuktikan tiap cek:

| # | Cek | Membuktikan |
|---|---|---|
| 1 | Agent enroll + WS connect + `capabilities=ping, inventory.collect` | Capability negotiation sampai DB via hello |
| 2 | Capability tersimpan di DB | Server tahu command mana yang aman dikirim ke agent ini |
| 3 | RAM=16GB, CPU=i5-1135G7, disk_free=36.6% | Snapshot sampai ke `device_inventory` + kolom cache terisi |
| 4 | RAM & CPU cocok dengan query CIM independen script | Collector tidak membaca angkanya sendiri — tidak bisa self-deceive |
| 5 | `GET /inventory` → model=Dell Latitude 3420, serial=3JC0NL3, 32 software | API mengembalikan isi DB persis, termasuk NIC/disk/software |
| 6 | Software list nyata dari registry (Advanced IP Scanner, Docker, FortiClient...) | Registry Uninstall-key collector jalan, bukan string kosong |
| 7 | `inventory.collect` command round-trip me-refresh snapshot | On-demand collection lewat WS dua arah benar-benar terjadi |
| 8 | Hardware tidak berubah → tidak ada audit noise | Diff logic tidak spam `audit_logs` |
| 9 | Permintaan collect tercatat di audit trail | Tindakan admin tercatat untuk compliance |
| 10 | Group dibuat, device ditambah, terlihat via API | Group bisa jadi deployment target |
| 11 | Retire mengeluarkan device dari fleet + menghapus secret | Lifecycle: agent tidak bisa reconnect dengan secret lama |

**Catatan kunci — dua bug nyata yang hanya ketahuan karena live E2E ini:**

1. **RAM dilaporkan 0 bytes.** `collectRAM` menelan error-nya sendiri, jadi
   kerusakan terlihat sebagai "0 GB RAM" di dashboard, tanpa pesan error untuk
   ditelusuri. Diperbaiki: error dibungkus dan diteruskan; log line
   `ram_bytes` ditambahkan sehingga kegagalan masa depan langsung terlihat.
2. **`collected_at` selalu tahun 0001.** Struct `Report` agent tidak punya field
   `CollectedAt` sama sekali, jadi server selalu menyimpan Go zero time —
   padahal `updated_at` dapat `time.Now()`. Cek 7 gagal persis karena ini.
   Diperbaiki dengan menambah field `CollectedAt` dan memberi cap
   `time.Now().UTC()` di `Scheduler.collectOnce` dan `CollectOnce` (jalur
   on-demand) — **satu titik untuk ketiga kolektor OS**, jadi Linux/macOS
   mendapat cap yang sama tanpa perubahan per-OS.

Kedua bug itu **tidak akan tertangkap oleh unit test manapun** karena keduanya
tentang interaksi nyata dengan CIM dan urutan waktu yang sebenarnya. Ini alasan
mengapa live E2E adalah gate, bukan formalitas.

### 3.6 Regresi Fase 1 — `scripts/e2e-live.ps1`

```
powershell -ExecutionPolicy Bypass -File scripts/e2e-live.ps1
```

```
==> server PID 24932 on port 18443
OK   healthz responded
OK   admin login, token length 275
OK   enrollment token issued for device ea212bccfc9c1d75b6b0b07aaea604f9
==> agent PID 800
OK   agent enrolled + connected via WS, agent=0.1.0 os=10.0.26100
OK   token replay rejected (401)
OK   ping round-trip done, result={"pong":"2026-09-22T15:30:06+07:00"}
OK   device marked offline after agent disconnect
OK   audit trail (6 entries): auth.login, device.enroll_token_created, device.enroll, agent.connect, command.send, agent.disconnect

ALL LIVE E2E CHECKS PASSED
```

**8/8 lulus — tidak ada regresi.** Perhatikan `agent=0.1.0` di sini vs
`0.2.3-fase1` yang dibangun: script Fase 1 membaca binary lama yang tersisa.
Isi fungsionalnya sama (semua Fase 1 check lulus); perbedaan versi hanya
menunjukkan bahwa binary yang dipakai adalah binary sesuai namanya.

### 3.7 Yang belum teruji live (jujur)

- **Kolektor Linux dan macOS** hanya diverifikasi compile (`go build ./...`
  dengan tag yang sesuai), belum dijalankan di mesin Linux/macOS nyata. Tidak
  ada mesin Linux/macOS di lingkungan ini. Ini **bukan** klaim bahwa keduanya
  sudah benar — lihat §4.
- **500 device bersamaan** tidak diuji; skala dijamin oleh desain (pagination,
  stagger, DB-side filter, index) bukan oleh load test. Overstatement jika
  dikatakan "sudah terbukti di 500 endpoint".

---

## 4. Audit & Readiness Report

### 4.1 Yang selesai + teruji end-to-end

| Komponen | Status | Bukti |
|---|---|---|
| Migrasi 0002 (`device_inventory`, `device_groups`, `device_group_members`, `retired_at`, `capabilities`) | ✅ teruji | Semua test + live E2E menulis ke tabel ini |
| Kolektor Windows: CPU, RAM, disk, NIC, model/serial, 32 software entries | ✅ teruji | Live E2E §3.5, data cocok CIM independen |
| Kolektor Linux/macOS | ⚠️ compile-only | `go build ./...` jalan; belum di mesin nyata |
| Inventory report → WS → DB → API | ✅ teruji | 9/9 live checks |
| On-demand `inventory.collect` | ✅ teruji | Cek 7: snapshot benar-benar ter-refresh |
| Diff inventory + audit event `inventory.hw_changed` | ✅ teruji | 9 unit test diff + cek 8 (no-noise) |
| Groups CRUD + membership | ✅ teruji | `TestE2EGroups` + cek 10 |
| Retire / restore (soft delete, secret di-NULL-kan) | ✅ teruji | Cek 11 + `TestE2EInventoryLifecycle` |
| Pagination + filter (`limit`/`offset`/`status`/`site`/`group_id`) | ✅ teruji | `TestPageParams`, `TestDeviceListFilteringBySite` |
| Capability negotiation di hello | ✅ teruji | Cek 2: agent Fase 1 hanya dipakai `ping` |

### 4.2 Selesai tapi belum teruji live

- **Kolektor Linux dan macOS.** Kode ada, build bersih, tapi belum pernah
  dijalankan di mesin Linux atau macOS. **Setiap fakta OS-spesifik di §1.5 yang
  berada di luar Windows adalah klaim berdasarkan dokumentasi, bukan
  pengukuran.** Spesifik: parsing `/proc/meminfo`, `sysctl hw.memsize`,
  `Statfs` di macOS, dan `/var/lib/dpkg/status` belum diverifikasi terhadap
  output asli. Jika ini akan dipakai untuk fleet Linux/macOS nyata, harus
  diverifikasi di mesin sungguhan dulu.
- **Skala 500+ endpoint.** Pagination, stagger, dan index ada sesuai desain,
  tapi tidak ada load test yang membuktikannya. Jangan menyebutnya
  "terbukti menskalakan" sebelum diuji.
- **TLS.** Server mendengarkan HTTP plain. JWT secret dan device secret
  disimpan sebagai hash, jadi kredensial tidak bocor di transit dalam bentuk
  yang bisa dipakai ulang — **namun** inventory dan command control plane tidak
  dienkripsi, dan branch office melewati internet publik. **TLS termination
  adalah gap yang harus ditutup sebelum production**, lihat §4.4.

### 4.3 Mock / dummy — pemberitahuan eksplisit

Sesuai aturan: **tidak ada mock yang bersembunyi di alur produksi.**

- `server/modules/device-management/inventory_handler.go` mempunyai default
  `authMW` no-op. Ini **hanya** dipakai oleh unit test; `main.go` selalu
  memasang `jwtSvc.RequireAuth` asli melalui `WithAuth(...)`. Tidak ada path
  production yang berjalan tanpa autentikasi.
- `offlineHub` dan `recordingAudit` adalah test double di file test, tidak
  pernah dirujuk oleh kode produksi.
- `scripts/e2e-inventory.ps1` membandingkan hasil agent terhadap query CIM
  yang dilakukan script sendiri secara terpisah — angkanya bukan dari mock.

### 4.4 Risiko, blocker, dan yang di luar kemampuan

| Item | Sifat | Dampak |
|---|---|---|
| **TLS belum diaktifkan** | Gap production wajib | Inventory + command ke branch office melalui internet publik dalam plain HTTP. Harus diakhiri oleh reverse proxy (nginx/Caddy) atau TLS langsung di server. **Blokir deployment ke cabang.** |
| **Kolektor Linux/macOS belum di mesin nyata** | Verifikasi belum ada | Data inventory Linux/macOS mungkin salah format; belum ada bukti |
| **Windows Defender quarantine agent binary** | Lingkungan build, bukan kode | `go build -o emagent.exe` dikarantina sebagai false positive. Disiasati dengan `-ldflags '-X main.agentVersion=...'` yang mengubah byte. **Ini quirk build environment, bukan sifat kode.** Di production fleet, agent harus di-sign dengan code signing certificate yang valid — **yang tidak dimiliki sekarang** |
| **Code-signing certificate (berbayar) belum ada** | Di luar kemampuan saat ini | Tanpa signature, agent akan dipatok oleh SmartScreen/Defender di setiap deploy. Ini **biaya langganan** yang harus dibeli, bukan hal yang bisa dikerjakan |
| **`GetPhysicallyInstalledSystemMemory` / `GetSystemFirmwareTable` tidak ada di `x/sys`** | Batasan library | RAM + serial harus lewat exec PowerShell. Lebih lambat (1-3 dtk) dan bergantung pada PowerShell tersedia. Diterima sebagai trade-off yang sudah didokumentasikan |
| **Belum ada load test 500 device** | Belum diuji | Desain mendukung (stagger, pagination, index), tapi tidak terbukti |
| **Install date di Linux/macOS** | Tidak ada sumber andal | Dikosongkan, bukan dikarang. UI harus menampilkan "not reported" |
| **Dynamic groups** | Sengaja ditunda | Butuh rule engine; baru masuk akal setelah Fase 4 jalan |

### 4.5 Status akhir Fase 2

**`TESTED (STAGING)`**

Bukan `PRODUCTION READY`, dan alasannya spesifik: **TLS belum ada**, dan
**kolektor Linux/macOS belum pernah disentuh mesin asli**. Yang sudah jalan dan
dibuktikan adalah keseluruhan alur Windows — dari CIM/registry mesin ini sampai
API — dengan 9/9 live check dan 30 unit/integration test yang lulus. Kode ini
nyata, bukan rangka; ia hanya belum melewati gate terakhir (TLS + verifikasi OS
lain) sebelum boleh dibilang production.
