# Status verifikasi 0.1

## Berhasil dilakukan di lingkungan build Linux

- gofmt, go vet ./..., go test -race ./... (6 tes perilaku).
- Integrasi HTTP nyata: admin membuat pairing, connector redeem, agent memperoleh lease, write/read file Unicode, system_exec, release.
- Pairing code sekali pakai; token admin salah ditolak; allowlist device; konflik lease dua agent; token hanya hash dalam state; identitas survive reload.
- Pencabutan device membatalkan pending job dan menolak polling berikutnya.
- Timeout membunuh subprocess/process group Linux.
- Validasi tipe argumen dan required fields tool.
- Cross-compile Windows x64 PE32+ executable dan Linux x64 static ELF.
- Build paket DEB dan inspeksi metadata/arsitektur.
- Jalankan binary Linux `version`.

## Belum diverifikasi / belum diimplementasikan

- Windows binary belum dijalankan di Windows; PowerShell/GUI input/screenshot belum diuji.
- Linux GUI X11 belum diuji pada desktop nyata; environment build tidak memiliki X server.
- Docker tidak tersedia di environment ini: Dockerfile/Compose belum dibangun atau dijalankan.
- Tidak pernah terhubung ke PC, VPS, homelab, MikroTik atau agent user. Tidak ada deploy.
- Native tray GUI, installer EXE/MSI, code signing, passkey/2FA, per-tool permissions, mTLS/keypair, OAuth/short-lived tokens belum dibuat.
- Linux Wayland GUI tidak didukung. RDP disconnected, lock screen, UAC secure desktop, service privilege broker belum didukung.
- Tidak ada WebSocket, video streaming, PTY interaktif, integrasi DOM/CDP, drag-and-drop, ataupun failover gateway.
- Persisten berarti reconnect + startup setelah user login; tidak dapat mengontrol layar login atau menyalakan mesin mati.
- Sesi sementara menghentikan connector setelah durasi, tidak menghapus pairing.
- Admin dashboard memakai token, belum login akun/passkey/2FA.

Lanjutkan berdasarkan docs/CODEX_HANDOFF.md. Jangan menyatakan versi ini memenuhi seluruh rencana awal atau siap produksi.
