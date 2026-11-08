#!/usr/bin/python3
"""Tests actual GStreamer capture/PNG and portal input mapping with a fake bus.
This does not establish GNOME consent, compositor injection or Ubuntu acceptance.
"""
import importlib.util
import pathlib
import threading
import time
import unittest

path=pathlib.Path(__file__).resolve().parents[1]/'internal/core/native/linux.py'
spec=importlib.util.spec_from_file_location('relaydesk_native',path)
helper=importlib.util.module_from_spec(spec);spec.loader.exec_module(helper)
import gi
gi.require_version('Gst','1.0')
from gi.repository import Gst
Gst.init(None)

class BackendTest(unittest.TestCase):
    def setUp(self):
        p=helper.Portal.__new__(helper.Portal)
        p.Gst=Gst;p.cancel=threading.Event();p.session='/session/test';p.stream=42;p.logical=(400,300)
        p.pipeline=Gst.parse_launch('videotestsrc is-live=true ! video/x-raw,width=800,height=600,framerate=10/1 ! videoconvert ! pngenc ! appsink name=sink max-buffers=1 drop=true sync=false')
        p.sink=p.pipeline.get_by_name('sink');p.pipeline.set_state(Gst.State.PLAYING)
        p.frame_id=None;p.frame_size=None
        self.calls=[]
        p.call=lambda *args: self.calls.append(args)
        self.portal=p
    def tearDown(self):self.portal.pipeline.set_state(Gst.State.NULL)
    def frame(self):return self.portal.execute('desktop_screenshot',{},threading.Event())
    def test_capture_scale_and_frame_consumption(self):
        frame=self.frame()
        self.assertEqual((frame['width'],frame['height']),(800,600));self.assertEqual(frame['scale_x'],2)
        self.portal.execute('desktop_click',{'x':600,'y':400},threading.Event())
        self.assertEqual(self.calls[0][1],'NotifyPointerMotionAbsolute')
        self.assertEqual(self.calls[0][3][-3:],(42,300.,200.))
        self.assertEqual([x[3][-1] for x in self.calls[1:]],[1,0])
        with self.assertRaisesRegex(RuntimeError,'fresh screenshot'):self.portal.execute('desktop_key',{'key':'enter'},threading.Event())
    def test_unicode_and_chord_releases(self):
        self.frame();self.portal.execute('desktop_type',{'text':'café 世界 🚀'},threading.Event())
        self.assertEqual(len(self.calls),len('café 世界 🚀')*2)
        self.assertTrue(all(self.calls[i][3][-1]==0 for i in range(1,len(self.calls),2)))
        self.calls.clear();self.frame();self.portal.execute('desktop_key',{'key':'ctrl+shift+p'},threading.Event())
        pressed=[x[3][-2] for x in self.calls[:3]];released=[x[3][-2] for x in self.calls[3:]]
        self.assertEqual(released,list(reversed(pressed)))
    def test_cancel_releases_pressed_key(self):
        self.frame();cancel=threading.Event()
        def call(*args):
            self.calls.append(args)
            if args[3][-1]==1:cancel.set()
        self.portal.call=call
        with self.assertRaisesRegex(RuntimeError,'cancelled'):self.portal.execute('desktop_key',{'key':'ctrl+l'},cancel)
        self.assertEqual(len(self.calls),2);self.assertEqual(self.calls[-1][3][-1],0)
    def test_bounds_and_cancelled_capture(self):
        self.frame()
        with self.assertRaisesRegex(RuntimeError,'outside'):self.portal.execute('desktop_move',{'x':800,'y':0},threading.Event())
        cancel=threading.Event();cancel.set()
        with self.assertRaisesRegex(RuntimeError,'cancelled'):self.portal.execute('desktop_screenshot',{},cancel)
        self.assertEqual(self.calls,[])

if __name__=='__main__':unittest.main()
