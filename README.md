# RelayDesk 0.1 — paket awal untuk dilanjutkan di Codex

Gateway MCP di homelab + connector Windows/Linux untuk dua agent yang sudah ada.
Tidak memerlukan pemasangan agent/model di komputer tujuan. Koneksi melalui jaringan WireGuard milik pengguna. Nama proyek sementara: RelayDesk.

**Status:** implementasi awal yang dapat dibangun dan diuji, bukan rilis produksi atau kontrol 100% yang sudah tervalidasi. Windows EXE adalah executable portable dengan wizard console, bukan installer GUI bertanda tangan. Paket DEB tersedia untuk Linux amd64/X11. Dockerfile dan Compose disediakan; image Docker belum dibangun/diuji di lingkungan pembuatan ini. Baca `docs/STATUS.md` sebelum memakai.

## Isi paket

- `dist/relaydesk.exe`: executable Windows x64, wizard pairing jika dibuka tanpa argumen.
- `dist/relaydesk_0.1.0_amd64.deb`: paket Debian/Ubuntu x64.
- `dist/relaydesk-linux-amd64`: binary Linux x64; bisa menjadi gateway, connector, atau MCP bridge.
- `Dockerfile`, `compose.yaml`: gateway container.
- `packaging/install-windows.ps1`: pemasangan per-user dan autostart opsional.
- `packaging/autostart-linux.sh`: aktif/nonaktif autostart saat desktop login.
- `cmd/`, `internal/`: source Go, hanya standard library, tanpa library runtime tambahan.
- `docs/CODEX_HANDOFF.md`: brief dan pekerjaan lanjutan untuk Codex.

## 1. Gateway di homelab

Ekstrak seluruh paket. Pilih IP homelab yang bisa dicapai melalui routing WireGuard CHR. Contoh `10.20.0.10` hanya placeholder; ganti dengan IP homelab lu.

```sh
chmod +x dist/relaydesk-linux-amd64
cp .env.example .env
# Edit .env: masukkan IP bind gateway dan token dari perintah berikut
./dist/relaydesk-linux-amd64 token
# Setelah .env diisi:
docker compose up -d --build
```

Buka `http://IP-HOMELAB:8787` lewat jaringan WireGuard. Login dengan admin token di `.env`. Jangan memakai token admin sebagai token agent. CHR/firewall harus mengizinkan agent dan PC menjangkau IP ini pada TCP 8787. Container default hanya dipublikasikan pada localhost sampai `RELAYDESK_BIND_IP` diganti.

Tanpa Docker:

```sh
export RELAYDESK_ADMIN_TOKEN="$(./dist/relaydesk-linux-amd64 token)"
# Simpan nilai ini di password manager sebelum menutup shell.
./dist/relaydesk-linux-amd64 gateway --listen 10.20.0.10:8787 --state ./data/state.json
```

HTTP hanya diperbolehkan secara eksplisit untuk jalur WireGuard terpercaya. Untuk jalur lain gunakan HTTPS: gateway mendukung `--tls-cert cert.pem --tls-key key.pem`; connector memverifikasi CA sistem dan tidak punya opsi insecure TLS. HTTP tidak membuktikan bahwa rute benar-benar melewati WireGuard: operator wajib memastikan routing/firewall. Jangan mempublikasikan port ini ke internet.

## 2. Tambahkan PC Windows

Di dashboard, buat kode pairing dengan nama perangkat. Di PC tujuan:

1. Buka `dist/relaydesk.exe`.
2. Isi URL gateway, kode pairing, dan nama perangkat.
3. Konfirmasi penggunaan WireGuard jika URL memakai HTTP.
4. Biarkan window connector terbuka. Ctrl+C memutus koneksi.

Untuk menyalin aplikasi ke LocalAppData dan mengaktifkan startup setelah login:

```powershell
powershell -NoProfile -File .\packaging\install-windows.ps1 -Persistent
```

Jika kebijakan PowerShell organisasi menolak script, gunakan executable portable atau ikuti kebijakan lokal. Script tidak menonaktifkan kebijakan tersebut. Hapus autostart dengan `-Uninstall`, lalu tutup connector. Jangan jalankan dua connector dari konfigurasi perangkat yang sama bersamaan.

Connector memakai PowerShell/.NET bawaan untuk input dan screenshot. Jalankan sebagai user desktop yang sedang login. Hak admin hanya tersedia jika proses memang diluncurkan dengan hak admin; tidak ada bypass UAC. RDP disconnected/locked, secure desktop UAC, dan login screen belum didukung/diuji.

## 3. Tambahkan PC Linux

```sh
sudo apt install ./dist/relaydesk_0.1.0_amd64.deb
relaydesk
```

Jalankan dari sesi desktop **X11/Xorg** sebagai user biasa. Paket memasang xdotool, ImageMagick, xclip, dan xdg-utils. Wayland ditolak untuk kontrol GUI dengan pesan eksplisit; shell dan file tetap berfungsi. Pilih sesi Xorg pada login jika tersedia, atau lanjutkan implementasi Wayland di Codex.

Autostart opsional:

```sh
sh /usr/share/relaydesk/autostart-linux.sh enable
# Mematikan:
sh /usr/share/relaydesk/autostart-linux.sh disable
```

## 4. Hubungkan kedua agent

Di dashboard buat dua akses: `agent-vps` dan `hermes`. Isi daftar ID PC yang diizinkan (dipisah koma), atau `*` untuk semua perangkat sekarang dan yang akan datang. Token hanya ditampilkan saat dibuat. Setiap token memberi akses penuh tool pada PC dalam cakupannya, setara hak OS connector; belum ada pembatasan per-tool.

### Remote MCP

- URL: `http://IP-HOMELAB:8787/mcp` melalui WireGuard.
- Header: `Authorization: Bearer TOKEN_AGENT`.
- Transport: Streamable HTTP, respons JSON stateless, versi protokol 2025-03-26.
- Server tidak menawarkan SSE GET; mengembalikan HTTP 405 sesuai mode JSON-only.

Gunakan UI/config MCP agent masing-masing untuk URL dan header. Bentuk konfigurasi remote berbeda antar-host; belum diuji langsung dengan Hermes.

### MCP stdio bridge (opsional)

Salin binary saja ke host agent, tanpa memasang connector atau model. Contoh struktur konfigurasi MCP generik:

```json
{
  "mcpServers": {
    "relaydesk": {
      "command": "/opt/relaydesk/relaydesk-linux-amd64",
      "args": ["mcp", "--gateway", "http://IP-HOMELAB:8787", "--wireguard-http"],
      "env": {"RELAYDESK_AGENT_TOKEN": "TOKEN_AGENT_KHUSUS"}
    }
  }
}
```

Tool: `devices_list`, `session_acquire`, `session_release`, `system_info`, `desktop_screenshot`, `desktop_move`, `desktop_click`, `desktop_type`, `desktop_key`, `desktop_scroll`, `browser_open`, `system_exec`, `files_read`, `files_write`, `files_list`.

Instruksi agent: pilih ID perangkat dari `devices_list`; ambil lease dengan `session_acquire`; lakukan tindakan; periksa screenshot/output; `session_release` setelah selesai. Jangan mengulang tindakan timeout tanpa mengecek hasilnya, karena tindakan mungkin sudah terlaksana.

Contoh: acquire → browser_open → screenshot → click/type/key → screenshot → release. VS Code dibuka via `system_exec` (`code <project>`) atau GUI; file dapat diedit langsung melalui file tools. Browser saat ini dikontrol melalui GUI; integrasi CDP/DOM belum tersedia.

## Persisten dan sementara

- Persisten: pairing tersimpan, connector retry otomatis, gunakan autostart setelah login.
- Sementara: `relaydesk connector --duration 1h`; proses berhenti setelah 1 jam. Identitas pairing tetap tersimpan. Ini bukan token satu jam; PC tersedia kembali bila connector dijalankan lagi.
- Putus lokal: Ctrl+C/tutup window. Cabut permanen: dashboard → Cabut akses. Pair ulang memakai konfigurasi baru.
- Connector melakukan polling outbound setiap 1 detik. Ini belum WebSocket/video streaming.
- Reboot: autostart desktop menunggu user login. Tidak ada login otomatis atau akses sebelum OS hidup.

## CLI admin dan pairing

```sh
export RELAYDESK_GATEWAY=http://IP-HOMELAB:8787
export RELAYDESK_WIREGUARD_HTTP=1
export RELAYDESK_ADMIN_TOKEN=TOKEN_ADMIN
relaydesk admin --op list
relaydesk admin --op pair --name pc-rumah
relaydesk admin --op agent --name hermes --devices ID_PC
relaydesk admin --op revoke --kind agent --id ID_AGENT
# Di PC: env kode pairing menghindari kode dalam daftar argumen proses
export RELAYDESK_PAIR_CODE=KODE_DARI_DASHBOARD
relaydesk pair --gateway "$RELAYDESK_GATEWAY" --wireguard-http --name pc-rumah
relaydesk connector --duration 1h
```

## Data dan batas operasi

State gateway ada di volume `/data` (native: `data/state.json`), berisi hash token dan daftar perangkat. Cadangkan state dan admin token. Audit JSONL mencatat actor, device, action, waktu tanpa isi perintah/file. Belum ada rotasi audit.

Kredensial connector disimpan lokal: Linux `~/.config/relaydesk/connector.json`; Windows di direktori `os.UserConfigDir()` (`%AppData%\relaydesk\connector.json`). File harus hanya dapat dibaca user pemilik. Linux memakai mode 0600; Windows mengikuti ACL profil user. Belum DPAPI/keychain/mTLS.

Satu aksi in-flight per PC. Lease default 5 menit, diperpanjang saat aksi. Batas eksekusi 1–300 detik; output terminal 1 MiB, file 4 MiB, screenshot 8 MiB Linux. Terminal adalah command noninteraktif, bukan sesi PTY persisten. Revocation menghentikan dispatch baru dan membatalkan pekerjaan aktif ketika connector mengecek status (sekitar 1–4 detik); tindakan yang sudah selesai tidak di-undo.

## Build dan test

Go 1.24+, dpkg-deb untuk DEB:

```sh
go test -race ./...
./packaging/build.sh
```

Sumber transport MCP: https://modelcontextprotocol.io/specification/2025-03-26/basic/transports
