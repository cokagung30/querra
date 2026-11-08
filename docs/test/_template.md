# Uji Manual M<x>.<y> – <Nama Milestone>

*Versi aplikasi `<versi>` · build dari commit `<hash>` atau lebih baru · aturan pengisian di [README.md](README.md)*

| Platform | Status | Diuji oleh, tanggal | Catatan |
| --- | --- | --- | --- |
| Windows | ⏳ | | |
| macOS | ⏳ | | |

## Konteks untuk Claude

<!-- Ringkas apa yang dibangun, keputusan yang perlu diingat, apa yang sudah diverifikasi,
     dan apa yang harus dilakukan dengan hasil uji (mis. memperbarui PLAN.md atau laporan). -->

## Persiapan

```bash
git pull
cd frontend && npm ci && cd ..
```

## A. Tes Otomatis

| ID | Langkah | Hasil yang diharapkan | Windows | macOS |
| --- | --- | --- | --- | --- |
| A1 | `wails3 task lint` | Lulus | ⏳ | ⏳ |
| A2 | `wails3 task test` | Lulus | ⏳ | ⏳ |

## B. Uji Manual

| ID | Langkah | Hasil yang diharapkan | Windows | macOS |
| --- | --- | --- | --- | --- |
| B1 | | | ⏳ | ⏳ |

## C. Build Produksi

| ID | Langkah | Hasil yang diharapkan | Windows | macOS |
| --- | --- | --- | --- | --- |
| C1 | `wails3 task package` | Build sukses, `git status` tetap bersih | ⏳ | ⏳ |

## Pengukuran

<!-- Tempel ringkasan angka (atau JSON) dari dev/perf bila milestone ini memengaruhi performa. -->

## Temuan

<!-- Satu butir per masalah: ID langkah, platform, apa yang terjadi, dan tindak lanjutnya. -->

## Penutupan

- [ ] Semua platform ✅ (atau ➖)
- [ ] Temuan sudah ditindaklanjuti
- [ ] Milestone dicentang di `PLAN.md`
