# Spike Fase 0 (M0.2): Validasi Stack

*Diukur 7 November 2026 · build `0.1.0-dev` · Wails v3.1.0 · TanStack Table 9.3 + TanStack Virtual 3.14 · CodeMirror 6*

Spike ini menjawab pertanyaan berisiko di PLAN M0.2 sebelum fitur sungguhan dibangun. Kodenya sementara (`internal/spike`, `internal/app/spikeservice.go`, `frontend/src/features/spike`) dan dihapus di M0.3. Skrip pengukurannya (`dev/perf/`) tetap dipakai untuk M4.8.

## 1. Ringkasan

| Pertanyaan M0.2 | Hasil (macOS) | Status |
| --- | --- | --- |
| Grid 100.000 × 20 lancar di WKWebView? | ±75 fps (batas refresh monitor) pada kecepatan scroll normal, p95 14–15 ms, tanpa frame panjang | ✅ |
| Waktu mulai < 2 detik? | Median 330 ms; peluncuran pertama setelah build baru 0,8–1,3 dtk | ✅ |
| Memori idle < 250 MB? | 76–138 MB (aplikasi + proses WebKit) | ✅ |
| Installer < 60 MB? | DMG universal 14 MB (`.app` 31 MB) | ✅ |
| `CancellablePromise` → ctx Go → query berhenti? | Ya. Butuh konfigurasi khusus agar sesi tetap hidup (§4.4) | ✅ dengan catatan |
| `go-keyring` di macOS, termasuk prompt setelah build ulang? | Tidak ada prompt, tapi proses lain bisa ikut membaca (§4.5) | ⚠️ perlu keputusan |
| WebView2 (Windows) | Belum. Workflow `spike.yml` sudah siap dijalankan | ⏳ |
| Build tanpa tanda tangan dibuka di Mac/PC lain | Belum. Butuh perangkat lain | ⏳ manual |
| Tailwind v4 di macOS 12 (R3) | Belum. Butuh perangkat macOS 12 | ⏳ manual |

**Gerbang keputusan:** di macOS kedua pilihan stack terbukti cukup.

- **Grid:** lanjut dengan TanStack Table v9 (hanya untuk model kolom: ukuran dan resize) + TanStack Virtual (virtualisasi baris dan kolom). Tidak perlu pustaka grid lain.
- **Desktop shell:** lanjut dengan Wails v3. Pembatalan, binding bertipe, dan event bertipe bekerja sesuai dokumentasi.

Keputusan final menunggu hasil WebView2 dari workflow Windows. Kalau grid di sana jauh di bawah target, keputusan grid dibuka lagi.

## 2. Lingkungan Uji

- Apple M4, RAM 16 GB, macOS 26.6.2, monitor eksternal 1920 × 1080. Median frame 13,3 ms, artinya monitor ini 75 Hz dan fps tidak bisa melebihi ±75.
- Build produksi (`wails3 task package`, ad-hoc signed) dijalankan langsung dari `bin/querra.app/Contents/MacOS/querra`.
- Database: PostgreSQL 18.2 dan MySQL 8.0.45 lokal (PhpWebStudy), database `querra_test` dari `wails3 task db:reset`.

## 3. Cara Mengulang

```bash
wails3 task package                                   # build produksi
python3 -I dev/perf/measure_macos.py bin/querra.app   # semua pengukuran, output JSON
ONLY=cancel python3 -I dev/perf/measure_macos.py      # satu bagian saja
```

Windows: buka tab **Actions → Spike measurements → Run workflow** di GitHub. Hasilnya muncul di ringkasan job dan sebagai artefak `spike-windows`. Runner GitHub tidak punya GPU, sehingga fps di sana adalah batas bawah.

## 4. Hasil

### 4.1 Waktu Mulai

Diukur dari proses diluncurkan sampai layar pertama selesai di-paint (dua `requestAnimationFrame` setelah render React pertama).

| Pengukuran | Hasil |
| --- | --- |
| Median 5 kali peluncuran | 330–332 ms |
| Peluncuran pertama setelah build baru | 833–1.317 ms |
| Bagian yang dihabiskan sebelum `main()` Go | ±5–20 ms |

Peluncuran pertama lebih lambat karena macOS memverifikasi signature binary baru dan cache file masih dingin. Ini juga yang akan dialami pengguna sekali setelah setiap pembaruan. Kondisi benar-benar dingin setelah reboot belum diukur.

### 4.2 Memori

Memori diukur dengan `footprint` (angka yang sama dengan kolom Memory di Activity Monitor), dijumlahkan untuk proses aplikasi dan proses WebKit (WebContent, GPU, Networking).

| Kondisi | Total | Aplikasi (Go) | WebContent | GPU |
| --- | --- | --- | --- | --- |
| Idle 10 dtk setelah mulai | 76–138 MB | 31 MB | 25–36 MB | 14–66 MB |
| Dataset 100k × 20 dimuat di Go | 341 MB | 142 MB | 114 MB | 78 MB |
| Tepat setelah benchmark scroll (200× `FetchRows`) | 617 MB | 242 MB | 284 MB | 84 MB |
| 20 dtk setelah benchmark | 452 MB | 241 MB | 184 MB | 20 MB |

Temuan:

- **Target idle tercapai dengan selisih besar.** Proses GPU WebKit paling bervariasi antar-run (14–98 MB).
- **Buffer hasil query di Go mahal jika setiap sel disimpan sebagai `*string`.** 2 juta sel memakan ±142 MB footprint (heap Go ±166 MB), atau ±70–85 byte per sel, kebanyakan overhead header string dan alokasi. Dengan batas default 10.000 baris (PLAN §3.4), biayanya ±15 MB dan masih wajar. Tapi untuk batas yang lebih besar, M2.2 sebaiknya menyimpan baris secara ringkas, mis. satu `[]byte` per baris plus offset sel, dan baru membuat `*string` saat `FetchRows`.
- **WebContent naik saat scroll cepat, lalu turun lagi.** Lonjakannya berasal dari sisa data `FetchRows` yang belum dibersihkan garbage collector. Cache halaman di frontend sendiri dibatasi 40 halaman (20.000 baris).

### 4.3 Grid

Benchmark otomatis menggulir grid dengan kecepatan tetap, satu langkah per animation frame (`src/features/spike/bench.ts`). Data diambil dari Go per halaman 500 baris lewat `FetchRows`, disimpan di cache LRU di frontend.

| Fase | Kecepatan | fps rata-rata | p95 frame | p99 frame | Frame > 50 ms | Frame dengan baris kosong |
| --- | --- | --- | --- | --- | --- | --- |
| `wheel-y` | 2.000 px/dtk ke bawah (scroll roda yang cepat) | 75 | 14 ms | 14–18 ms | 0 | 1 dari 600 |
| `fling-y` | ±270.000 px/dtk (seluruh 100k baris dalam 10 dtk) | 48–67 | 24–28 ms | 30–55 ms | 1–6 | ±30–40 % |
| `wheel-x` | Melintasi 20 kolom dalam 4 dtk | 75 | 15 ms | 15–16 ms | 0 | 0 |

| `FetchRows` (500 baris × 20 kolom) | Hasil |
| --- | --- |
| Rata-rata | 4,3–5,4 ms |
| p95 | 10–13 ms |
| Maksimum | 16–22 ms |

Temuan:

- Pada kecepatan scroll manusia, grid berjalan di batas refresh monitor dan data hampir tidak pernah terlambat.
- Fase `fling-y` sengaja ekstrem: setiap frame melompat ±165 baris, jadi hampir setiap frame butuh halaman baru. Baris sempat kosong sesaat. Ini wajar untuk paginasi dari Go, dan bisa diperhalus di M1.7 dengan prefetch halaman di depan arah scroll dan placeholder yang lebih halus.
- Resize kolom dengan TanStack Table v9 (`columnSizingFeature` + `columnResizingFeature`) berjalan bersama virtualisasi kolom.
- ESLint (`react-hooks/incompatible-library`) memperingatkan bahwa React Compiler tidak bisa memoize `useVirtualizer`. Ini tidak masalah, karena React Compiler tidak dipakai.

### 4.4 Pembatalan Query

Skenarionya: query `sleep 30 detik` dijalankan pada koneksi yang di-pin (seperti tab editor), lalu frontend memanggil `cancel()` pada `CancellablePromise` setelah 1 detik. Setengah detik kemudian, server dicek apakah sesi itu masih menjalankan query, dan koneksi yang sama dicoba lagi dengan `SELECT 1`.

| Strategi | Kembali setelah | Query berhenti di server | Sesi tetap bisa dipakai |
| --- | --- | --- | --- |
| PostgreSQL, pgx default (`DeadlineContextWatcherHandler`) | 999 ms | ✅ | ❌ koneksi diputus |
| **PostgreSQL, `CancelRequestContextWatcherHandler`** | 1.107 ms | ✅ (SQLSTATE 57014) | ✅ |
| MySQL, go-sql-driver default | 1.001 ms | ❌ tetap jalan sampai 30 dtk | ❌ koneksi diputus |
| **MySQL, `KILL QUERY` dari koneksi lain** (PLAN §3.8) | 1.003 ms | ✅ | ✅ |

Temuan:

- Wails v3 meneruskan `cancel()` dari frontend ke `context.Context` Go dengan andal. Hasil panggilan yang dibatalkan dibuang (promise ditolak dengan `CancelError`), jadi hasil seperti ini dikirim lewat event bertipe (`spike:cancel-result`).
- **Koreksi PLAN §3.8:** pgx *tidak* menjaga sesi secara default. Adapter PostgreSQL wajib memasang `BuildContextWatcherHandler` yang mengembalikan `&pgconn.CancelRequestContextWatcherHandler{Conn: conn, DeadlineDelay: 5 * time.Second}`. Deadline hanya cadangan untuk server yang mengabaikan cancel request.
- **Risiko R2 (MySQL) terkonfirmasi, dan mitigasinya terbukti bekerja.** Tanpa `KILL QUERY`, query yatim tetap jalan di server. Dalam uji pertama, query yatim ini bahkan mengacaukan pengecekan uji berikutnya.
- Ketika `SLEEP()` dihentikan dengan `KILL QUERY`, MySQL mengembalikan nilai 1 alih-alih error. Query lain mengembalikan error 1317. Adapter harus menganggap keduanya sebagai "dibatalkan".

### 4.5 Keychain (`go-keyring`)

Skenarionya: build A menulis nilai dummy, proses lain membacanya, lalu build B (signature ad-hoc berbeda) membaca dan menghapusnya.

| Langkah | Hasil |
| --- | --- |
| Build A menulis | 23 ms, tanpa prompt |
| `/usr/bin/security find-generic-password -w` dari shell | Nilai terbaca tanpa prompt (hanya di-encode base64) |
| Build B (cdhash berbeda) membaca | 16 ms, tanpa prompt |
| Build B menghapus | 317 ms, tanpa prompt |

Di macOS, `go-keyring` tidak memakai API Keychain native. Library ini menjalankan `/usr/bin/security`, sehingga aplikasi yang dipercaya oleh item keychain adalah `security`, bukan Querra. Akibatnya:

- **Baiknya:** tidak ada prompt izin setelah setiap pembaruan, padahal build ad-hoc selalu mendapat signature baru. Ini cocok dengan distribusi tanpa sertifikat berbayar.
- **Buruknya:** program apa pun yang berjalan sebagai user yang sama bisa membaca password koneksi tanpa prompt. Password tetap tidak tersimpan sebagai teks biasa di disk (sesuai PRD), tapi isolasi antar-aplikasinya lebih lemah daripada API native. Kondisi ini kira-kira setara dengan Credential Manager di Windows, yang juga bisa dibaca proses lain milik user yang sama.

Alternatifnya, API Keychain native via cgo (misalnya `github.com/keybase/go-keychain`) mengikat item ke signature aplikasi. Tanpa Developer ID, signature berubah setiap pembaruan, sehingga macOS meminta izin ulang untuk **setiap** item setelah update. Alternatif ini tidak diuji di spike.

## 5. Belum Diuji

| Item | Yang perlu dilakukan | Oleh |
| --- | --- | --- |
| Grid, waktu mulai, memori, dan ukuran installer di WebView2 | Jalankan workflow **Spike measurements** di GitHub Actions, lalu tempel ringkasannya ke dokumen ini | Pemilik (memicu workflow) |
| Grid di Windows dengan GPU sungguhan | Jalankan `dev/perf/measure_windows.ps1` di PC Windows, atau cukup coba scroll grid secara manual | Pemilik/rekan |
| Installer tanpa tanda tangan di Windows (SmartScreen → Run anyway) | Unduh installer dari artefak workflow, lalu pasang di PC Windows | Pemilik/rekan |
| DMG tanpa notarization di Mac lain (Gatekeeper → Open Anyway) | Kirim `bin/querra.dmg` lewat AirDrop atau unduhan supaya mendapat atribut karantina, lalu buka di Mac lain | Pemilik/rekan |
| Tailwind v4 di macOS 12 (R3) | Buka aplikasi di macOS 12. Jika tidak ada perangkat, risiko diterima dan diuji saat Beta | Pemilik |

## 6. Dampak ke Rencana

- **PLAN §3.8:** konfigurasi pembatalan PostgreSQL dikoreksi (`CancelRequestContextWatcherHandler`), dan `KILL QUERY` untuk MySQL ditandai sudah tervalidasi.
- **PLAN §3.4:** catatan penyimpanan buffer hasil yang ringkas untuk M2.2.
- **PLAN §3.6:** catatan perilaku `go-keyring` di macOS dan keputusan backend keychain (di bawah).
- **PLAN M1.7:** prefetch halaman searah scroll untuk mengurangi baris kosong saat scroll sangat cepat.
- **Risiko R2:** dimitigasi dan terbukti. **R3:** masih terbuka.

## 7. Keputusan untuk Pemilik

**Backend keychain di macOS.** Rekomendasi: tetap memakai `go-keyring` untuk v1.

| Pilihan | Keamanan | Pengalaman pengguna tanpa Developer ID |
| --- | --- | --- |
| `go-keyring` (via `/usr/bin/security`), **rekomendasi** | Proses lain milik user yang sama bisa membaca tanpa prompt; setara Windows | Tidak ada prompt, termasuk setelah update |
| API Keychain native (cgo) | Proses lain memicu prompt | Prompt izin untuk setiap koneksi setelah **setiap** update |

Alasannya: Querra dipakai pribadi dan oleh rekan dekat tanpa sertifikat berbayar, dan model ancaman "malware yang sudah berjalan sebagai user Anda" juga tidak dilindungi di Windows. Pilihan ini ditinjau ulang bila suatu saat memakai Developer ID (PLAN R10), karena dengan signature yang stabil, API native tidak lagi memunculkan prompt setelah update.
