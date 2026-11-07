# Brief untuk Codex

User ingin aplikasi gateway self-hosted dengan flow seperti TeamViewer agar agent buatannya dan Hermes mengontrol seluruh PC Windows/Linux dari HP. Agent tetap satu di VPS dan satu di homelab. MikroTik CHR sudah menghubungkan PC/VPS melalui WireGuard. Semua target memakai desktop GUI. Gateway akan berjalan di homelab. User tidak ingin Context.dev dipakai. Jangan minta user menjelaskan ulang arsitektur.

## Tujuan produk

Install connector sekali → pairing ID/kode → kedua agent memakai gateway MCP → pilih PC → kontrol browser, VS Code, terminal, file, pengaturan OS. Mode persisten/sementara. Identitas perangkat stabil. Gateway memusatkan izin dan konflik sesi. Tidak menjanjikan kontrol 100% ketika OS mati, GUI terkunci, atau privilege tidak tersedia.

## Implementasi yang sudah ada

Go standard library saja. `cmd/relaydesk/main.go` punya gateway, wizard pairing, connector, admin CLI, stdio MCP bridge. `internal/core` punya auth bearer hashed, pairing single use, allowlist per-device, leases, queue polling, revocation, audit, dashboard web, MCP HTTP, file/command tools, desktop backend Windows PowerShell/.NET dan Linux X11 (xdotool/ImageMagick/xclip).

Paket binary Windows amd64, Linux amd64 dan DEB dibuat lewat packaging/build.sh. Windows binary portable + installer script; belum installer GUI. Container build definition dan Compose ada.

## Lanjutan 2026-11-06 yang sudah dilakukan

Per-tool policy (dashboard/CLI dan enforcement + discovery), cancellation HTTP/MCP/stdio, bridge konkuren dan shutdown SIGTERM sudah diimplementasikan. Perbaikan X11 mencakup origin/dimensi screenshot, error backend, hang mousemove, clipboard Unicode dan chord alias. DEB punya icon/desktop action, paired-only XDG autostart, dan cleanup user yang mempertahankan pairing kecuali eksplisit dihapus. Windows uninstall script diperbaiki statis saja.

Tes resmi SDK Python MCP lulus untuk HTTP dan stdio, GUI black-box lulus pada Xvfb + Tk, Docker gateway lulus (termasuk shutdown dengan job delivered), dan apt install/remove DEB dengan dependensi nyata lulus dalam container Debian sementara. Lihat STATUS.md dan test-results.txt untuk batas dan reproduksi. Tes ini tidak menghubungkan agent user/Hermes atau desktop Windows.

## Refactor dan lisensi 2026-11-07

Permintaan terbaru user: hardcode identitas, refactor dan mencegah penjiplakan/
rebrand. Sudah dijelaskan tidak ada jaminan teknis absolut untuk source public;
user meminta lanjut dengan asumsi lisensi proprietary. LICENSE/NOTICE dan
THIRD_PARTY_NOTICES.md ditambahkan; jangan mengklaim trademark terdaftar, code
signing atau repo private. Lihat docs/BRAND_PROTECTION.md. IP/token tidak dihardcode.

`identity.go` menjadi sumber nama/versi; CLI `license` membaca teks legal embed.
Dashboard HTML memakai html/template untuk identitas. Gateway dipisah ke
models/auth/http/admin/connector_handler/dispatch; batas resource di limits.go.
`cancelSessionsLocked` dipakai revoke/policy/release/shutdown, dengan regresi
isolasi dua sesi. Paket menyertakan notice; build menjalankan check-branding.sh
dan membuat SHA256SUMS tanpa signature. DEB copyright menggabungkan notice asli
+ Go supaya tetap tersedia pada image yang mengecualikan docs lainnya.
Windows script menyalin notice dari dist; belum diuji Windows asli.

## Kerjakan berikutnya, jangan mengklaim sudah selesai

Audit 2026-11-07 sudah memperbaiki dashboard dan keamanan source; baca [SECURITY_REVIEW.md](SECURITY_REVIEW.md). Dashboard sekarang memakai embedded HTML/JS/CSS di `internal/core/web`, CSP tanpa inline script, pilihan PC/preset dan default view; create agent API wajib `tools` eksplisit, kompatibilitas policy legacy hanya untuk state yang sudah ada. Response gateway disnapshot saat lock lalu dikirim setelah unlock. Penyimpanan file privat, validasi body/result dan pembatasan file tools memiliki tes regresi. Chromium, race suite+Xvfb, SDK MCP dan build lulus; govulncheck Linux/Windows tidak menemukan kerentanan terlapor. Native connector GUI dan GNOME Wayland tetap belum dibuat. Jangan menganggap audit Go sebagai scan CVE seluruh image OS.

Permintaan UI terbaru sudah diimplementasikan: ringkasan utama minimal, menu Perangkat/Agent/Tambah PC/Tambah agent membuka slide drawer modal dari kanan, palet krem dengan terracotta. Native dialog ditambah focus trap dan restore fokus, dukungan Escape/backdrop/reduced motion. Form draft bertahan ketika ditutup; hasil berisi secret dibersihkan saat ditutup. Tutup biasa ditahan selama mutasi pending, tetapi Kunci dashboard tetap membatalkan request. Browser regression telah disesuaikan dan lulus, termasuk viewport 320 px. Hindari mengembalikan semua form ke halaman ringkasan atau tone navy lama.

Prioritas user terbaru (2026-11-07): **desktop aktif saat user bekerja**, pada GNOME Wayland serta Windows Server/10/11. User telah menurunkan prioritas RDP disconnected/terkunci; unattended dan kontrol UAC otomatis bukan syarat tahap ini. UAC dapat disetujui user secara lokal. Jangan meminta user mengulang pilihan desktop atau mengarahkan wajib pindah X11. Ikuti [DESKTOP_BACKEND_PLAN.md](DESKTOP_BACKEND_PLAN.md) serta riset GNOME/UAC/RDP yang ditautkan di sana. Riset sudah dilakukan, implementasi backend belum. Versi distro/GNOME dan edisi/build Windows belum diketahui; jangan mengasumsikan semua capability upstream tersedia pada host.

Tambahan yang disetujui user, belum diimplementasikan: **GNOME Wayland aktif → Pause/Stop lokal, hotkey dan takeover manusia → indikator sesi lokal → `relaydesk doctor`**. Rencana backend memuat perilaku dan acceptance setiap fitur. Pause membatalkan pekerjaan termasuk shell/file, Resume eksplisit tanpa replay, dan Stop tetap bekerja ketika gateway putus. Takeover otomatis hanya diklaim bila backend dapat membedakan input manusia dari injeksi; jangan mengasumsikan portal GNOME menyediakan pemantauan input global. Indikator menampilkan perangkat/agent dan aktivitas sebenarnya. Doctor read-only, output teks/JSON, meredaksi secret dan tidak memakai polling job sebagai probe.

Alur agent memakai screenshot baru sebelum aksi GUI penting, memeriksa hasil, dan tidak mereplay aksi timeout. Perubahan izin per-device/tool, lease, cancellation dan larangan replay tetap berlaku. Riset GNOME Remote Login persisten, worker RDP dan broker secure desktop disimpan untuk tahap lanjutan; jangan menjadikannya penghambat fitur desktop aktif.

1. Uji backend Windows di VM asli, input Unicode/chord/multimonitor, screenshot origin, default browser, VS Code; uji desktop Linux X11 fisik dan multimonitor. Tes Xvfb black-box sudah ada, belum menggantikan pengujian desktop penuh.
2. Uji remote MCP dan stdio dengan agent user serta Hermes; lanjutkan validasi lifecycle dengan host agent sebenarnya. Smoke SDK resmi lokal/schema untuk HTTP dan stdio sudah lulus.
3. Implementasi Wayland melalui portal/compositor yang didukung pada sesi kerja aktif, dengan consent yang sesuai. Saat ini GUI Wayland sengaja ditolak. Prioritas fitur mengikuti urutan di atas, bukan nomor inventaris ini.
4. Pause/Stop lokal, hotkey, takeover, indikator sesi dan doctor sesuai acceptance rencana. Native UI GNOME tidak boleh bergantung pada asumsi ekstensi tray terpasang. Validasi startup/login desktop nyata setelah DEB install; icon/desktop metadata, helper uninstall dan apt install/remove container sudah diuji.
5. Tahap lanjutan: wizard kredensial, installer EXE/MSI bertanda tangan, Windows user-session helper/service broker least privilege dan dukungan UAC/lock/login/RDP disconnected. Jangan menonaktifkan UAC atau auto-login secara diam-diam.
6. WebSocket/mTLS/device keypair dan short-lived agent auth. Saat ini token bearer individual dengan hashed storage di gateway; bukan keypair atau OAuth.
7. PTY interaktif, streaming output, file transfer chunked, CDP browser integration, monitor enumeration, drag, metadata screenshot lanjutan/monitor enumeration, capability negotiation per tool. Origin dan dimensi PNG X11 sudah tersedia.
8. Audit rotation, rate limits, durable jobs/idempotency, upgrade orchestration, takeover/pause lokal dan telemetry health. Per-tool permissions, cancellation notification/disconnect dan graceful shutdown SIGTERM sudah ada; jangan mengklaim semuanya selesai.
9. Pertahankan Docker/DEB integration tests. Base image sudah dipin dan smoke test lulus; lanjutkan vulnerability scanning dan review upgrades sebelum deployment. Compose baru divalidasi config, runtime smoke memakai docker run dengan pembatasan setara.

## Constraints

- Jangan deploy ke homelab/user machine tanpa koneksi/target yang diberikan.
- Jangan masukkan credential nyata ke Git.
- HTTP hanya untuk rute WireGuard yang diverifikasi; alternatif HTTPS built-in.
- Status testing ada di STATUS.md. Lanjutkan dari source ini, jangan mulai ulang.
