Paket siap unduh untuk mencoba RelayDesk — tidak perlu membangun source sendiri.

### Perubahan sejak preview 1

- Lease tidak lagi terpotong menjadi sekitar 2 menit setelah satu aksi; tiap aksi memperbarui lease ke 5 menit.
- Linux X11: `desktop_key` sekarang mengirim F1–F24, Insert, Print dan sejenisnya; nama tombol tak dikenal menjadi error, bukan sukses palsu.
- Marker Stop hanya ditulis oleh Stop lokal eksplisit. Logout, display hilang atau SIGTERM tidak lagi mematikan autostart berikutnya.
- Gateway: setelah 30 autentikasi gagal per menit dari satu alamat IP, request ber-credential dari alamat itu dijawab HTTP 429. Audit log dirotasi pada 8 MiB. Kode pairing kedaluwarsa dibersihkan.
- Wizard menunggu Enter setelah error supaya pesan terbaca. DEB menyertakan contoh unit systemd gateway di `/usr/share/relaydesk/relaydesk-gateway.service` (tidak diaktifkan otomatis).
- Windows, **belum dijalankan di Windows asli**: `browser_open` yang gagal menjadi error; logoff/shutdown tidak dicatat sebagai Stop.

Versi aplikasi dan paket DEB tetap `0.1.0`. Jika preview 1 sudah terpasang, gunakan `sudo apt install --reinstall ./relaydesk_0.1.0_amd64.deb`. Di Windows, tutup connector lalu ganti `relaydesk.exe`. Pairing tetap tersimpan.

| Kebutuhan | File yang diunduh |
| --- | --- |
| Windows x64, langsung jalankan | **relaydesk.exe** — EXE portable |
| Windows x64, paket lengkap + script pemasangan | **RelayDesk-0.1.0-windows-amd64.zip** |
| Ubuntu/Debian amd64 | **relaydesk_0.1.0_amd64.deb** |
| Gateway/Linux portable | **relaydesk-linux-amd64** |

### Windows

Buka `relaydesk.exe`, isi URL gateway dan pairing code dari dashboard. Window kontrol mulai Pause; klik Resume untuk mengizinkan agent bekerja. EXE ini **portable dan unsigned**, bukan installer wizard Setup.exe.

Untuk paket lengkap, ekstrak ZIP. Binary ada di `dist/relaydesk.exe`. Opsi pemasangan per-user/autostart:

```powershell
powershell -NoProfile -File .\packaging\install-windows.ps1 -Persistent
```

### Ubuntu / Debian

Dari direktori unduhan:

```sh
sudo apt install ./relaydesk_0.1.0_amd64.deb   # upgrade dari preview 1: tambahkan --reinstall
relaydesk
```

Jalankan `relaydesk` sebagai user desktop, tanpa sudo. Pada GNOME, pastikan `xdg-desktop-portal-gnome` terpasang. Klik Resume lalu setujui dialog berbagi satu monitor dan input. Gateway homelab harus sudah berjalan sesuai README; token agent dan pairing code memiliki fungsi berbeda.

### Verifikasi dan status

- Source commit: `@COMMIT@`.
- `BUILDINFO.txt`: versi toolchain dan target build. `SHA256SUMS`: checksum seluruh aset. Checksum **bukan tanda tangan penerbit**.
- ZIP menyertakan notice dan panduan. Penggunaan mengikuti lisensi proprietary; hak komponen pihak ketiga tetap berlaku.
- Build Linux/Windows/DEB, race tests, vet dan instalasi DEB terisolasi lulus. Untuk preview 2 dijalankan ulang: tes X11 dan GTK di Xvfb, portal tiruan dan dashboard Chromium. SDK MCP, Docker dan dependency install Ubuntu 22.04 terakhir diuji pada preview 1.
- Ini **preview**, bukan klaim siap produksi. GNOME dan Windows asli serta dua agent pengguna masih memerlukan acceptance manual. Takeover otomatis GNOME tidak tersedia; gunakan Pause/shortcut. UAC secure desktop dan RDP disconnected belum didukung.

[Panduan lengkap setup dan pengujian manual](https://github.com/firefuck/connector-for-agent/blob/@COMMIT@/docs/MANUAL_ACCEPTANCE.md) · [README](https://github.com/firefuck/connector-for-agent/blob/@COMMIT@/README.md)
