#!/usr/bin/python3
"""Run with dbus-run-session. Real Gio request/Response routing, fake portal."""
import importlib.util
from pathlib import Path
import threading
import unittest
from gi.repository import Gio, GLib

spec=importlib.util.spec_from_file_location('native',Path(__file__).resolve().parents[1]/'internal/core/native/linux.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)

class RequestTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.bus=Gio.bus_get_sync(Gio.BusType.SESSION,None)
        cls.bus.call_sync('org.freedesktop.DBus','/org/freedesktop/DBus','org.freedesktop.DBus','RequestName',GLib.Variant('(su)',(module.DEST,0)),None,Gio.DBusCallFlags.NONE,2000,None)
        cls.loop=GLib.MainLoop();cls.thread=threading.Thread(target=cls.loop.run);cls.thread.start()
    @classmethod
    def tearDownClass(cls):cls.loop.quit();cls.thread.join(timeout=3)
    def test_accept_and_deny(self):
        xml='<node><interface name="org.freedesktop.portal.RemoteDesktop"><method name="CreateSession"><arg type="a{sv}" direction="in"/><arg type="o" direction="out"/></method></interface></node>'
        interface=Gio.DBusNodeInfo.new_for_xml(xml).interfaces[0]
        for code in (0,1,2):
            def called(conn,sender,path,interface,method,params,invocation):
                options=params.unpack()[0]
                handle='/org/freedesktop/portal/desktop/request/'+sender[1:].replace('.','_')+'/'+options['handle_token']
                invocation.return_value(GLib.Variant('(o)',(handle,)))
                conn.emit_signal(sender,handle,'org.freedesktop.portal.Request','Response',GLib.Variant('(ua{sv})',(code,{'session_handle':GLib.Variant('o','/session/test')})))
            registration=self.bus.register_object(module.PATH,interface,called,None,None)
            portal=module.Portal.__new__(module.Portal);portal.bus=self.bus;portal.cancel=threading.Event()
            try:
                if code==0:self.assertEqual(portal.request(module.RD,'CreateSession','(a{sv})',[{}])['session_handle'],'/session/test')
                else:
                    with self.assertRaisesRegex(RuntimeError,'denied or cancelled'):portal.request(module.RD,'CreateSession','(a{sv})',[{}])
            finally:self.bus.unregister_object(registration)

if __name__=='__main__':unittest.main()
