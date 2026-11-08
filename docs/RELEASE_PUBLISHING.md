# Publikasi paket RelayDesk

## Status 8 November 2026

[Release v0.1.0-preview.1](https://github.com/firefuck/connector-for-agent/releases/tag/v0.1.0-preview.1)
sudah diterbitkan sebagai prerelease dengan 10 aset:

- Windows x64 portable `relaydesk.exe` dan ZIP lengkap.
- Ubuntu/Debian amd64 DEB dan binary Linux portable.
- `LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md`.
- `QUICKSTART.txt`, `BUILDINFO.txt`, dan `SHA256SUMS`.

Source paket/tag: `f054c36af62e1e7d42d561c90d793d07688cffe7`.
Semua aset diunduh kembali dari GitHub sebelum publikasi dan SHA-256 cocok.
Sesudah publikasi, link download publik EXE/DEB/ZIP/checksum diperiksa tanpa token.
Build/race suite/vet serta instalasi DEB terisolasi lulus. Ini bukan pengujian
Windows/GNOME asli. EXE unsigned, bukan installer Inno Setup.

Publikasi dilakukan dengan upload langsung yang diautentikasi. Workflow
[Publish RelayDesk preview](https://github.com/firefuck/connector-for-agent/actions/workflows/release.yml)
tersedia, tetapi [run pertama](https://github.com/firefuck/connector-for-agent/actions/runs/42474649905)
tidak dimulai karena pembatasan billing akun. Keberhasilan upload langsung tidak
membuktikan masalah billing Actions sudah selesai.

## Publikasi versi berikutnya

Jangan rerun workflow preview untuk mengganti aset versi yang sudah published.
Workflow sengaja menolak penggantian tersebut. Siapkan versi dan tag baru terlebih
dahulu: perbarui versi aplikasi, nama aset/tag dalam bundler dan workflow,
quickstart, catatan rilis, serta tautan README.

Untuk Actions, pemilik perlu menyelesaikan pembatasan billing sebelum menjalankan
workflow baru. Token bawaan `GITHUB_TOKEN` digunakan dengan izin Contents write.
Alternatifnya, build dan upload dari komputer pemilik yang memiliki autentikasi
GitHub dengan izin Contents write pada repository ini. Simpan credential lewat
secret environment atau fasilitas autentikasi lokal, bukan source/chat.

Dari checkout bersih commit yang ingin dirilis, di Linux:

```sh
go test -race ./...
go vet ./...
sh packaging/build.sh
python3 packaging/prepare-release.py
cd dist/release
sha256sum -c SHA256SUMS
```

`dist/release/` berisi seluruh aset; `dist/release-notes.md` berisi catatan rilis.
Buat draft untuk tag baru di GitHub Releases dengan commit sumber yang tercantum
di `BUILDINFO.txt`, salin catatan rilis, dan unggah seluruh aset. Pertahankan opsi
prerelease selama acceptance desktop asli belum selesai. Periksa nama/ukuran
serta checksum hasil unduhan, baru publish. Setelah published, periksa link
unduhan publik dan perbarui README, STATUS serta handoff.

Jangan mengganti aset Release yang sudah dipublikasikan dengan build berbeda.
Panduan pemakaian dan acceptance desktop: [MANUAL_ACCEPTANCE.md](MANUAL_ACCEPTANCE.md).
