# Riset Windows: UAC secure desktop untuk RelayDesk

Riset diperbarui 2026-11-07 memakai dokumentasi Microsoft dan source upstream. Target user: Windows Server, Windows 11, dan Windows 10; session/aplikasi yang sama harus tetap dapat dikontrol setelah client RDP ditutup. Tidak memakai Context.dev. Ini rancangan implementasi dan kriteria uji; bukan klaim fitur sudah dibuat atau diuji di Windows.

## Kesimpulan

Kontrol UAC secure desktop bisa menjadi target produk remote desktop yang dipasang dengan izin administrator, tetapi connector PowerShell per-user sekarang belum memiliki arsitektur itu. Menjalankan executable sebagai administrator, menambahkan `uiAccess=true`, atau memasang service saja tidak menjamin kontrol secure desktop.

RelayDesk memerlukan broker Windows service yang kecil dan helper terpisah pada sesi/desktop yang benar. Akses sistem untuk jalur capture/input harus sangat terbatas. Perintah shell, file, browser, dan aplikasi biasa tetap berjalan sebagai user tujuan. Broker tidak menjadi endpoint shell SYSTEM.

## Batas Windows yang terdokumentasi

1. **UIPI dan integrity level.** `SendInput` biasa hanya dapat mengirim ke proses dengan tingkat integritas setara atau lebih rendah. Nilai return/GetLastError juga tidak cukup untuk mengidentifikasi UIPI sebagai penyebab kegagalan. Karena itu keberhasilan membuat subprocess PowerShell tidak membuktikan input sampai ke UI elevated. Sumber: [SendInput](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-sendinput).
2. **`uiAccess` memiliki cakupan khusus.** Microsoft mensyaratkan manifest, Authenticode signature, serta pemasangan pada lokasi terlindungi seperti Program Files untuk aplikasi assistive technology. Flag ini bukan pemberian hak admin. Microsoft juga membedakan akses UI elevated dengan UI pada system IL: untuk UI sistem tersebut proses harus diluncurkan di UAC desktop sebagai SYSTEM/system IL, dan `uiAccess` tidak menambah akses pada kasus itu. Panduan tersebut menyatakan `uiAccess` tidak seharusnya dipakai aplikasi non-assistive. Jadi flag ini bukan fondasi universal remote-control RelayDesk. Sumber: [Security Considerations for Assistive Technologies](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-securityoverview).
3. **Session 0 bukan desktop user.** Service modern tidak boleh menggunakan pola legacy interactive service. Microsoft menyarankan aplikasi terpisah pada sesi user dan IPC yang dibatasi ACL. Service yang membuka window interaktif memperluas permukaan serangan. Sumber: [Interactive Services](https://learn.microsoft.com/en-us/windows/win32/services/interactive-services).
4. **Desktop dan session harus cocok.** `OpenInputDesktop` bekerja pada window station proses yang bisa menerima input. Pada session disconnected, handle yang dikembalikan menunjuk desktop yang akan aktif bila user tersambung kembali; handle ini bukan jaminan bahwa capture/input saat disconnected sudah tersedia. Sumber: [OpenInputDesktop](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-openinputdesktop).
5. **Capture secure desktop butuh privilege nyata.** Microsoft mendokumentasikan bahwa `IDXGIOutput1::DuplicateOutput` dapat mengembalikan `E_ACCESSDENIED`; untuk secure desktop diperlukan proses LOCAL_SYSTEM. API ini juga secara eksplisit dapat mengembalikan `DXGI_ERROR_SESSION_DISCONNECTED`. Jadi privilege capture dan kondisi sesi adalah dua prasyarat berbeda. Sumber: [DuplicateOutput](https://learn.microsoft.com/en-us/windows/win32/api/dxgi1_2/nf-dxgi1_2-idxgioutput1-duplicateoutput).
6. **Pergantian desktop memutus object capture.** `AcquireNextFrame` dapat mengembalikan `DXGI_ERROR_ACCESS_LOST` saat desktop/display mode berubah. Duplication object harus dilepas dan dibuat ulang. Screenshot lama tidak boleh dipakai seolah masih merupakan frame desktop aktif. Sumber: [AcquireNextFrame](https://learn.microsoft.com/en-us/windows/win32/api/dxgi1_2/nf-dxgi1_2-idxgioutputduplication-acquirenextframe).
7. **Binding thread memiliki batas.** `SetThreadDesktop` tidak dapat dipakai bebas pada thread yang sudah memiliki window/hook di desktop sebelumnya. Desktop target juga harus milik window station proses. Ini mendukung rancangan worker/thread khusus atau restart helper saat desktop berubah. Sumber: [SetThreadDesktop](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setthreaddesktop).

## Hardening credential Windows Januari 2026

[Microsoft Support KB5080542](https://support.microsoft.com/en-us/servicing/os/windows/docs/2026/01/new-behavior-restricting-certain-applications-to-autofill-credentials-introduced-by-the-windows-janu) membahas update bertanggal **13 Januari 2026 atau lebih baru**, dengan cakupan eksplisit Server 2016/2019/2022/2025, Windows 11 23H2/24H2/25H2, serta Windows 10 1607/1809/21H2/22H2.

Dialog autentikasi menolak input remote/otomatis dari konteks yang tidak dipercaya. Sumber lokal tepercaya yang disebut meliputi keyboard fisik, accessibility app tepercaya dengan UIAccess, dan aplikasi dengan integritas administrator. Ini bukan larangan semua remote input dan bukan jaminan `desktop_type` biasa bisa mengisi kredensial. Microsoft menyebut elevation sebagai mitigasi sementara; rekomendasi permanennya memakai antarmuka autentikasi Windows yang didukung, bukan pengetikan kredensial otomatis.

Implikasi desain RelayDesk: capture secure desktop, kontrol tombol consent, dan input kredensial harus dinilai terpisah. Keberhasilan screenshot sebagai SYSTEM tidak membuktikan credential submission diterima. Jangan mengiklankan autofill hanya karena service/helper memakai SYSTEM atau admin. Deteksi kegagalan, jangan replay password, dan dukung jalur autentikasi yang sesuai backend/OS. Keputusan ini tidak mematikan UAC atau mengganti kebijakan Windows.

## Rancangan yang disarankan

Bagian berikut merupakan keputusan desain yang diturunkan dari batas API di atas, bukan jaminan Microsoft bahwa setiap konfigurasi OS/GPU akan berhasil.

- Installer administrator memasang binary yang ditandatangani, service, dan direktori dengan ACL ketat. Uninstall menghentikan worker/service dan menghapus binary terpasang; kredensial pairing mengikuti pilihan operator.
- Service broker berjalan dengan privilege yang diperlukan untuk mengelola session/helper. Ia tidak mengakses internet melalui antarmuka kontrol baru dan tidak mengekspos shell SYSTEM. Gateway MCP dan dua agent yang ada tetap menjadi antarmuka remote; topologi WireGuard/CHR dipertahankan.
- Helper biasa berjalan dalam token user dan session yang dipilih secara eksplisit. Helper menangani file/exec/browser/GUI biasa dengan hak user itu. Pemilihan session bukan sekadar mengambil console session ID; RDP dan fast user switching dapat menghasilkan beberapa session berbeda.
- Worker capture/input secure desktop adalah komponen terpisah yang hanya dipercaya untuk operasi GUI yang diotorisasi, pada input desktop/session yang tepat. Lifecycle token dan process/desktop handles perlu diaudit serta ditutup. Worker tidak memuat extension atau menerima executable path/command dari agent.
- IPC lokal mengikat identitas service, helper, session ID, device ID, dan lease/job gateway. Gunakan named pipe dengan ACL minimal, pemeriksaan peer, pesan berbatas ukuran, deadline, dan cancellation. User lain tidak boleh mengirim pekerjaan melalui pipe atau mengubah helper/binary yang diluncurkan broker.
- Saat UAC muncul, connector melaporkan transisi desktop. Input baru menunggu frame baru dan identitas desktop yang valid. Capability `desktop_secure_control` baru diumumkan setelah akses nyata tersedia; jika tidak, kembalikan error terstruktur, bukan screenshot hitam dengan status sukses.
- Pertahankan dialog consent/credential Windows. Pisahkan dukungan menerima/menolak consent-only prompt dari dukungan credential-entry; kemampuan terakhir default unavailable sampai jalur autentikasi yang didukung terbukti bekerja pada OS/build itu. Remote operator yang berwenang berinteraksi melalui jalur yang telah diverifikasi; jangan membuat aturan menerima semua UAC. Jangan menonaktifkan UAC/secure desktop, mengganti kebijakan UIAccess, menambahkan auto-login, atau menyimpan password di log untuk menyatakan fitur berhasil.
- Jangan menjalankan ulang klik/pengetikan yang timeout saat transisi UAC. Refresh state dan minta keputusan berikutnya. Lease, policy tool, audit redaction, dan pembatalan yang sudah ada tetap diberlakukan.

Microsoft menyediakan kontrak process/session dan menyatakan `WTSQueryUserToken` hanya untuk service yang sangat dipercaya; penggunaan API itu memerlukan LocalSystem dan privilege yang sesuai. Token user dari API ini tidak otomatis menjadi token SYSTEM secure-desktop helper. [CreateProcessAsUserW](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessasuserw), [WTSQueryUserToken](https://learn.microsoft.com/en-us/windows/win32/api/wtsapi32/nf-wtsapi32-wtsqueryusertoken), dan [Desktop Security and Access Rights](https://learn.microsoft.com/en-us/windows/win32/winstation/desktop-security-and-access-rights) adalah acuan untuk implementasi, bukan alasan memperluas ACL desktop secara global.

Source upstream [RustDesk Windows platform](https://github.com/rustdesk/rustdesk/blob/master/src/platform/windows.rs) dan [RustDesk service lifecycle](https://github.com/rustdesk/rustdesk/blob/master/src/server/service.rs) dapat menjadi pembanding arsitektur service/session/desktop-switch. Source tersebut bukan bukti bahwa RelayDesk sudah kompatibel atau bahwa seluruh teknik internal upstream perlu disalin. Lisensi dan risiko keamanan harus ditinjau sebelum mengadopsi kode.

## Kriteria uji sebelum mengiklankan dukungan

| Kasus Windows nyata | Hasil yang harus dibuktikan |
| --- | --- |
| Aplikasi user normal | Input/screenshot valid, file/exec tetap user; tidak ada eskalasi hak akibat tersedianya broker |
| Aplikasi elevated pada default desktop | Input mencapai target yang benar; kegagalan UIPI terlihat sebagai error, bukan sukses palsu |
| Admin consent UAC pada secure desktop | Frame baru menampilkan prompt; accept dan cancel bekerja; kembali ke desktop normal tanpa replay |
| Standard user UAC credential prompt | Tidak mengasumsikan consent-only; gunakan autentikasi yang didukung atau status unavailable jelas; credential tidak masuk audit, debug log, atau state gateway |
| Windows terpatch Januari 2026+ dengan input virtual tidak dipercaya | Tolakan credential input terdeteksi; tidak mengklaim sukses atau mengulang pengetikan password |
| Jalur credential autentikasi yang didukung backend pada OS terpatch | Buktikan alur yang dipakai, token/session tepat, serta sukses/gagal autentikasi; privilege capture SYSTEM saja tidak dianggap bukti |
| UAC ketika lease dicabut/expired | Tidak ada input baru; worker berhenti sesuai deadline; tidak menerima prompt otomatis |
| UAC bersamaan disconnect/reconnect RDP | Session ID/desktop yang benar dipertahankan atau backend mengembalikan status unavailable yang jelas |
| Switch user, lock/unlock, logoff/login | Hak/sesi helper lama tidak dipakai pada user baru; tidak melewati autentikasi Windows |
| Multimonitor, mixed DPI, rotasi, GPU mode change | Origin, ukuran, mapping input serta capture handle diperbarui; tidak mengirim ke koordinat frame lama |
| Service crash/restart dan upgrade | Helper yatim dihentikan, pekerjaan tidak di-replay, credential dan ACL tetap benar |
| User/session lain atau pipe client palsu | Tidak bisa memanggil worker, meminta shell SYSTEM, atau mengganti target user/session |

Jalankan matriks terpisah pada Windows 10, Windows 11, dan Windows Server yang dipakai user, dengan edisi/build, GPU fisik/VM, serta kebijakan lokal/domain yang benar-benar berlaku. Pada Server, uji beberapa user/session bersamaan; prompt UAC session A tidak boleh mengarahkan input/capture ke session B. Setelah client RDP ditutup, verifikasi window/aplikasi dan session ID yang sama, bukan sekadar desktop console atau session baru. Hasil Linux cross-build tidak memverifikasi satu pun perilaku UAC di tabel ini. Secure desktop/login dan disconnected RDP harus dipisahkan dalam status capability; keberhasilan salah satunya tidak membuktikan lainnya.
