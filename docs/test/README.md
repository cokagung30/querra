# Uji Manual per Milestone

Folder ini berisi checklist uji manual lintas platform, satu file per milestone. Tujuannya agar pengujian bisa berpindah antara Windows dan macOS (dan antar-sesi Claude Code) tanpa kehilangan konteks. Status dan hasil uji tersimpan di repositori, bukan hanya di obrolan.

Tes otomatis (`wails3 task test`, `test:integration`) tetap menjadi lapis utama. File di sini hanya untuk hal yang perlu dicek oleh manusia: tampilan, shortcut di webview sungguhan, menu native, installer, dan pengukuran performa.

## Daftar File

| File | Milestone | Windows | macOS |
| --- | --- | --- | --- |
| [M0.2.md](M0.2.md) | Spike validasi stack (sisa uji manual) | ✅ | ⏳ |
| [M0.3.md](M0.3.md) | Shell UI dan fondasi desain | ✅ | ✅ (B12b ⏳) |
| [M0.5.md](M0.5.md) | Distribusi Windows lewat Microsoft Store | ✅ | ➖ |
| [M1.3.md](M1.3.md) | Layanan koneksi | ➖ (domain dibuat di sini; tes lewat CI) | ⏳ (lapisan Wails) |
| [M1.1.md](M1.1.md) | Penyimpanan lokal dan rahasia | ✅ | ⏳ |
| [M1.2.md](M1.2.md) | Kontrak driver dan adapter PostgreSQL | ➖ (diblokir Smart App Control); CI ✅ | ⏳ |

Perbarui tabel ini setiap kali status sebuah file berubah.

## Penamaan

- `M<fase>.<nomor>.md` untuk setiap milestone yang punya uji manual, mis. `M1.4.md`.
- Mulai M4.6 (ada rilis sungguhan), uji regresi per rilis memakai `v<versi>.md`, mis. `v1.0.0.md`.
- Salin [_template.md](_template.md) untuk file baru.

## Status

| Tanda | Arti |
| --- | --- |
| ✅ | Lulus |
| ❌ | Gagal; jelaskan di bagian **Temuan** |
| ⏳ | Belum dicek |
| ➖ | Tidak berlaku untuk platform ini |

## Build Windows dari CI

Di PC Windows dengan Smart App Control aktif, `wails3` dan biner Go buatan lokal bisa diblokir (PLAN R10). Karena itu uji manual di Windows memakai build dari CI. Setiap run CI yang hijau mengunggah build produksi Windows sebagai artifact `querra-windows`, yang tersimpan 14 hari.

1. Buka repo di GitHub → **Actions** → workflow **CI**, lalu pilih run untuk commit yang mau diuji (harus hijau).
2. Di bagian **Artifacts**, unduh `querra-windows` lewat browser (perlu login GitHub). Isinya:
   - `querra-amd64-installer.exe`: installer NSIS;
   - `querra.exe`: bisa dijalankan tanpa instal;
   - `querra-amd64.msix`: paket untuk diunggah ke Partner Center. Paket ini belum ditandatangani, jadi tidak bisa dipasang langsung. Microsoft menandatanganinya saat submission ke Store (PLAN M0.5).

   Di PC dengan Smart App Control aktif, installer dan exe tanpa tanda tangan diblokir. Untuk PC seperti itu, pakai versi dari Microsoft Store.
3. Ekstrak zip-nya lewat File Explorer, lalu jalankan installer seperti pengguna biasa.
4. Catat apa yang muncul sebelum aplikasi terbuka (UAC, SmartScreen, atau blokir Smart App Control) di checklist milestone. Unduhan lewat browser ditandai "dari internet" seperti unduhan teman, jadi langkah ini sekaligus menguji R10. Kalau diblokir, tulis di **Temuan** dan beri tahu Claude.

Lewat terminal juga bisa: `gh run list --workflow CI --limit 5`, lalu `gh run download <run-id> -n querra-windows -D "$env:TEMP\querra-ci"`. File yang diunduh lewat `gh` tidak bertanda "dari internet", jadi tidak mewakili pengalaman teman untuk R10.

## Alur Kerja Lintas Platform

1. Saat milestone selesai di satu platform, Claude membuat atau memperbarui `docs/test/<milestone>.md`, mengisi hasil platform itu, lalu kolom platform lain dibiarkan ⏳.
2. Commit dan push (mis. `docs(test): M0.3 macOS checklist`).
3. Di mesin lain: `git pull`, lalu buka Claude Code dan katakan, misalnya: *"Lanjutkan uji di docs/test/M0.3.md di macOS."* Claude membaca file ini dan `CLAUDE.md`, jadi konteksnya tersambung lagi.
4. Kerjakan langkahnya. Isi kolom platform itu sendiri, atau laporkan hasilnya ke Claude dan biarkan Claude yang mengisi. Hasil pengukuran ditempel di bagian **Pengukuran**.
5. Commit dan push hasilnya (mis. `docs(test): M0.3 macOS results`).
6. Setelah semua platform ✅, centang milestone di `PLAN.md`. Temuan yang butuh perbaikan dicatat di bagian **Temuan** dan dikerjakan sebelum milestone ditutup.
