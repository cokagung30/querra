# Uji Manual per Milestone

Folder ini berisi checklist uji manual lintas platform, satu file per milestone. Tujuannya agar pengujian bisa berpindah antara Windows dan macOS (dan antar-sesi Claude Code) tanpa kehilangan konteks. Status dan hasil uji tersimpan di repositori, bukan hanya di obrolan.

Tes otomatis (`wails3 task test`, `test:integration`) tetap menjadi lapis utama. File di sini hanya untuk hal yang perlu dicek oleh manusia: tampilan, shortcut di webview sungguhan, menu native, installer, dan pengukuran performa.

## Daftar File

| File | Milestone | Windows | macOS |
| --- | --- | --- | --- |
| [M0.2.md](M0.2.md) | Spike validasi stack (sisa uji manual) | ✅ | ⏳ |
| [M0.3.md](M0.3.md) | Shell UI dan fondasi desain | ✅ (sebagian ⏳) | ✅ |
| [M1.1.md](M1.1.md) | Penyimpanan lokal dan rahasia | ✅ | ⏳ |

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

## Alur Kerja Lintas Platform

1. Saat milestone selesai di satu platform, Claude membuat atau memperbarui `docs/test/<milestone>.md`, mengisi hasil platform itu, lalu kolom platform lain dibiarkan ⏳.
2. Commit dan push (mis. `docs(test): M0.3 macOS checklist`).
3. Di mesin lain: `git pull`, lalu buka Claude Code dan katakan, misalnya: *"Lanjutkan uji di docs/test/M0.3.md di macOS."* Claude membaca file ini dan `CLAUDE.md`, jadi konteksnya tersambung lagi.
4. Kerjakan langkahnya. Isi kolom platform itu sendiri, atau laporkan hasilnya ke Claude dan biarkan Claude yang mengisi. Hasil pengukuran ditempel di bagian **Pengukuran**.
5. Commit dan push hasilnya (mis. `docs(test): M0.3 macOS results`).
6. Setelah semua platform ✅, centang milestone di `PLAN.md`. Temuan yang butuh perbaikan dicatat di bagian **Temuan** dan dikerjakan sebelum milestone ditutup.
