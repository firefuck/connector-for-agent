# Audit GUI, bug dan keamanan — 2026-11-07

Cakupan: review source Go, dashboard, alur HTTP/MCP, kredensial lokal, lifecycle job, operasi file dan packaging. Uji memakai localhost, browser Chromium, Xvfb dan container sementara dengan identitas sintetis. Tidak menghubungi PC, VPS, homelab atau agent user. Tidak memakai Context.dev. Ini audit terbatas dan perbaikan source, bukan sertifikasi bebas kerentanan atau pentest semua OS.

## GUI yang tersedia

Dashboard web disederhanakan: langkah pairing, label form, checkbox PC, preset izin, salin kode/token, hapus hasil, status koneksi dan pemegang lease, izin lanjutan, konfirmasi revoke, serta layout HP. Default akses baru di dashboard adalah melihat layar dan info PC. Permintaan mutasi tidak diretry otomatis; submit ganda diblokir. Lock/HTTP 401 membersihkan token/form/hasil dan membatalkan request; respons terlambat tidak membuka dashboard lagi. Token tidak disimpan dalam localStorage/sessionStorage/cookie.

Revisi visual berikutnya pada tanggal yang sama memindahkan menu ke slide drawer dengan halaman ringkasan dan warna krem/terracotta. Menutup drawer hasil menghapus kode/token; draft form biasa dipertahankan. Close/Escape ditahan selama mutasi pending, sedangkan Kunci tetap dapat digunakan. Uji browser mencakup fokus keyboard dalam modal dan pengembaliannya, backdrop, reduced motion, viewport 320 px dan respons terlambat setelah lock. Kontrol akses server tidak berubah oleh revisi visual ini.

Connector PC masih wizard console. Native UI/tray, Pause/Stop/hotkey, takeover, doctor dan GNOME Wayland belum diimplementasikan. Indikator pemegang lease di dashboard tidak sama dengan indikator aktivitas lokal pada PC.

## Temuan dan perbaikan

Tingkat di bawah adalah penilaian praktis untuk penggunaan gateway privat, bukan skor CVSS.

| Temuan | Dampak / prasyarat | Perbaikan dan bukti regresi |
| --- | --- | --- |
| Respons admin/pair/connector menulis ke jaringan sambil menahan mutex global | Sedang: pembaca respons lambat dapat menghambat polling dan cancellation perangkat lain | Snapshot JSON saat terkunci, kirim setelah unlock. Tes response writer yang diblokir membuktikan polling lain tetap berjalan |
| Connector membaca body sebelum memeriksa token | Sedang: request tanpa autentikasi bisa memicu parsing/alokasi body hingga 12 MiB | Auth sebelum membaca body, lalu periksa kembali setelah parsing untuk menghadapi revoke konkuren. Tes body spy membuktikan body tidak dibaca pada token salah |
| Decoder non-MCP menerima JSON pertama tanpa memeriksa sisa body | Sedang: trailing JSON/padding dapat melewati maksud batas request dan membuat parsing ambigu | Wajib tepat satu JSON. Batas pairing 4 KiB, admin/poll/check 64 KiB, result tetap 12 MiB; tes trailing data dan padding melewati batas |
| WriteFile ke config yang sudah ada mempertahankan mode file dan mengikuti symlink; state memakai nama `.tmp` tetap | Sedang, memerlukan file/dir lokal dengan izin atau symlink bermasalah: kebocoran credential atau penimpaan target lain | Temp file acak privat, sync dan rename. Tes mode config 0600, preservasi target symlink dan cleanup ketika rename gagal. Jaminan izin diuji pada Linux; ACL Windows mengikuti profil user dan belum diuji langsung |
| File tools menerima FIFO/device; listing membaca seluruh direktori sebelum memotong hasil | Sedang: agent yang diizinkan memakai tool file dapat memacetkan connector atau menyebabkan alokasi berlebih | Read/write hanya regular file, periksa handle sebelum truncate; Linux membuka nonblocking untuk mencegah race FIFO. List membaca maksimum 2000 entri. Tes FIFO, direktori, overwrite dan direktori >2000 entri |
| Pembuatan agent baru tanpa `tools` memberi akses penuh melalui semantik legacy | Sedang: kelalaian pemanggil admin menghasilkan izin lebih luas dari yang dimaksud | API baru wajib array eksplisit; `[]` menolak semua, `["*"]` semua. State lama tetap kompatibel; dashboard default view. Tes create tanpa/null policy ditolak dan pemulihan policy legacy tetap lulus |
| Result job tidak memeriksa delivered/deadline secara langsung | Bug lifecycle: hasil prematur/terlambat bisa diterima pada jendela sebelum cleanup | Periksa pemilik perangkat, delivered dan deadline. Tes hasil prematur, perangkat lain, expired dan hasil valid |
| Client dapat mencetak raw error response dan diam-diam memotong respons besar | Kebocoran token ke log jika gateway/proxy merefleksikannya; diagnosis respons salah | Hanya error JSON yang dibaca sebagai pesan, token client direduksi, karakter kontrol dibersihkan, pesan dibatasi 1 KiB; body >12 MiB ditolak eksplisit. Tes refleksi token, raw proxy body dan respons oversized |
| Dashboard belum memiliki CSP, raw JSON hasil kurang mudah dibaca, klik berulang dapat membuat akses ganda | Hardening browser dan risiko kesalahan operator | JS/CSS embedded sebagai resource terpisah, CSP tanpa unsafe-inline/eval, no-referrer, no-store dan frame deny. DOM memakai textContent. Uji Chromium payload XSS literal, submit ganda dan lock saat request berjalan |
| State dengan map null/entry null bisa menyebabkan panic setelah reload | Bug availability saat state rusak/diedit | Inisialisasi map null, tolak entry null atau ID tidak cocok; tes reload |

Semua perbaikan di atas sudah masuk source lokal. Header origin rejection, auth bearer, ACL perangkat/tool, lease eksklusif, bounded decoding, cancellation dan larangan replay dipertahankan.

## Validasi

- `gofmt`, `go vet ./...`, dan `go test -race ./...` lulus. Uji opt-in X11 + Xvfb/Tk juga lulus bersama suite dengan race detector setelah memakai direktori dependensi lokal yang sudah tersedia. Percobaan pertama opt-in gagal karena Xvfb tidak ada di PATH default, bukan karena assertion GUI gagal.
- Build Linux amd64, Windows amd64 dan DEB lulus. Instalasi/remove DEB terisolasi lulus; tes dpkg ini melewati dependensi dan tidak memulai sesi desktop.
- Docker image terbaru berhasil dibangun dan smoke test lulus: Compose policy, non-root/read-only, health, auth/pairing, state hashed, restart, revoke dan SIGTERM dengan job MCP delivered. Percobaan awal gagal karena disk workspace penuh; hanya cache build RelayDesk lama dibersihkan, lalu build/test diulang hingga lulus. Pemeriksaan yang ikut gagal membuat file sementara saat disk penuh juga diulang.
- Chromium/Playwright: login salah/benar, pairing, payload XSS literal, pilihan PC, default view, submit ganda, ubah izin, revoke, layout 390 px, token tidak disimpan di storage browser, lock saat response masih ditahan. Screenshot diuji secara visual; tidak ada page error.
- SDK MCP Python resmi 1.30.0: HTTP dan stdio, initialize/ping, 15 schema, lease, exec, Unicode files, release, discovery serta penolakan tool terlarang lulus.
- `govulncheck` 1.9.0, toolchain Go 1.27.3: analisis source Linux dan target Windows mengembalikan `No vulnerabilities found`. Hasil ini bergantung database dan cakupan analisis Go; tidak membuktikan Windows GUI berjalan atau seluruh OS image aman.
- `govulncheck -mode=binary` terhadap executable dari image Docker terbaru (Go 1.26.10) juga mengembalikan `No vulnerabilities found`. Ini scan binary Go, belum scan paket Alpine/image OS.

## Batas dan pekerjaan tersisa

- Belum ada uji runtime Windows Server/10/11, GNOME Wayland, UAC secure desktop, RDP disconnected, atau koneksi ke Hermes/agent user. Build dan scan target Windows tidak menggantikan tes tersebut.
- Sejak 2026-11-12: autentikasi gagal dibatasi 30/menit per alamat IP (lalu 429 untuk semua request ber-credential dari alamat itu), perbandingan admin token constant-time, dan audit dirotasi pada 8 MiB. Ini bukan rate limit per-identitas atau pembatas request yang sah; client di belakang satu NAT/reverse proxy berbagi satu budget. Belum ada quota state maupun scan CVE seluruh paket image/container. Header/body limits dan limit request MCP bukan pengganti semuanya. Gateway tetap ditujukan ke jaringan privat.
- Izin tool bukan sandbox OS. `system_exec` adalah shell penuh; `files_read` dapat membaca file yang bisa dibaca user connector, termasuk credential; `files_write` dapat mengubah startup/script. Input desktop juga dapat menjalankan terminal. Preset view tidak memasukkan tool file, shell atau input.
- Token lokal Windows belum DPAPI/keychain. Secret baru harus disimpan operator saat ditampilkan; respons create yang hilang tidak boleh diretry otomatis karena identitas mungkin sudah tercipta. Periksa daftar dan revoke identitas yang tokennya hilang sebelum membuat ulang.
- Cancellation tidak mengembalikan efek yang sudah terjadi. Operasi I/O kernel/network filesystem yang macet dan subprocess yang sengaja melepaskan diri dari grup tidak memiliki jaminan penghentian mutlak. Penggunaan regular file menghilangkan kasus FIFO yang diuji, bukan semua kemungkinan macetnya filesystem.
- Bila penyimpanan revoke gagal, server melaporkan error dan mencabut akses di memori; state lama di disk dapat aktif lagi setelah restart. Perbaiki penyimpanan dan ulangi revoke hingga berhasil sebelum restart. Ini masih batas persistensi yang perlu ditangani operator.
