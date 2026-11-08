# Riset Singkat: Pola UX Klien Database

*8 November 2026 · Bagian dari M0.3 (PRD §12, Fase 0: riset kompetitor)*

Catatan ini merangkum pola UX yang umum di klien database populer, untuk memilih bentuk shell Querra. Sumbernya pengetahuan umum tentang produk-produk ini dan dokumentasi publiknya, bukan uji mendalam di versi terbaru masing-masing. Detail per produk bisa berbeda di versi yang lebih baru.

## 1. Produk yang Dilihat

| Produk | Platform dan teknologi | Kesan umum |
| --- | --- | --- |
| TablePlus | macOS, Windows, Linux; native; komersial | Acuan PRD. Ringan, cepat, tab, penanda environment per koneksi |
| DBeaver Community | Lintas platform; Java (Eclipse) | Fitur paling lengkap, tetapi berat dan UI-nya padat |
| DataGrip | Lintas platform; JVM (JetBrains); komersial | Editor dan introspeksi sangat kuat, berat untuk pekerjaan ringan |
| Beekeeper Studio | Lintas platform; Electron; open source + edisi berbayar | Sederhana dan modern, tetapi memakai memori khas Electron |
| Postico 2 | macOS saja; PostgreSQL | Sangat native dan fokus |
| pgAdmin 4 | Aplikasi web (bisa dibungkus desktop); PostgreSQL | Lengkap untuk administrasi PG, terasa lambat untuk pekerjaan harian |
| Sequel Ace / HeidiSQL | macOS (Sequel Ace) dan Windows (HeidiSQL); MySQL/MariaDB | Ringan, gaya lama, terikat satu OS |

## 2. Pola yang Umum

1. **Tata letak tiga bagian.** Sidebar kiri berisi koneksi lalu pohon objek database. Area kanan berisi tab. Di tab query, editor ada di atas dan hasil di bawah, dengan pembatas yang bisa digeser. Status bar menampilkan koneksi aktif, jumlah baris, dan durasi query. Pola ini dipakai TablePlus, DBeaver, DataGrip, dan Beekeeper Studio.
2. **Semua hal dibuka sebagai tab.** Isi tabel, query, dan struktur tabel masing-masing menjadi tab, dengan shortcut standar untuk membuka, menutup, dan berpindah tab. Masalah yang sering muncul adalah tab menumpuk. Editor kode seperti VS Code menjawabnya dengan tab pratinjau yang diganti saat pengguna membuka item lain. Pola ini layak dipertimbangkan untuk M1.7.
3. **Penanda environment pada koneksi.** TablePlus memberi tag (mis. local, staging, production) dan warna per koneksi. DBeaver punya *connection type* (Development, Test, Production) dengan warna, konfirmasi sebelum eksekusi SQL, dan auto-commit yang bisa dimatikan untuk Production. DataGrip memberi warna per data source dan opsi read-only. Pola umumnya adalah warna yang selalu terlihat ditambah rem tambahan untuk Production. Ini sejalan dengan PLAN §3.7.
4. **Perubahan grid ditahan sampai di-commit.** TablePlus menahan edit sel sampai pengguna menyimpan (Cmd+S) dan bisa memperlihatkan SQL yang akan dijalankan. DBeaver punya alur Save/Cancel yang serupa. Querra mengambil pola ini di M3.2–M3.3.
5. **Filter tanpa menulis SQL.** Bilah filter di atas grid (kolom, operator, nilai) adalah cara tercepat memeriksa data, dan biasanya bisa ditukar ke klausa WHERE mentah. Querra mengambilnya di M1.7.
6. **Buka cepat.** Pengguna mengetik nama tabel untuk langsung membukanya (Cmd+P di TablePlus, "Go to"/Search Everywhere di DataGrip). Querra menjadwalkannya di M4.4.
7. **Jalankan statement di kursor atau semuanya.** Cmd/Ctrl+Enter menjalankan statement di posisi kursor atau seleksi, dengan perintah terpisah untuk menjalankan semua statement. Ini relevan untuk M2.2–M2.3.
8. **Shortcut yang bisa diubah.** Klien yang matang (DataGrip, DBeaver) menyediakan keymap yang bisa diatur. Karena itu Querra membangun registry perintah sejak M0.3, dan editor keymap menyusul di M4.5.

## 3. Keputusan untuk Shell M0.3

- **Tata letak:** sidebar kiri (daftar koneksi, lalu pohon skema di M1.6), bilah tab, area konten, panel hasil di bawah, dan status bar. Sidebar dan panel hasil bisa diubah ukurannya dan disembunyikan; ukurannya diingat antar-sesi.
- **Minim chrome:** tanpa toolbar besar. Aksi berupa perintah ber-shortcut, dengan tombol ikon kecil di bilah tab dan status bar yang menampilkan shortcut-nya di tooltip.
- **Shortcut default mengikuti kebiasaan umum:** Mod+T tab baru, Mod+W tutup tab, Ctrl+Tab / Ctrl+Shift+Tab pindah tab, Mod+B sidebar, dan Mod+J panel hasil (seperti VS Code). Mod adalah Cmd di macOS dan Ctrl di Windows.
- **Tema mengikuti sistem.** Token terang/gelap sudah ada sejak awal; pilihan manual menyusul di M3.5.
- **Warna environment sebagai token** (`env-local`, `env-staging`, `env-production`), supaya setiap tempat yang menampilkan koneksi memakai warna yang sama.
- **Title bar bawaan OS.** Title bar kustom seperti di TablePlus atau Postico ditunda, karena butuh penanganan berbeda per OS (area drag, tombol jendela Windows) dan belum memberi nilai di tahap ini.

## 4. Yang Dihindari

- UI padat penuh panel dan toolbar seperti DBeaver atau pgAdmin, yang menyulitkan pekerjaan cepat.
- Dialog modal berlapis untuk pekerjaan rutin. Konfirmasi hanya untuk operasi berisiko (PLAN §3.7).
- Menjalankan ulang statement secara otomatis setelah koneksi putus (PLAN §3.8).
