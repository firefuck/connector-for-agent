# Status verifikasi RelayDesk 0.1

Diperbarui 2026-11-07. Hasil uji awal berasal dari 2026-11-06; audit GUI/keamanan dan regresi tambahan dijalankan 2026-11-07. Source dilanjutkan dari handoff, bukan implementasi ulang. Ini belum rilis produksi dan belum memenuhi seluruh backlog.

## Audit GUI dan keamanan 2026-11-07

Dashboard web kini memakai ringkasan singkat dan slide drawer terpisah untuk Perangkat, Agent, Tambah PC dan Tambah agent. Palet diganti dari navy ke krem/putih hangat dengan aksen terracotta. Pilihan PC lewat checkbox, preset default melihat layar, salin hasil, status koneksi/lease dan penguncian tetap tersedia. Ini bukan native UI connector; fitur desktop dalam backlog di bawah belum berubah.

Regresi drawer lulus di Chromium desktop serta lebar 390/320 px: focus trap Tab/Shift+Tab, Escape/backdrop, pengembalian fokus ke menu, preservasi draft, blok tutup selama mutasi berjalan, lock saat request pending, pembersihan kode/token ketika hasil ditutup, dan reduced motion. Tes awal menemukan Tab dapat keluar dari drawer; penanganan keyboard ditambahkan dan pengujian ulang lulus. Race suite, vet dan build Linux/Windows/DEB juga lulus setelah perubahan ini. Screenshot memakai gateway lokal dengan data contoh.

Perbaikan source: tidak menahan mutex saat menulis respons jaringan, auth connector sebelum membaca body, parsing tepat satu JSON dengan batas per endpoint, penyimpanan credential/state memakai temp privat acak, file tools menolak FIFO/device dan membatasi listing, policy eksplisit untuk agent baru, validasi hasil job, redaksi error client, CSP dashboard, dan validasi state saat reload. Rincian temuan, dampak dan batas ada di [SECURITY_REVIEW.md](SECURITY_REVIEW.md).

Lulus: race suite termasuk Xvfb/Tk, vet, build Linux/Windows/DEB, instalasi/remove DEB terisolasi, browser Chromium desktop/HP, serta SDK MCP resmi HTTP/stdio. `govulncheck` 1.9.0 pada source Linux dan target Windows (Go 1.27.3) tidak menemukan kerentanan yang dilaporkan. Ini bukan bukti seluruh OS/image bebas kerentanan atau dukungan GUI Windows/Wayland. Percobaan Xvfb pertama memerlukan perbaikan PATH ke dependensi lokal, kemudian lulus.

Docker build/smoke terbaru juga lulus setelah membersihkan cache build RelayDesk lama yang memenuhi disk. Scan Go binary dari image terbaru (Go 1.26.10) tidak menemukan kerentanan terlapor; paket Alpine/image OS belum dipindai. Kegagalan pemeriksaan sementara akibat disk penuh diulang hingga lulus.

## Kebutuhan desktop dan riset lanjutan 2026-11-07

User mengonfirmasi GNOME Wayland dan Windows Server/10/11. Fokus terbaru adalah desktop aktif, user login dan tidak terkunci, saat user bekerja. RDP disconnected/terkunci, unattended dan kontrol UAC otomatis ditunda; user dapat menyetujui UAC secara lokal. Dukungan X11 tidak memenuhi kebutuhan GNOME Wayland tersebut.

Riset sumber primer dan rancangan backend ada di [DESKTOP_BACKEND_PLAN.md](DESKTOP_BACKEND_PLAN.md), [RESEARCH_WAYLAND.md](RESEARCH_WAYLAND.md), [RESEARCH_WINDOWS_UAC.md](RESEARCH_WINDOWS_UAC.md), dan [RESEARCH_WINDOWS_RDP.md](RESEARCH_WINDOWS_RDP.md). Riset mencakup GNOME Remote Login persisten sejak GNOME 47, portal/PipeWire/libei untuk desktop aktif, broker/helper Windows per-session, batas capture RDP disconnected, dan hardening credential Windows 2026.

Tidak ada backend Wayland, privileged broker UAC, atau worker RDP baru yang diimplementasikan/diuji dari riset ini. Status belum didukung di bagian bawah tetap berlaku. Riset backend berupa dokumentasi; audit/perbaikan GUI dan keamanan pada tanggal yang sama tidak dianggap implementasi fitur backend tersebut.

## Tambahan yang disetujui, belum diimplementasikan

Urutan: GNOME Wayland aktif → Pause/Stop lokal, hotkey dan takeover manusia → indikator sesi lokal → CLI `relaydesk doctor`. Spesifikasi dan acceptance ada di [rencana backend desktop](DESKTOP_BACKEND_PLAN.md).

- Pause membatalkan job antre/aktif termasuk shell/file, Resume eksplisit tanpa replay, Stop lokal tidak bergantung pada gateway. Takeover otomatis memerlukan deteksi input manusia yang andal; backend yang tidak mendukung harus melaporkannya dan menyediakan kontrol lokal eksplisit.
- Indikator menampilkan agent/perangkat serta aktivitas melihat layar, mengirim input, shell/file, pause atau offline.
- Doctor melakukan diagnosis read-only pairing/gateway/backend/dependensi/izin, menyediakan teks/JSON dan meredaksi secret. Config terpasang tidak sama dengan pairing valid atau izin GUI siap.
- Screenshot baru sebelum aksi GUI penting dan pemeriksaan hasil menjadi alur yang direncanakan; enforcement frame/sesi belum tersedia. Larangan replay otomatis tetap berlaku pada implementasi yang ada.

Penambahan ini merupakan backlog/spesifikasi, belum fitur binary atau hasil pengujian baru.

## Implementasi yang ditambahkan

- Allowlist tool per agent: `*` untuk semua, daftar eksplisit untuk pembatasan, `[]` menolak semua. Create API kini mewajibkan policy eksplisit; state lama tanpa policy/null tetap unrestricted sampai admin mengubahnya. Dashboard dan CLI dapat membuat/mengubah policy; perubahan membatalkan job dan lease milik agent itu. Policy tidak mengubah allowlist perangkat. `system_exec` tetap shell penuh sebagai user connector, bukan sandbox.
- `tools/list` memfilter tool sesuai policy; setiap call tetap memverifikasi izin walaupun client memakai schema lama.
- MCP: error JSON-RPC untuk input malformed/invalid tool arguments, pemeriksaan versi header, ID aktif duplikat ditolak, maksimum 256 request aktif. Cancellation notification dibatasi token agent pemilik request; HTTP disconnect membatalkan pekerjaan. Ini bukan rate limiter atau durable job store.
- Bridge stdio konkuren: maksimum 32 request dan 16 notification aktif, timeout notification 10 detik. Ping/cancellation tidak menunggu tool atau notification selesai. EOF/SIGTERM/output error menghentikan pekerjaan; stdout yang tersumbat tidak menahan cleanup worker. Pemilik stream menutup I/O saat cancellation; CLI menutup stdin/stdout miliknya.
- SIGINT/SIGTERM gateway membatalkan job, mengosongkan lease, menutup audit, dan menunggu HTTP shutdown. Tidak ada replay otomatis dan tidak ada undo efek OS yang sudah selesai.
- X11: screenshot menyertakan origin `(0,0)` dan ukuran PNG; error backend diteruskan. Diperbaiki hang gerak/klik pada koordinat sama, lifetime clipboard Unicode, dan alias chord (termasuk `escape`).
- DEB: icon/desktop entry/action, autostart paired-only yang mengikuti XDG, helper uninstall lokal dengan pilihan eksplisit menghapus pairing. Windows script uninstall diperbaiki melalui review source; belum dijalankan di Windows.
- Container: base image Go 1.26.10/Alpine 3.24.4 dipin digest resmi, healthcheck loopback, smoke test gateway dan instalasi DEB disposable. Digest diperiksa di registry; belum ada pemindaian vulnerability image.

## Berhasil diuji di environment Linux sesi ini

- `gofmt`, `go test -race ./...`, `go vet ./...`, `packaging/build.sh`; binary Linux static amd64, Windows PE32+ amd64, dan DEB berhasil dibangun. Native build memakai Go 1.27.3; Docker memakai Go 1.26.10.
- HTTP nyata: pairing single-use, token admin salah, ACL per-device, konflik lease, Unicode file roundtrip, exec, release, token hanya hash di state, identitas survive reload, revocation dan timeout subprocess Linux.
- Policy tool: discovery/enforcement, policy kosong dan kompatibilitas state lama, persistensi, rollback ketika save gagal, perubahan izin membatalkan sesi dan mencegah dispatch/replay.
- Cancellation: isolasi antar-token, delivered job menjadi inactive, late result ditolak, HTTP disconnect via bridge, ping konkuren, notification yang terhambat, stdout backpressure, cancellation saat menunggu mutex, dan gateway Close idempotent.
- SDK MCP Python resmi `mcp==1.30.0` dan `jsonschema==4.26.0`: initialize, ping, 15 schema tool, discovery, lease, Unicode files, exec, release, filtered tools dan penolakan tool terlarang pada **HTTP dan stdio**. Memakai gateway/connector lokal dan kredensial sintetis sementara.
- Black-box X11: server **Xvfb terisolasi** 800×600 + widget Tk nyata. Validasi PNG/origin/ukuran, pointer move, click pada koordinat sama, teks `café 世界 🚀`, Ctrl+L, Escape, dan dua langkah scroll. Juga lulus dengan race detector. Ini bukan pengujian desktop fisik/multimonitor/browser default/VS Code.
- Paket DEB: transaksi dpkg install/configure/remove di root sementara; binary, desktop metadata, icon, autostart, preservasi/penghapusan pairing eksplisit dan cleanup file paket. Tes ini sendiri melewati dependensi.
- **Instalasi apt lengkap** di container Debian trixie terisolasi: dependensi xdotool/ImageMagick/xclip/xdg-utils terpasang, binary berjalan, desktop metadata/icon/helper valid, lalu paket dihapus bersih. Tidak menjalankan sesi GUI desktop di container tersebut.
- Docker build + gateway nyata dengan UID 10001, root filesystem read-only, cap-drop, no-new-privileges, healthcheck, auth/pairing, policy tool, state hashed/mode 0600. SIGTERM saat job MCP sudah delivered mengembalikan error ke caller dan exit 0; restart mempertahankan identitas tanpa replay. Compose divalidasi konfigurasinya; runtime smoke memakai `docker run` dengan batas yang sama.
- Semua target test adalah localhost/container sementara. Tidak terhubung atau deploy ke PC, VPS, homelab, CHR, atau dua agent asli. Tidak memakai Context.dev.

Perintah reproduksi ada di README; ringkasan hasil di `docs/test-results.txt`.

## Belum diverifikasi / belum diimplementasikan

- Windows binary/PowerShell/GUI, Unicode/chord/multimonitor, browser default dan VS Code belum diuji di Windows asli. Native UI/tray, installer EXE/MSI bertanda tangan dan Windows session helper/service broker belum dibuat.
- X11 desktop fisik, multimonitor, login/autostart desktop nyata, browser default dan VS Code belum diuji. Xvfb tidak membuktikan perilaku desktop penuh.
- Hermes dan agent user belum dihubungkan; uji SDK lokal tidak menggantikan validasi konfigurasi/transport kedua agent tersebut.
- Wayland GUI, RDP disconnected, layar terkunci/login dan UAC secure desktop belum didukung.
- Passkey/2FA, mTLS/device keypair, OAuth/short-lived tokens belum dibuat. Token tetap bearer individual dengan hash pada gateway; credential lokal belum DPAPI/keychain.
- WebSocket, video streaming, PTY interaktif, output streaming, chunked file transfer, DOM/CDP, monitor enumeration, drag-and-drop, capability negotiation per tool, dan failover gateway belum tersedia.
- Rotasi audit, rate limit, durable jobs/idempotency, upgrade orchestration, takeover/pause lokal dan health telemetry lanjutan belum dibuat.
- Persisten masih berarti reconnect/startup setelah user login; tidak menyalakan PC mati atau mengontrol layar login. Sesi sementara menghentikan proses, bukan menghapus pairing.
- Dashboard masih login token; bukan akun/passkey/2FA. Paket/remove tidak mencabut token gateway otomatis; admin perlu revoke.
