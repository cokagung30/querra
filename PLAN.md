# PLAN: Querra – Rencana Implementasi

*Turunan dari PRD v0.1 (7 November 2026) · Status: belum di-scaffold · Pelaksana: 1 developer + Claude Code*

Dokumen ini menerjemahkan PRD (`docs/PRD Querra – Database Management Client.md`) menjadi keputusan teknis, arsitektur, dan milestone yang bisa dikerjakan berurutan. Dokumen ini hidup: centang tugas saat selesai, dan perbarui bagian arsitektur bila implementasi menyimpang.

## 0. Cara Memakai Dokumen Ini

- Milestone diberi kode `M<fase>.<nomor>` (mis. `M1.7`) dan dikerjakan berurutan. Setiap milestone adalah potongan vertikal yang bisa didemokan, berukuran kira-kira 1–4 sesi kerja dengan Claude Code.
- Jangan mulai fitur milestone berikutnya sebelum milestone aktif memenuhi *Definition of Done* (§4.7).
- Kode milestone dipakai di pesan commit, mis. `feat(grid): M1.7 server-side pagination`.
- Estimasi fase mengikuti PRD (total ±20 minggu). Pada mode solo, hambatan terbesar biasanya bukan menulis kode, tetapi verifikasi manual lintas OS, code signing, dan pengujian dengan database sungguhan.

## 1. Keputusan Teknis

| Area | Pilihan | Catatan |
| --- | --- | --- |
| Desktop shell | **Wails v3** (beta) | Versi di-pin persis di `go.mod`. API Wails hanya dipakai di `main.go` dan `internal/app/`. |
| Backend | Go 1.26 | Versi toolchain di-pin di `go.mod` dan CI. |
| Driver DB | `pgx/v5` (lewat `database/sql` + `pgx/v5/stdlib`), `go-sql-driver/mysql`, `modernc.org/sqlite` | Semua lewat `database/sql` agar adapter seragam; perilaku khusus engine ditangani di adapter. |
| SSH tunnel | `golang.org/x/crypto/ssh` | Dialer kustom yang langsung dipasang ke driver, tanpa membuka port lokal. |
| Rahasia | `zalando/go-keyring` | Keychain (macOS), Credential Manager (Windows). |
| Penyimpanan lokal | SQLite (modernc, tanpa cgo) di `os.UserConfigDir()/Querra/querra.db` | Migrasi SQL di-embed, versi skema disimpan di `PRAGMA user_version`. |
| Logging | `log/slog` ke file berotasi di direktori log aplikasi | Log tidak boleh berisi rahasia atau isi data. |
| Frontend | React + TypeScript + Vite (template `react-ts` dari Wails) | |
| UI | Tailwind CSS v4 + shadcn/ui (Radix), `lucide-react` | Lihat risiko R3 untuk macOS 12. |
| State | Zustand (UI, tab, perubahan tertunda) + TanStack Query (cache data dari Go) | |
| Grid | TanStack Table + TanStack Virtual | Divalidasi lewat spike M0.2. |
| SQL editor | **CodeMirror 6** + `@codemirror/lang-sql` | Mendukung dialek PostgreSQL/MySQL/SQLite dan autocomplete berbasis skema. |
| Layout | `react-resizable-panels` | |
| i18n | `i18next` + `react-i18next` | Bahasa Inggris dulu, struktur siap untuk `id`. |
| Pengujian | `go test`, uji integrasi (build tag `integration` + Docker Compose), Vitest + Testing Library | |
| Lint | `golangci-lint`, ESLint + Prettier, `tsc --noEmit` | |
| Package manager | npm | Sudah terpasang dan menjadi default template. |
| CI/CD | GitHub Actions (`macos-latest`, `windows-latest`) | |

## 2. Penyesuaian terhadap PRD

1. **Monaco diganti CodeMirror 6.** Monaco menambah beberapa MB ke bundle dan memakan memori, padahal targetnya memori idle < 250 MB dan waktu mulai < 2 detik. `@codemirror/lang-sql` sudah menyediakan dialek dan autocomplete skema yang dibutuhkan FR-12.
2. **Wails v3 (beta) dipilih, bukan v2.** v3 memberi `CancellablePromise` yang terhubung ke `context.Context` di Go (langsung menjawab FR-13), binding TypeScript dan event bertipe, serta dukungan multi-window. Risiko status beta dikelola lewat pin versi dan lapisan Wails yang tipis (R1).
3. **Keychain dimajukan dari Fase 2 ke Fase 1.** Koneksi yang disimpan di Fase 1 sudah butuh password. Tidak ada fase yang boleh menyimpan password sebagai teks biasa, bahkan untuk sementara.
4. **Fondasi tema dan registry shortcut dibangun di Fase 0.** Token warna, warna environment, dan registry perintah dibuat sejak awal. Toggle mode gelap tetap di Fase 3 dan UI pengaturan shortcut di Fase 4. Dengan begitu NFR "shortcut dapat diubah" tidak berujung pada refactor besar.
5. **Mode hanya-baca ditegakkan di dua lapis** (sesi database dan pemeriksaan di Go), bukan hanya di UI.
6. **Metrik crash butuh keputusan.** Target "crash < 1% sesi" (PRD §11) butuh data, sementara PRD melarang telemetri tanpa izin. Usulan: crash report *opt-in* (default mati) ditambah log lokal yang bisa dilampirkan pengguna. Lihat §8.

## 3. Arsitektur

### 3.1 Lapisan

```text
Frontend (WebView)          React · features/* · Zustand · TanStack Query · CodeMirror · TanStack Table
        │  ▲
        │  │  frontend/bindings (hasil generate, TypeScript) + typed events
        ▼  │
internal/app                Wails services — tipis: validasi input, panggil domain, petakan error
        │
        ▼                   (Go biasa, tanpa import Wails)
Domain                      session · query · resultset · changeset · sqlscan · export · importer
                            connection · store · secrets · settings · apperr
        │
        ▼                   interface driver.Driver
internal/driver/<engine>    postgres · mysql · sqlite        internal/sshtunnel
```

Arah dependensi selalu ke bawah. Paket domain tidak mengimpor Wails, jadi `go test ./internal/...` bisa berjalan di mana saja, termasuk runner CI Linux.

### 3.2 Struktur Repositori (Target)

```text
querra/
├── main.go                  # bootstrap Wails: services, window, menu
├── internal/
│   ├── app/                 # Wails services (satu file per service) + MarshalError bersama
│   ├── apperr/              # tipe error terstruktur + pemetaan error driver → pesan ramah
│   ├── connection/          # model ConnectionConfig, validasi, impor/ekspor JSON
│   ├── driver/              # interface Driver/Conn/Session/Introspector/Dialect + registry
│   │   ├── postgres/
│   │   ├── mysql/           # juga MariaDB
│   │   └── sqlite/
│   ├── sshtunnel/           # klien SSH, known_hosts, dialer, keepalive
│   ├── session/             # koneksi aktif, sesi per tab, reconnect, mode hanya-baca
│   ├── query/               # eksekusi multi-statement, pemeriksaan keamanan, pencatatan riwayat
│   ├── sqlscan/             # tokenizer: pemecah statement, deteksi query destruktif
│   ├── resultset/           # buffer hasil di Go + paging per resultID
│   ├── changeset/           # perubahan tertunda dari grid → SQL per dialek
│   ├── export/              # CSV, JSON, SQL INSERT (streaming)
│   ├── importer/            # impor CSV (pratinjau, pemetaan, batch insert)
│   ├── store/               # SQLite lokal + migrations/ (embed)
│   ├── secrets/             # interface Store + implementasi keyring + fake in-memory
│   └── settings/
├── frontend/
│   ├── bindings/            # HASIL GENERATE — jangan diedit
│   └── src/
│       ├── app/             # shell: layout, manajemen tab, provider
│       ├── features/        # connections, explorer, grid, editor, results, history,
│       │                    # changes, export, import, settings
│       ├── components/ui/   # komponen shadcn/ui
│       ├── lib/             # registry perintah/shortcut, format sel, util
│       ├── stores/          # Zustand store lintas fitur
│       └── i18n/            # en.json (id.json menyusul)
├── build/                   # konfigurasi build Wails: ikon, Info.plist, NSIS, signing
├── dev/                     # docker-compose.yml + seed SQL untuk database uji
├── docs/                    # PRD, laporan spike, ADR, checklist QA
├── Taskfile.yml
├── PLAN.md
└── CLAUDE.md
```

### 3.3 Kontrak Driver (Sketsa)

```go
package driver

type Engine string

const (
	Postgres Engine = "postgres"
	MySQL    Engine = "mysql" // juga MariaDB
	SQLite   Engine = "sqlite"
)

// Driver adalah titik masuk satu engine; didaftarkan ke registry saat init.
type Driver interface {
	Engine() Engine
	Capabilities() Capabilities // mis. HasSchemas, MultiDatabase, SupportsReturning
	Dialect() Dialect
	Open(ctx context.Context, cfg Config, dial DialFunc) (Conn, error)
}

// Conn mewakili satu target (server + database) yang sedang terbuka.
type Conn interface {
	Pool() *sql.DB                                   // grid, introspeksi, ekspor
	NewSession(ctx context.Context) (Session, error) // koneksi yang di-pin untuk satu tab editor
	Introspector() Introspector
	Close() error
}

// Session menyimpan state sesi (transaksi, SET, temp table) milik satu tab editor.
// Implementasi wajib menghormati pembatalan ctx tanpa merusak sesi (lihat §3.8).
type Session interface {
	Query(ctx context.Context, stmt string) (*sql.Rows, error)
	Exec(ctx context.Context, stmt string) (sql.Result, error)
	Close() error
}

type Introspector interface {
	Databases(ctx context.Context) ([]Database, error)
	Schemas(ctx context.Context) ([]Schema, error)
	Objects(ctx context.Context, schema string) ([]Object, error) // table, view, matview, function
	Table(ctx context.Context, ref TableRef) (*TableInfo, error)  // kolom, PK, index, FK, constraint
	EstimateRows(ctx context.Context, ref TableRef) (int64, error)
}

type Dialect interface {
	QuoteIdent(name string) string
	Placeholder(n int) string                     // "$1" vs "?"
	BuildSelect(q TableQuery) (string, []any)     // filter, sort, limit/offset
	BuildChange(c RowChange) (string, []any)      // insert/update/delete satu baris
	ReadOnlySessionSQL() string
}
```

Engine baru cukup mengimplementasikan interface ini. UI tidak boleh bercabang berdasarkan engine, kecuali lewat `Capabilities`.

### 3.4 Alur Data Hasil Query

**SQL editor**

1. Frontend memanggil `QueryService.Execute(tabID, sql, opts)` dan mendapat `CancellablePromise`.
2. Go memecah teks menjadi statement (`sqlscan`), menjalankan pemeriksaan keamanan (§3.7), lalu mengeksekusi statement satu per satu di `Session` milik tab tersebut.
3. Statement yang menghasilkan baris dibaca sampai `maxRows` (default 10.000, bisa diubah) ke buffer `resultset`. Yang dikembalikan hanya ringkasan: `{resultID, columns, rowCount, truncated, durationMs}`, bukan barisnya.
4. Grid meminta jendela data lewat `ResultService.FetchRows(resultID, offset, limit)` (200–500 baris) saat digulir. TanStack Virtual hanya merender baris yang terlihat.
5. Buffer dibebaskan saat tab ditutup atau query dijalankan ulang. Total memori buffer dibatasi oleh setting, dan hasil di tab yang tidak aktif dikeluarkan lebih dulu (LRU).

**Grid tabel (dari penjelajah)**

- Paginasi di sisi server: `DataService.FetchPage(tableRef, filters, sort, page, pageSize)` dikompilasi oleh `Dialect.BuildSelect` menjadi `SELECT … LIMIT/OFFSET` dengan parameter. Ukuran halaman default 1.000 baris.
- Jumlah baris memakai estimasi statistik (`pg_class.reltuples`, `information_schema.TABLES.TABLE_ROWS`). Hitungan pasti (`COUNT(*)`) hanya dijalankan atas permintaan.

### 3.5 Representasi Nilai Sel

- Semua nilai sel melintasi bridge sebagai `string | null` (di Go: `*string`), **tidak pernah** sebagai number JavaScript. Dengan begitu int64, BIGINT UNSIGNED, dan NUMERIC/DECIMAL tidak kehilangan presisi.
- Setiap kolom membawa metadata: nama, tipe asli di database, `kind` (`text`, `integer`, `decimal`, `bool`, `datetime`, `date`, `time`, `json`, `uuid`, `binary`, `array`, `other`), nullable, dan apakah termasuk PK. Cara render dan editor sel dipilih dari `kind`.
- Nilai biner dikirim sebagai hex `0x…`. Nilai yang sangat panjang dipotong di grid (mis. 4 KB pertama + flag `truncated`); nilai lengkapnya diambil lewat panggilan terpisah.
- Waktu diformat ISO-8601 di Go dengan mempertahankan semantik tipe (`timestamptz` vs `timestamp`).
- Nilai hasil edit dari UI dikirim sebagai string lalu dikonversi oleh database lewat parameter (dengan cast eksplisit per dialek bila perlu).

### 3.6 Penyimpanan Lokal dan Rahasia

| Tabel | Isi |
| --- | --- |
| `connection_groups` | `id, name, sort_order` |
| `connections` | `id (uuid), group_id, name, engine, env, color, host, port, username, database, ssl_json, ssh_json, options_json, read_only, sort_order, created_at, updated_at`; **tanpa rahasia** |
| `known_hosts` | `host, port, key_type, fingerprint_sha256, added_at` |
| `query_history` | `id, connection_id, database, sql, started_at, duration_ms, row_count, status, error`; diindeks per koneksi dan waktu; ada retensi |
| `snippets` | `id, name, sql, engine (nullable), tags, created_at, updated_at` |
| `settings` | `key, value_json`, termasuk keybinding |

- Keychain: service `app.querra`, account `conn/<uuid>/password`, `conn/<uuid>/ssh-password`, `conn/<uuid>/ssh-passphrase`. Menghapus koneksi juga menghapus entri keychain-nya.
- Untuk private key SSH, yang disimpan adalah **path file**, bukan isinya. Credential Manager Windows membatasi ukuran rahasia.
- Frontend tidak pernah menerima password yang sudah tersimpan, hanya flag `hasPassword`.
- Ekspor koneksi (FR-05) berupa JSON dengan field `version` dan tanpa field rahasia. Koneksi hasil impor ditandai "password diperlukan".

### 3.7 Keamanan Koneksi Production (Berlapis)

| Lapis | Mekanisme |
| --- | --- |
| Visual | Warna environment di sidebar, tab, dan toolbar; badge `PRODUCTION` di status bar. Production harus terlihat jelas di setiap tempat koneksi muncul (FR-03). |
| Mode hanya-baca | Flag per koneksi. Di form, flag ini otomatis menyala saat environment Production dipilih (pengguna boleh mematikannya). |
| Sesi database | PG: `SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY`; MySQL: `SET SESSION TRANSACTION READ ONLY`; SQLite: `mode=ro` / `PRAGMA query_only = ON`. Diterapkan pada setiap koneksi baru di pool dan sesi. |
| Pemeriksaan di Go | `sqlscan` menandai `DELETE`/`UPDATE` tanpa `WHERE`, `DROP`, `TRUNCATE`, dan `ALTER … DROP`. Jika tokenizer ragu, statement dianggap berisiko (*fail-safe*). |
| Konfirmasi | Di koneksi Production, `Execute` menolak dengan kode `confirmation_required` beserta daftar statement berisiko. UI menampilkan dialog (pengguna mengetik nama koneksi) lalu memanggil ulang dengan `confirmToken` berupa hash dari SQL yang ditandai. Token yang tidak cocok ditolak, sehingga bug di UI tidak bisa melewati pemeriksaan ini. |
| Grid dan impor | Commit perubahan tertunda dan impor CSV di Production selalu meminta konfirmasi dengan ringkasan (n update, n insert, n delete). |

Catatan: mode hanya-baca melindungi dari *kesalahan*, bukan kontrol akses. Pengguna masih bisa menjalankan `SET TRANSACTION READ WRITE`. Untuk jaminan sungguhan, sarankan user database yang hanya punya hak baca. Hal ini perlu disebut di dokumentasi pengguna.

### 3.8 Pembatalan, Sesi, dan Reconnect

- Setiap binding yang menyentuh database menerima `context.Context` sebagai argumen pertama. Pembatalan promise di frontend membatalkan ctx ini.
- **PostgreSQL:** pgx mengirim *cancel request* saat ctx dibatalkan, dan sesi tetap bisa dipakai.
- **MySQL/MariaDB:** `go-sql-driver/mysql` menutup koneksi saat ctx dibatalkan. Query tetap berjalan di server dan state sesi hilang. Karena itu adapter menjalankan query dengan ctx internal. Saat ctx dari pengguna batal, adapter mengirim `KILL QUERY <connection_id>` dari koneksi pool lain, sehingga sesi tetap hidup.
- **SQLite:** pembatalan lewat ctx (interrupt).
- Satu tab editor memiliki satu `Session` (koneksi yang di-pin), sehingga `BEGIN`/`COMMIT`, `SET`, dan temp table konsisten. Satu tab hanya menjalankan satu query pada satu waktu.
- **Koneksi putus:** pool tersambung ulang secara otomatis. Sesi tab yang hilang ditandai, UI memberi tahu bahwa transaksi dan state sesi ikut hilang, lalu sesi baru dibuat pada eksekusi berikutnya. Statement **tidak pernah** diulang otomatis.
- **SSH:** keepalive setiap 30 detik; tunnel yang putus di-dial ulang dengan backoff.

### 3.9 Penanganan Error

- `apperr.Error{Code, Message, Detail, Hint, Position}` memakai field yang diekspor. Wails v3 menyerialisasi field ini ke JSON, sehingga sampai di frontend sebagai `error.cause`. `MarshalError` dipasang di semua service untuk membungkus error lain menjadi `apperr` dengan kode `internal`.
- Error driver dipetakan per engine ke kode yang stabil: `dns_failed`, `connection_refused`, `timeout`, `auth_failed`, `database_not_found`, `tls_required`, `tls_verify_failed`, `ssh_auth_failed`, `ssh_host_key_unknown`, `ssh_host_key_mismatch`, `permission_denied`, `syntax_error`, `query_cancelled`, `connection_lost`, `read_only`, `confirmation_required`, `internal`.
- Tombol Test Connection (FR-04) melaporkan tahap demi tahap: SSH → TCP → TLS → autentikasi → ping. Pengguna bisa melihat tahap mana yang gagal beserta saran perbaikannya.
- Goroutine yang dibuat oleh service wajib memakai `recover`. Query yang gagal tidak boleh membuat aplikasi crash.

## 4. Roadmap dan Milestone

### 4.1 Fase 0 – Fondasi dan Validasi (±2 minggu)

- [ ] **M0.1 Scaffolding**
  - `git init`; `wails3 init -n querra -t react-ts -mod <module path>`; pin versi Wails v3 dan toolchain Go.
  - Struktur folder sesuai §3.2, Tailwind v4 + shadcn/ui, ESLint + Prettier, golangci-lint.
  - Task tambahan di Taskfile: `test`, `test:integration`, `lint`, `db:up`, `db:down`, `bindings`.
  - `dev/docker-compose.yml`: PostgreSQL 17, MySQL 8.4, MariaDB 11, dan openssh-server, plus data seed (lihat §5).
  - CI: lint, unit test Go dan frontend, serta build di `macos-latest` dan `windows-latest`.
  - Selesai jika `wails3 dev` membuka jendela di macOS dan CI hijau di kedua OS.
- [ ] **M0.2 Spike validasi stack** (hasil dicatat di `docs/spikes/fase-0.md`)
  - Grid: 100.000 baris × 20 kolom data dummy dari Go lewat `FetchRows` bertahap; ukur kelancaran scroll di WKWebView (macOS) dan WebView2 (Windows).
  - Waktu mulai (cold start), memori idle, dan ukuran installer untuk build berisi shell + CodeMirror + grid.
  - Pembatalan: `CancellablePromise` → ctx Go → `pg_sleep(30)` benar-benar berhenti.
  - `go-keyring`: tulis, baca, dan hapus di kedua OS.
  - Tampilan Tailwind v4 di macOS 12 (R3).
  - **Gerbang keputusan:** lanjut dengan TanStack Table atau ganti pustaka grid; lanjut dengan Wails v3 atau mundur ke v2.
- [ ] **M0.3 Shell UI dan fondasi desain**
  - Layout: sidebar, tab bar, area konten, panel hasil, status bar.
  - Token desain (terang/gelap), warna environment, font monospace untuk grid dan editor.
  - Registry perintah dan shortcut (`lib/commands`): setiap aksi didaftarkan dengan id dan keybinding default.
  - i18n terpasang; semua string UI melewati `t()`.
  - Catatan riset singkat tentang pola UX klien database populer di `docs/research.md` (PRD Fase 0).
- [ ] **M0.4 Administrasi dengan waktu tunggu panjang** (berjalan paralel)
  - Daftar Apple Developer Program dan siapkan code signing Windows (mis. Azure Trusted Signing atau sertifikat OV).
  - Cek merek dagang dan ketersediaan domain "Querra".
  - Putuskan model bisnis dan lisensi (§8) sebelum repositori dibuka untuk publik.

### 4.2 Fase 1 – Koneksi, Penjelajah, Grid Baca-saja untuk PostgreSQL (±4 minggu)

- [ ] **M1.1 Penyimpanan lokal dan rahasia**
  - `store`: buka atau buat database di direktori config, migrasi ter-embed, repositori untuk koneksi, grup, dan settings.
  - `secrets`: interface `Store`, implementasi keyring, dan fake in-memory untuk pengujian.
  - Unit test untuk migrasi dan repositori (memakai file SQLite sementara).
- [ ] **M1.2 Kontrak driver dan adapter PostgreSQL**
  - Interface §3.3 dan registry driver.
  - Open dengan opsi SSL (`disable`/`prefer`/`require`/`verify-ca`/`verify-full`, CA, sertifikat dan key klien), konfigurasi pool, `application_name=Querra`.
  - Introspector berbasis `pg_catalog`: database, schema, tabel, view, materialized view, fungsi, kolom, PK, index, FK, constraint.
  - Pemetaan tipe PG ke `kind` dan konversi nilai ke string (§3.5).
  - Uji integrasi terhadap PostgreSQL di Docker.
- [ ] **M1.3 Layanan koneksi** (FR-01, FR-03, FR-04, FR-05)
  - CRUD, duplikasi, grup, pengurutan, environment (Local/Staging/Production) dan warna, flag hanya-baca.
  - Test Connection bertahap dengan pesan error yang mudah dipahami (§3.9).
  - Ekspor dan impor JSON tanpa rahasia.
- [ ] **M1.4 UI koneksi**
  - Layar awal: daftar koneksi per grup, pencarian, buka dengan Enter atau klik ganda.
  - Form koneksi (tab General / SSL / SSH / Advanced; tab SSH nonaktif sampai M2.7), tombol Test, penanda Production.
- [ ] **M1.5 Manajer sesi**
  - Buka dan tutup koneksi; satu pool per pasangan (koneksi, database). Berpindah database di PG berarti membuat pool baru.
  - Mode hanya-baca diterapkan pada setiap koneksi baru di pool.
  - Status koneksi dikirim ke UI lewat typed event (`connection:status`).
- [ ] **M1.6 Penjelajah skema** (FR-06)
  - Tree yang dimuat bertahap: database → schema → Tables / Views / Functions. Ada filter nama, refresh, dan navigasi keyboard.
  - Metadata di-cache dengan TanStack Query dan diinvalidasi saat refresh.
- [ ] **M1.7 Grid data baca-saja** (FR-07, FR-08)
  - Tabel dibuka di tab dengan paginasi sisi server dan virtualisasi baris serta kolom.
  - Sort per kolom (klik header). Filter per kolom (`=`, `≠`, `<`, `>`, `LIKE`, `IN`, `IS NULL`, `IS NOT NULL`) dikompilasi lewat Dialect dengan parameter. Tersedia juga input WHERE mentah.
  - Render berdasarkan `kind`; NULL dibedakan dari string kosong. Kolom bisa diubah lebarnya, sel atau baris bisa disalin (TSV), dan ada panel detail untuk nilai panjang atau JSON.
  - Jumlah baris berupa estimasi, dengan tombol untuk hitungan pasti.
  - Selesai jika halaman pertama tabel 1 juta baris tampil < 1 detik (DB lokal) dan scroll lancar di kedua OS.
- [ ] **M1.8 Tampilan struktur tabel**
  - Tab "Structure": kolom (tipe, nullable, default, komentar), PK, index, FK (bisa dinavigasi ke tabel tujuan), dan constraint lain.

### 4.3 Fase 2 – SQL Editor, Riwayat, SSH Tunnel (±4 minggu)

- [ ] **M2.1 `sqlscan` (tokenizer)**
  - Memecah multi-statement per dialek dengan memperhatikan string, identifier ber-quote, komentar `--`, `/* */`, dan `#` (MySQL), dollar-quoting PG, dan `E''`.
  - Menentukan statement di posisi kursor dan menyediakan offset untuk menyorot lokasi error.
  - Deteksi statement destruktif (§3.7) dengan uji table-driven yang luas, termasuk kasus jebakan: `WHERE` di dalam subquery, komentar, atau string; CTE `WITH … DELETE`; body fungsi ber-dollar-quote.
- [ ] **M2.2 Eksekusi query** (FR-11, FR-13, FR-14)
  - `Session` per tab dengan eksekusi berurutan. Hasil tersedia per statement (baris, baris terdampak, durasi, atau error), dengan opsi berhenti saat error.
  - Buffer `resultset` + `FetchRows`, batas baris dengan flag `truncated`, dan pembebasan buffer.
  - Pembatalan lewat ctx (PostgreSQL).
- [ ] **M2.3 UI editor**
  - CodeMirror 6 dengan dialek sesuai engine, banyak tab editor (P1), dan isi tab dipulihkan saat aplikasi dibuka ulang.
  - Jalankan statement di kursor (`Mod+Enter`), seleksi, atau semua (`Mod+Shift+Enter`), plus tombol Stop.
  - Panel hasil: sub-tab per statement, waktu eksekusi, jumlah baris terdampak, dan error dengan sorotan posisi.
- [ ] **M2.4 Autocomplete skema** (FR-12)
  - Sumbernya cache introspeksi koneksi aktif (schema, tabel, kolom), dipasang ke opsi `schema` di `lang-sql`. Alias tabel dan keyword dialek juga dikenali.
  - Cache diperbarui otomatis setelah DDL berhasil dijalankan.
- [ ] **M2.5 Pengaman Production** (FR-15)
  - Alur `confirmation_required` + `confirmToken` (§3.7) dengan dialog ketik nama koneksi.
  - Statement tulis di koneksi hanya-baca diblokir dengan pesan yang jelas.
- [ ] **M2.6 Riwayat query** (P1)
  - Setiap eksekusi dicatat. Panel riwayat punya pencarian dan filter per koneksi, bisa membuka ulang query ke editor, dan mendukung retensi serta penghapusan riwayat.
- [ ] **M2.7 SSH tunnel** (FR-02)
  - Autentikasi dengan password atau private key (OpenSSH/PEM, dengan passphrase); ssh-agent bersifat opsional.
  - `known_hosts`: membaca `~/.ssh/known_hosts` dan store lokal. Host baru menampilkan fingerprint dan meminta persetujuan (TOFU); kunci yang berubah ditolak dengan peringatan.
  - Dialer kustom ke pgx, keepalive, dan redial.
  - Test Connection melaporkan tahap SSH secara terpisah. Uji integrasi memakai container openssh-server.
- [ ] **M2.8 Keandalan koneksi**
  - Deteksi koneksi putus, reconnect pool otomatis, dan pemberitahuan saat sesi tab hilang (§3.8). Diuji dengan menghentikan container di tengah query.

### 4.4 Fase 3 – MySQL/MariaDB, Edit Inline, Ekspor, Mode Gelap (±4 minggu)

- [ ] **M3.1 Adapter MySQL/MariaDB**
  - Open dengan TLS kustom (`mysql.RegisterTLSConfig`) dan dial SSH (`mysql.RegisterDialContext`), memakai nama unik per koneksi karena registry-nya global. Menggunakan `parseTime` dan charset `utf8mb4`.
  - Introspector via `information_schema`; "database" di MySQL ditampilkan sebagai schema di tree.
  - Pembatalan dengan `KILL QUERY` (§3.8).
  - Seluruh suite integrasi dijalankan terhadap MySQL 8.4 dan MariaDB 11.
- [ ] **M3.2 Changeset di backend** (FR-09, FR-10)
  - Model perubahan: update sel, insert baris, dan delete baris, dikunci dengan nilai PK asli.
  - `PreviewChanges` mengembalikan SQL yang akan dijalankan. `CommitChanges` menjalankan semuanya dalam satu transaksi; setiap statement harus mengenai tepat 1 baris, kalau tidak seluruh transaksi di-rollback dengan pesan yang jelas.
  - Tabel tanpa PK bersifat hanya-baca. Opsi "tetap edit" tersedia dengan konfirmasi dan memakai pencocokan semua kolom dengan batas 1 baris.
- [ ] **M3.3 UI edit inline**
  - Editor sel sesuai `kind` (teks, angka, boolean, tanggal/waktu, JSON), plus aksi set NULL dan set DEFAULT.
  - Tambah, duplikasi, dan hapus baris; sel yang berubah disorot. Panel tinjau perubahan dilengkapi pratinjau SQL, dengan aksi Commit (`Mod+S`) dan Discard.
  - Konfirmasi ringkasan di Production; edit dinonaktifkan sepenuhnya di koneksi hanya-baca.
  - Peringatan saat menutup tab atau berpindah halaman selagi ada perubahan tertunda.
- [ ] **M3.4 Ekspor** (FR-16)
  - CSV (delimiter, header, quoting, BOM UTF-8 opsional untuk Excel). JSON berupa array objek, dengan angka ditulis dari teks aslinya tanpa konversi float. SQL INSERT sesuai dialek dengan ukuran batch yang bisa diatur.
  - Sumber ekspor: hasil query atau seluruh tabel (di-stream langsung dari database). Jika hasil query terpotong, eksekusi ulang secara streaming hanya ditawarkan untuk statement baca (`SELECT`/`WITH` tanpa DML).
  - Memakai dialog simpan file native, progres dikirim lewat event, dan ekspor bisa dibatalkan.
- [ ] **M3.5 Mode gelap** (P1)
  - Pilihan terang / gelap / ikuti sistem, termasuk tema CodeMirror dan grid. Pilihan disimpan di settings.
- [ ] **M3.6 Keyboard dan aksesibilitas tahap 1**
  - Navigasi grid (panah, Tab, Enter untuk edit, Esc untuk batal), tree, dan tab; fokus selalu terlihat; label ARIA pada komponen utama.

### 4.5 Fase 4 – Impor CSV, SQLite, Polishing, Packaging (±3 minggu)

- [ ] **M4.1 Impor CSV** (FR-17, P2)
  - Pilih file; deteksi delimiter, encoding, dan header; pratinjau 100 baris; pemetaan kolom (otomatis berdasarkan nama); validasi tipe. Target impor adalah tabel yang sudah ada.
  - Insert per batch dalam transaksi, laporan baris yang gagal, progres, dan bisa dibatalkan. Di Production wajib konfirmasi.
- [ ] **M4.2 Adapter SQLite** (P2)
  - Koneksi berbasis file (buka atau buat), mode hanya-baca dengan `mode=ro`, introspeksi lewat `sqlite_schema` dan pragma. Pengujian memakai file sementara, tanpa Docker.
- [ ] **M4.3 Snippet** (P2)
  - Simpan dan kelola snippet, sisipkan ke editor, dan cari snippet.
- [ ] **M4.4 Pencarian global** (P2)
  - Quick open (`Mod+P`) untuk tabel, view, dan fungsi dari semua koneksi yang terbuka, memakai cache metadata.
- [ ] **M4.5 Pengaturan dan shortcut**
  - Halaman settings: batas baris, ukuran halaman, font, tema, dan retensi riwayat. Editor keybinding untuk mengubah, me-reset, dan mendeteksi bentrok shortcut.
  - Audit i18n (tidak ada string hardcoded) dan audit navigasi keyboard penuh.
- [ ] **M4.6 Packaging dan signing**
  - macOS: universal binary (arm64 + amd64), codesign dengan hardened runtime, notarize + staple, lalu `.dmg`.
  - Windows: installer NSIS yang ditandatangani, dengan bootstrapper WebView2 untuk Windows 10.
  - Workflow rilis GitHub Actions saat tag `v*` dibuat; artefak diunggah ke GitHub Releases; anggaran ukuran (< 60 MB) dicek otomatis.
- [ ] **M4.7 Performa dan keandalan**
  - Ukur ulang anggaran di §6 pada perangkat acuan; profil memori dengan pprof; analisis bundle (editor dan grid di-code-split).
  - Log ke file dan menu "Open Logs Folder"; crash report sesuai keputusan di §8.
- [ ] **M4.8 (Opsional, bila waktu cukup) Visualisasi relasi sederhana** (P2)
  - Diagram FK satu tabel (relasi masuk dan keluar) di tab Structure.

### 4.6 Beta (±3 minggu)

- [ ] **B.1** Distribusi beta tertutup (build yang sudah ditandatangani) dan saluran umpan balik.
- [ ] **B.2** Triage dan perbaikan bug; uji regresi manual di macOS 12, macOS terbaru (Intel dan Apple Silicon), serta Windows 10 dan 11.
- [ ] **B.3** Dokumentasi: README, panduan pengguna singkat (termasuk catatan mode hanya-baca di §3.7), dan catatan rilis.
- [ ] **B.4** Rilis publik v1.0.

### 4.7 Definition of Done (Setiap Milestone)

1. `wails3 task lint` dan `wails3 task test` hijau. Kode yang menyentuh database juga lulus `wails3 task test:integration`.
2. Diuji manual di macOS. Build Windows dari CI dijalankan dan dicoba minimal sekali per fase.
3. Tidak ada string UI yang hardcoded dan tidak ada rahasia di log maupun SQLite lokal.
4. Checkbox di PLAN.md dicentang. CLAUDE.md diperbarui bila ada konvensi baru.

## 5. Strategi Pengujian

| Lapis | Alat | Cakupan |
| --- | --- | --- |
| Unit Go | `go test`, table-driven | `sqlscan`, builder Dialect, `changeset`, writer ekspor, pemetaan error, encoding sel, `store` (SQLite sementara) |
| Integrasi Go | build tag `integration`, Docker Compose (`wails3 task db:up`) | Adapter per engine: introspeksi, tipe data, pembatalan, hanya-baca, commit changeset, SSH tunnel, reconnect |
| Frontend | Vitest + Testing Library | Store changeset, registry shortcut, format sel, komponen grid dan editor yang penting; binding di-mock lewat wrapper `features/*/api.ts` |
| Manual lintas OS | `docs/qa-checklist.md` | Alur utama PRD §9 di macOS dan Windows di setiap akhir fase |

Data seed di `dev/`:

- `big_table` dengan 1 juta baris (uji paginasi dan performa).
- `all_types` berisi semua tipe utama per engine (uji `kind` dan presisi).
- `no_pk` (uji FR-10) dan rantai FK (uji struktur dan navigasi).
- Identifier tidak lazim: spasi, huruf besar, unicode, dan kata kunci SQL (uji quoting).

## 6. Anggaran Performa

| Metrik | Target | Cara ukur | Kapan diukur |
| --- | --- | --- | --- |
| Waktu mulai | < 2 detik | Dari proses dimulai sampai shell UI interaktif (penanda waktu di frontend) | M0.2, M4.7 |
| Memori idle | < 250 MB | Activity Monitor / Task Manager, **termasuk proses webview** | M0.2, M4.7 |
| Scroll 100.000 baris | Tanpa patah-patah yang terlihat (~60 fps) | Panel Performance di DevTools webview | M0.2, M1.7, M4.7 |
| Ukuran installer | < 60 MB | Ukuran `.dmg` dan installer Windows | M0.2, M4.6 |
| Halaman pertama tabel 1 juta baris | < 1 detik (DB lokal) | Log durasi di Go | M1.7 |

## 7. Risiko Tambahan (di Luar PRD §13)

| ID | Risiko | Mitigasi |
| --- | --- | --- |
| R1 | Wails v3 masih beta, sehingga ada kemungkinan breaking change atau bug | Pin versi; API Wails hanya di `main.go` dan `internal/app/`; spike M0.2 sebagai gerbang keputusan; baca changelog sebelum upgrade |
| R2 | Pembatalan query di MySQL memutus sesi | `KILL QUERY` dari koneksi terpisah (§3.8), dengan uji integrasi |
| R3 | Tailwind v4 dirancang untuk Safari 16.4+, sementara macOS 12 yang Safari-nya belum diperbarui memakai WebKit lama | Uji di macOS 12 pada M0.2. Jika bermasalah, tetapkan syarat Safari ≥ 16.4 atau turun ke Tailwind v3.4 |
| R4 | Toolchain Go baru bisa menaikkan versi minimum macOS | Baca release notes sebelum upgrade Go; pin toolchain di `go.mod` dan CI |
| R5 | Angka besar kehilangan presisi di JavaScript | Semua sel dikirim sebagai string (§3.5) |
| R6 | Deteksi query destruktif kecolongan | Tokenizer *fail-safe*, uji kasus jebakan, dan mode hanya-baca sebagai lapis kedua |
| R7 | Sebagian Windows 10 belum punya WebView2 | Bootstrapper WebView2 di installer |
| R8 | Batas ukuran rahasia di Credential Manager | Simpan path private key, bukan isinya |
| R9 | Waktu verifikasi di Windows terbatas karena pengembangan utama di macOS | VM atau mesin uji Windows, build CI di setiap push, checklist QA per fase |

## 8. Keputusan Terbuka

| Pertanyaan | Rekomendasi awal | Tenggat |
| --- | --- | --- |
| Module path Go | Mis. `github.com/<owner>/querra` | M0.1 |
| Model bisnis dan lisensi | Memengaruhi file LICENSE, pembatasan fitur, dan kebutuhan sistem lisensi; harus diputuskan sebelum repositori publik | Akhir Fase 0 |
| Merek dagang dan domain "Querra" | Cek sebelum aset branding dibuat | M0.4 |
| SQL Server di MVP? | Tidak. Arsitektur adapter memungkinkan penambahan setelah MVP | Akhir Fase 1 |
| Target pengguna awal | Menentukan saluran beta dan prioritas polishing | Sebelum Beta |
| Auto-update di v1? | Disarankan ya, karena klien database butuh jalur cepat untuk patch keamanan. Evaluasi opsi updater (bawaan Wails v3 bila tersedia, atau solusi eksternal) | Awal Fase 4 |
| Crash reporting vs larangan telemetri | Opt-in, default mati, ditambah log lokal | Awal Fase 4 |

## 9. Setelah MVP (Backlog)

- Fitur P3: ubah struktur tabel lewat GUI, pembanding skema, sistem plugin.
- SQL Server, versi Linux, ERD, asisten AI untuk menulis query.
- Satu jendela per koneksi (multi-window Wails v3), keyset pagination untuk halaman yang dalam, visualisasi EXPLAIN, SSH jump host.
