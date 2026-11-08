# RelayDesk desktop helper. Private stdin/stdout JSON pipes; no network listener.
# Ubuntu 22.04 baseline: Gtk3, portal RemoteDesktop Notify* and PipeWire.
import base64
import json
import os
import queue
import secrets
import struct
import sys
import threading
import time

import gi
gi.require_version('Gtk', '3.0')
from gi.repository import Gio, GLib, Gtk, Gdk

DEST = 'org.freedesktop.portal.Desktop'
PATH = '/org/freedesktop/portal/desktop'
RD = 'org.freedesktop.portal.RemoteDesktop'
SC = 'org.freedesktop.portal.ScreenCast'
WAYLAND = os.environ.get('XDG_SESSION_TYPE') == 'wayland'
MAX_FRAME = 8 * 1024 * 1024
output_lock = threading.Lock()

def emit(**data):
    with output_lock:
        print(json.dumps(data, ensure_ascii=True), flush=True)

def probe():
    checks = []
    def add(name, state, detail): checks.append(dict(name=name, state=state, detail=detail))
    try:
        gi.require_version('Gst', '1.0')
        from gi.repository import Gst
        Gst.init(None)
        for factory in ('pipewiresrc', 'videoconvert', 'pngenc', 'appsink'):
            add(factory, 'ready' if Gst.ElementFactory.find(factory) else 'missing', 'GStreamer element availability; capture not started.')
        bus = Gio.bus_get_sync(Gio.BusType.SESSION, None)
        for interface in (RD, SC):
            v = bus.call_sync(DEST, PATH, 'org.freedesktop.DBus.Properties', 'Get',
                GLib.Variant('(ss)', (interface, 'version')), None, Gio.DBusCallFlags.NONE, 2000, None)
            add(interface.rsplit('.', 1)[-1], 'ready', 'Portal version %s available.' % v.unpack()[0])
        add('desktop_consent', 'needs_consent', 'Use Resume in the local RelayDesk window and approve the GNOME sharing dialog. No consent requested by doctor.')
    except Exception:
        add('portal', 'unknown', 'Session bus, portal or GStreamer unavailable. Install backend dependencies and log into GNOME.')
    print(json.dumps(checks))

if '--probe' in sys.argv:
    probe()
    sys.exit(0)

class Portal:
    def __init__(self, lost):
        gi.require_version('Gst', '1.0')
        from gi.repository import Gst
        self.Gst = Gst
        Gst.init(None)
        self.bus = Gio.bus_get_sync(Gio.BusType.SESSION, None)
        self.session = None
        self.pipeline = None
        self.fd = None
        self.stream = None
        self.logical = None
        self.frame_size = None
        self.frame_id = None
        self.lost = lost
        self.cancel = threading.Event()
        self.closed_subscription = None
        self.request_path = None
        self.lock_subscription = self.bus.signal_subscribe('org.gnome.ScreenSaver','org.gnome.ScreenSaver','ActiveChanged','/org/gnome/ScreenSaver',None,Gio.DBusSignalFlags.NONE,self.lock_changed,None)

    def lock_changed(self,*args):
        if args[-2].unpack()[0]:
            self.cancel.set();self.frame_id=None
            GLib.idle_add(self.lost,'Desktop locked. Unlock and Resume locally; old actions are not replayed.')

    def call(self, interface, method, signature, args, path=PATH):
        return self.bus.call_sync(DEST, path, interface, method, GLib.Variant(signature, args),
            None, Gio.DBusCallFlags.NONE, 3000, None)

    def request(self, interface, method, signature, args):
        token = 'r' + secrets.token_hex(12)
        args[-1]['handle_token'] = GLib.Variant('s', token)
        sender = self.bus.get_unique_name()[1:].replace('.', '_')
        path = '/org/freedesktop/portal/desktop/request/' + sender + '/' + token
        event, response = threading.Event(), []
        def received(*a):
            response.extend(a[-2].unpack())
            event.set()
        subscription = self.bus.signal_subscribe(DEST, 'org.freedesktop.portal.Request', 'Response', path,
            None, Gio.DBusSignalFlags.NONE, received, None)
        self.request_path = path
        try:
            actual = self.call(interface, method, signature, tuple(args)).unpack()[0]
            if actual != path: raise RuntimeError('Portal returned an unexpected request handle')
            until = time.monotonic() + 120
            while not event.wait(.1):
                if self.cancel.is_set() or time.monotonic() > until:
                    try: self.call('org.freedesktop.portal.Request', 'Close', '()', (), path)
                    except Exception: pass
                    raise RuntimeError('Sharing request cancelled or timed out')
            if response[0] != 0: raise RuntimeError('Sharing consent denied or cancelled')
            return response[1]
        finally:
            self.request_path = None
            self.bus.signal_unsubscribe(subscription)

    def start(self):
        self.close()
        self.cancel.clear()
        token = 's' + secrets.token_hex(12)
        result = self.request(RD, 'CreateSession', '(a{sv})', [{'session_handle_token': GLib.Variant('s', token)}])
        self.session = result['session_handle']
        def closed(*args):
            self.cancel.set()
            self.frame_id = None
            GLib.idle_add(self.lost, 'Sharing permission ended. Resume to request new consent.')
        self.closed_subscription = self.bus.signal_subscribe(DEST, 'org.freedesktop.portal.Session', 'Closed',
            self.session, None, Gio.DBusSignalFlags.NONE, closed, None)
        self.request(RD, 'SelectDevices', '(oa{sv})', [self.session, {'types': GLib.Variant('u', 3)}])
        self.request(SC, 'SelectSources', '(oa{sv})', [self.session, {
            'types': GLib.Variant('u', 1), 'multiple': GLib.Variant('b', False), 'cursor_mode': GLib.Variant('u', 2)}])
        result = self.request(RD, 'Start', '(osa{sv})', [self.session, '', {}])
        if result.get('devices', 0) & 3 != 3: raise RuntimeError('Both keyboard and pointer permission are required')
        streams = result.get('streams', [])
        if len(streams) != 1: raise RuntimeError('Select exactly one monitor')
        self.stream, props = streams[0]
        self.logical = props.get('size')
        if not self.logical or min(self.logical) <= 0: raise RuntimeError('Portal did not provide logical monitor dimensions')
        result, fds = self.bus.call_with_unix_fd_list_sync(DEST, PATH, SC, 'OpenPipeWireRemote',
            GLib.Variant('(oa{sv})', (self.session, {})), None, Gio.DBusCallFlags.NONE, 3000, None, None)
        self.fd = fds.get(result.unpack()[0])
        self.pipeline = self.Gst.parse_launch('pipewiresrc name=source do-timestamp=true ! videoconvert ! pngenc ! appsink name=sink max-buffers=1 drop=true sync=false')
        source = self.pipeline.get_by_name('source')
        source.set_property('fd', self.fd)
        source.set_property('path', str(self.stream))
        self.sink = self.pipeline.get_by_name('sink')
        if self.pipeline.set_state(self.Gst.State.PLAYING) == self.Gst.StateChangeReturn.FAILURE:
            raise RuntimeError('PipeWire capture could not start')
        self.frame_id = None

    def close(self):
        self.cancel.set()
        self.frame_id = None
        if self.closed_subscription is not None:
            self.bus.signal_unsubscribe(self.closed_subscription)
            self.closed_subscription = None
        if self.pipeline is not None:
            self.pipeline.set_state(self.Gst.State.NULL)
            self.pipeline = None
        if self.fd is not None:
            os.close(self.fd)
            self.fd = None
        if self.session:
            try: self.call('org.freedesktop.portal.Session', 'Close', '()', (), self.session)
            except Exception: pass
            self.session = None

    def notify(self, method, suffix, *args):
        if self.cancel.is_set() or not self.session: raise RuntimeError('Desktop consent is inactive')
        self.call(RD, method, '(oa{sv}' + suffix + ')', (self.session, {}, *args))

    def release(self,method,value):
        # Release already-pressed input even if local Pause cancelled the job.
        # Closing the portal session is the final fallback if consent was revoked.
        if self.session:
            try: self.call(RD,method,'(oa{sv}iu)',(self.session,{},value,0))
            except Exception: self.cancel.set()

    def execute(self, action, args, cancelled):
        def check():
            if cancelled.is_set() or self.cancel.is_set(): raise RuntimeError('Desktop operation cancelled; inspect state before retrying')
        check()
        if not self.pipeline: raise RuntimeError('needs_consent: press Resume locally')
        if action == 'desktop_screenshot':
            # Drain the queue, then request a new buffer. Never return a frame
            # retained from a previous request or portal session.
            self.sink.emit('try-pull-sample', 0)
            until = time.monotonic() + 5
            sample = None
            while sample is None and time.monotonic() < until:
                check()
                sample = self.sink.emit('try-pull-sample', 100 * self.Gst.MSECOND)
            if sample is None: raise RuntimeError('No fresh PipeWire frame; screen may be locked or stream stopped')
            buf = sample.get_buffer()
            if buf.get_size() > MAX_FRAME: raise RuntimeError('Screenshot exceeds 8 MiB')
            png = buf.extract_dup(0, buf.get_size())
            if png[:8] != b'\x89PNG\r\n\x1a\n': raise RuntimeError('Invalid captured PNG')
            width, height = struct.unpack('>II', png[16:24])
            if width <= 0 or height <= 0: raise RuntimeError('Invalid capture dimensions')
            self.frame_size = (width, height)
            self.frame_id = secrets.token_hex(16)
            self.frame_time = time.monotonic()
            return dict(mimeType='image/png', base64=base64.b64encode(png).decode(), width=width, height=height,
                origin_x=0, origin_y=0, logical_width=self.logical[0], logical_height=self.logical[1],
                scale_x=width/self.logical[0], scale_y=height/self.logical[1], monitor_id=str(self.stream),
                session_id=self.session.rsplit('/',1)[-1], frame_id=self.frame_id, captured_at=time.time(),
                coordinates='pixels relative to the selected monitor')
        if not self.frame_id or time.monotonic()-self.frame_time > 30:
            raise RuntimeError('Take a fresh screenshot before input (maximum frame age 30 seconds)')
        if args.get('frame_id') and args['frame_id'] != self.frame_id: raise RuntimeError('Stale frame_id')
        # Reject a monitor resize observed since the accepted frame.
        latest=self.sink.emit('try-pull-sample',0)
        if latest is not None:
            b=latest.get_buffer()
            if b.get_size()>=24:
                header=b.extract_dup(0,24)
                if struct.unpack('>II',header[16:24])!=self.frame_size:
                    self.frame_id=None
                    raise RuntimeError('Monitor dimensions changed; take a new screenshot')
        # Consume before sending input: even a partial/failed input needs a new frame.
        self.frame_id = None
        if action in ('desktop_move','desktop_click'):
            x,y = args['x'], args['y']
            w,h = self.frame_size
            if not (0 <= x < w and 0 <= y < h): raise RuntimeError('Coordinates outside selected monitor')
            self.notify('NotifyPointerMotionAbsolute','udd',self.stream,x*self.logical[0]/w,y*self.logical[1]/h)
            if action == 'desktop_click':
                button = {'left':272,'right':273,'middle':274}.get(args.get('button','left'))
                if button is None: raise RuntimeError('Invalid mouse button')
                count=args.get('count',1)
                if count not in (1,2): raise RuntimeError('count must be 1 or 2')
                for _ in range(count):
                    check()
                    self.notify('NotifyPointerButton','iu',button,1)
                    try: check()
                    finally: self.release('NotifyPointerButton',button)
                    time.sleep(.08)
        elif action == 'desktop_scroll':
            if abs(args['steps'])>100: raise RuntimeError('steps must be -100..100')
            self.notify('NotifyPointerAxisDiscrete','ui',0,-args['steps'])
        elif action in ('desktop_key','desktop_type'):
            if action == 'desktop_type':
                keys = (Gdk.KEY_Return if c in ('\n','\r') else Gdk.KEY_Tab if c=='\t' else Gdk.unicode_to_keyval(ord(c)) for c in args['text'].replace('\r\n','\n'))
                for key in keys:
                    check()
                    self.notify('NotifyKeyboardKeysym','iu',key,1)
                    self.release('NotifyKeyboardKeysym',key)
            else:
                aliases={'ctrl':'Control_L','alt':'Alt_L','shift':'Shift_L','win':'Super_L','enter':'Return','esc':'Escape','escape':'Escape','tab':'Tab','space':'space','backspace':'BackSpace','delete':'Delete','left':'Left','right':'Right','up':'Up','down':'Down','home':'Home','end':'End','pageup':'Page_Up','pagedown':'Page_Down'}
                keys=[]
                for k in args['key'].lower().split('+'):
                    key=Gdk.keyval_from_name(aliases.get(k,k.upper() if k.startswith('f') and k[1:].isdigit() else k))
                    if not key or key == 0xffffff: raise RuntimeError('Unsupported key chord')
                    keys.append(key)
                pressed=[]
                try:
                    for key in keys:
                        check(); self.notify('NotifyKeyboardKeysym','iu',key,1); pressed.append(key)
                finally:
                    for key in reversed(pressed):
                        self.release('NotifyKeyboardKeysym',key)
        else: raise RuntimeError('Unsupported portal operation')
        return {'status':'sent','verification':'Take a new screenshot to verify the result'}

class Window:
    def __init__(self):
        self.paused=True
        self.busy=False
        self.portal=None
        self.cancelled=threading.Event()
        self.requests=queue.Queue(maxsize=4)
        self.pending={}
        self.pending_lock=threading.Lock()
        self.generation=0
        self.window=Gtk.Window(title='RelayDesk — kontrol lokal')
        self.window.set_default_size(420,260)
        self.window.set_border_width(20)
        self.window.connect('delete-event', self.stop)
        box=Gtk.Box(orientation=Gtk.Orientation.VERTICAL,spacing=12)
        self.window.add(box)
        self.label=Gtk.Label(label='Dijeda. Klik Resume untuk mengizinkan agent bekerja.')
        self.label.set_line_wrap(True); self.label.set_xalign(0)
        box.pack_start(self.label,True,True,0)
        self.identity=Gtk.Label(label='Gateway belum terhubung'); self.identity.set_xalign(0); self.identity.set_line_wrap(True)
        box.pack_start(self.identity,True,True,0)
        hint=Gtk.Label(label='Pause menghentikan shell, file dan kontrol desktop.\nTakeover otomatis tidak tersedia pada portal GNOME.\nShortcut jendela: Ctrl+Alt+P. Shortcut global: lihat panduan.')
        hint.set_line_wrap(True); hint.set_xalign(0); box.pack_start(hint,False,False,0)
        buttons=Gtk.Box(spacing=8); box.pack_start(buttons,False,False,0)
        for label,callback in [('Pause',self.pause),('Resume',self.resume),('Stop',self.stop)]:
            button=Gtk.Button(label=label); button.connect('clicked',callback); buttons.pack_start(button,True,True,0)
        self.window.connect('key-press-event',self.key)
        self.window.show_all()
        threading.Thread(target=self.reader,daemon=True).start()
        threading.Thread(target=self.worker,daemon=True).start()
        emit(event='ready',backend='portal' if WAYLAND else 'x11',takeover=False)

    def key(self,window,event):
        if event.state & Gdk.ModifierType.CONTROL_MASK and event.state & Gdk.ModifierType.MOD1_MASK and Gdk.keyval_name(event.keyval).lower()=='p':
            self.pause(); return True
        return False

    def pause(self,*args):
        self.generation+=1
        self.paused=True; self.cancelled.set()
        with self.pending_lock:
            for cancelled in self.pending.values(): cancelled.set()
        if self.portal: self.portal.cancel.set()
        self.label.set_text('Dijeda lokal. Job lama tidak akan dilanjutkan.')
        emit(event='pause')
        return False

    def lost(self,message):
        self.pause(); self.label.set_text(message); return False

    def resume(self,*args):
        if self.busy: return
        if not self.paused: return
        self.busy=True
        generation=self.generation
        self.label.set_text('Meminta izin berbagi monitor…' if WAYLAND else 'Melanjutkan…')
        def start():
            try:
                if WAYLAND:
                    if self.portal is None: self.portal=Portal(self.lost)
                    self.portal.start()
                GLib.idle_add(self.resumed,generation)
            except Exception as e:
                if self.portal: self.portal.close()
                GLib.idle_add(self.failed,str(e))
        threading.Thread(target=start,daemon=True).start()

    def resumed(self,generation):
        self.busy=False
        if generation!=self.generation:
            if self.portal: self.portal.close()
            return False
        self.paused=False
        self.label.set_text('Siap. Agent perlu mengambil lease baru.')
        emit(event='resume'); return False

    def failed(self,message):
        self.busy=False; self.paused=True
        self.label.set_text('Belum siap: '+message[:300]); emit(event='pause'); return False

    def stop(self,*args):
        self.pause(); emit(event='stop')
        Gtk.main_quit(); return True

    def status(self,data):
        # Plain labels, never markup; display only identities and activity class.
        self.identity.set_text('PC: %s\nAgent: %s' % (data.get('device_name') or data.get('device_id') or 'belum diketahui', data.get('agent_name') or data.get('agent_id') or 'tidak aktif'))
        if not self.paused: self.label.set_text('Status: '+data.get('state','unknown'))
        return False

    def reader(self):
        try:
            while True:
                line=sys.stdin.buffer.readline(12*1024*1024+1)
                if not line or len(line)>12*1024*1024: break
                data=json.loads(line)
                if data.get('op')=='status': GLib.idle_add(self.status,data['status'])
                elif data.get('op')=='cancel':
                    with self.pending_lock:
                        cancelled=self.pending.get(data.get('id'))
                        if cancelled: cancelled.set()
                elif data.get('op')=='pause': GLib.idle_add(self.pause)
                elif data.get('op')=='stop': GLib.idle_add(self.stop)
                elif data.get('op')=='execute':
                    cancelled=threading.Event()
                    with self.pending_lock:
                        self.pending[data['id']]=cancelled
                    data['_cancelled']=cancelled
                    try: self.requests.put_nowait(data)
                    except queue.Full:
                        with self.pending_lock: self.pending.pop(data['id'],None)
                        emit(id=data['id'],error='Desktop helper busy')
        finally: GLib.idle_add(self.stop)

    def worker(self):
        while True:
            data=self.requests.get()
            try:
                if self.paused or not self.portal: raise RuntimeError('needs_consent: resume locally first')
                result=self.portal.execute(data['action'],data.get('args',{}),data['_cancelled'])
                emit(id=data['id'],data=result)
            except Exception as e: emit(id=data['id'],error=str(e)[:500])
            finally:
                with self.pending_lock: self.pending.pop(data['id'],None)

if __name__=='__main__':
    window=Window()
    try: Gtk.main()
    finally:
        if window.portal: window.portal.close()
