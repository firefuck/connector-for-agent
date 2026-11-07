# Riset Windows: desktop dan aplikasi tetap dapat dikontrol setelah viewer ditutup

Tanggal: 2026-11-06. Target user: Windows Server, Windows 10 dan Windows 11; sesi dan aplikasi yang sudah terbuka harus tetap menjadi target. Ini hasil riset/desain, bukan implementasi atau pengujian Windows. Topologi gateway homelab, dua agent, dan WireGuard/CHR dipertahankan.

## Temuan platform

- Disconnect berbeda dari logoff: `WTSDisconnectSession` melepas client tanpa menutup sesi. Proses dapat tetap berjalan, tetapi kebijakan timeout/logoff dapat mengakhiri sesi kemudian. Sumber: [WTSDisconnectSession](https://learn.microsoft.com/en-us/windows/win32/api/wtsapi32/nf-wtsapi32-wtsdisconnectsession), [session time limits](https://learn.microsoft.com/en-us/windows/client-management/mdm/policy-csp-admx-terminalserver#ts_sessions_disconnected_timeout_2).
- Keberadaan proses bukan bukti tersedianya GUI yang bisa ditangkap. Desktop Duplication secara eksplisit dapat mengembalikan `DXGI_ERROR_SESSION_DISCONNECTED`; menambah service atau virtual monitor tidak dengan sendirinya membuktikan capture sesi RDP disconnected dapat dilakukan. [DuplicateOutput](https://learn.microsoft.com/en-us/windows/win32/api/dxgi1_2/nf-dxgi1_2-idxgioutput1-duplicateoutput), [Indirect Display Driver](https://learn.microsoft.com/en-us/windows-hardware/drivers/display/indirect-display-driver-model-overview).
- Identitas sesi harus eksplisit: WTS membedakan active, connected, disconnected, lock/unlock serta logon/logoff. Service menerima perubahan melalui HandlerEx; helper GUI bisa memakai WTS notifications. [WTS_CONNECTSTATE_CLASS](https://learn.microsoft.com/en-us/windows/win32/api/wtsapi32/ne-wtsapi32-wts_connectstate_class), [WTSRegisterSessionNotification](https://learn.microsoft.com/en-us/windows/win32/api/wtsapi32/nf-wtsapi32-wtsregistersessionnotification), [WM_WTSSESSION_CHANGE](https://learn.microsoft.com/en-us/windows/win32/termserv/wm-wtssession-change).
- RDS menyediakan policy single-session-per-user agar login berikutnya kembali ke sesi yang ada. Jangan mengubah policy domain/user diam-diam. Edisi Windows, RDS role/licensing dan policy menentukan koneksi yang diizinkan; host Windows Home bukan target built-in RDP. [TS_SINGLE_SESSION](https://learn.microsoft.com/en-us/windows/client-management/mdm/policy-csp-admx-terminalserver#ts_single_session), [persyaratan Remote Desktop](https://support.microsoft.com/en-us/windows/experience/connectivity-networking/how-to-use-remote-desktop).

## Rancangan yang direkomendasikan untuk RelayDesk

Bagian ini merupakan rekomendasi engineering berdasarkan batas API di atas, belum bukti semua varian OS bekerja.

1. Pisahkan umur viewer/agent request dari koneksi desktop. Worker RDP yang dikelola gateway mempertahankan koneksi terautentikasi ke Windows melalui WireGuard, walaupun browser HP ditutup atau satu tool call selesai. Tindakan tetap memerlukan lease dan izin; koneksi retained tidak memberi hak input tanpa lease.
2. Untuk sesi yang terlanjur disconnected, reconnect secara terautentikasi sebagai pemilik sesi, lalu verifikasi session ID, user SID dan aplikasi penanda sebelum mengirim input. Jika tidak bisa membuktikan itu sesi yang diminta, jangan mengirim input atau diam-diam membuka sesi lain.
3. Gambar/input untuk mode ini melalui channel RDP (contoh implementasi worker memakai FreeRDP), bukan mengandalkan `CopyFromScreen`/`SendInput` di sesi yang disconnected. FreeRDP menyediakan mouse, keyboard/Unicode dan auto-reconnect: [API input](https://pub.freerdp.com/api/include_2freerdp_2input_8h.html), [implementasi client/reconnect](https://github.com/FreeRDP/FreeRDP/blob/master/client/common/client.c). Kegagalan auth/policy harus ditampilkan, bukan diulang tanpa batas.
4. Console desktop dan RDP desktop menjadi target berbeda yang terlihat jelas. Menghubungkan akun yang sama tidak otomatis menjamin session ID yang sama pada semua konfigurasi Server/Windows client. Perangkat yang sama tidak boleh berarti bebas mengontrol semua sesi user.
5. Service broker Windows menginventaris sesi, mempertahankan connector/background operation, serta mengelola helper dalam sesi yang dipilih. User helper menjalankan aplikasi/file/exec sebagai user; privileged helper khusus secure desktop dibahas terpisah di RESEARCH_WINDOWS_UAC.md. Menjalankan semua shell agent sebagai SYSTEM bukan desain yang dipilih.
6. Kredensial RDP dikelola eksplisit, tidak dikirim ke model, argumen CLI, audit atau state plaintext. Tentukan vault/encrypted credential reference dan lifecycle revocation sebelum mode unattended diaktifkan. Host key/certificate/NLA harus diverifikasi. Jangan menyediakan opsi default untuk mengabaikan validasi sertifikat.
7. Observer viewer disconnect tidak mencabut pairing. Stop/revoke/takeover lokal menghapus hak input dan membatalkan job; penghentian worker tidak boleh logoff user otomatis. Bedakan transport reconnect dengan replay aksi: reconnect boleh, aksi klik/exec yang hasilnya ambigu tidak diulang otomatis.

Mempertahankan koneksi backend membuat target secara teknis kembali/terus connected. Itu memenuhi tujuan pengguna tetap mengontrol aplikasi saat viewer lepas, tetapi bukan klaim Windows menyediakan capture universal saat sesi benar-benar tetap berstatus WTSDisconnected. Jika syaratnya sesi harus terus WTSDisconnected sambil menerima semua GUI input, belum ada dasar dokumentasi untuk menjamin itu.

`tscon /dest:console` bukan fondasi produk: transfer dapat mengubah desktop yang terlihat lokal dan punya syarat izin/kompatibilitas. Bahkan dokumentasi tscon tidak memberi jaminan universal untuk console target. Jangan menjalankannya otomatis, membiarkan desktop unlocked, menonaktifkan UAC, mengubah timeout domain, atau menyalakan auto-login. [Dokumentasi tscon](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/tscon).

## Kriteria lulus sebelum klaim dukungan

| Kasus | Bukti yang harus dikumpulkan |
| --- | --- |
| Viewer HP/agent client ditutup | Worker tetap connected; session ID/SID dan PID aplikasi sama; screenshot baru dan input berhasil |
| mstsc user ditutup terlebih dahulu | Backend reconnect ke session ID yang sama; penanda aplikasi lama tetap ada; tidak mengontrol sesi baru |
| Jaringan RDP putus di tengah klik/exec | Status degraded; aksi tidak direplay; reconnect memverifikasi ulang sesi dan lease |
| Lock/unlock, disconnect/reconnect, pergantian monitor/DPI | State event terdeteksi; buffer capture lama tidak dipakai sebagai frame baru; desktop target benar |
| UAC consent dan credential prompt | Desktop yang benar terlihat; kebutuhan kredensial/consent tetap berlaku; helper tidak mencatat password |
| Logoff/policy timeout | Sesi dinyatakan ended; tidak menjanjikan mempertahankan aplikasi yang OS sudah tutup |
| Server dua user/sesi | ACL sesi/SID dan lease mencegah akses silang; reconnect tidak mengambil sesi user lain |
| Windows 10/11 | Catat edisi/build dan batas satu sesi; tidak memakai patch RDP untuk menambah sesi |
| Headless/monitor dilepas | Verifikasi output display benar-benar tersedia; driver IDD hanya jika dipasang eksplisit dan diuji |

Matriks wajib dijalankan pada build/edisi Windows 10, Windows 11 dan Windows Server yang benar-benar digunakan. Belum ada target atau VM Windows yang diuji pada sesi riset ini.
