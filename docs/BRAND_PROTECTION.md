# Identitas, lisensi dan distribusi RelayDesk

RelayDesk memakai lisensi proprietary pada material asli proyek. LICENSE
mewajibkan izin tertulis untuk penggunaan, modifikasi, distribusi dan rebranding
pihak lain di luar hak yang sudah berlaku. NOTICE mencantumkan identitas dan
repository asal. Hak komponen pihak ketiga tetap mengikuti lisensinya sendiri;
lihat THIRD_PARTY_NOTICES.md. Ini tidak mengklaim merek sudah terdaftar.

## Yang diterapkan

- Nama, ID dan versi ada di `identity.go`, dikompilasi ke CLI, dashboard, health
  dan MCP. Tidak ada flag/env untuk white-label. IP, URL, token dan path deployment
  tetap dikonfigurasi operator; credential/private key tidak ditanam di binary.
- `relaydesk license` menampilkan notice dan lisensi tertanam, termasuk pada
  binary portable yang dipindahkan tanpa source.
- Build menyertakan LICENSE, NOTICE dan THIRD_PARTY_NOTICES.md dalam `dist/`, DEB
  dan image Docker. Installer Windows menyalin notice dari `dist/` dan menghapus
  hanya file aplikasi yang dikenal ketika uninstall. Script Windows belum diuji
  di Windows asli.
- `packaging/check-branding.sh`, dipanggil build, memeriksa identitas CLI, teks
  legal tertanam, notice portable, metadata DEB, desktop entry dan binary paket.
  Tujuannya mendeteksi ketidakkonsistenan distribusi yang tidak sengaja.
- `dist/SHA256SUMS` mencakup dua binary, DEB dan notice dari build tersebut.
  Manifest dibuat hanya setelah build dan pemeriksaan berhasil.

## Batas perlindungan

Repository tetap **public**; visibility tidak diubah oleh pekerjaan ini.
Pembaca source secara teknis dapat menyalin, mengubah konstanta, menghapus
pemeriksaan build, atau membangun ulang dengan nama lain. Lisensi membatasi izin
hukum, bukan membuat tindakan tersebut mustahil. Hak yang sudah diberikan oleh
lisensi sebelumnya, perjanjian, hukum atau ketentuan platform tidak ditarik
secara retroaktif. Salinan yang sudah tersebar tidak hilang dengan mengganti
visibility repository.

Checksum tidak ditandatangani: jika penyerang mengganti binary dan manifest,
checksum tetap bisa cocok. Belum ada Authenticode, tanda tangan rilis, verifikasi
publisher saat install, atau layanan aktivasi. Tidak ada kill switch atau
pengiriman credential ke server lisensi.

Untuk membatasi akses source berikutnya, pemilik dapat menjadikan repository
private dan memberikan akses terbatas. Itu merupakan perubahan pengaturan repo
tersendiri yang belum dilakukan. Untuk distribusi publik, langkah berikutnya
adalah penandatanganan rilis/installer dengan identitas pemilik yang diverifikasi
serta private key di penyimpanan rahasia di luar repo. Pendaftaran merek dan
penegakan lisensi memerlukan proses tersendiri; bukan fitur software ini.
