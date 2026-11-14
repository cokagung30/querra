# PLAN: Querra – Rencana Implementasi

*Turunan dari PRD v0.2 (7 November 2026) · Status: belum di-scaffold · Pelaksana: 1 developer + Claude Code · Pemakai: pribadi dan rekan dekat (alat internal balikaryadigital.com)*

Dokumen ini menerjemahkan PRD (`docs/PRD Querra – Database Management Client.md`) menjadi keputusan teknis, arsitektur, dan milestone yang bisa dikerjakan berurutan. Dokumen ini hidup: centang tugas saat selesai, dan perbarui bagian arsitektur bila implementasi menyimpang.

## 0. Cara Memakai Dokumen Ini

- Milestone diberi kode `M<fase>.<nomor>` (mis. `M1.7`) dan dikerjakan berurutan. Setiap milestone adalah potongan vertikal yang bisa didemokan, berukuran kira-kira 1–4 sesi kerja dengan Claude Code.
- Jangan mulai fitur milestone berikutnya sebelum milestone aktif memenuhi *Definition of Done* (§4.7).
- Kode milestone dipakai di pesan commit, mis. `feat(grid): M1.7 server-side pagination`.
- Estimasi fase mengikuti PRD (total ±20 minggu). Pada mode solo, hambatan terbesar biasanya bukan menulis kode, tetapi verifikasi manual lintas OS, packaging, dan pengujian dengan database sungguhan.

## 1. Keputusan Teknis

| Area | Pilihan | Catatan |
| --- | --- | --- |
| Desktop shell | **Wails v3** (v3.1.0, stabil) | Versi di-pin persis di `go.mod`. API Wails hanya dipakai di `main.go` dan `internal/app/`. |
| Backend | Go 1.26 | Versi toolchain di-pin di `go.mod` dan CI. |
| Driver DB | `pgx/v5` (lewat `database/sql` + `pgx/v5/stdlib`), `go-sql-driver/mysql`, `modernc.org/sqlite` | Semua lewat `database/sql` agar adapter seragam; perilaku khusus engine ditangani di adapter. |
| SSH tunnel | `golang.org/x/crypto/ssh` | Dialer kustom yang langsung dipasang ke driver, tanpa membuka port lokal. |
| Rahasia | `zalando/go-keyring` | Keychain (macOS), Credential Manager (Windows). |
| Penyimpanan lokal | SQLite (modernc, tanpa cgo) di `os.UserConfigDir()/Querra/querra.db` | Migrasi SQL di-embed, versi skema disimpan di `PRAGMA user_version`. |
| Logging | `log/slog` ke file berotasi di direktori log aplikasi | Log tidak boleh berisi rahasia atau isi data. |
| Frontend | React 19 + TypeScript + Vite (template `react` dari Wails) | |
| UI | Tailwind CSS v4 + shadcn/ui (Radix), `lucide-react` | Lihat risiko R3 untuk macOS 12. |
| State | Zustand (UI, tab, perubahan tertunda) + TanStack Query (cache data dari Go) | |
| Grid | TanStack Table + TanStack Virtual | Divalidasi lewat spike M0.2. |
| SQL editor | **CodeMirror 6** + `@codemirror/lang-sql` | Mendukung dialek PostgreSQL/MySQL/SQLite dan autocomplete berbasis skema. |
| Layout | `react-resizable-panels` | |
| i18n | `i18next` + `react-i18next` | Bahasa Inggris dulu, struktur siap untuk `id`. |
| Pengujian | `go test`, uji integrasi (build tag `integration`), Vitest + Testing Library | Tanpa Docker di mesin pengembangan: lokal memakai server database native, CI memakai service container di runner Ubuntu (§5). |
| Lint | `golangci-lint`, ESLint + Prettier, `tsc --noEmit` | |
| Package manager | npm | Sudah terpasang dan menjadi default template. |
| CI/CD | GitHub Actions (`macos-latest`, `windows-latest`) | Build, uji, dan rilis (artefak + manifest update). |
| Identitas | Module `github.com/cokagung30/querra`, bundle ID `com.balikaryadigital.querra` | |
| Distribusi | `.dmg` (macOS) dan installer NSIS (Windows), tanpa sertifikat code signing berbayar | Alat internal; lihat §8. |
| Auto-update | Updater bawaan Wails v3 | Setiap rilis ditandatangani dengan kunci ed25519 sendiri (`wails3 updater`). |

## 2. Penyesuaian terhadap PRD v0.1

Semua penyesuaian di bawah sudah dimasukkan ke PRD v0.2.

1. **Monaco diganti CodeMirror 6.** Monaco menambah beberapa MB ke bundle dan memakan memori, padahal targetnya memori idle < 250 MB dan waktu mulai < 2 detik. `@codemirror/lang-sql` sudah menyediakan dialek dan autocomplete skema yang dibutuhkan FR-12.
2. **Wails v3 dipilih, bukan v2.** v3 memberi `CancellablePromise` yang terhubung ke `context.Context` di Go (langsung menjawab FR-13), binding TypeScript dan event bertipe, serta dukungan multi-window. Saat scaffolding (7 November 2026), v3 ternyata sudah stabil (v3.0.0 rilis 26 Oktober 2026), dan versinya di-pin ke v3.1.0. Karena rilis stabilnya masih baru, risikonya dikelola lewat pin versi dan lapisan Wails yang tipis (R1).
3. **Keychain dimajukan dari Fase 2 ke Fase 1.** Koneksi yang disimpan di Fase 1 sudah butuh password. Tidak ada fase yang boleh menyimpan password sebagai teks biasa, bahkan untuk sementara.
4. **Fondasi tema dan registry shortcut dibangun di Fase 0.** Token warna, warna environment, dan registry perintah dibuat sejak awal. Toggle mode gelap tetap di Fase 3 dan UI pengaturan shortcut di Fase 4. Dengan begitu NFR "shortcut dapat diubah" tidak berujung pada refactor besar.
5. **Mode hanya-baca ditegakkan di dua lapis** (sesi database dan pemeriksaan di Go), bukan hanya di UI.
6. **Tanpa layanan crash reporting.** Dengan pemakai yang sedikit, crash cukup dicatat ke log lokal dan dikirim manual bila perlu. Ini sejalan dengan larangan telemetri.
7. **Distribusi internal tanpa code signing berbayar.** Querra tidak diperdagangkan dan hanya dipakai sendiri serta oleh rekan dekat, jadi v1 tidak memakai Apple Developer Program maupun sertifikat Windows. Akibatnya macOS (Gatekeeper) dan Windows (SmartScreen) menampilkan peringatan saat aplikasi pertama kali dibuka. Panduannya ditulis di B.3.
8. **Auto-update sejak v1** memakai updater bawaan Wails v3, dengan rilis yang ditandatangani kunci ed25519 sendiri. Keamanan pembaruan tidak bergantung pada sertifikat berbayar.

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
│   ├── settings/
│   └── perf/                # dataset + reporter untuk pengukuran dev/perf (§6)
├── frontend/
│   ├── bindings/            # HASIL GENERATE — jangan diedit
│   └── src/
│       ├── app/             # shell: layout, manajemen tab, provider
│       ├── features/        # connections, explorer, grid, editor, results, history,
│       │                    # changes, export, import, settings, perf (benchmark §6)
│       ├── components/ui/   # komponen shadcn/ui
│       ├── lib/             # registry perintah/shortcut, format sel, util
│       ├── stores/          # Zustand store lintas fitur
│       └── i18n/            # en.json (id.json menyusul)
├── build/                   # konfigurasi build Wails: ikon, Info.plist, NSIS, signing
├── dev/                     # seed SQL + testdb.sh untuk menyiapkan database uji
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
    Capabilities() Capabilities // Schemas, Databases, MaterializedViews, Returning
    Dialect() Dialect
    Codec() Codec // kind per kolom hasil + nilai → teks (§3.5)
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
    BuildSelect(q TableQuery) (string, []any)     // filter, sort, limit/offset (M1.7)
    BuildChange(c RowChange) (string, []any)      // insert/update/delete satu baris (M3.2)
    ReadOnlySessionSQL() string
}

type Codec interface {
    Column(ct *sql.ColumnType) ResultColumn // nama, tipe asli, kind
    Format(col ResultColumn, v any) string  // nilai non-NULL → teks sel
}
```

Kode sebenarnya ada di `internal/driver` (M1.2). `driver.Columns` dan `driver.ScanRow` membaca `*sql.Rows` lewat `Codec` milik engine.

Engine baru cukup mengimplementasikan interface ini. UI tidak boleh bercabang berdasarkan engine, kecuali lewat `Capabilities`.

### 3.4 Alur Data Hasil Query

#### SQL editor

1. Frontend memanggil `QueryService.Execute(tabID, sql, opts)` dan mendapat `CancellablePromise`.
2. Go memecah teks menjadi statement (`sqlscan`), menjalankan pemeriksaan keamanan (§3.7), lalu mengeksekusi statement satu per satu di `Session` milik tab tersebut.
3. Statement yang menghasilkan baris dibaca sampai `maxRows` (default 10.000, bisa diubah) ke buffer `resultset`. Yang dikembalikan hanya ringkasan: `{resultID, columns, rowCount, truncated, durationMs}`, bukan barisnya.
4. Grid meminta jendela data lewat `ResultService.FetchRows(resultID, offset, limit)` (200–500 baris) saat digulir. TanStack Virtual hanya merender baris yang terlihat.
5. Buffer dibebaskan saat tab ditutup atau query dijalankan ulang. Total memori buffer dibatasi oleh setting, dan hasil di tab yang tidak aktif dikeluarkan lebih dulu (LRU).
6. Buffer disimpan ringkas, mis. satu `[]byte` per baris plus offset sel, dan `*string` baru dibuat saat `FetchRows`. Spike M0.2 menunjukkan penyimpanan satu `*string` per sel memakan ±70–85 byte per sel: 100k × 20 sel menjadi ±142 MB.

#### Grid tabel (dari penjelajah)

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
| `connections` | `id (uuid), group_id, name, engine, env, color, host, port, username, database_name, ssl_json, ssh_json, options_json, read_only, sort_order, created_at, updated_at`; **tanpa rahasia** |
| `known_hosts` | `host, port, key_type, fingerprint_sha256, added_at` |
| `query_history` | `id, connection_id, database, sql, started_at, duration_ms, row_count, status, error`; diindeks per koneksi dan waktu; ada retensi |
| `snippets` | `id, name, sql, engine (nullable), tags, created_at, updated_at` |
| `settings` | `key, value_json`, termasuk keybinding |

- Keychain: service `com.balikaryadigital.querra`, account `conn/<uuid>/password`, `conn/<uuid>/ssh-password`, `conn/<uuid>/ssh-passphrase`. Menghapus koneksi juga menghapus entri keychain-nya.
- Di macOS, `go-keyring` menjalankan `/usr/bin/security`, bukan API Keychain native. Akibatnya tidak ada prompt izin setelah update build ad-hoc, tapi proses lain milik user yang sama bisa membaca rahasia tanpa prompt, kira-kira setara Credential Manager di Windows. Detail dan keputusannya ada di `docs/spikes/fase-0.md` §4.5 dan §7.
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
- **PostgreSQL:** handler default pgx (`DeadlineContextWatcherHandler`) menghentikan query tapi **memutus koneksi**. Adapter wajib memasang `BuildContextWatcherHandler` yang mengembalikan `&pgconn.CancelRequestContextWatcherHandler{Conn: conn, DeadlineDelay: 5 * time.Second}`. Dengan begitu yang dikirim adalah cancel request protokol (error SQLSTATE 57014) dan sesi tetap hidup. Kedua perilaku ini terbukti di spike M0.2 (`docs/spikes/fase-0.md` §4.4).
- **MySQL/MariaDB:** `go-sql-driver/mysql` menutup koneksi saat ctx dibatalkan. Query tetap berjalan di server dan state sesi hilang. Karena itu adapter menjalankan query dengan ctx internal. Saat ctx dari pengguna batal, adapter mengirim `KILL QUERY <connection_id>` dari koneksi pool lain, sehingga sesi tetap hidup. Strategi ini sudah dibuktikan di spike M0.2. `SLEEP()` yang dihentikan mengembalikan 1, sedangkan query lain mengembalikan error 1317; keduanya diperlakukan sebagai `query_cancelled`.
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

- [x] **M0.1 Scaffolding** — *selesai 7 November 2026; CI hijau di macOS, Windows, dan Ubuntu (integrasi)*
  - [x] Repo `github.com/cokagung30/querra`; `wails3 init -n querra -t react -mod github.com/cokagung30/querra`; Wails di-pin ke v3.1.0 dan toolchain Go ke go1.26.2. Folder `build/ios`, `build/android`, dan `build/linux` dihapus karena targetnya hanya macOS dan Windows.
  - [x] Kerangka folder awal (`internal/app`, `frontend/src/{app,components/ui,lib}`); folder lain di §3.2 dibuat oleh milestone yang membutuhkannya. React 19, Tailwind v4 + shadcn/ui (Radix, preset Nova), ESLint + Prettier, Vitest, golangci-lint v2.
  - [x] Task tambahan di Taskfile: `test`, `test:integration`, `lint`, `db:reset`, `db:drop`, `bindings`.
  - [x] Database uji tanpa Docker (§5): `dev/testdb.sh` membuat dan mengisi `querra_test*` di server native yang dikonfigurasi lewat `dev/test.env`. Reset, seed, dan drop sudah dicoba di PostgreSQL 18.2 dan MySQL 8.0.45 lokal; MariaDB hanya diuji di CI.
  - [x] CI: unit test Go dan frontend serta build di `macos-latest` dan `windows-latest` (lint dan cek file hasil generate hanya di macOS), plus job integrasi di `ubuntu-latest` dengan PostgreSQL 17, MySQL 8.4, dan MariaDB 11.4 (`.github/workflows/ci.yml`). *Hijau di ketiga push 7 November 2026, terakhir `50f8dba`.*
  - Selesai jika `wails3 dev` membuka jendela di macOS dan CI hijau di kedua OS.
- [ ] **M0.2 Spike validasi stack** (hasil di `docs/spikes/fase-0.md`; sisa uji manual di `docs/test/M0.2.md`) — *pengukuran macOS dan Windows selesai; tinggal uji manual*
  - [x] Grid 100.000 baris × 20 kolom dari Go lewat `FetchRows` bertahap di WKWebView: ±75 fps (batas monitor) pada scroll normal.
  - [x] Grid yang sama di WebView2 (PC Windows sungguhan, `dev/perf/measure_windows.ps1`): ±143 fps (batas monitor 144 Hz) pada scroll normal. `FetchRows` ±3× lebih lambat daripada di macOS, jadi prefetch M1.7 dikalibrasi di Windows.
  - [x] Waktu mulai (median 330 ms), memori idle (76–138 MB), dan ukuran DMG universal (14 MB) di macOS. Windows: median 599 ms, idle 159 MB, installer NSIS 7,7 MB. Peluncuran pertama setelah build baru di Windows 2,4–2,8 dtk, hampir seluruhnya sebelum Go mulai (kemungkinan pemindaian Defender; laporan §4.6).
  - [x] Pembatalan: `CancellablePromise` → ctx Go → query berhenti, untuk PostgreSQL dan MySQL. Hasilnya mengoreksi §3.8.
  - [x] `go-keyring` di macOS: tidak ada prompt setelah build ulang ad-hoc, tapi proses lain bisa ikut membaca (keputusan di laporan §7). Windows memakai Credential Manager, yang teruji tidak langsung lewat build CI.
  - [ ] Build tanpa tanda tangan berbayar dibuka di Mac lain (Open Anyway) dan di Windows (SmartScreen → Run anyway, termasuk dengan Smart App Control aktif). *Windows 8 November 2026: installer bertanda "diunduh dari internet" langsung terbuka setelah UAC, tanpa SmartScreen dan tanpa blokir Smart App Control. **14 November 2026: build CI dan Querra hasil instalasi 8 November diblokir Smart App Control** (laporan §4.7). Windows lalu didistribusikan lewat Microsoft Store (§8.1, M0.5). Mac lain belum.*
  - [ ] Tampilan Tailwind v4 di macOS 12 (R3). *Manual, butuh perangkat macOS 12.*
  - [x] **Gerbang keputusan (final, macOS dan Windows):** lanjut dengan TanStack Table v9 + TanStack Virtual dan dengan Wails v3.
- [ ] **M0.3 Shell UI dan fondasi desain** — *selesai dan diuji manual di macOS (semua langkah ✅; startup median 337 ms, idle 83 MB) dan Windows (startup median 706 ms, idle 130 MB; laporan `docs/spikes/fase-0.md` §8); tinggal tujuh langkah kecil di Windows dan B12b di macOS (checklist di `docs/test/M0.3.md`)*
  - [x] Layout: sidebar, tab bar, area konten, panel hasil, status bar (`frontend/src/app/AppShell.tsx`, `react-resizable-panels` v4). Sidebar dan panel hasil bisa disembunyikan lewat perintah atau dengan menggeser pembatasnya. Ukuran panel diingat di localStorage webview (`lib/storage.ts`); pengaturan penting tetap di store Go (§3.6). Tab query masih placeholder sampai editor M2.3.
  - [x] Token desain terang/gelap (tema mengikuti sistem lewat `app/theme.ts`; pilihan manual di M3.5), warna environment `env-local`, `env-staging`, `env-production` di `index.css`, serta font monospace Geist Mono (`font-mono`) untuk grid dan editor.
  - [x] Registry perintah dan shortcut (`lib/commands`): setiap aksi didaftarkan dengan id, judul i18n, dan keybinding default (`Mod` = Cmd di macOS, Ctrl di Windows). Hanya ada satu listener keydown (`installKeybindings`). Override per id dan deteksi bentrok sudah ada; penyimpanan override menyusul di M4.5. macOS memakai menu native sendiri (`internal/app/menu.go`: App, Edit, Window), karena menu bawaan Wails mengambil Cmd+W (tutup jendela) dan Cmd+R (reload webview, yang membuang tab dan perubahan tertunda).
  - [x] i18n terpasang (`i18next` + `react-i18next`, `src/i18n/en.json`); semua string UI melewati `t()`, dan key-nya dicek TypeScript (`src/i18n/i18next.d.ts`).
  - [x] Catatan riset singkat tentang pola UX klien database populer di `docs/research.md` (PRD Fase 0).
  - [x] Kode spike M0.2 dihapus. Mode autorun startup, idle, dan grid untuk `dev/perf` pindah ke `internal/perf`, `internal/app/perfservice.go`, dan `frontend/src/features/perf` (variabel `QUERRA_PERF_*`). Probe pembatalan dan keychain hanya tersisa di riwayat git (commit `e08c486`).
- [x] **M0.4 Identitas dan repositori**
  - [x] Nama Querra dan bundle ID `com.balikaryadigital.querra` di `build/config.yml` (dikerjakan bersama M0.1).
  - [x] Tanpa lisensi open source: README menyatakan hak cipta pribadi dan bahwa Querra tidak untuk diperdagangkan (dikerjakan bersama M0.1).
  - [x] Repositori `cokagung30/querra` dibuat publik (8 November 2026), jadi rilis auto-update memakai GitHub Releases di repo ini (§8.1).
  - [x] Ikon aplikasi dari logo Querra (`build/icon-src/`): squircle navy untuk macOS (termasuk ikon Icon Composer macOS 26), logo transparan untuk Windows, serta ikon dan background DMG tanpa branding Wails.
- [ ] **M0.5 Spike distribusi Windows lewat Microsoft Store** (keputusan §8.1, R10)
  - [ ] Pemilik: akun developer Microsoft Store (Individual atau Company; jenisnya tidak bisa diubah kemudian), verifikasi identitas, dan reservasi nama aplikasi di Partner Center.
  - [x] Identitas paket dari Partner Center dicatat di `build/windows/msix/app_manifest.xml`: `BaliDigitalKarya.Querra`, `CN=D674B8A2-7FFA-4BFB-AFD0-709702A95026`, "Bali Digital Karya". Nilai ini publik, bukan rahasia. Manifest dirawat manual dan dipasang lewat `--manifest`, karena tanpa flag itu Wails membuat manifest sendiri dari `config.yml`. Ikon diambil dari `build/icon-src/logo.png` (`--icon`).
  - [ ] Paket MSIX dari Wails (`wails3 task windows:create:msix:package`, MakeAppx dari Windows SDK) di CI, ikut artifact `querra-windows` sebagai `querra-amd64.msix`. *Langkah CI sudah ada, belum pernah jalan.* Cek dampak container MSIX: lokasi `querra.db` (`%AppData%` divirtualisasi), Credential Manager, data WebView2, dan uninstall.
  - [ ] Submission pertama sebagai listing tersembunyi atau private audience. Kebutuhan sertifikasi Store (mis. kebijakan privasi) dicek di sini.
  - Selesai jika versi Store terpasang dan berjalan di PC Windows dengan Smart App Control aktif, lalu sisa uji M0.3 Windows dijalankan di versi itu.

### 4.2 Fase 1 – Koneksi, Penjelajah, Grid Baca-saja untuk PostgreSQL (±4 minggu)

- [ ] **M1.1 Penyimpanan lokal dan rahasia** — *selesai di Windows; menunggu smoke test keychain di macOS (checklist di `docs/test/M1.1.md`)*
  - [x] `store` (`internal/store`): buka atau buat database di direktori config (`DefaultPath`), migrasi ter-embed (`migrations/NNNN_*.sql`, satu transaksi per migrasi, menolak skema dari versi Querra yang lebih baru), serta repositori `Groups`, `Connections`, dan `Settings`. Pragma per koneksi: `foreign_keys`, WAL, `busy_timeout`; satu koneksi pool. Migrasi `0001` hanya berisi tabel M1.1; `known_hosts`, `query_history`, dan `snippets` ditambahkan oleh migrasi milestone yang membutuhkannya (M2.7, M2.6, M4.3). Kolom `database` menjadi `database_name`, karena `DATABASE` adalah kata kunci SQLite. Belum dipasang ke `main.go`; layanan koneksi (M1.3) yang membukanya.
  - [x] `secrets` (`internal/secrets`): interface `Store`, implementasi keyring (`Keyring`, go-keyring v0.2.8), dan fake in-memory (`Memory`) yang memvalidasi persis sama (id koneksi, jenis rahasia, nilai maksimal 1.024 byte; pesan error tidak pernah memuat nilai rahasia).
  - [x] Unit test untuk migrasi dan repositori (file SQLite sementara, termasuk path dengan spasi) dan kontrak `secrets` (fake dan provider mock go-keyring). Smoke test opsional ke keychain sungguhan: `go test -tags keychain ./internal/secrets -run TestKeychainSmoke -v`; di Windows Credential Manager lulus pada 8 November 2026.
- [ ] **M1.2 Kontrak driver dan adapter PostgreSQL** — *selesai 14 November 2026; CI hijau (unit test di macOS dan Windows, integrasi PostgreSQL 17). Di PC Windows tes Go tidak bisa jalan karena Smart App Control. Tinggal uji integrasi lokal di Mac dengan PostgreSQL 18 (checklist di `docs/test/M1.2.md`)*
  - [x] Interface §3.3 dan registry driver (`internal/driver`). `Driver` mendapat `Codec` untuk §3.5. `Dialect` baru berisi `QuoteIdent`, `Placeholder`, dan `ReadOnlySessionSQL`; `BuildSelect` menyusul di M1.7 dan `BuildChange` di M3.2.
  - [x] Open dengan opsi SSL (`disable`/`prefer`/`require`/`verify-ca`/`verify-full`, CA, sertifikat dan key klien), konfigurasi pool, `application_name=Querra`. Semuanya lewat parser libpq milik pgx, jadi perilakunya sama dengan psql. Password tidak pernah masuk connection string, dan `PGPASSWORD`/`~/.pgpass` tidak ikut terpakai. Mode eksekusi `DescribeExec`: tanpa cache prepared statement (aman lewat PgBouncer dan setelah DDL), dan argumen string diberi tipe oleh server. Pembatalan memakai cancel request (§3.8). Setiap sesi editor punya pool satu koneksi sendiri, sehingga `SET`, transaksi, dan temp table tidak bocor ke pool bersama.
  - [x] Introspector berbasis `pg_catalog`: database, schema (TOAST dan temp disembunyikan, `pg_catalog`/`information_schema` ditandai sistem), tabel (termasuk partitioned dan foreign table; partisi tidak didaftar), view, materialized view, fungsi dan procedure (dengan signature), kolom (tipe, `kind`, nullable, default, komentar), PK, index (termasuk ekspresi), FK (aksi ON UPDATE/DELETE), constraint unique/check/exclusion, dan estimasi baris (`reltuples`, -1 bila belum ada statistik). Katalog yang dipakai butuh PostgreSQL 11+.
  - [x] Pemetaan tipe PG ke `kind` dan konversi nilai ke string (§3.5). Teks keluaran PostgreSQL sendiri dipertahankan (numeric, json, interval, array, …); float mengikuti format PG 12+; bytea menjadi `0x…`; timestamptz ditampilkan di zona waktu lokal.
  - [ ] Uji integrasi terhadap PostgreSQL (lokal dan CI, §5): `internal/driver/postgres/integration_test.go` (koneksi, SSL, sesi tahan pembatalan, read-only, introspeksi seed, nilai semua kolom `all_types`). *Lulus di CI (PostgreSQL 17); Mac (PostgreSQL 18) belum.*
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
  - Prefetch halaman searah scroll, karena di spike M0.2 baris sempat kosong saat scroll sangat cepat (±30–40 % frame di macOS, ±69 % di Windows). Kalibrasi jumlah prefetch di Windows. Pola dasar grid diambil dari grid benchmark di `frontend/src/features/perf` (`PerfGrid.tsx`, `pageCache.ts`): cache halaman LRU di luar React (`useSyncExternalStore`), TanStack Table hanya untuk model kolom, dan virtualisasi baris + kolom.
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
  - Test Connection melaporkan tahap SSH secara terpisah. Uji integrasi memakai server SSH in-process di dalam test Go (`golang.org/x/crypto/ssh`, dengan dukungan port forwarding), jadi tidak butuh `sshd` dari luar.
- [ ] **M2.8 Keandalan koneksi**
  - Deteksi koneksi putus, reconnect pool otomatis, dan pemberitahuan saat sesi tab hilang (§3.8). Diuji dengan memutus koneksi dari sisi server di tengah query (`pg_terminate_backend`, `KILL`) dan dengan menutup tunnel SSH in-process.

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
- [ ] **M4.6 Packaging dan distribusi**
  - macOS: universal binary (arm64 + amd64) dengan ad-hoc signing (`codesign -s -`), lalu `.dmg`. Tanpa notarization (§8.2).
  - Windows: paket MSIX untuk Microsoft Store (pola dari spike M0.5), dikirim ke Partner Center dari workflow rilis. Installer NSIS tanpa tanda tangan, dengan bootstrapper WebView2 untuk Windows 10, tetap dibuat sebagai artifact CI untuk PC tanpa Smart App Control.
  - Workflow rilis GitHub Actions saat tag `v*` dibuat; artefak macOS diunggah ke GitHub Releases (§8.1); anggaran ukuran (< 60 MB) dicek otomatis.
  - Checklist rilis (`docs/test/v<versi>.md`): versi Store dipasang dan dibuka di PC Windows dengan Smart App Control aktif sebelum dibagikan (R10). Bila diblokir, rilis ditahan.
- [ ] **M4.7 Auto-update**
  - Jalankan `wails3 updater genkey` sekali saja. `updater.key` hanya disimpan di secret CI dan cadangan offline, tidak pernah di-commit; `updater.key.pub` di-embed ke aplikasi.
  - CI membuat manifest bertanda tangan (`wails3 updater manifest`) dan memverifikasinya (`wails3 updater verify`) sebelum diunggah.
  - Di aplikasi: cek pembaruan saat start (bisa dimatikan di settings) dan lewat menu, lalu tampilkan catatan rilis. Pengguna yang memutuskan kapan memasang. Pembaruan tidak dipasang selagi ada perubahan tertunda atau query yang berjalan.
  - Di Windows, pembaruan diurus Microsoft Store (§8.1), jadi updater Wails dan pemeriksaan pembaruannya hanya aktif di build macOS.
  - Uji alur pembaruan pada build ad-hoc-signed di macOS, dan pembaruan versi Store di Windows.
- [ ] **M4.8 Performa dan keandalan**
  - Ukur ulang anggaran di §6 pada perangkat acuan; profil memori dengan pprof; analisis bundle (editor dan grid di-code-split).
  - Log ke file dan menu "Open Logs Folder" untuk melampirkan log saat melaporkan masalah.
- [ ] **M4.9 (Opsional, bila waktu cukup) Visualisasi relasi sederhana** (P2)
  - Diagram FK satu tabel (relasi masuk dan keluar) di tab Structure.

### 4.6 Beta (±3 minggu)

- [ ] **B.1** Uji internal: Querra dipakai sehari-hari oleh pemilik dan dibagikan ke rekan dekat lewat jalur auto-update. Umpan balik dicatat di GitHub Issues.
- [ ] **B.2** Triage dan perbaikan bug; uji regresi manual di macOS 12, macOS terbaru (Intel dan Apple Silicon), serta Windows 10 dan 11 (sejauh perangkatnya tersedia).
- [ ] **B.3** Dokumentasi: README, panduan instalasi build tanpa tanda tangan (Gatekeeper dan SmartScreen), panduan pengguna singkat (termasuk catatan mode hanya-baca di §3.7), dan catatan rilis.
- [ ] **B.4** Rilis v1.0 untuk pemakaian internal.

### 4.7 Definition of Done (Setiap Milestone)

1. `wails3 task lint` dan `wails3 task test` hijau. Kode yang menyentuh database juga lulus `wails3 task test:integration`.
2. Diuji manual di macOS. Build Windows dari CI dijalankan dan dicoba minimal sekali per fase. Langkah dan hasil uji manual per platform dicatat di `docs/test/<milestone>.md`, supaya pengujian bisa dilanjutkan di mesin lain.
3. Tidak ada string UI yang hardcoded dan tidak ada rahasia di log maupun SQLite lokal.
4. Checkbox di PLAN.md dicentang. CLAUDE.md diperbarui bila ada konvensi baru.

## 5. Strategi Pengujian

| Lapis | Alat | Cakupan |
| --- | --- | --- |
| Unit Go | `go test`, table-driven | `sqlscan`, builder Dialect, `changeset`, writer ekspor, pemetaan error, encoding sel, `store` (SQLite sementara) |
| Integrasi Go | build tag `integration`; lokal: server native yang disiapkan `wails3 task db:reset`; CI: service container di `ubuntu-latest` | Adapter per engine: introspeksi, tipe data, pembatalan, hanya-baca, commit changeset, SSH tunnel (server SSH in-process), reconnect |
| Frontend | Vitest + Testing Library | Store changeset, registry shortcut, format sel, komponen grid dan editor yang penting; binding di-mock lewat wrapper `features/*/api.ts` |
| Manual lintas OS | `docs/qa-checklist.md` | Alur utama PRD §9 di macOS dan Windows di setiap akhir fase |

Database uji (tanpa Docker di mesin pengembangan):

| Lingkungan | Engine | Cara menyiapkan |
| --- | --- | --- |
| Lokal | Server native yang sudah ada (saat ini PostgreSQL 18 dan MySQL 8.0 dari PhpWebStudy) | Salin `dev/test.env.example` ke `dev/test.env`, lalu `wails3 task db:reset`. Engine yang `HOST`-nya kosong dilewati. |
| CI | PostgreSQL 17, MySQL 8.4, MariaDB 11.4 | Service container di `ubuntu-latest`, diisi dengan skrip yang sama (`dev/testdb.sh`) |

`dev/testdb.sh` hanya menyentuh objek bernama `querra_test*` (database `querra_test`, `querra_test_sales`, dan user `querra_test`), sehingga database lain di server tidak terganggu. Test integrasi terhubung sebagai `querra_test` dengan host dan port dari variabel `QUERRA_DEV_*`. Job integrasi di CI hanya menguji paket domain, karena `internal/app` mengimpor Wails yang di Linux butuh GTK/WebKit.

Data seed di `dev/seed/`:

- `big_table` dengan 1 juta baris (uji paginasi dan performa).
- `all_types` berisi semua tipe utama per engine (uji `kind` dan presisi).
- `no_pk` (uji FR-10) dan rantai FK (uji struktur dan navigasi).
- Identifier tidak lazim: spasi, huruf besar, unicode, dan kata kunci SQL (uji quoting).

## 6. Anggaran Performa

Alat ukur: `dev/perf/measure_macos.py` (macOS) dan `dev/perf/measure_windows.ps1` (Windows, juga dijalankan oleh workflow **Perf measurements**). Keduanya menjalankan mode autorun `PerfService` (`QUERRA_PERF_AUTORUN` = `startup`, `idle`, atau `grid`; lihat `internal/app/perfservice.go`). Tanpa variabel itu `PerfService` tidak melakukan apa pun, sehingga build rilis bisa diukur langsung. Grid benchmark masih memakai grid spike sampai grid sungguhan selesai di M1.7. Hasil pertama ada di `docs/spikes/fase-0.md`.

| Metrik | Target | Cara ukur | Kapan diukur |
| --- | --- | --- | --- |
| Waktu mulai | < 2 detik | Dari proses dimulai sampai shell UI interaktif (penanda waktu di frontend) | M0.2, M4.8 |
| Memori idle | < 250 MB | Activity Monitor / Task Manager, **termasuk proses webview** | M0.2, setiap milestone UI di Windows (WebView2 sudah ±150 MB tanpa UI), M4.8 |
| Scroll 100.000 baris | Tanpa patah-patah yang terlihat (~60 fps) | Panel Performance di DevTools webview | M0.2, M1.7, M4.8 |
| Ukuran installer | < 60 MB | Ukuran `.dmg` dan installer Windows | M0.2, M4.6 |
| Halaman pertama tabel 1 juta baris | < 1 detik (DB lokal) | Log durasi di Go | M1.7 |

## 7. Risiko Tambahan (di Luar PRD §13)

| ID | Risiko | Mitigasi |
| --- | --- | --- |
| R1 | Wails v3 baru stabil (v3.0.0, Oktober 2026), sehingga rilis awalnya masih mungkin membawa bug atau perubahan perilaku | Pin versi; API Wails hanya di `main.go` dan `internal/app/`; spike M0.2 sebagai gerbang keputusan; baca changelog sebelum upgrade |
| R2 | Pembatalan query di MySQL memutus sesi | `KILL QUERY` dari koneksi terpisah (§3.8); terbukti di spike M0.2 dan nanti dikunci dengan uji integrasi |
| R3 | Tailwind v4 dirancang untuk Safari 16.4+, sementara macOS 12 yang Safari-nya belum diperbarui memakai WebKit lama | Uji di macOS 12 pada M0.2. Jika bermasalah, tetapkan syarat Safari ≥ 16.4 atau turun ke Tailwind v3.4 |
| R4 | Toolchain Go baru bisa menaikkan versi minimum macOS | Baca release notes sebelum upgrade Go; pin toolchain di `go.mod` dan CI |
| R5 | Angka besar kehilangan presisi di JavaScript | Semua sel dikirim sebagai string (§3.5) |
| R6 | Deteksi query destruktif kecolongan | Tokenizer *fail-safe*, uji kasus jebakan, dan mode hanya-baca sebagai lapis kedua |
| R7 | Sebagian Windows 10 belum punya WebView2 | Bootstrapper WebView2 di installer |
| R8 | Batas ukuran rahasia di Credential Manager | Simpan path private key, bukan isinya |
| R9 | Waktu verifikasi di Windows terbatas karena pengembangan utama di macOS | PC Windows 11 sungguhan tersedia sejak 8 November 2026 (dipakai untuk pengukuran M0.2), build CI di setiap push, checklist QA per fase |
| R10 | Build tanpa tanda tangan berbayar memicu peringatan Gatekeeper/SmartScreen, dan aturan Gatekeeper bisa makin ketat di versi macOS baru. Di Windows 11, Smart App Control bisa memblokir aplikasi tanpa tanda tangan tanpa pilihan "Run anyway" | Diuji di M0.2: di PC Windows uji dengan Smart App Control aktif, installer hanya memicu UAC (laporan §4.7); peringatan browser saat mengunduh diuji di M4.6. Smart App Control menilai setiap biner sendiri-sendiri: pada 14 November 2026, di PC yang sama, `wails3.exe` dan installer lolos, sementara biner tes Go, `vet.exe`, dan `gopls.exe` diblokir. Lolos sekali tidak menjamin versi berikutnya, jadi **setiap build rilis diinstal dan dibuka dulu di PC Windows dengan Smart App Control aktif sebelum dibagikan** (M4.6). **Risiko ini terjadi pada 14 November 2026:** build CI (installer dan exe) diblokir, begitu juga Querra hasil instalasi 8 November yang sebelumnya berjalan (laporan §4.7). Karena itu Windows didistribusikan lewat Microsoft Store dengan paket MSIX yang ditandatangani Microsoft (§8.1, spike M0.5). Smart App Control di PC itu tetap dinyalakan, karena tidak bisa diaktifkan lagi tanpa instal ulang Windows. Panduan instalasi (B.3); pertimbangkan Apple Developer Program bila Gatekeeper makin ketat |
| R11 | Kunci privat updater bocor, sehingga pihak lain bisa menerbitkan pembaruan palsu | Kunci hanya di secret CI dan cadangan offline, tidak pernah di repositori |

## 8. Keputusan

### 8.1 Sudah Diputuskan (7 November 2026)

| Pertanyaan | Keputusan | Dampak pada rencana |
| --- | --- | --- |
| Module path Go | `github.com/cokagung30/querra` | M0.1 |
| Model bisnis dan lisensi | Untuk pemakaian pribadi, tidak diperdagangkan | Tanpa sistem lisensi dan pembatasan fitur; tanpa lisensi open source (M0.4) |
| Merek | Tetap "Querra", sebagai alat yang dipakai usaha balikaryadigital.com | Bundle ID `com.balikaryadigital.querra`; pemeriksaan merek dagang formal belum diperlukan |
| SQL Server di MVP? | Belum | Tetap di backlog (§9); arsitektur adapter memungkinkan penambahan nanti |
| Target pengguna awal | Pemilik dan rekan dekat | Beta menjadi uji internal (§4.6); metrik PRD §11 disesuaikan |
| Auto-update di v1? | Ya | Milestone M4.7, memakai updater bawaan Wails v3 dengan rilis bertanda tangan |
| Visibilitas repositori dan lokasi rilis | Repositori `cokagung30/querra` publik; rilis lewat GitHub Releases di repo itu | M4.6 dan M4.7; aplikasi tidak perlu token GitHub |
| Distribusi Windows (diputuskan 14 November 2026, R10) | Lewat **Microsoft Store** sebagai paket MSIX. Akun developer gratis, dan Store menandatangani paketnya, sehingga Smart App Control dan SmartScreen tidak memblokir. Pilihan lain yang ditimbang: sertifikat OV berbayar, Azure Artifact Signing (individu hanya AS/Kanada), atau tetap tanpa tanda tangan | Spike M0.5; M4.6 (paket MSIX untuk Store; installer NSIS tanpa tanda tangan tetap jadi artifact CI untuk PC tanpa Smart App Control); M4.7 (di Windows pembaruan lewat Store, updater Wails hanya untuk macOS) |

### 8.2 Mengikuti Rekomendasi (Bisa Diubah)

| Pertanyaan | Rekomendasi yang dipakai | Alasan |
| --- | --- | --- |
| Code signing dan notarization berbayar | Tidak di v1: macOS memakai ad-hoc signing; Windows ditandatangani gratis oleh Microsoft Store (§8.1) | Pemakai sedikit dan saling kenal, jadi biaya Apple Developer Program dan sertifikat berbayar tidak sebanding. Ditinjau ulang bila distribusi diperluas (R10). |
| Crash reporting | Tanpa layanan crash reporting; log lokal + menu "Open Logs Folder" | Sejalan dengan larangan telemetri dan cukup untuk pemakai yang sedikit |

### 8.3 Masih Terbuka

Tidak ada untuk saat ini.

## 9. Setelah MVP (Backlog)

- Fitur P3: ubah struktur tabel lewat GUI, pembanding skema, sistem plugin.
- SQL Server, versi Linux, ERD, asisten AI untuk menulis query.
- Distribusi lebih luas: code signing dan notarization berbayar, lisensi, situs produk.
- Satu jendela per koneksi (multi-window Wails v3), keyset pagination untuk halaman yang dalam, visualisasi EXPLAIN, SSH jump host.
