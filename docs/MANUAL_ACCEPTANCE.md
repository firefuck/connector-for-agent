# Panduan uji manual desktop dan dua agent

Diperbarui 2026-11-08. Target pengguna: Ubuntu 22 (baseline implementasi 22.04,
GNOME 42) dan Windows 11 VPS; Windows 10/Server perlu diuji terpisah. Pengguna
memilih menguji Windows sendiri. Tidak ada akses ke VPS/agent asli yang digunakan
oleh pengembangan ini. Jangan menandai semua platform lulus berdasarkan build.

## Persiapan dan batas

1. Cadangkan config connector dan state gateway secara privat. Jangan unggah file
   config, `.env`, token, clipboard atau isi pekerjaan ke laporan pengujian.
2. Gunakan gateway dan binary connector dari commit yang sama. Gateway lama tidak
   mendukung generasi sesi lokal atau endpoint doctor baru. Hentikan connector
   lama sebelum mengganti binary; ID/pairing tetap dipakai.
3. Uji di desktop user biasa yang aktif dan tidak terkunci. Agent VPS dan Hermes
   tetap pada host lama; WireGuard/CHR tidak diubah. Jangan jalankan sebagai
   SYSTEM/root untuk mencoba melewati prompt OS.
4. Gunakan aplikasi/file dummy. Shell/file mengikuti hak user connector dan bukan
   sandbox. Kontrol lokal bukan batas keamanan terhadap proses jahat yang sudah
   berjalan sebagai user yang sama atau administrator.
5. Belum ada binary/installer dengan tanda tangan penerbit terverifikasi dalam
   repo. Source bersifat proprietary. Checksum tidak menggantikan tanda tangan.

## Ubuntu 22.04 GNOME Wayland

Catat versi tanpa credential:

```sh
cat /etc/os-release
gnome-shell --version
printf '%s\n' "$XDG_SESSION_TYPE"
dpkg-query -W xdg-desktop-portal xdg-desktop-portal-gnome pipewire
```

Build paket di mesin Linux dengan Go 1.24+ dan dpkg, kemudian instal pada Ubuntu:

```sh
sh packaging/build.sh
(cd dist && sha256sum -c SHA256SUMS)
sudo apt install ./dist/relaydesk_0.1.0_amd64.deb
# Pada GNOME, pastikan backend portal GNOME terpasang (juga jika memakai --no-install-recommends):
sudo apt install xdg-desktop-portal-gnome
relaydesk
```

DEB membawa dependency Python GI/GTK3, GStreamer PipeWire/base/good, portal serta
utility X11. Tidak perlu memasang plugin remote pada agent. Gateway Docker tidak
memerlukan paket GUI tersebut. Jika menjalankan binary portable, pasang dependency
tersebut sendiri; script Python/PowerShell helper sudah tertanam di binary.

Wizard console meminta URL gateway, pairing code sekali pakai dan nama PC hanya
jika belum dipair. Setelah itu window kontrol lokal terbuka dalam keadaan **Pause**.
Klik **Resume**, setujui dialog berbagi GNOME, pilih **satu monitor** dan izinkan
keyboard/pointer. Tanpa consent yang lengkap, connector tetap dijeda. Backend
memakai API portal Notify* yang tersedia pada baseline lama, bukan memaksa libei
atau GNOME terbaru. Tidak menyimpan restore token; Resume setelah Pause meminta
sesi portal/consent baru sesuai kebijakan desktop.

Untuk shortcut global Pause, jalankan sebagai user desktop:

```sh
python3 /usr/share/relaydesk/hotkey-linux.py
# Jika Ctrl+Alt+P bentrok, pilih accelerator lain:
python3 /usr/share/relaydesk/hotkey-linux.py --key '<Control><Alt>F12'
# Untuk binary/config custom:
python3 packaging/hotkey-linux.py --binary /path/relaydesk --config /path/connector.json
# Hapus hanya shortcut RelayDesk:
python3 /usr/share/relaydesk/hotkey-linux.py --remove
```

Installer shortcut mempertahankan shortcut GNOME lainnya. Verifikasi kombinasi
itu benar-benar aktif; kebijakan desktop atau konflik bisa menolaknya. Ctrl+Alt+P
juga tersedia ketika window RelayDesk fokus. **Takeover otomatis dari semua input
fisik tidak tersedia pada portal GNOME**; gunakan tombol/shortcut Pause. Tidak
mengklaim gerakan mouse manusia bisa dipantau global lewat portal tersebut.

Kontrol CLI Linux memakai Unix socket privat user, tanpa listener jaringan:

```sh
relaydesk control --op status
relaydesk control --op pause
relaydesk control --op stop
# Tambahkan --config jika pairing disimpan di lokasi custom.
```

Resume hanya dari window lokal. Tidak ada `control --op resume` atau tool MCP
untuk melewati Pause. `Stop` menyimpan marker `connector.json.stopped`, menutup
connector, dan mempertahankan pairing. Startup `connector --autostart` menghormati
marker itu. Jalankan `relaydesk` atau `relaydesk connector` secara eksplisit untuk
mulai lagi. Hapus shortcut sebelum menghapus paket jika sudah dipasang.

## Windows 11 VPS, Windows 10, Windows Server

Salin `dist/relaydesk.exe` dan tiga notice dari `dist`. Mulai dari sesi RDP user
yang **masih terbuka dan tidak terkunci**, lalu jalankan EXE. Pairing tetap memakai
wizard console; setelah pairing tampil window WinForms dengan Pause/Resume/Stop.
Klik Resume sebelum agent mengambil lease.

Script install per-user masih tersedia:

```powershell
powershell -NoProfile -File .\packaging\install-windows.ps1 -Persistent
.\dist\relaydesk.exe doctor --json
```

Shortcut global Windows memakai Ctrl+Alt+P. Window menampilkan apakah pendaftaran
shortcut berhasil. Hook input fisik keyboard/mouse meminta Pause dan mengabaikan
flag injected yang ditandai Windows. Uji mouse/keyboard nyata serta injeksi agent
secara terpisah; ini belum divalidasi di Windows asli. Keyboard virtual, driver
khusus atau software input lain dapat ditandai berbeda. Window juga meminta Pause pada notifikasi lock/logoff/disconnect sesi. Jika hook
atau session guard gagal, window menyatakan kegagalan; tombol Pause tetap tersedia.

Build installer wizard EXE pada mesin Windows dengan Go dan Inno Setup 6:

```powershell
# Sesuaikan path ISCC dengan instalasi Inno Setup di PC build.
powershell -NoProfile -File .\packaging\build-windows.ps1 `
  -ISCC 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe'
```

Hasil yang dituju `dist\RelayDesk-Setup-0.1.0-x64.exe`. Installer ini per-user,
memuat lisensi, shortcut Start Menu, opsi autostart dan uninstaller; pairing tidak
dihapus saat uninstall. Script/definisi installer disediakan, tetapi kompilasi
Inno Setup dan install/uninstall Windows belum dijalankan dalam environment Linux
ini. Periksa hasilnya sendiri sebelum distribusi.

Untuk tanda tangan yang dapat dipercaya, sertifikat code-signing yang sah beserta
private key harus tersedia di certificate store **CurrentUser\My** pada mesin
build yang berwenang. Jangan memasukkan PFX/password/private key ke Git/chat.

```powershell
powershell -NoProfile -File .\packaging\build-windows.ps1 `
  -ISCC 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe' `
  -CertificateThumbprint THUMBPRINT_SERTIFIKAT_MILIK_LO
Get-AuthenticodeSignature .\dist\relaydesk.exe
Get-AuthenticodeSignature .\dist\RelayDesk-Setup-0.1.0-x64.exe
```

Build menandatangani binary sebelum bundling, lalu installer, memverifikasi status
signature dan baru membuat manifest checksum Windows. Tanpa sertifikat, output
jelas **unsigned**. Jangan menonaktifkan SmartScreen/kebijakan PowerShell/UAC untuk
mengklaim instalasi berhasil. Signed installer tidak menjamin reputasi SmartScreen.

UAC secure desktop, login screen dan RDP disconnected tetap di luar target ini.
Jika UAC muncul, setujui lokal; backend menolak input desktop yang tidak tersedia.
Jangan menganggap aplikasi elevated selalu bisa dikontrol dari connector biasa.

## Alur yang wajib dipakai oleh agent

1. `devices_list`: pastikan ID PC benar, heartbeat baru dan `paused` bernilai false.
2. `session_acquire`: satu lease eksklusif; agent kedua tidak boleh merebut lease.
3. `system_info`, lalu `desktop_screenshot`.
4. Baca gambar. Gunakan koordinat screenshot. Untuk Wayland koordinat berasal dari
   monitor yang dipilih, origin `(0,0)`; helper mengubah pixel menjadi koordinat
   logis berdasarkan metadata portal. Jangan menambahkan origin monitor fisik lain.
5. Kirim **satu** input (`desktop_click`, `desktop_move`, `desktop_type`,
   `desktop_key`, atau `desktop_scroll`), sebaiknya dengan `frame_id` hasil capture.
6. Screenshot lagi untuk memverifikasi hasil dan mendapatkan frame untuk input
   berikutnya. Frame hanya berlaku 30 detik dan dikonsumsi oleh satu percobaan
   input. Pause/Resume, lease baru dan pergantian agent membatalkan frame sebelumnya.
7. `session_release` setelah selesai. Setelah timeout, putus koneksi atau Pause,
   periksa keadaan; jangan mereplay klik, shell atau penulisan file secara otomatis.

Enforcement frame berlaku pada connector interaktif. Mode `--headless` tersedia
untuk tes X11/shell, tanpa window dan tanpa guard frame interaktif; Wayland menolak
mode itu. Jangan memakai mode tes tersebut untuk memenuhi acceptance kontrol lokal.

## Matriks uji dan hasil yang harus dicatat

| Uji | Langkah | Hasil yang diharapkan |
| --- | --- | --- |
| Pairing | Pair sekali, Stop, mulai lagi | ID sama, tidak meminta pairing ulang |
| Mulai lokal | Jalankan connector | Window terlihat, awal Pause, agent tidak dapat acquire |
| Consent GNOME | Resume lalu tolak dialog | Tetap Pause; tidak melaporkan GUI berhasil |
| Consent diterima | Resume, pilih satu monitor + keyboard/pointer | Agent dapat acquire; screenshot sesuai monitor |
| Capture | Ambil screenshot desktop dummy | PNG valid, dimensi/origin/frame ID benar; bukan layar lain |
| Input | Screenshot → klik → screenshot | Target dan posisi benar; input tanpa frame baru ditolak |
| Unicode | Ketik `café 世界 🚀` di editor dummy | Karakter tepat, tidak sekadar keycode layout US |
| Chord/scroll | Ctrl+L, Escape, scroll 2 langkah | Hasil benar, tidak ada modifier tertahan |
| DPI/scaling | Ulangi pada 100%/125%/200% bila tersedia | Klik tetap cocok dengan pixel screenshot |
| Monitor | Pilih tiap monitor pada sesi baru; cabut/resize | Sesi/frame lama ditolak bila tidak valid; minta capture/consent baru |
| Browser/VS Code | Buka lalu edit file dummy melalui GUI | Hasil terlihat; tidak menganggap perintah launch berarti sukses |
| Pause shell | Jalankan shell panjang lalu Pause | Subprocess dihentikan; tool error; tidak lanjut setelah Resume |
| Pause file/input | Pause saat aksi antre/aktif | Aksi berikutnya ditolak; efek yang sudah terjadi tidak di-undo |
| Offline Stop | Putuskan gateway, tekan Stop | Connector berhenti tanpa menunggu gateway; pairing tetap ada |
| Autostart | Stop lalu jalankan `connector --autostart` | Tidak memulai ulang; start manual membuka window Pause lagi |
| Takeover Windows | Resume; gerakkan mouse/ketik fisik | Pause; injeksi agent sendiri tidak Pause; catat bila driver berbeda |
| Shortcut | Uji kombinasi dari aplikasi lain | Pause bekerja atau kegagalan/conflict tercatat; fallback tombol tersedia |
| Lock/unlock | Lock saat kontrol, lalu unlock | Tidak mengirim input ke desktop lock; GNOME perlu Resume/consent baru |
| Dua agent | A acquire PC; B acquire PC sama | B ditolak; dua PC berbeda dapat dikontrol terpisah |
| Policy/revoke | Batasi tool/cabut agent saat job aktif | Job dibatalkan; token/izin lama tidak diterima |
| Uninstall | Stop lalu uninstall | Binary/shortcut hilang; config tetap kecuali hapus eksplisit |

Input otomatis dapat menggerakkan atau mengetik pada aplikasi nyata. Uji hanya di
aplikasi/file dummy yang dipilih. Capture berhasil tidak membuktikan input benar;
selalu periksa hasil visual. Pada Windows, ulangi per edisi/build yang akan dipakai.

## Menghubungkan agent VPS dan Hermes

Gunakan dua token agent terpisah dari dashboard dengan PC dan tool yang eksplisit.
Pakai konfigurasi MCP HTTP/stdio pada README. Format file host agent/Hermes
bergantung versi host; tidak ada perubahan otomatis ke konfigurasi kedua agent.
Untuk setiap agent, jalankan urutan di atas terhadap PC uji. Catat nama/versi host,
transport, nama tool, status sukses/error dan hasil visual. Jangan sertakan token.
Jika host tidak menampilkan `frame_id`, gunakan hasil teks metadata screenshot;
input tanpa frame_id tetap membutuhkan capture baru yang tersimpan di connector.

## Diagnosis dan pelaporan

```sh
relaydesk doctor
relaydesk doctor --json > relaydesk-doctor.json
# Config custom:
relaydesk doctor --config /path/connector.json --json
```

Schema JSON `schema_version:1`, daftar `checks` dan `exit_code`. Exit 0 berarti
semua pemeriksaan yang dilaporkan ready; exit 1 berarti ada missing/unsupported/
unknown/needs_consent. Doctor tidak meminta consent, menjalankan GUI, melakukan
pairing, memperbarui heartbeat, mengambil job atau lease. Pada desktop, exit 1
bisa normal karena izin capture/input tidak dibuktikan oleh probe read-only.
Error token ditolak dibedakan dari jaringan/TLS/version yang belum terverifikasi.

Laporkan commit, OS/build, versi GNOME/portal bila Linux, transport agent, langkah
uji, hasil yang diharapkan/aktual, output doctor yang diperiksa, dan waktu kejadian.
Jangan kirim connector.json/state.json/.env atau screenshot desktop sensitif.
