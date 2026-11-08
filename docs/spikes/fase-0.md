# Spike Fase 0 (M0.2): Validasi Stack

*Diukur 7 November 2026 (macOS) dan 8 November 2026 (Windows) · build `0.1.0-dev` · Wails v3.1.0 · TanStack Table 9.3 + TanStack Virtual 3.14 · CodeMirror 6*

Spike ini menjawab pertanyaan berisiko di PLAN M0.2 sebelum fitur sungguhan dibangun. Kodenya sementara (`internal/spike`, `internal/app/spikeservice.go`, `frontend/src/features/spike`) dan dihapus di M0.3. Skrip pengukurannya (`dev/perf/`) tetap dipakai untuk M4.8.

> **Sejak M0.3:** kode spike sudah dihapus. Probe pembatalan dan keychain hanya ada di riwayat git (commit `e08c486`). Mode autorun startup, idle, dan grid pindah ke `PerfService` (`internal/app/perfservice.go`), dengan variabel `QUERRA_PERF_*` menggantikan `QUERRA_SPIKE_*`. Perintah di §3 tetap berlaku, kecuali pengukuran `cancel`.

## 1. Ringkasan

| Pertanyaan M0.2 | Hasil | Status |
| --- | --- | --- |
| Grid 100.000 × 20 lancar di WKWebView? | ±75 fps (batas refresh monitor) pada kecepatan scroll normal, p95 14–15 ms, tanpa frame panjang | ✅ |
| Grid yang sama lancar di WebView2? | ±143 fps (batas monitor 144 Hz) pada kecepatan scroll normal, p95 7 ms, tanpa frame panjang. `FetchRows` ±3× lebih lambat daripada di macOS (§4.6) | ✅ |
| Waktu mulai < 2 detik? | Median 330 ms (macOS) dan 599 ms (Windows). Peluncuran pertama setelah build baru: 0,8–1,3 dtk (macOS), 2,4–2,8 dtk (Windows, §4.6) | ✅, kecuali peluncuran pertama di Windows ⚠️ |
| Memori idle < 250 MB? | 76–138 MB (macOS) dan 159 MB (Windows), termasuk proses webview | ✅ |
| Installer < 60 MB? | DMG universal 14 MB (`.app` 31 MB); installer NSIS 7,7 MB (`.exe` 16 MB) | ✅ |
| `CancellablePromise` → ctx Go → query berhenti? | Ya. Butuh konfigurasi khusus agar sesi tetap hidup (§4.4) | ✅ dengan catatan |
| `go-keyring` di macOS, termasuk prompt setelah build ulang? | Tidak ada prompt, tapi proses lain bisa ikut membaca (§4.5) | ⚠️ perlu keputusan |
| Build tanpa tanda tangan dibuka di Mac/PC lain | Windows: installer langsung terbuka setelah UAC, tanpa SmartScreen dan tanpa blokir Smart App Control (§4.7). Mac lain belum | ✅ Windows, ⏳ Mac |
| Tailwind v4 di macOS 12 (R3) | Belum. Butuh perangkat macOS 12 | ⏳ manual |

**Gerbang keputusan:** di macOS dan Windows kedua pilihan stack terbukti cukup.

- **Grid:** lanjut dengan TanStack Table v9 (hanya untuk model kolom: ukuran dan resize) + TanStack Virtual (virtualisasi baris dan kolom). Tidak perlu pustaka grid lain.
- **Desktop shell:** lanjut dengan Wails v3. Pembatalan, binding bertipe, dan event bertipe bekerja sesuai dokumentasi.

Hasil WebView2 (§4.6) mengonfirmasi keputusan ini, jadi gerbang keputusan ditutup.

## 2. Lingkungan Uji

- **macOS:** Apple M4, RAM 16 GB, macOS 26.6.2, monitor eksternal 1920 × 1080. Median frame 13,3 ms, artinya monitor ini 75 Hz dan fps tidak bisa melebihi ±75.
- Build produksi (`wails3 task package`, ad-hoc signed) dijalankan langsung dari `bin/querra.app/Contents/MacOS/querra`.
- Database: PostgreSQL 18.2 dan MySQL 8.0.45 lokal (PhpWebStudy), database `querra_test` dari `wails3 task db:reset`.
- **Windows:** laptop AMD Ryzen 7 170 (Radeon 680M terintegrasi, plus NVIDIA RTX 3050 Laptop), RAM 16 GB, Windows 11 Home build 26300, WebView2 Runtime 157.0.4341.42, layar 1920 × 1080 144 Hz. Median frame 7 ms, artinya fps tidak bisa melebihi ±144. Microsoft Defender (real-time) dan Smart App Control aktif.
- Build produksi (`wails3 task package`, tanpa tanda tangan) dijalankan langsung dari `bin/querra.exe`. Pengukuran Windows tidak memakai database.

## 3. Cara Mengulang

```bash
wails3 task package                                   # build produksi
python3 -I dev/perf/measure_macos.py bin/querra.app   # semua pengukuran, output JSON
ONLY=grid python3 -I dev/perf/measure_macos.py        # satu bagian saja
```

Windows (Windows PowerShell 5.1 atau pwsh 7):

```powershell
wails3 task package                                    # build produksi + installer NSIS
.\dev\perf\measure_windows.ps1 -Exe bin/querra.exe     # startup, idle, dan grid; output JSON
```

Workflow **Perf measurements** (**Actions → Perf measurements → Run workflow**; dulu bernama Spike measurements) menjalankan skrip yang sama di runner GitHub. Runner itu tidak punya GPU, sehingga fps di sana hanya batas bawah; angka di §4.6 berasal dari PC sungguhan.

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

### 4.6 Windows (WebView2)

Diukur dengan `dev/perf/measure_windows.ps1` di PC Windows pada §2. Memori di Windows adalah *private working set* (kolom Memory di Task Manager), dijumlahkan untuk `querra.exe` dan semua proses `msedgewebview2.exe` turunannya. Angkanya tidak persis sama dengan `footprint` di macOS, tapi keduanya adalah angka yang dilihat pengguna, jadi sama-sama berlaku untuk anggaran PRD.

**Waktu mulai**

| Pengukuran | Hasil |
| --- | --- |
| Median 5 kali peluncuran | 599 ms (peluncuran ke-2 sampai ke-5: 561–652 ms) |
| Peluncuran pertama build baru | 2.833 ms. Diulang dengan salinan exe ber-hash baru: 2.355 ms |
| Bagian sebelum Go mulai, peluncuran pertama exe baru | ±1,8–2,1 dtk |
| Bagian sebelum Go mulai, peluncuran berikutnya | ±65–95 ms |

Temuan:

- Peluncuran biasa jauh di bawah target 2 detik.
- **Peluncuran pertama setelah build baru melebihi 2 detik**, dan hampir seluruh selisihnya terjadi sebelum kode Go berjalan. Pola ini (hanya pada exe dengan hash baru, sebelum `main()`) cocok dengan pemindaian real-time Microsoft Defender terhadap exe baru tanpa tanda tangan. Penyebab itu belum dibuktikan, karena Defender tidak dimatikan untuk uji ini. Pengguna mengalaminya sekali setelah setiap instalasi atau pembaruan, setara dengan verifikasi signature di macOS.
- Pada salinan exe tadi, peluncuran kedua juga butuh 2,4 dtk, tapi kali ini di dalam proses Go (2.345 ms sejak Go mulai). Penyebabnya belum diketahui, dan peluncuran ketiga kembali normal (706 ms). Perlu diamati lagi di M4.8 dengan shell UI sungguhan.

**Memori**

| Kondisi | Total | Aplikasi (Go) | WebView2 (6 proses) |
| --- | --- | --- | --- |
| Idle 10 dtk setelah mulai | 159 MB | 9 MB | 151 MB |
| Dataset 100k × 20 dimuat di Go | 331 MB | 118 MB | 213 MB |
| Tepat setelah benchmark scroll (199× `FetchRows`) | 742 MB | 221 MB | 522 MB |
| 20 dtk setelah benchmark | 545 MB | 221 MB | 325 MB |

Temuan:

- Target idle tercapai, tapi sisa ruangnya lebih kecil daripada di macOS: proses WebView2 sendiri sudah ±150 MB sebelum ada UI sungguhan. Shell UI (M0.3), CodeMirror, dan grid akan menambah memori renderer, jadi anggaran 250 MB perlu dicek di Windows setiap kali milestone UI selesai, bukan hanya di M4.8.
- Satu proses renderer naik ke ±360 MB saat scroll sangat cepat, lalu turun ke ±170 MB dalam 20 dtk. Polanya sama dengan WebContent di macOS.

**Grid**

| Fase | fps rata-rata | p95 frame | p99 frame | Frame > 50 ms | Frame dengan baris kosong |
| --- | --- | --- | --- | --- | --- |
| `wheel-y` | 143 | 7,1 ms | 7,2 ms | 0 | 4 dari 1.148 |
| `fling-y` | 56 | 42 ms | 56 ms | 11 | 390 dari 564 (±69 %) |
| `wheel-x` | 143 | 7,2 ms | 7,4 ms | 0 | 0 |

| `FetchRows` (500 baris × 20 kolom) | Windows | macOS (§4.3) |
| --- | --- | --- |
| Rata-rata | 16 ms | 4,3–5,4 ms |
| p95 | 27 ms | 10–13 ms |
| Maksimum | 42 ms | 16–22 ms |

Temuan:

- Pada kecepatan scroll manusia, vertikal maupun horizontal, grid berjalan di batas refresh monitor 144 Hz tanpa frame panjang.
- **`FetchRows` ±3× lebih lambat daripada di macOS.** Data yang sama sudah ada di memori Go, jadi selisihnya berasal dari jembatan Wails di WebView2, dari CPU yang berbeda, atau dari keduanya. Keduanya belum dipisahkan.
- Akibatnya, pada `fling-y` baris kosong naik menjadi ±69 % frame (macOS ±30–40 %). Ini memperkuat kebutuhan prefetch searah scroll di M1.7, dan jumlah halaman yang di-prefetch sebaiknya dikalibrasi di Windows. Keputusan grid tidak perlu dibuka lagi, karena hanya kasus ekstrem yang terpengaruh.

**Installer:** NSIS 7,7 MB (`querra.exe` 16,1 MB), sudah termasuk bootstrapper WebView2 (1,8 MB) yang memasang runtime bila belum ada (R7).

### 4.7 Installer Tanpa Tanda Tangan di Windows

Diuji 8 November 2026 di PC Windows pada §2 (Smart App Control **aktif**, Defender real-time aktif). Installer NSIS disalin lalu diberi stream `Zone.Identifier` berisi `ZoneId=3`, sehingga Windows memperlakukannya sebagai file yang diunduh dari internet. File itu dibuka dengan klik dua kali di File Explorer.

| Langkah | Hasil |
| --- | --- |
| SmartScreen ("Windows protected your PC") | Tidak muncul |
| Smart App Control | Tidak memblokir; log Code Integrity tidak mencatat event apa pun |
| UAC | Muncul (publisher tidak dikenal), lalu wizard NSIS terbuka |
| `querra.exe` hasil instalasi | Berjalan normal |

Temuan:

- Di PC ini, pengguna hanya perlu menyetujui UAC. Ini lebih ringan daripada yang diperkirakan R10.
- Penyebab SmartScreen dan Smart App Control tidak bereaksi belum diketahui. Hasilnya bisa berbeda di PC lain atau setelah Windows memperbarui reputasi file.
- Unduhan lewat browser punya pemeriksaan tersendiri (SmartScreen di Edge atau Safe Browsing di Chrome, mis. peringatan "not commonly downloaded"). Ini baru bisa diuji dengan rilis GitHub sungguhan di M4.6, dan panduan instalasi (B.3) tetap perlu menjelaskan **Run anyway** untuk berjaga-jaga.

## 5. Belum Diuji

| Item | Yang perlu dilakukan | Oleh |
| --- | --- | --- |
| Installer yang diunduh lewat browser di Windows | Unduh installer dari rilis GitHub sungguhan (M4.6) dengan Edge dan Chrome, lalu catat peringatan browser dan SmartScreen. Uji dengan salinan bertanda `ZoneId=3` sudah lolos (§4.7) | Pemilik/rekan |
| DMG tanpa notarization di Mac lain (Gatekeeper → Open Anyway) | Kirim `bin/querra.dmg` lewat AirDrop atau unduhan supaya mendapat atribut karantina, lalu buka di Mac lain | Pemilik/rekan |
| Tailwind v4 di macOS 12 (R3) | Buka aplikasi di macOS 12. Jika tidak ada perangkat, risiko diterima dan diuji saat Beta | Pemilik |

## 6. Dampak ke Rencana

- **PLAN §3.8:** konfigurasi pembatalan PostgreSQL dikoreksi (`CancelRequestContextWatcherHandler`), dan `KILL QUERY` untuk MySQL ditandai sudah tervalidasi.
- **PLAN §3.4:** catatan penyimpanan buffer hasil yang ringkas untuk M2.2.
- **PLAN §3.6:** catatan perilaku `go-keyring` di macOS dan keputusan backend keychain (di bawah).
- **PLAN M1.7:** prefetch halaman searah scroll untuk mengurangi baris kosong saat scroll sangat cepat. Kalibrasi di Windows, karena `FetchRows` di sana ±3× lebih lambat.
- **PLAN §6:** memori idle Windows (159 MB) dicek setiap milestone UI, dan peluncuran pertama setelah pembaruan di Windows (2,4–2,8 dtk) diukur lagi di M4.8.
- **Risiko R2:** dimitigasi dan terbukti. **R3:** masih terbuka. **R10:** di PC uji, Smart App Control dan SmartScreen tidak memblokir installer tanpa tanda tangan (§4.7); peringatan browser saat mengunduh diuji di M4.6.

## 7. Keputusan untuk Pemilik

**Backend keychain di macOS.** Rekomendasi: tetap memakai `go-keyring` untuk v1.

| Pilihan | Keamanan | Pengalaman pengguna tanpa Developer ID |
| --- | --- | --- |
| `go-keyring` (via `/usr/bin/security`), **rekomendasi** | Proses lain milik user yang sama bisa membaca tanpa prompt; setara Windows | Tidak ada prompt, termasuk setelah update |
| API Keychain native (cgo) | Proses lain memicu prompt | Prompt izin untuk setiap koneksi setelah **setiap** update |

Alasannya: Querra dipakai pribadi dan oleh rekan dekat tanpa sertifikat berbayar, dan model ancaman "malware yang sudah berjalan sebagai user Anda" juga tidak dilindungi di Windows. Pilihan ini ditinjau ulang bila suatu saat memakai Developer ID (PLAN R10), karena dengan signature yang stabil, API native tidak lagi memunculkan prompt setelah update.

## 8. Pengukuran Ulang Setelah M0.3 (Windows)

*8 November 2026 · PC Windows pada §2 · `dev/perf/measure_windows.ps1` dengan mode autorun `PerfService`*

M0.3 mengganti halaman spike dengan shell sungguhan (sidebar, tab, panel hasil, status bar, i18n) dan menghapus driver database serta keychain dari binary. Grid benchmark tetap memakai grid spike, kini sebagai overlay di atas shell dengan ukuran area yang sama.

| Pengukuran | M0.2 (halaman spike, §4.6) | M0.3 (shell) |
| --- | --- | --- |
| Waktu mulai, median 5 kali | 599 ms | 706 ms |
| Memori idle (total) | 159 MB | 130 MB |
| Ukuran `querra.exe` / installer | 16,1 / 7,7 MB | 10,3 / 6,0 MB |
| Grid `wheel-y` | 143 fps, p95 7,1 ms, 0 frame > 50 ms | 121 fps, p95 14 ms, 1 frame > 50 ms |
| Grid `fling-y` | 56 fps, baris kosong ±69 % frame | 58 fps, baris kosong ±75 % frame |
| Grid `wheel-x` | 143 fps | 141 fps |
| `FetchRows` rata-rata / maks | 16 / 42 ms | 21 / 237 ms |
| Memori 20 dtk setelah benchmark | 545 MB | 572 MB |

Temuan:

- Shell sungguhan menambah ±100 ms pada waktu mulai. Angkanya masih jauh di bawah target 2 detik.
- Memori idle lebih rendah, tetapi jumlah memori proses WebView2 memang bervariasi antar-run (lihat juga §4.2 untuk macOS). Jangan menyimpulkan penghematan dari satu run.
- `wheel-y` turun ke 121 fps dengan satu frame panjang, dan ada satu `FetchRows` lambat (237 ms). Ini hasil satu run, jadi bisa saja noise. Grid tetap lancar untuk target ±60 fps, dan angkanya diukur ulang dengan grid sungguhan di M1.7.
- Binary mengecil karena pgx, driver MySQL, dan go-keyring ikut terhapus bersama spike. Ukurannya akan naik lagi di M1.1–M1.2.

