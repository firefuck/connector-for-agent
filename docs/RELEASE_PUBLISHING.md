# Publikasi paket RelayDesk

## Status 8 November 2026

Paket Windows x64 portable EXE, ZIP lengkap, Linux amd64 dan DEB sudah dibangun
di workspace pengembangan. Checksum dan isi ZIP diperiksa; build/race suite/vet
serta instalasi DEB terisolasi lulus. Ini bukan pengujian desktop Windows/GNOME
asli. EXE unsigned, bukan installer Inno Setup.

Release `v0.1.0-preview.1` masih **draft tanpa aset** saat pemeriksaan terakhir.
Unggah langsung dari sesi pengembangan ditolak dengan HTTP 401. Workflow
[Publish RelayDesk preview](https://github.com/firefuck/connector-for-agent/actions/workflows/release.yml)
sudah dipush, tetapi [run pertama](https://github.com/firefuck/connector-for-agent/actions/runs/42474649905)
gagal sebelum runner dimulai dengan pesan GitHub:

> The job was not started because your account is locked due to a billing issue.

## Melanjutkan lewat GitHub Actions

1. Pemilik akun menyelesaikan pembatasan billing melalui pengaturan GitHub.
2. Buka workflow **Publish RelayDesk preview**, pilih **Run workflow**, branch
   `main`. Gunakan run baru agar mengambil dokumentasi terbaru.
3. Workflow menjalankan race suite/vet, membangun EXE/DEB, menyiapkan ZIP/notices
   dan checksum, lalu mengunggah aset ke draft. Token bawaan `GITHUB_TOKEN`
   digunakan; tidak perlu menaruh PAT di source atau chat.
4. Workflow memeriksa state dan ukuran aset, serta digest jika tersedia, sebelum
   menerbitkan prerelease. Jika gagal, periksa langkah yang gagal; jangan
   menerbitkan draft kosong secara manual.
5. Buka [Release](https://github.com/firefuck/connector-for-agent/releases/tag/v0.1.0-preview.1)
   dan pastikan EXE, ZIP, DEB, Linux binary, legal notices, `QUICKSTART.txt`,
   `BUILDINFO.txt`, dan `SHA256SUMS` tersedia. Unduh aset lalu cocokkan checksum.
6. Setelah berhasil, perbarui status publikasi di README, STATUS dan handoff.

Workflow menolak mengganti Release yang sudah published. Versi baru memerlukan
tag, versi aplikasi, nama aset dan tautan dokumentasi yang sesuai.

## Alternatif unggah dari komputer pemilik

Jika Actions masih dibatasi, pemilik dapat memakai komputer dengan Go dan dpkg
serta autentikasi GitHub yang memiliki izin upload Release. Dari checkout bersih
commit yang ingin dirilis, di Linux:

```sh
go test -race ./...
go vet ./...
sh packaging/build.sh
python3 packaging/prepare-release.py
cd dist/release
sha256sum -c SHA256SUMS
```

`dist/release/` berisi seluruh aset; `dist/release-notes.md` berisi catatan rilis.
Di GitHub Releases, edit draft `v0.1.0-preview.1`, gunakan commit sumber yang
tercantum di `BUILDINFO.txt`, salin catatan rilis, dan unggah seluruh aset.
Pertahankan opsi prerelease. Pastikan tidak ada aset versi sebelumnya, periksa
nama/ukuran dan checksum hasil unduhan, baru publish. Jangan mengganti aset
Release yang sudah dipublikasikan dengan build berbeda.

Panduan pemakaian dan acceptance desktop: [MANUAL_ACCEPTANCE.md](MANUAL_ACCEPTANCE.md).
