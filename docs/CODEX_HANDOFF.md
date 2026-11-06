# Brief untuk Codex

User ingin aplikasi gateway self-hosted dengan flow seperti TeamViewer agar agent buatannya dan Hermes mengontrol seluruh PC Windows/Linux dari HP. Agent tetap satu di VPS dan satu di homelab. MikroTik CHR sudah menghubungkan PC/VPS melalui WireGuard. Semua target memakai desktop GUI. Gateway akan berjalan di homelab. User tidak ingin Context.dev dipakai. Jangan minta user menjelaskan ulang arsitektur.

## Tujuan produk

Install connector sekali → pairing ID/kode → kedua agent memakai gateway MCP → pilih PC → kontrol browser, VS Code, terminal, file, pengaturan OS. Mode persisten/sementara. Identitas perangkat stabil. Gateway memusatkan izin dan konflik sesi. Tidak menjanjikan kontrol 100% ketika OS mati, GUI terkunci, atau privilege tidak tersedia.

## Implementasi yang sudah ada

Go standard library saja. `cmd/relaydesk/main.go` punya gateway, wizard pairing, connector, admin CLI, stdio MCP bridge. `internal/core` punya auth bearer hashed, pairing single use, allowlist per-device, leases, queue polling, revocation, audit, dashboard web, MCP HTTP, file/command tools, desktop backend Windows PowerShell/.NET dan Linux X11 (xdotool/ImageMagick/xclip).

Paket binary Windows amd64, Linux amd64 dan DEB dibuat lewat packaging/build.sh. Windows binary portable + installer script; belum installer GUI. Container build definition dan Compose ada.

## Kerjakan berikutnya, jangan mengklaim sudah selesai

1. Uji backend Windows di VM asli, input Unicode/chord/multimonitor, screenshot origin, default browser, VS Code; uji Linux X11 asli. Tambahkan tes black-box GUI.
2. Uji remote MCP dan stdio dengan agent user serta Hermes; validasi lifecycle/schema secara menyeluruh menggunakan SDK/client resmi.
3. Native connector UI/tray, stop/pause, sesi aktif, wizard kredensial, installer EXE/MSI bertanda tangan. DEB integration desktop dan uninstall bersih.
4. Windows user-session helper + service broker least privilege; tentukan cakupan UAC/lock/login/RDP disconnected. Jangan menonaktifkan UAC atau auto-login secara diam-diam.
5. Implementasi Wayland melalui portal/compositor yang didukung, dengan consent yang sesuai. Saat ini GUI Wayland sengaja ditolak.
6. WebSocket/mTLS/device keypair dan short-lived agent auth. Saat ini token bearer individual dengan hashed storage di gateway; bukan keypair atau OAuth.
7. PTY interaktif, streaming output, file transfer chunked, CDP browser integration, monitor enumeration, drag, screenshot metadata lengkap, capability negotiation per tool.
8. Per-tool permissions, audit rotation, rate limits, durable jobs/idempotency, cancellation notification, graceful shutdown/upgrade, takeover lokal, telemetry health.
9. Jalankan Docker build/integration test; review pinned base images dan dependency upgrades sebelum deployment.

## Constraints

- Jangan deploy ke homelab/user machine tanpa koneksi/target yang diberikan.
- Jangan masukkan credential nyata ke Git.
- HTTP hanya untuk rute WireGuard yang diverifikasi; alternatif HTTPS built-in.
- Status testing ada di STATUS.md. Lanjutkan dari source ini, jangan mulai ulang.
