# Fase 7 — Architectural Blueprint: User Management

**Versi:** 1.0.0  
**Tanggal:** 2026-09-24  
**Status:** DRAFT (READY FOR IMPLEMENTATION)  
**Target Modul:** `server/modules/user-management`, `web-console/src/pages/UsersPage.tsx`

---

## 1. Latar Belakang & Kebutuhan Bisnis

Saat ini, platform hanya menyediakan satu user admin yang di-bootstrap melalui environment variable `ADMIN_PASSWORD`. Untuk deployment multi-operator di jaringan cabang enterprise, diperlukan kemampuan pengelolaan pengguna (*user lifecycle management*) yang lengkap:
1. Membuat (*create*), memperbarui (*update*), dan menonaktifkan (*deactivate*) akun pengguna.
2. Menetapkan dan mengubah peran (*role assignment*): `viewer`, `technician`, `admin`.
3. Mengubah kata sandi (*password change*) baik secara mandiri (self-service) maupun oleh administrator (admin reset).
4. Menyediakan daftar seluruh pengguna yang dapat dilihat oleh admin.
5. Mencatat seluruh operasi pengelolaan pengguna ke dalam audit log.

---

## 2. Skema Basis Data

Tabel `users` sudah ada dari Fase 1 (kolom: `id`, `username`, `password_hash`, `role`, `created_at`, `updated_at`). Fase 7 menambahkan kolom untuk dukungan deaktivasi:

```sql
-- 0007_user_management.sql
ALTER TABLE users ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN last_login_at DATETIME;
ALTER TABLE users ADD COLUMN display_name TEXT;
```

---

## 3. REST API Endpoints

| Method | Endpoint | Akses RBAC | Deskripsi |
|---|---|---|---|
| `GET` | `/api/users` | Admin | Daftar seluruh pengguna (tanpa password_hash) |
| `GET` | `/api/users/{id}` | Admin | Detail pengguna tunggal |
| `POST` | `/api/users` | Admin | Membuat pengguna baru |
| `PUT` | `/api/users/{id}` | Admin | Memperbarui display_name, role, is_active |
| `PUT` | `/api/users/{id}/password` | Admin | Reset kata sandi pengguna |
| `PUT` | `/api/users/me/password` | All (self-service) | Mengubah kata sandi sendiri |
| `DELETE` | `/api/users/{id}` | Admin | Menonaktifkan pengguna (soft delete via `is_active = 0`) |

---

## 4. Rencana Eksekusi Bertahap

1. **Step 1**: Buat skrip migrasi database `0007_user_management.sql`.
2. **Step 2**: Implementasikan modul backend `server/modules/user-management` (model, repository, handler, routing).
3. **Step 3**: Integrasikan antarmuka Web Console `UsersPage.tsx`.
4. **Step 4**: Tulis unit & integration tests.
5. **Step 5**: Buat skrip E2E `scripts/e2e-user-management.ps1`.
6. **Step 6**: Buat laporan audit `docs/readiness-reports/phase-7-user-management.md` dan perbarui scorecard.
