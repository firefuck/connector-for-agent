# Riset backend Wayland RelayDesk

Diperbarui 2026-11-07. Ini hasil pembacaan dokumentasi upstream, **bukan implementasi atau pengujian Wayland**. RelayDesk saat ini masih menolak GUI Wayland. User sudah memastikan memakai **GNOME Wayland**; versi GNOME, distro dan paket portal terpasang belum diketahui. Dua agent, gateway homelab, serta WireGuard/CHR tetap dipertahankan.

## Kesimpulan

Wayland bisa diremote, tetapi `xdotool`/capture X11 yang sekarang dipakai tidak menjadi backend universal untuk semua aplikasi Wayland. RelayDesk perlu backend yang diberi akses oleh compositor, atau adaptor ke server remote desktop compositor. Tidak perlu menambah model/agent AI pada PC atau plugin remote pada kedua agent yang sudah mendukung MCP. Dependensi/backend native pada connector tetap diperlukan; ini berbeda dari kemampuan yang sudah tersedia di binary sekarang.

## Keputusan untuk GNOME milik user

- **Aplikasi yang sudah terbuka pada desktop fisik:** prioritaskan helper portal RemoteDesktop + ScreenCast/PipeWire dan input libei, atau adaptor GNOME Desktop Sharing. Keduanya menargetkan sesi yang sudah ada. Keberadaan portal tidak menjamin setiap capability/persistence tersedia pada versi distro user.
- **Sesi kerja remote yang bertahan saat client disconnect:** GNOME **47** memperkenalkan persistent remote login sessions. Sesi remote tetap hidup agar user bisa login kembali dan melanjutkan pekerjaan. Ini kemampuan Remote Login, bukan bukti aplikasi dari login fisik otomatis dipindahkan ke sesi remote. [Release notes GNOME 47](https://release.gnome.org/47/#persistent-remote-desktop-sessions).
- **Headless dan ketahanan service restart:** GNOME 50 menambahkan dukungan Remote Login untuk sesi yang dimulai melalui `gnome-headless-session`, dengan sesi yang tetap berjalan ketika system service remote desktop restart. Ini mode tersendiri, bukan pengganti transparan desktop fisik user. [Release notes GNOME 50](https://release.gnome.org/50/#remote-desktop).
- **Lock screen pada desktop sharing:** upstream menyatakan remote assistance memutus koneksi saat layar dikunci. Jangan mengiklankan akses desktop fisik terkunci hanya berdasarkan persistence portal atau Remote Login. [Konfigurasi upstream](https://github.com/GNOME/gnome-remote-desktop/blob/main/docs/configuration.md).

Rancangan produk: connector mempertahankan sesi capture/input sesuai consent terpisah dari viewer HP dan lease agent. Setelah viewer ditutup, agent masih memakai tool MCP yang sama selama sesi/izin backend valid. Reconnect wajib memverifikasi user/session dan window penanda. Jika compositor menutup sesi atau meminta consent lagi, laporkan keadaan tersebut; jangan berpindah ke desktop lain. Untuk kebutuhan unattended penuh, operator memilih mode Remote Login secara eksplisit dan menjalankan aplikasi di sesi persisten tersebut sejak awal. Belum ada bukti universal bahwa GNOME dapat memindahkan semua aplikasi dari sesi fisik terkunci ke headless tanpa perubahan sesi.

## Jalur standar untuk desktop yang sedang aktif

Portal `org.freedesktop.portal.RemoteDesktop` membuat sesi kontrol, memilih perangkat input, lalu `Start` meminta izin user. Sesi yang sama bisa dipakai untuk `ScreenCast.SelectSources` dan `OpenPipeWireRemote`. `persist_mode=2` meminta izin bertahan sampai dicabut; keberhasilan harus diperiksa, bukan diasumsikan. `restore_token` dipakai sekali dan diganti token baru; monitor hilang atau izin dicabut dapat menyebabkan dialog muncul lagi. Untuk sesi gabungan, persistence dikelola melalui RemoteDesktop, bukan ScreenCast. [API RemoteDesktop upstream](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.RemoteDesktop.html).

Frame diambil dari stream PipeWire yang disetujui. Metadata stream dibutuhkan untuk memetakan pixel screenshot ke koordinat logis desktop, terutama scaling dan banyak monitor. API ScreenCast terbaru memakai `pipewire-serial`/`PW_KEY_TARGET_OBJECT` untuk targeting; implementasi harus mendeteksi versi dan properti yang benar-benar tersedia pada host. [API ScreenCast upstream](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.ScreenCast.html).

Input modern sebaiknya memakai `ConnectToEIS` dan libei. Sesudah koneksi EIS dipakai, metode D-Bus `Notify*` tidak boleh dicampur untuk sesi itu. libei berkomunikasi dengan server input compositor lewat UNIX socket; compositor tetap dapat membatasi input. [RemoteDesktop](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.RemoteDesktop.html), [dokumentasi libei](https://libinput.pages.freedesktop.org/libei/).

Persistence portal tidak menjanjikan akses layar login, user session yang sudah logout, atau layar terkunci pada setiap compositor. Ketiga keadaan itu perlu kontrak dan tes terpisah.

## Pilihan per desktop dan batasnya

| Desktop/backend | Yang didokumentasikan upstream | Implikasi RelayDesk |
| --- | --- | --- |
| GNOME Remote Desktop | Mode remote assistance mengakses sesi aktif dan koneksinya ditutup ketika layar dikunci. Mode GDM Remote Login memberi login grafis melalui RDP. Mode single-user headless memakai sesi grafis headless yang disiapkan terpisah. | Pilih existing-session sharing atau remote login/headless secara eksplisit; jangan menyamakan keduanya atau menjanjikan sesi fisik yang sama. |
| KDE KRdp | README menyediakan share desktop/current session atau mode satu koneksi dengan virtual display. README masih menyatakan membutuhkan user session yang login. | Cek versi Plasma/KRdp terpasang dan uji lock/disconnect; jangan memasang auto-login sebagai solusi bawaan. |
| KDE, pengembangan 2026 | Maintainer menjelaskan perbaikan unattended mode menuju Plasma 6.8: host tampil login screen, output fisik dihilangkan selama remote, virtual output menyesuaikan client. Artikel menyebut pekerjaan ini dasar untuk full headless. | Pengumuman dan master upstream bukan bukti fitur tersedia di paket distro user; verifikasi versi dan perilakunya. |
| wlroots/Sway | `xdg-desktop-portal-wlr` saat diriset hanya mengimplementasikan Screenshot dan ScreenCast. WayVNC mendukung compositor wlroots dan dapat dipakai pada sesi headless; GNOME, KDE, Weston tidak didukung oleh WayVNC. | ScreenCast yang bekerja belum membuktikan input RemoteDesktop bekerja. Pertimbangkan backend WayVNC khusus compositor yang kompatibel. |
| Hyprland | Wiki upstream menyebut server remote yang mendukung protokol capture wlroots, termasuk WayVNC, untuk akses display virtual. | Validasi versi/protokol host; jangan menganggap seluruh fitur RemoteDesktop portal tersedia karena portal screenshot ada. |

Sumber primer: [GNOME configuration](https://github.com/GNOME/gnome-remote-desktop/blob/main/docs/configuration.md), [KRdp README](https://github.com/KDE/krdp), [artikel maintainer KDE 2026-08-25](https://planet.kde.org/david-edmundson-2026-08-25-whats-happening-in-kde-remote-desktop-improved-unattended-mode-and-more/), [portal-wlr README](https://github.com/emersion/xdg-desktop-portal-wlr), [WayVNC](https://github.com/any1/wayvnc), [Hyprland Virtual GPU](https://wiki.hypr.land/configuring/extra/virtual-gpu/).

## Kandidat prototipe tercepat

TigerVNC `w0vncserver` resmi mendukung compositor wlroots atau compositor yang mengimplementasikan RemoteDesktop portal. Ia membagikan display yang sudah ada dan tidak membuat virtual display. [Manual upstream](https://tigervnc.org/doc/w0vncserver.html).

Usulan teknik, belum diuji: jalankan server sebagai user desktop, bind loopback dengan autentikasi, lalu connector mengakses framebuffer/input lewat adaptor RFB lokal. Gateway tetap menyajikan tool MCP yang sama. Jangan mengekspos port VNC baru ke jaringan secara default. Bandingkan dulu dengan helper portal native; adaptor perlu memastikan dukungan clipboard Unicode, pixel format, ukuran framebuffer, resize, cancellation, TLS/auth dan bounded decoding. RFB pada jalur ini tidak menyelesaikan login/headless dengan sendirinya.

## Rencana implementasi yang bisa ditindaklanjuti

1. Target pertama sudah ditetapkan GNOME Wayland. Catat distro, versi GNOME/Mutter/gnome-remote-desktop/portal dan PipeWire. Tambahkan deteksi kemampuan nyata: portal input, capture, persistence, clipboard, dan status consent. `XDG_SESSION_TYPE=wayland` saja tidak cukup. Laporkan tool unsupported atau needs-consent secara terstruktur.
2. Pilih satu host nyata user sebagai acceptance target pertama. Implementasikan backend portal + PipeWire/libei melalui helper native yang dibundel, atau prototipe adaptor `w0vncserver`/WayVNC sesuai compositor. Binary gateway Go tetap tidak perlu ikut memakai runtime desktop. Dependensi harus masuk packaging/version policy; jangan sekadar menonaktifkan penolakan Wayland saat ini.
3. Pertahankan ID perangkat dan izin/per-device lease. Satu sesi capture/input hidup per connector, ditutup saat lokal Stop, cancellation, revoke, atau portal menutup sesi. Simpan restore token secara privat dan atomik, rotasi setelah sukses, serta minta consent lagi bila restore gagal. Jangan simpan token ke log/audit.
4. Definisikan koordinat per-stream, scaling, origin, dan monitor ID sebelum input absolut. Unicode perlu jalur clipboard yang didukung portal/backend atau pengetikan yang diuji terhadap layout; keycode saja tidak menjamin teks literal Unicode.
5. Tes nyata pada GNOME dan/atau KDE versi host: consent pertama, restore tanpa prompt jika diizinkan, restart connector, revoke, lock/unlock, logout, hotplug, mixed DPI, Unicode, chord, drag, screenshot metadata, dan lokal takeover. Uji Sway/Hyprland terpisah. Headless/remote-login diuji sebagai mode terpisah dari desktop sharing.

Tidak ada klaim fitur di atas sudah berfungsi pada PC user. Tidak memakai Context.dev.
