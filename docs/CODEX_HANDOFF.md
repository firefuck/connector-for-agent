# Handoff RelayDesk — 2026-11-08

Baca AGENTS.md, README.md, STATUS.md dan MANUAL_ACCEPTANCE.md. Jangan gunakan
Context.dev. Pertahankan dua agent (VPS dan Hermes), gateway homelab serta
WireGuard/CHR. Tidak ada deployment/credential produksi di workspace.

## Keputusan pengguna

- Target desktop aktif ketika user bekerja: Ubuntu 22 (baseline 22.04/GNOME 42),
  Windows 11 VPS, serta nantinya Windows 10/Server.
- RDP disconnected/terkunci dan UAC otomatis bukan prioritas. Jangan mematikan
  UAC/lock atau memasang auto-login.
- Pengguna memilih menguji Windows secara manual; dokumentasi harus lengkap.
  Tidak perlu meminta lagi akses VPS untuk pekerjaan ini.
- Lisensi proprietary/branding tertanam sudah dipush pada f6432f0. Repo tetap
  public; tidak ada anti-copy absolut. Jangan memasukkan secret ke binary.
- Dashboard tetap cream/terracotta dengan slide drawer per menu, bukan semua
  form di halaman ringkasan. Connector baru mempunyai window kontrol lokal;
  pairing awal tetap wizard console.

## Implementasi lanjutan

`connector.go` menjalankan loop dengan LocalSession/generasi lokal. Pause/Resume
mengganti generation, membatalkan context kerja dan menginvalidasi job/lease pada
gateway. `Device.Paused` mencegah acquire/dispatch. `/connector-state` melakukan
sinkronisasi; `/diagnose` hanya autentikasi read-only tanpa dequeue/heartbeat.
Stop menyimpan marker sebelum menghentikan loop supaya autostart tidak balapan
menyalakan connector lagi. Tidak ada Resume lewat MCP atau Unix control API.

`interactive.go` menghubungkan window helper melalui private stdin/stdout pipes,
Unix control Linux (0700 dir/0600 socket + flock) atau file lock Windows. Normal
connector mulai Pause. `--headless` khusus tes X11/shell dan ditolak pada Wayland.
`--autostart` menghormati marker Stop; start manual menghapus marker.

`native/linux.py` embedded: GTK3, RemoteDesktop + ScreenCast portal, PipeWire lewat
GStreamer, Notify* untuk baseline GNOME 42 (bukan libei). Consent lokal memilih
satu monitor dan kedua input. Capture PNG, scaling pixel/logical, Unicode keysym,
chord release, cancel, signal session Closed dan GNOME lock. Tidak menyimpan
restore token; takeover otomatis GNOME tidak tersedia. Shortcut GNOME dikonfigurasi
secara eksplisit lewat packaging/hotkey-linux.py, menjaga shortcut lain.

`native/windows.ps1`: window WinForms, shortcut Ctrl+Alt+P, low-level hooks untuk
Pause saat input fisik (mengabaikan injected flags). Windows backend memeriksa
input desktop dan menolak desktop selain Default. Seluruh perilaku Windows perlu
acceptance asli. Jangan mengklaim build PE membuktikan UAC/input/installer bekerja.

`frame.go`: connector interaktif mewajibkan screenshot baru sebelum tiap input,
30 detik, sekali pakai, terikat generasi, lease dan agent. MCP meneruskan frame/session/
monitor/scale metadata sebagai teks JSON di samping image. Core Execute/headless
sendiri tidak menerapkan guard ini. Ini bukan sandbox terhadap full-shell agent
atau proses lain yang sudah memiliki hak user yang sama.

`doctor`: JSON schema 1/teks, timeout total 12 detik/probe 5 detik, config regular
file bounded, diagnosis portal read-only, tidak mencetak raw URL/token/error.
Exit 1 untuk check belum ready termasuk needs_consent/unknown.

Packaging menambahkan dependency GUI Linux. Inno Setup source dan script build/
Authenticode Windows tersedia; sertifikat belum disediakan dan Windows build
installer belum dijalankan. Tidak boleh menyebut artifact sudah signed.

## Verifikasi dan pekerjaan berikutnya

Lihat STATUS.md dan test-results.txt untuk hasil yang benar-benar dijalankan.
Tes baru: session_test.go, test_local_controls.py (GTK/Xvfb nyata, shell cancel,
offline Stop dan marker), test_portal_requests.py (Gio + fake portal), serta
test_portal_backend.py (GStreamer nyata + fake portal input).

Mock portal/Xvfb bukan GNOME. Acceptance Ubuntu 22.04 asli mencakup consent
accept/deny/revoke, lock/unlock, scaling/monitor/Unicode. Windows 11/10/Server,
installer, signature dan dua agent asli mengikuti MANUAL_ACCEPTANCE.md oleh user.
Takeover otomatis GNOME tidak tersedia; tombol dan shortcut adalah fallback nyata.
Backlog lanjutan mTLS, streaming, durable jobs, rate limit, audit rotation, broker
UAC/RDP dan sebagainya belum diimplementasikan oleh perubahan ini.

Jalankan gofmt, race suite, vet dan packaging/build.sh sesuai AGENTS. Di workspace
ini Go berada di /workspace/scratch/go/bin; dependency X11/typelib uji diekstrak
ke scratch. Jangan commit dist, config, .env, Python caches atau credential.
