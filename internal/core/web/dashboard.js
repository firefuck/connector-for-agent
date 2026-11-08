'use strict';
const el = id => document.getElementById(id);
let token = '', epoch = 0, busy = false, controller = new AbortController();
let devices = [], agents = [], leases = {};
let drawerMenu = '', drawerReturnFocus = null;
const drawerTitles = {devices: 'Perangkat', agents: 'Agent', pair: 'Tambah PC', agent: 'Tambah agent', result: 'Siap dihubungkan'};
const base = ['devices_list', 'session_acquire', 'session_release', 'system_info', 'desktop_screenshot'];
const policies = {view: base, desktop: [...base, 'desktop_move', 'desktop_click', 'desktop_type', 'desktop_key', 'desktop_scroll', 'browser_open'], full: ['*']};
const hints = {view: 'Bisa melihat layar; tidak mengirim input atau membaca file lewat tool file.', desktop: 'Mouse dan keyboard dapat mengoperasikan aplikasi, termasuk terminal yang terbuka. Izin ini memberi kontrol luas atas sesi desktop.', full: 'Semua tool diizinkan, termasuk shell dan file dengan hak user connector.', custom: 'Sertakan devices_list, session_acquire, session_release untuk alur kontrol. Izin tool bukan sandbox aplikasi atau pembatas folder.'};
function node(tag, text, className) { const n = document.createElement(tag); if (text !== undefined) n.textContent = text; if (className) n.className = className; return n; }
function clearResult() { el('result').replaceChildren(); el('result-help').textContent = ''; el('result-card').hidden = true; }
function openDrawer(menu) {
  if (!token || !drawerTitles[menu]) return;
  if (!el('drawer').open) drawerReturnFocus = document.activeElement;
  if (drawerMenu === 'result' && menu !== 'result') clearResult();
  drawerMenu = menu;
  for (const panel of document.querySelectorAll('[data-panel]')) panel.hidden = panel.dataset.panel !== menu;
  el('drawer-title').textContent = drawerTitles[menu];
  el('drawer-feedback').append(el('feedback'));
  if (!el('drawer').open) el('drawer').showModal();
  document.querySelector('.drawer-body').scrollTop = 0;
  el('drawer-title').focus();
}
function closeDrawer(force = false) {
  if (busy && !force) { el('notice').textContent = 'Tunggu proses selesai, atau kunci dashboard untuk keluar.'; return; }
  if (el('drawer').open) el('drawer').close();
  drawerMenu = ''; clearResult();
  for (const panel of document.querySelectorAll('[data-panel]')) panel.hidden = true;
  el('page-feedback').append(el('feedback'));
  if (drawerReturnFocus?.isConnected && !drawerReturnFocus.disabled) drawerReturnFocus.focus();
  drawerReturnFocus = null;
}
function lock() {
  epoch++; controller.abort(); controller = new AbortController(); token = ''; busy = false;
  closeDrawer(true);
  devices = []; agents = []; leases = {}; clearResult();
  el('token').value = ''; el('login-form').reset(); el('pair-form').reset(); el('agent-form').reset();
  el('pcs').replaceChildren(); el('agents').replaceChildren(); el('device-options').replaceChildren();
  el('device-summary').replaceChildren();
  for (const id of ['online-count', 'agent-count', 'lease-count']) el(id).textContent = '0';
  el('device-count').textContent = 'Belum ada perangkat';
  el('main').hidden = true; el('login').hidden = false; el('notice').textContent = ''; el('error').hidden = true;
  setBusy(false); updatePolicy(); el('token').focus();
}
function setBusy(value) { busy = value; for (const b of document.querySelectorAll('button')) if (!['logout', 'drawer-lock'].includes(b.id)) b.disabled = value; el('main').setAttribute('aria-busy', String(value)); el('drawer').setAttribute('aria-busy', String(value)); }
async function api(path, data = {}) {
  const generation = epoch;
  let response;
  try { response = await fetch('/admin/' + path, {method: 'POST', headers: {Authorization: 'Bearer ' + token, 'Content-Type': 'application/json'}, body: JSON.stringify(data), signal: AbortSignal.any([controller.signal, AbortSignal.timeout(15000)]), cache: 'no-store', redirect: 'error'}); }
  catch (e) { if (generation !== epoch) throw new DOMException('Locked', 'AbortError'); throw Error('Koneksi terputus atau timeout. Periksa daftar sebelum mencoba lagi; perubahan mungkin sudah tersimpan.'); }
  if (generation !== epoch) throw new DOMException('Locked', 'AbortError');
  if (response.status === 401) { lock(); throw Error('Admin token salah atau sudah berubah. Masuk kembali.'); }
  const value = await response.json();
  if (generation !== epoch) throw new DOMException('Locked', 'AbortError');
  if (!response.ok) throw Error(value.error || 'Gateway HTTP ' + response.status);
  return value;
}
function task(fn) { return async event => {
  if (event) event.preventDefault(); if (busy) return;
  const generation = epoch; setBusy(true); el('error').hidden = true; el('notice').textContent = '';
  try { await fn(); } catch (e) { if (e.name !== 'AbortError' && (generation === epoch || !token)) { el('error').textContent = e.message; el('error').hidden = false; } }
  finally { if (generation === epoch) setBusy(false); }
}; }
function secretField(label, value) {
  const group = node('div'); const caption = node('label', label); const row = node('div', undefined, 'secret-row');
  const input = node('input'); input.value = value; input.readOnly = true; input.setAttribute('aria-label', label); input.autocomplete = 'off'; input.spellcheck = false;
  const copy = node('button', 'Salin', 'secondary'); copy.type = 'button';
  copy.onclick = async () => { try { await navigator.clipboard.writeText(value); el('notice').textContent = label + ' disalin.'; } catch { input.focus(); input.select(); el('notice').textContent = 'Teks dipilih. Gunakan Salin / Ctrl+C.'; } };
  row.append(input, copy); group.append(caption, row); el('result').append(group);
}
function showResult(title, help, fields) {
  clearResult(); el('result-title').textContent = title; el('result-help').textContent = help;
  for (const [label, value] of fields) secretField(label, value);
  openDrawer('result');
}
function selectedDevices() { return [...el('device-options').querySelectorAll('input:checked')].map(n => n.value); }
function renderDeviceOptions() {
  const checked = new Set(selectedDevices()); el('device-options').replaceChildren();
  const available = devices.filter(d => !d.revoked);
  if (!available.length) el('device-options').textContent = 'Belum ada PC. Selesaikan pairing dulu.';
  for (const d of available) {
    const label = node('label', undefined, 'check'); const input = node('input'); input.type = 'checkbox'; input.value = d.id; input.checked = checked.has(d.id); input.disabled = el('all-devices').checked;
    label.append(input, node('span', d.name + ' · ' + d.id)); el('device-options').append(label);
  }
}
function online(d) { const age = Date.now() - Date.parse(d.last_seen); return Number.isFinite(age) && age >= -5000 && age < 10000; }
function platformName(d) { return {windows: 'Windows', linux: 'Linux'}[d.platform] || 'Belum terhubung'; }
function permissionName(a) {
  if (a.tools == null || a.tools.includes('*')) return 'Akses penuh';
  if (!a.tools.length) return 'Tidak ada izin';
  for (const preset of ['view', 'desktop']) if (a.tools.length === policies[preset].length && a.tools.every(t => policies[preset].includes(t))) return preset === 'view' ? 'Lihat layar dan info PC' : 'Kontrol desktop dan browser';
  return 'Izin khusus · ' + a.tools.length + ' tool';
}
function renderOverview() {
  const active = devices.filter(d => !d.revoked);
  el('online-count').textContent = active.filter(online).length;
  el('device-count').textContent = active.length ? 'Dari ' + active.length + ' perangkat' : 'Belum ada perangkat';
  el('agent-count').textContent = agents.filter(a => !a.revoked).length;
  el('lease-count').textContent = active.filter(d => leases[d.id] && Date.parse(leases[d.id].until) > Date.now()).length;
  const box = el('device-summary'); box.replaceChildren();
  if (!active.length) {
    const empty = node('div', undefined, 'empty-state'); empty.append(node('strong', 'Mulai dari PC pertama lo.'), node('p', 'Hubungkan PC untuk mulai berbagi akses dengan agent.'));
    const button = node('button', '＋ Tambah PC'); button.onclick = () => openDrawer('pair'); empty.append(button); box.append(empty);
  }
  for (const d of active.slice(0, 5)) {
    const row = node('button', undefined, 'device-row'); row.setAttribute('aria-haspopup', 'dialog'); row.setAttribute('aria-controls', 'drawer'); row.onclick = () => openDrawer('devices');
    const icon = node('span', '▣', 'device-icon'); icon.setAttribute('aria-hidden', 'true');
    const copy = node('span', undefined, 'device-copy'); copy.append(node('strong', d.name || 'PC tanpa nama'), node('small', platformName(d)));
    row.append(icon, copy, node('span', online(d) ? (d.paused ? 'Dijeda lokal' : 'Online') : 'Offline', 'status' + (online(d) ? ' online' : ''))); box.append(row);
  }
}
function render() {
  renderOverview();
  const pcs = el('pcs'); pcs.replaceChildren();
  if (!devices.length) pcs.append(node('p', 'Belum ada PC. Mulai dengan Tambah PC.'));
  for (const d of devices) {
    const row = node('article', undefined, 'entry'); const title = node('div', undefined, 'entry-title');
    title.append(node('strong', d.name || 'PC tanpa nama'), node('span', d.revoked ? 'Dicabut' : online(d) ? (d.paused ? 'Dijeda lokal' : 'Online') : 'Offline', 'status' + (!d.revoked && online(d) ? ' online' : '')));
    const capabilities = (d.capabilities || []).map(c => ({desktop_x11: 'Desktop X11', desktop_windows_interactive: 'Desktop Windows', browser_open: 'Browser', files: 'File', system_exec: 'Terminal'}[c] || c));
    row.append(title, node('code', d.id), node('p', platformName(d) + ' · ' + (capabilities.join(', ') || 'Kemampuan belum dilaporkan'), 'hint'));
    const lease = leases[d.id]; if (lease && Date.parse(lease.until) > Date.now()) { const a = agents.find(a => a.id === lease.owner); row.append(node('p', 'Sesi dipegang: ' + (a ? a.name : lease.owner), 'hint')); }
    if (!d.revoked) row.append(revokeButton(d, 'device')); pcs.append(row);
  }
  const box = el('agents'); box.replaceChildren();
  if (!agents.length) box.append(node('p', 'Belum ada agent. Mulai dengan Tambah agent.'));
  for (const a of agents) {
    const row = node('article', undefined, 'entry'); const title = node('div', undefined, 'entry-title');
    title.append(node('strong', a.name || 'Agent tanpa nama'), node('span', a.revoked ? 'Dicabut' : 'Aktif', 'status')); row.append(title, node('code', a.id));
    const scope = (a.devices || []).map(id => id === '*' ? 'Semua PC sekarang dan nanti' : (devices.find(d => d.id === id)?.name || id));
    row.append(node('p', 'PC: ' + scope.join(', '), 'hint'), node('p', 'Izin: ' + permissionName(a), 'hint'));
    if (!a.revoked) {
      const details = node('details'); details.append(node('summary', 'Ubah izin tool'));
      const input = node('input'); input.value = a.tools == null ? '*' : a.tools.join(','); input.setAttribute('aria-label', 'Izin tool ' + a.name);
      const save = node('button', 'Simpan izin'); save.onclick = task(async () => { if (!confirm('Ubah izin ' + a.name + '? Sesi aktif agent ini akan dibatalkan.')) return; await api('permissions', {id: a.id, tools: splitTools(input.value)}); await refresh(); el('notice').textContent = 'Izin diperbarui. Agent perlu mengambil sesi baru.'; });
      details.append(input, node('p', 'Kosong menolak semua; * memberi akses penuh.', 'hint'), save); row.append(details, revokeButton(a, 'agent'));
    }
    box.append(row);
  }
  renderDeviceOptions();
}
function revokeButton(item, kind) { const b = node('button', 'Cabut akses', 'danger'); b.onclick = task(async () => { if (!confirm('Cabut akses ' + item.name + '? Untuk menghubungkan lagi, buat akses baru.')) return; await api('revoke', {kind, id: item.id}); await refresh(); el('notice').textContent = 'Akses dicabut.'; }); return b; }
async function refresh() {
  const data = await api('list'); devices = data.devices.sort((a,b) => a.name.localeCompare(b.name)); agents = data.agents.sort((a,b) => a.name.localeCompare(b.name)); leases = data.leases || {};
  render(); el('updated').textContent = 'Diperbarui ' + new Date().toLocaleTimeString('id-ID');
}
function splitTools(s) { return s.split(',').map(s => s.trim()).filter(Boolean); }
function updatePolicy() { el('custom-policy').hidden = el('preset').value !== 'custom'; el('policy-hint').textContent = hints[el('preset').value]; }
el('login-form').onsubmit = task(async () => { token = el('token').value.trim(); el('token').value = ''; await refresh(); el('login').hidden = true; el('main').hidden = false; });
el('logout').onclick = lock;
el('drawer-lock').onclick = lock;
el('drawer-close').onclick = () => closeDrawer();
el('drawer').addEventListener('cancel', event => { event.preventDefault(); closeDrawer(); });
el('drawer').addEventListener('keydown', event => {
  if (event.key !== 'Tab') return;
  const controls = [...el('drawer').querySelectorAll('button:not(:disabled), input:not(:disabled), select:not(:disabled), summary, a[href], [tabindex="0"]')].filter(n => n.getClientRects().length);
  const first = controls[0], last = controls[controls.length - 1];
  if (!first) { event.preventDefault(); el('drawer-title').focus(); return; }
  if (event.shiftKey && (document.activeElement === first || !controls.includes(document.activeElement))) {
    event.preventDefault(); last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault(); first.focus();
  }
});
el('drawer').addEventListener('click', event => {
  const rect = el('drawer').getBoundingClientRect();
  if (event.target === el('drawer') && (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom)) closeDrawer();
});
for (const button of document.querySelectorAll('[data-drawer]')) button.onclick = () => openDrawer(button.dataset.drawer);
document.querySelector('.nav-home').onclick = () => el('overview-title').scrollIntoView({block: 'start'});
el('refresh').onclick = task(refresh);
el('clear-result').onclick = () => closeDrawer();
el('preset').onchange = updatePolicy;
el('all-devices').onchange = renderDeviceOptions;
el('pair-form').onsubmit = task(async () => { const name = el('pcname').value.trim(); if (!name) throw Error('Isi nama PC terlebih dahulu.'); const result = await api('pair', {name}); showResult('Kode pairing untuk ' + name, 'Masukkan di wizard RelayDesk pada PC tujuan. Berlaku sampai ' + new Date(Date.now() + result.expires_in * 1000).toLocaleTimeString('id-ID') + ', sekali pakai. Alamat gateway harus dapat dijangkau PC tujuan.', [['Alamat gateway', location.origin], ['Kode pairing', result.code]]); });
el('agent-form').onsubmit = task(async () => { const name = el('agentname').value.trim(); const scope = el('all-devices').checked ? ['*'] : selectedDevices(); if (!name || !scope.length) throw Error('Isi nama agent dan pilih minimal satu PC.'); const tools = el('preset').value === 'custom' ? splitTools(el('tools').value) : policies[el('preset').value]; const result = await api('agent', {name, devices: scope, tools}); showResult('Akses baru untuk ' + name, 'Pasang URL MCP dan bearer token di konfigurasi agent. Simpan token sebelum menutup hasil ini.', [['URL MCP', location.origin + '/mcp'], ['ID agent', result.id], ['Token agent', result.token]]); await refresh(); });
setInterval(() => {
  const browsingList = ['devices', 'agents'].includes(drawerMenu) && !el('drawer').querySelector('details[open]') && !el('drawer').querySelector('.entry:focus-within');
  if (token && !busy && !document.hidden && !el('main').hidden && (!el('drawer').open || browsingList)) task(refresh)();
}, 5000);
window.addEventListener('pagehide', lock);
