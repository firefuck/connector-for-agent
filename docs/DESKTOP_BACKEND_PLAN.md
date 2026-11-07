# Rencana backend desktop RelayDesk

Diperbarui 2026-11-07 setelah konfirmasi user. Dokumen ini spesifikasi dan backlog yang disetujui, belum implementasi backend atau hasil uji OS target.

## Kebutuhan yang dikunci

User memakai GNOME Wayland serta Windows Server, Windows 10 dan Windows 11. Agent VPS dan Hermes tetap memakai gateway MCP di homelab lewat WireGuard/CHR. Fokus versi berikutnya adalah mengontrol desktop dan aplikasi pada sesi user yang sedang login, aktif, dan tidak terkunci, saat user bekerja di PC tersebut. Tidak menambah AI/model di PC dan tidak mengharuskan pindah X11.

User sudah menurunkan prioritas RDP disconnected/terkunci. Menjaga kontrol saat desktop terkunci, login screen, sesi unattended, atau UAC secure desktop bukan syarat tahap ini. Jika prompt UAC muncul, user dapat menyetujuinya secara lokal; connector harus melaporkan keterbatasan dan menahan input yang tidak dapat diarahkan dengan benar. Tidak mengubah kebijakan UAC, lock, atau login OS secara otomatis.

## Prioritas yang disetujui

| Urutan | Tambahan | Hasil yang dituju | Status |
| --- | --- | --- | --- |
| 1 | GNOME Wayland pada desktop aktif | Screenshot dan input pada sesi kerja yang sama dengan consent user | Belum diimplementasikan |
| 2 | Pause/Stop lokal, hotkey, dan prioritas kontrol manusia | User dapat mengambil alih dan menghentikan pekerjaan agent dari PC tujuan | Belum diimplementasikan |
| 3 | Indikator sesi lokal | User melihat identitas agent/perangkat dan aktivitas kontrol | Belum diimplementasikan |
| 4 | CLI `relaydesk doctor` | Diagnosis pairing, gateway, backend desktop, dependensi dan izin | Belum diimplementasikan |

Pengambilan screenshot baru sebelum aksi GUI penting, pemeriksaan hasil, dan larangan replay setelah timeout menjadi bagian alur kontrol di semua tahap. Tidak ada klaim semua fitur tabel sudah tersedia pada binary saat ini.

## Backend desktop aktif

| Target | Komponen dan keputusan | Batas yang perlu diverifikasi |
| --- | --- | --- |
| GNOME Wayland | Helper per-user memakai RemoteDesktop portal + ScreenCast/PipeWire + libei, atau adaptor Desktop Sharing GNOME setelah verifikasi kemampuan host | Consent, versi portal/compositor, koordinat/scaling, Unicode, monitor, cancel, lock/revoke |
| Windows Server/10/11 | Backend user-session untuk desktop interaktif aktif; aplikasi/exec/file memakai token user | SID, session ID, input desktop, privilege aplikasi, DPI, monitor, Unicode |
| Linux X11 | Pertahankan backend yang sudah diuji dengan Xvfb | Uji desktop fisik dan multimonitor tetap diperlukan |

Versi distro/GNOME dan edisi/build Windows belum diketahui. Kemampuan upstream tidak otomatis tersedia pada host user. Riset rinci dan sumber primer: [Wayland/GNOME](RESEARCH_WAYLAND.md), [UAC](RESEARCH_WINDOWS_UAC.md), [RDP](RESEARCH_WINDOWS_RDP.md).

## Pause/Stop, hotkey, dan takeover manusia

- **Pause:** dari kontrol lokal, cegah dispatch baru dan batalkan pekerjaan agent yang antre/aktif pada perangkat. Cakupan termasuk shell/file agar agent tidak terus bekerja lewat tool lain saat kontrol GUI dijeda. Efek yang sudah terjadi tidak dapat di-undo.
- **Resume:** tindakan eksplisit user lokal. Jangan menjalankan kembali pekerjaan yang dibatalkan; agent perlu mengambil sesi kontrol baru dan memeriksa keadaan desktop.
- **Stop:** batalkan pekerjaan dan putuskan connector. Pairing disimpan; koneksi kembali memerlukan start eksplisit. Autostart atau supervisor tidak boleh langsung membatalkan pilihan Stop selama sesi kerja yang sama.
- **Hotkey lokal:** dapat dikonfigurasi dan tetap bekerja ketika gateway tidak terjangkau. Integrasi GNOME harus memakai fasilitas shortcut yang tersedia dan consent jika diperlukan. Shortcut bentrok/ditolak harus terlihat; sediakan tombol lokal yang tetap dapat dipakai.
- **Prioritas manusia:** input mouse/keyboard lokal yang dapat dibedakan dari input injeksi agent memicu Pause. Gerakan atau ketikan hasil injeksi agent sendiri tidak boleh memicu takeover. Resume tidak otomatis setelah idle agar agent tidak mendadak melanjutkan saat user bekerja.
- **Batas deteksi:** jangan menganggap portal GNOME memberikan pemantauan semua input lokal. Backend harus menyatakan apakah takeover otomatis tersedia. Bila deteksi yang andal tidak tersedia, tampilkan keterbatasan tersebut dan tetap sediakan Pause/hotkey eksplisit; tandai takeover otomatis belum didukung.

Kontrol lokal harus dapat membatalkan kerja tanpa menunggu gateway. Sinkronkan pembatalan dan pelepasan lease dengan gateway ketika koneksi tersedia. CLI/API agent tidak boleh dapat meresume Pause lokal. IPC kontrol lokal harus membatasi akses ke user pemilik connector; jangan membuka endpoint kontrol tanpa autentikasi ke jaringan.

## Indikator sesi lokal

Tampilkan nama/ID perangkat, nama/ID agent pemegang lease, dan status `idle`, `viewing`, `controlling`, `paused`, `offline`, atau `needs_consent`. Tampilkan shell/file sebagai aktivitas tersendiri supaya tampilan tidak mengatakan idle saat pekerjaan non-GUI berjalan. Jika identitas belum diketahui atau koneksi putus, tampilkan keadaan tersebut, bukan identitas lama seolah masih aktif.

Indikator menyediakan Pause/Resume/Stop dengan status yang mengikuti keadaan connector. Untuk GNOME, pilih panel/window yang tersedia pada instalasi standar; jangan mengasumsikan ekstensi tray terpasang. Jangan menampilkan token, isi perintah, clipboard, atau isi file. Lease eksklusif dan ACL tetap berlaku untuk kedua agent yang sudah ada.

## CLI diagnosis

Rancang `relaydesk doctor` dengan output teks serta `--json` untuk integrasi. Pemeriksaan bersifat read-only:

- Keberadaan dan validitas config/pairing lokal tanpa mencetak token atau isi config. File config saja belum membuktikan token masih diterima gateway.
- DNS/koneksi gateway, TLS dan autentikasi perangkat melalui endpoint diagnosis yang tidak mengambil job, lease, atau menjalankan aksi. Jangan memakai `/poll` sebagai probe kesehatan.
- OS, tipe sesi desktop, backend yang dipilih, dan dependensi backend. Pada GNOME periksa ketersediaan session bus, portal, PipeWire dan dukungan input yang dapat dideteksi.
- Bedakan `ready`, `missing`, `unsupported`, `unknown`, dan `needs_consent`. Dependensi terpasang tidak berarti izin capture/input sudah diberikan. Pemeriksaan yang membutuhkan prompt consent atau percobaan GUI harus dilaporkan belum diverifikasi oleh pemeriksaan read-only.
- Timeout setiap probe dan keseluruhan diagnosis. Laporkan langkah perbaikan spesifik, exit code terdokumentasi, serta JSON dengan schema stabil. Redaksi token, pairing code, header auth dan secret dari error jaringan/URL.

Doctor tidak melakukan pairing ulang, mengubah ACL, memasang paket, menonaktifkan TLS/UAC, atau mengirim input otomatis.

## Kontrak sesi dan aksi GUI

Session ID/user identity menjadi bagian target dan audit. Jika sesi berubah, compositor mencabut consent, Windows input desktop berubah, atau transport putus, tahan input dan invalidasi frame lama. Setelah koneksi kembali, ambil frame baru dan verifikasi sesi. Jangan mengulang klik, pengetikan atau exec dengan hasil ambigu.

Sebelum aksi GUI penting, agent mengambil screenshot baru dari sesi/monitor tujuan dan memeriksa targetnya; sesudah aksi, periksa screenshot atau hasil yang relevan. Sertakan identitas sesi, waktu capture, monitor/origin/dimensi dan scale bila tersedia. Pergantian sesi/monitor/scaling membatalkan asumsi koordinat dari frame lama. Instruksi agent dan metadata membantu alur ini; enforcement berbasis frame memerlukan kontrak API tersendiri dan tidak boleh diklaim ada hanya dari dokumentasi.

Capability harus menyatakan keadaan nyata (`ready`, `needs_consent`, `locked`, `disconnected`, `session_ended`, `unsupported`) serta identitas backend/sesi. Keberadaan executable atau service bukan bukti capability ready. Screenshot hitam/error tidak boleh dilaporkan sebagai kontrol yang sukses. Auth, ACL device/tool, lease eksklusif, bounded decoding, deadline dan cancellation tetap dipertahankan.

## Acceptance sebelum menandai selesai

| Area | Bukti yang diperlukan |
| --- | --- |
| GNOME Wayland aktif | GNOME nyata: consent accept/deny/revoke, screenshot, mouse, Unicode/chord, scaling/monitor, cancellation, lock lalu unlock tanpa replay; Xvfb tidak menggantikan uji ini |
| Windows desktop aktif | Windows Server/10/11 nyata: screenshot/input pada sesi user yang benar, Unicode/chord, DPI/monitor, browser dan VS Code; prompt UAC manual tidak dilaporkan otomatis didukung |
| Pause/Stop | Saat job antre dan aktif, termasuk subprocess: pembatalan lokal dan penolakan pekerjaan baru; tetap berfungsi ketika gateway putus; pairing tidak terhapus |
| Resume | Memerlukan tindakan lokal; tidak menjalankan job lama, tidak dapat dipicu agent; lease baru dan pemeriksaan keadaan sebelum aksi berikutnya |
| Takeover dan hotkey | Input manusia memicu Pause pada backend yang mendukung; input injeksi tidak; fallback lokal tersedia ketika shortcut atau deteksi tidak didukung |
| Indikator | Identitas dua agent dan perangkat akurat saat lease berganti; status view/input/shell/file/pause/offline mengikuti aktivitas; tidak memuat secret |
| Doctor | Config hilang/rusak, token ditolak, gateway mati/TLS gagal, dependensi hilang, Wayland unsupported/needs-consent, timeout, redaksi secret, output JSON dan tidak ada side effect |
| Siklus GUI | Frame baru sebelum aksi penting, verifikasi setelah aksi, perubahan sesi/monitor tidak memakai koordinat lama, hasil timeout tidak direplay |

Jalankan pemeriksaan Go dan build sesuai AGENTS.md setelah perubahan kode. Catat OS/versi, perintah, hasil aktual dan batas pengujian di STATUS.md. Build Windows atau mock portal bukan bukti perilaku Windows/GNOME.

## Tahap lanjutan, bukan penghambat desktop aktif

- GNOME Remote Login persisten (kemampuan upstream sejak GNOME 47): bedakan sesi remote dari desktop fisik yang sedang dipakai.
- Worker RDP retained/reconnect: verifikasi SID/session ID/PID dan aplikasi yang sama setelah viewer ditutup, tanpa replay. SYSTEM service saja tidak menjamin capture sesi disconnected.
- Windows UAC secure desktop: installed broker dan worker per-session terpisah dari shell/file user. Aktivasi lokal dan izin admin per-device eksplisit; pairing lama/wildcard tool tidak otomatis mengaktifkan privilege baru. IPC terautentikasi, GUI-only, deadline/cancel, credential vault, dan uji hardening credential Windows terbaru diperlukan.

Tidak ada perubahan kebijakan OS, deployment, koneksi ke host user, atau pengujian GNOME/Windows yang dilakukan hanya dengan menulis rencana ini. Tidak memakai Context.dev.
