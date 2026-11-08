# Status verifikasi RelayDesk 0.1

Diperbarui 2026-11-08. Ini belum rilis produksi atau bukti seluruh OS/agent asli
sudah lolos acceptance. Pengguna menetapkan Ubuntu 22 (baseline 22.04) dan memilih
menguji Windows 11 VPS sendiri. [Panduan manual](MANUAL_ACCEPTANCE.md) memuat setup,
langkah uji, hasil yang diharapkan, installer/signing dan pelaporan tanpa credential.

## Fitur desktop aktif yang sekarang diimplementasikan

| Area | Implementasi | Batas bukti |
| --- | --- | --- |
| GNOME Wayland | Helper embedded Python GI/GTK3, RemoteDesktop + ScreenCast portal, PipeWire/GStreamer, Notify* kompatibel baseline GNOME 42 | Uji Gio/fake portal dan GStreamer sumber uji; belum consent/input GNOME asli |
| Capture/input | Satu monitor yang dipilih user, PNG, dimensi/scale/session/frame metadata, koordinat pixel/logical, Unicode keysym/chord, cancel, revoke/lock signal | Mixed DPI, hotplug, aplikasi GNOME dan clipboard/IME nyata masih acceptance manual |
| Kontrol lokal | Window GTK/WinForms, awal Pause, Resume eksplisit, Stop/offline Stop, indikator agent/perangkat dan kelas aktivitas | GTK nyata diuji di Xvfb; WinForms/hotkey/hook belum dijalankan di Windows |
| Pause dan lease | Cancel context shell/file/GUI, epoch lokal membuang job lama dan lease; Resume perlu acquire baru | Efek yang sudah terjadi tidak di-undo; full-shell bukan sandbox |
| Stop/autostart | Marker privat disimpan sebelum penghentian; autostart menghormati Stop; pairing dipertahankan | Login/autostart desktop asli masih manual |
| Shortcut/takeover | GNOME custom shortcut eksplisit + Unix IPC privat, shortcut window; Windows Ctrl+Alt+P dan low-level physical-input hooks + session notifications | Takeover otomatis GNOME tidak tersedia; Windows injection flags/driver harus diuji |
| Frame guard | Screenshot baru sebelum tiap input pada connector interaktif, umur 30 detik, sekali pakai, terikat agent, lease dan generasi | `--headless` adalah mode tes tanpa guard/window, ditolak pada Wayland |
| Doctor | Teks/JSON schema 1, timeout, config regular/bounded, endpoint auth read-only, status dependency/consent, tanpa raw URL/token/error | Tidak melakukan capture/input; unknown/needs_consent bukan bukti GUI gagal atau siap |
| Installer Windows | Source Inno Setup per-user, build/sign script, validasi signature saat sertifikat diberikan | Belum dibangun/dipasang di Windows; sertifikat belum tersedia, tidak ada klaim signed artifact |

Tidak menambah model/AI pada PC, tidak mengubah dua agent, WireGuard/CHR atau
kredensial produksi. Pairing pertama masih wizard console, kemudian window kontrol
lokal. Gateway tetap Go standard library; helper runtime desktop memakai paket OS.

## Pengujian yang dijalankan untuk perubahan ini

- `gofmt`, `go vet`, `go test -race ./... -count=1`, termasuk Xvfb/Tk. Regresi
  mencakup Pause/Resume/Stop, pembatalan job/lease dan no replay, frame invalidation,
  endpoint doctor tanpa side effect, redaksi error dan config bounded.
- Build Linux amd64, Windows amd64 (cross-compile), DEB, pengecekan identitas/legal
  notices dan checksum. Build Windows bukan bukti PowerShell/GUI/installer berjalan.
- GTK window nyata pada Xvfb: tombol Resume, screenshot melalui MCP dengan frame
  metadata, satu input berhasil, reuse frame ditolak; Pause CLI membunuh shell,
  gateway tersinkron, Stop saat gateway mati, config tetap ada dan autostart
  menghormati marker Stop.
- Tes portal melalui Gio D-Bus pada session bus privat dengan service portal
  tiruan: response accept/deny/cancel dan request handle. Ini bukan consent GNOME.
- GStreamer nyata dengan `videotestsrc`, PNG encoder dan appsink; fake input bus
  memeriksa scaling 2x, bounds, Unicode keysyms, urutan chord/release serta cancel.
  Ini bukan frame dari PipeWire desktop atau injeksi ke compositor asli.
- SDK MCP Python resmi 1.30.0/jsonschema 4.26.0: HTTP dan stdio, 15 schema tool,
  discovery/lease/file/exec/release dan izin. Target localhost, bukan dua agent asli.
- Browser Chromium: dashboard drawer desktop/390/320 px, auth/permission/revoke,
  XSS literal, focus trap, Escape/backdrop, draft, pending mutation dan secret cleanup.
- Dpkg install/remove di root sementara dengan dependency dilewati; apt install/
  remove lengkap di **container Ubuntu 22.04** dengan dependency GTK/GStreamer/
  PipeWire dan pengecekan element runtime. Container tidak menjalankan GNOME login.

- Docker build/runtime gateway lulus: non-root/read-only, Compose policy, auth,
  pairing, hashed state, restart, revoke dan SIGTERM dengan job pending; legal
  notices terbaca oleh user runtime.
- `govulncheck` 1.9.0 (Go 1.27.3), source Linux dan target Windows: tidak ada
  kerentanan yang dilaporkan. Ini tidak memindai semua paket OS/Python/GTK/GStreamer.

Tes lokal menemukan dan memperbaiki dua race/perilaku: hasil shell yang dibatalkan
semula belum `isError`; dan Stop dapat keluar sebelum marker tersimpan/ACK IPC
terkirim. Tes black-box diulang hingga lulus. Detail tambahan di test-results.txt.

## Publikasi paket

[Prerelease v0.1.0-preview.1](https://github.com/firefuck/connector-for-agent/releases/tag/v0.1.0-preview.1)
sudah diterbitkan pada 8 November 2026 dengan 10 aset: EXE portable, ZIP Windows,
DEB, binary Linux, legal notices, quickstart, build info dan checksum. Source
paket: `f054c36af62e1e7d42d561c90d793d07688cffe7`. Semua aset diunduh ulang dan
SHA-256 cocok. Link publik EXE/DEB/ZIP/checksum juga diperiksa tanpa token.
Publikasi langsung berhasil; Actions masih terblokir billing. EXE unsigned.
Lihat [catatan publikasi](RELEASE_PUBLISHING.md).

## Yang belum dapat dinyatakan selesai

- Acceptance GNOME Ubuntu 22.04 asli: consent accept/deny/revoke, lock/unlock,
  monitor/scaling/hotplug, Unicode/IME, browser dan VS Code. Host asli belum diakses.
- Windows 11/10/Server: WinForms, physical takeover versus injected input,
  shortcut/session notification, DPI/multimonitor, UAC manual, installer/uninstall
  dan signing. Pengguna menjalankan uji manual; tidak ada sertifikat publisher di sini.
- Agent VPS dan Hermes asli belum dihubungkan. SDK lokal tidak menggantikan uji host.
- Takeover otomatis GNOME tidak disediakan portal; fallback Pause/shortcut tersedia.
  Tidak ada libei backend, portal persistence/restore token atau multi-stream sharing.
- Kontrol lock/login/UAC secure desktop dan RDP disconnected tetap di luar target
  desktop aktif. Tidak ada privileged service broker atau auto-login.
- mTLS/device keypair, short-lived auth/OAuth, passkey/2FA, DPAPI/keychain,
  WebSocket/video/PTY/output streaming, chunked transfer, DOM/CDP, drag,
  monitor enumeration, failover, rate limits, audit rotation, durable jobs dan
  upgrade orchestration tetap backlog lanjutan.

## Fitur dan audit sebelumnya yang dipertahankan

Dashboard cream/terracotta dengan slide drawer. Bearer hashed, ACL device/tool,
exclusive leases, bounded bodies/file tools, CSP, private atomic config, redaksi,
cancellation HTTP/MCP/stdio dan shutdown, tanpa replay. Linux X11 memakai utility
OS; Windows user-session PowerShell/.NET; tidak otomatis mengangkat privilege.

Refactor models/auth/http/admin/connector/dispatch dan limits tetap dipakai.
Identitas produk di identity.go; LICENSE proprietary, NOTICE, third-party notices,
`relaydesk license`, pemeriksaan branding dan checksum. Repo tetap public; editor
source bisa mengubah pemeriksaan/branding. Lisensi membatasi izin hukum, bukan
anti-copy absolut. Checksum unsigned tidak membuktikan publisher.

Audit/scan Go pada 2026-11-07 sebelumnya tidak menemukan kerentanan terlapor;
lihat SECURITY_REVIEW.md dan test-results.txt untuk versi/tool/batas. Jangan
menganggap hasil historis sebagai scan CVE terbaru semua package OS/GTK/GStreamer.
