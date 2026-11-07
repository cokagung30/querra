# PRD: Querra – Database Management Client

*Versi 0.1 (Draft) · 7 November 2026 · Nama produk: Querra*

## 1. Ringkasan

Querra adalah aplikasi desktop native-feel untuk macOS dan Windows yang membantu developer dan administrator mengelola database: terhubung ke server, menjelajahi struktur, menjalankan query SQL, serta melihat dan mengedit data lewat antarmuka tabel yang cepat dan bersih. Pengalaman yang dituju mirip TablePlus: ringan, cepat dibuka, dan fokus pada pekerjaan harian.

## 2. Latar Belakang dan Masalah

Developer sering berpindah antara beberapa database (lokal, staging, produksi) dan beberapa jenis engine. Alat yang ada punya kelemahan umum:

- Aplikasi berbasis Electron berat dan lambat saat membuka tabel besar.
- Alat gratis cenderung kaku, tampilannya usang, atau tidak konsisten antar OS.
- Alat komersial yang matang berbayar dan sebagian fiturnya terkunci.
- Mengelola banyak koneksi (SSH tunnel, SSL, environment berbeda) sering merepotkan dan berisiko salah target.

Querra menjawab ini dengan aplikasi yang ringan, aman untuk kredensial, dan jelas membedakan environment.

## 3. Tujuan dan Non-Tujuan

**Tujuan**

1. Menyediakan klien database yang cepat (aplikasi terbuka dalam kurang dari 2 detik, tabel besar tetap lancar digulir).
2. Mendukung PostgreSQL dan MySQL/MariaDB pada MVP, dengan SQLite sebagai pelengkap.
3. Satu codebase yang menghasilkan aplikasi untuk macOS dan Windows.
4. Menyimpan kredensial dengan aman di keychain bawaan OS.
5. Mengurangi risiko human error di database produksi.

**Non-tujuan (di luar MVP)**

- Desain ERD visual yang kompleks dan reverse engineering skema.
- Fitur kolaborasi tim dan sinkronisasi cloud.
- Dukungan NoSQL penuh (MongoDB, Redis) dan versi Linux.
- Fitur AI untuk menulis query (dipertimbangkan setelah MVP).

## 4. Target Pengguna

| Persona | Kebutuhan utama |
| --- | --- |
| Backend developer | Cek data cepat, jalankan query, ubah data saat debugging, akses lewat SSH tunnel |
| Full-stack / mobile developer | Melihat isi tabel, menyunting beberapa baris, ekspor data uji |
| DBA / sysadmin | Melihat struktur, indeks, dan relasi; ekspor dan impor; kontrol akses ke produksi |
| Mahasiswa / peneliti | Antarmuka sederhana untuk belajar SQL dan mengolah dataset |

## 5. Ruang Lingkup MVP

### 5.1 Fitur inti

- Manajemen koneksi (buat, ubah, duplikasi, kelompokkan, beri warna dan label environment).
- Penjelajah database: daftar database, schema, tabel, view, kolom, indeks, dan foreign key.
- Grid data: lihat, filter, urutkan, paginasi, dan edit langsung (inline) dengan perubahan yang ditahan sampai di-commit.
- SQL editor: syntax highlighting, autocomplete tabel dan kolom, banyak tab, riwayat query, dan snippet tersimpan.
- Ekspor hasil query atau tabel ke CSV, JSON, dan SQL.
- Impor CSV ke tabel.
- Tampilan struktur tabel (kolom, tipe data, constraint, indeks).

### 5.2 Prioritas fitur

| Prioritas | Fitur |
| --- | --- |
| P0 | Koneksi PostgreSQL/MySQL, penjelajah skema, grid data, SQL editor, keychain, SSH tunnel |
| P1 | Edit inline dengan commit, ekspor CSV/JSON/SQL, riwayat query, tab ganda, mode gelap |
| P2 | Impor CSV, snippet, visualisasi relasi sederhana, SQLite, pencarian global |
| P3 | Ubah struktur tabel lewat GUI, pembanding skema, plugin |

## 6. Kebutuhan Fungsional

**Koneksi**

- FR-01: Pengguna dapat membuat koneksi dengan host, port, user, password, database, dan opsi SSL.
- FR-02: Pengguna dapat memakai SSH tunnel (password atau private key) untuk setiap koneksi.
- FR-03: Pengguna dapat memberi label environment (Local, Staging, Production) dan warna; koneksi Production ditandai jelas di seluruh UI.
- FR-04: Tombol "Test Connection" memberi pesan galat yang bisa dipahami.
- FR-05: Koneksi dapat diimpor dan diekspor tanpa menyertakan password.

**Penjelajah dan grid**

- FR-06: Pohon navigasi menampilkan database, schema, tabel, view, fungsi.
- FR-07: Grid menampilkan jutaan baris secara lancar lewat paginasi atau virtualisasi.
- FR-08: Pengguna dapat memfilter dan mengurutkan per kolom tanpa menulis SQL.
- FR-09: Edit sel, tambah baris, dan hapus baris ditampung sebagai perubahan tertunda, lalu ditinjau dan di-commit atau dibatalkan.
- FR-10: Tabel tanpa primary key ditandai hanya-baca atau meminta konfirmasi sebelum edit.

**SQL editor**

- FR-11: Menjalankan satu query atau beberapa query sekaligus, dan menampilkan hasil per query.
- FR-12: Autocomplete mengenali skema koneksi aktif.
- FR-13: Pengguna dapat membatalkan query yang sedang berjalan.
- FR-14: Menampilkan waktu eksekusi dan jumlah baris terdampak.
- FR-15: Query destruktif (DELETE/UPDATE tanpa WHERE, DROP, TRUNCATE) di koneksi Production memunculkan konfirmasi tambahan.

**Ekspor dan impor**

- FR-16: Ekspor hasil query dan tabel ke CSV, JSON, dan SQL INSERT.
- FR-17: Impor CSV dengan pemetaan kolom dan pratinjau.

## 7. Kebutuhan Non-Fungsional

- **Performa:** waktu mulai kurang dari 2 detik; penggunaan memori idle di bawah 250 MB; menggulir hasil 100.000 baris tetap mulus.
- **Ukuran:** installer di bawah 60 MB.
- **Keamanan:** password disimpan di Keychain (macOS) dan Credential Manager (Windows); tidak ada kredensial dalam teks biasa; tanpa telemetri tanpa persetujuan eksplisit.
- **Keandalan:** koneksi yang terputus dapat tersambung ulang otomatis; aplikasi tidak crash saat query gagal.
- **Kompatibilitas:** macOS 12 ke atas (Intel dan Apple Silicon), Windows 10 dan 11.
- **Aksesibilitas:** navigasi keyboard penuh dan shortcut yang dapat diubah.
- **Lokalisasi:** antarmuka awal dalam bahasa Inggris, dengan struktur siap untuk bahasa Indonesia.

## 8. Arsitektur dan Tech Stack (Usulan)

- **Framework desktop:** Wails (Go + webview bawaan OS).
- **Backend/core:** Go; driver `pgx`, `go-sql-driver/mysql`, `modernc.org/sqlite`; SSH tunnel dengan `golang.org/x/crypto/ssh`.
- **Frontend:** React + TypeScript, Monaco Editor untuk SQL editor, TanStack Table dengan virtualisasi untuk grid.
- **Penyimpanan lokal:** SQLite untuk daftar koneksi, riwayat, dan snippet; `go-keyring` untuk rahasia.
- **Distribusi:** `.dmg` (notarized) untuk macOS dan installer yang ditandatangani untuk Windows; CI/CD lewat GitHub Actions.

Lapisan driver dibuat sebagai antarmuka (adapter) per engine supaya engine baru dapat ditambahkan tanpa mengubah UI.

## 9. Alur Pengguna Utama

1. **Koneksi pertama:** buka aplikasi, pilih "New Connection", isi parameter, uji, simpan, lalu terhubung.
2. **Menjelajah data:** pilih tabel di sidebar, grid terbuka, terapkan filter, dan gulir data.
3. **Menjalankan query:** buka tab editor, tulis query dengan autocomplete, jalankan, lihat hasil dan waktu eksekusi, lalu ekspor bila perlu.
4. **Mengedit data:** ubah sel di grid, tinjau daftar perubahan tertunda, lalu commit.

## 10. Keamanan dan Privasi

- Kredensial hanya tersimpan di keychain OS.
- Koneksi Production memakai peringatan visual dan konfirmasi untuk operasi berisiko.
- Opsi mode hanya-baca per koneksi.
- Seluruh data dan riwayat query tersimpan lokal; tidak ada data yang dikirim ke server pihak ketiga.
- Dukungan SSL/TLS dan verifikasi sertifikat.

## 11. Metrik Keberhasilan

- Waktu dari instalasi sampai koneksi pertama berhasil di bawah 3 menit.
- Waktu mulai aplikasi di bawah 2 detik pada perangkat standar.
- Tingkat crash kurang dari 1% sesi.
- Pada tahap beta: minimal 50 pengguna aktif mingguan dengan skor kepuasan 4/5 atau lebih.

## 12. Roadmap

| Fase | Cakupan | Perkiraan |
| --- | --- | --- |
| Fase 0 | Riset kompetitor, prototipe UI, validasi stack Wails | 2 minggu |
| Fase 1 | Koneksi, penjelajah skema, grid baca-saja (PostgreSQL) | 4 minggu |
| Fase 2 | SQL editor, riwayat, keychain, SSH tunnel | 4 minggu |
| Fase 3 | MySQL, edit inline, ekspor, mode gelap | 4 minggu |
| Fase 4 | Impor CSV, SQLite, polishing, packaging dan signing | 3 minggu |
| Beta | Uji tertutup, perbaikan bug, rilis publik | 3 minggu |

Estimasi ini kasar dan perlu disesuaikan dengan jumlah developer dan waktu yang tersedia.

## 13. Risiko dan Mitigasi

| Risiko | Dampak | Mitigasi |
| --- | --- | --- |
| Perbedaan perilaku webview antar OS | Tampilan atau performa tidak konsisten | Uji rutin di kedua OS sejak Fase 0; hindari fitur webview eksperimental |
| Grid lambat pada data besar | Pengalaman buruk | Virtualisasi, paginasi sisi server, batas baris default |
| Edit data merusak data produksi | Kehilangan data | Perubahan tertunda, konfirmasi, mode hanya-baca |
| Proses code signing dan notarization | Rilis tertunda | Siapkan akun developer Apple dan sertifikat Windows lebih awal |
| Kompleksitas dukungan banyak engine | Jadwal molor | Mulai dari PostgreSQL, gunakan arsitektur adapter |
| Kemiripan dengan produk lain | Masalah merek atau persepsi | Cek merek dagang, bangun identitas dan nilai tambah sendiri |

## 14. Pertanyaan Terbuka

- Model bisnis: gratis, open source, atau berbayar (lisensi per perangkat)?
- Apakah ada kebutuhan dukungan SQL Server pada MVP?
- Apakah nama "Querra" sudah aman dari sisi merek dagang dan ketersediaan domain?
- Siapa target awal: komunitas developer umum, atau pengguna internal tertentu?
- Perlukah fitur auto-update sejak versi pertama?
