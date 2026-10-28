#!/usr/bin/env python3
"""Exercise an actual Linux window on an owned Xvfb display, in safe mode.

Requires Xvfb, ImageMagick import, Tesseract, libX11, and libXtst. No core,
subscription, system proxy, TUN, file picker, or existing display is used.
"""
import argparse
import ctypes
import json
import os
from pathlib import Path
import re
import select
import signal
import subprocess
import tempfile
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--binary', required=True)
parser.add_argument('--xvfb', default='Xvfb')
parser.add_argument('--output', required=True)
parser.add_argument('--cpu', action='store_true')
args = parser.parse_args()
output = Path(args.output).resolve()
output.mkdir(parents=True, exist_ok=True)
(output/'cache').mkdir(exist_ok=True)
(output/'config').mkdir(exist_ok=True)
env = os.environ.copy()
if args.cpu:
    env['MYGO_GPU'] = '0'
else:
    env.pop('MYGO_GPU', None)
env.update(GDK_BACKEND='x11', XDG_CACHE_HOME=str(output/'cache'), XDG_CONFIG_HOME=str(output/'config'))


def run(*cmd):
    return subprocess.run(cmd, env=env, check=True, text=True, capture_output=True, timeout=15).stdout


def stop(process):
    if process is not None and process.poll() is None:
        os.killpg(process.pid, signal.SIGTERM)
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait()


xvfb = app = None
xdisplay = None
with tempfile.TemporaryDirectory(prefix='flclash-native-') as data, (output/'xvfb.log').open('w') as server_log, (output/'app.log').open('w') as app_log:
    profile_id = 'e6e51d736d274f509e3dd0b96c94fb15'
    (Path(data)/'profiles').mkdir()
    (Path(data)/'profiles'/(profile_id+'.yaml')).write_text('proxies: []\nproxy-groups: []\nrules: ["MATCH,DIRECT"]\n')
    (Path(data)/'settings.json').write_text(json.dumps({'active':profile_id,'closeToTray':False,'profiles':[{'id':profile_id,'name':'SmokeFixture','updated':'2026-01-01T00:00:00Z'}]}))
    read_fd, write_fd = os.pipe()
    try:
        xvfb = subprocess.Popen([args.xvfb, '-displayfd', str(write_fd), '-screen', '0', '1400x1000x24', '-nolisten', 'tcp'], pass_fds=(write_fd,), stdout=server_log, stderr=server_log, env=env, start_new_session=True)
        os.close(write_fd)
        if not select.select([read_fd], [], [], 10)[0]:
            raise RuntimeError('Owned Xvfb did not announce its display')
        display_number = os.read(read_fd, 32).decode().strip()
        if not display_number.isdigit():
            raise RuntimeError('Owned Xvfb failed to open a display')
        env['DISPLAY'] = ':'+display_number
        x11 = ctypes.CDLL('libX11.so.6')
        xtest = ctypes.CDLL('libXtst.so.6')
        x11.XOpenDisplay.argtypes = [ctypes.c_char_p]
        x11.XOpenDisplay.restype = ctypes.c_void_p
        xdisplay = x11.XOpenDisplay(env['DISPLAY'].encode())
        if not xdisplay:
            raise RuntimeError('Cannot open owned display')
        x11.XFlush.argtypes = [ctypes.c_void_p]
        x11.XSync.argtypes = [ctypes.c_void_p,ctypes.c_int]
        x11.XSetInputFocus.argtypes = [ctypes.c_void_p,ctypes.c_ulong,ctypes.c_int,ctypes.c_ulong]
        x11.XRaiseWindow.argtypes = [ctypes.c_void_p,ctypes.c_ulong]
        x11.XResizeWindow.argtypes = [ctypes.c_void_p,ctypes.c_ulong,ctypes.c_uint,ctypes.c_uint]
        x11.XCloseDisplay.argtypes = [ctypes.c_void_p]
        x11.XStringToKeysym.argtypes = [ctypes.c_char_p]
        x11.XStringToKeysym.restype = ctypes.c_ulong
        x11.XKeysymToKeycode.argtypes = [ctypes.c_void_p, ctypes.c_ulong]
        x11.XKeysymToKeycode.restype = ctypes.c_uint
        xtest.XTestFakeMotionEvent.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_ulong]
        xtest.XTestFakeButtonEvent.argtypes = [ctypes.c_void_p, ctypes.c_uint, ctypes.c_int, ctypes.c_ulong]
        xtest.XTestFakeKeyEvent.argtypes = [ctypes.c_void_p, ctypes.c_uint, ctypes.c_int, ctypes.c_ulong]
        app = subprocess.Popen(['dbus-run-session', '--', str(Path(args.binary).resolve()), '--safe-mode', '--data-dir', data], stdout=app_log, stderr=app_log, env=env, start_new_session=True)
        window_id = None
        origin = (0, 0)
        width = 1200
        deadline = time.monotonic()+60
        while time.monotonic() < deadline:
            if app.poll() is not None:
                raise RuntimeError('App exited during startup')
            tree = run('xwininfo', '-root', '-tree')
            found = re.search(r'(0x[0-9a-f]+) "FlClash".*? (\d+)x(\d+)\+(-?\d+)\+(-?\d+)', tree)
            if found:
                window_id = found[1]
                width = int(found[2])
                origin = (int(found[4]), int(found[5]))
                break
            time.sleep(.1)
        if window_id is None:
            raise RuntimeError('Native window did not map')

        x11.XRaiseWindow(xdisplay,int(window_id,16))
        x11.XSetInputFocus(xdisplay,int(window_id,16),2,0)
        x11.XSync(xdisplay,0)

        def capture(name):
            path = output/(name+'.png')
            run('import', '-window', window_id, str(path))
            return path

        def wait_text(name, required, absent=(), region=None):
            deadline = time.monotonic()+60
            while time.monotonic() < deadline:
                if app.poll() is not None:
                    raise RuntimeError('App exited while waiting for '+name)
                path = capture(name)
                ocr_path = path
                if region:
                    ocr_path = output/(name+'-ocr.png')
                    run('convert',str(path),'-crop',region,'+repage',str(ocr_path))
                text = ''
                for mode in (['6','3','11'] if region else ['3','11']):
                    text += ' '+run('tesseract', str(ocr_path), 'stdout', '--psm', mode).lower()
                    text = re.sub(r'\s+', ' ', text)
                    compact = "".join(text.split())
                    if all("".join(word.lower().split()) in compact for word in required) and all("".join(word.lower().split()) not in compact for word in absent):
                        (output/(name+'.txt')).write_text(text)
                        print(name+' passed', flush=True)
                        return path
                time.sleep(.1)
            raise RuntimeError('Native view did not reach '+name)

        def click(x, y):
            xtest.XTestFakeMotionEvent(xdisplay, -1, origin[0]+x, origin[1]+y, 0)
            xtest.XTestFakeButtonEvent(xdisplay, 1, 1, 0)
            xtest.XTestFakeButtonEvent(xdisplay, 1, 0, 0)
            x11.XSync(xdisplay,0)

        def press_key(name):
            key = x11.XKeysymToKeycode(xdisplay, x11.XStringToKeysym(name))
            xtest.XTestFakeKeyEvent(xdisplay,key,1,0)
            xtest.XTestFakeKeyEvent(xdisplay,key,0,0)
            x11.XFlush(xdisplay)

        def escape():
            press_key(b'Escape')

        def add_profile_shortcut():
            modifier = x11.XKeysymToKeycode(xdisplay, x11.XStringToKeysym(b'Control_L'))
            key = x11.XKeysymToKeycode(xdisplay, x11.XStringToKeysym(b'n'))
            xtest.XTestFakeKeyEvent(xdisplay,modifier,1,0)
            xtest.XTestFakeKeyEvent(xdisplay,key,1,0)
            xtest.XTestFakeKeyEvent(xdisplay,key,0,0)
            xtest.XTestFakeKeyEvent(xdisplay,modifier,0,0)
            x11.XFlush(xdisplay)

        def text_box(path, words, left_of=None):
            lines = run('tesseract', str(path), 'stdout', '--psm', '3', 'tsv').splitlines()[1:]
            boxes = [line.split('\t', 11) for line in lines]
            boxes = [box for box in boxes if len(box)==12 and box[11].strip()]
            wanted = words.lower().split()
            for i in range(len(boxes)-len(wanted)+1):
                group=boxes[i:i+len(wanted)]
                if [box[11].lower() for box in group] != wanted:
                    continue
                x,y = int(group[0][6]), int(group[0][7])
                if left_of is not None and x >= left_of:
                    continue
                end = int(group[-1][6])+int(group[-1][8])
                return x,y,end-x,int(group[0][9])
            raise RuntimeError('Cannot locate '+words)

        def text_click(path, words, left_of=None):
            x,y,w,h = text_box(path,words,left_of)
            click(x+w//2,y+h//2)

        dashboard = wait_text('dashboard', ['Dashboard','Total traffic','SAFE MODE'])
        text_click(dashboard, 'Disconnected')
        wait_text('restart-confirmation', ['Core status','restart the core'], region='600x450+300+175')
        escape()
        dashboard = wait_text('restart-dismissed', ['Total traffic'], absent=['restart the core'])
        click(24, 58)
        wait_text('sidebar-collapsed', ['Total traffic'], absent=['Tools'])
        click(24, 58)
        dashboard = wait_text('sidebar-expanded', ['Live connections', 'Tools', 'Total traffic'])
        text_click(dashboard, 'Profiles', left_of=220)
        profiles = wait_text('profiles', ['Profiles','SmokeFixture'], absent=['Total traffic'])
        add_profile_shortcut()
        wait_text('import-dialog', ['Add profile','Age secret key','example.com/subscription'])
        escape()
        profiles = wait_text('dialog-dismissed', ['SmokeFixture'], absent=['Age secret key'])
        text_click(profiles, 'Dashboard', left_of=220)
        dashboard = wait_text('dashboard-return', ['Total traffic'])
        # The rightmost info card's trailing details button.
        click(width-48,407)
        wait_text('memory-sheet', ['Resident memory','Core is not running'], absent=())
        escape()
        wait_text('memory-dismissed', ['Total traffic'], absent=['Resident memory'])
        x11.XResizeWindow(xdisplay,int(window_id,16),820,800)
        x11.XSync(xdisplay,0)
        wait_text('dashboard-laptop', ['Dashboard','Outbound mode'])
        click(24,58)
        wait_text('sidebar-laptop-overlay', ['Tools'], region='220x650+0+32')
        click(600,450)
        wait_text('sidebar-laptop-dismissed', ['Dashboard'], absent=['Tools'], region='220x650+0+32')
        x11.XResizeWindow(xdisplay,int(window_id,16),560,800)
        x11.XSync(xdisplay,0)
        narrow = wait_text('dashboard-narrow', ['Network speed','Outbound mode'])
        text_click(narrow,'Tools')
        tools = wait_text('dock-tools', ['Settings','Backup and restore'])
        text_click(tools,'Profiles')
        profiles = wait_text('dock-profiles', ['SmokeFixture','Profiles'])
        text_click(profiles,'Dashboard')
        narrow = wait_text('dock-dashboard', ['Network speed','Outbound mode'])
        x,y,w,h = text_box(narrow,'Disconnected')
        # Core label inset 14 + glyph 20 + gap 8, toolbar gap 8,
        # and half the preceding edit button 22. Native chrome reserves space.
        click(x-72,y+h//2)
        wait_text('dashboard-edit', ['Edit dashboard','Network speed'])
        escape()
        wait_text('dashboard-edit-dismissed', ['Network speed'], absent=['Edit dashboard'])
        x11.XResizeWindow(xdisplay,int(window_id,16),380,400)
        x11.XSync(xdisplay,0)
        wait_text('desktop-minimum', ['Dashboard','Profiles','Proxies'])
        geometry = run('xwininfo','-id',window_id)
        if not re.search(r'Width:\s+380\b',geometry) or not re.search(r'Height:\s+400\b',geometry):
            raise RuntimeError('Window did not reach the original 380 by 400 minimum')
        x11.XResizeWindow(xdisplay,int(window_id,16),1200,800)
        x11.XSync(xdisplay,0)
        dashboard = wait_text('dashboard-wide-return', ['Total traffic'], absent=['Edit dashboard'])
        text_click(dashboard, 'SmokeFixture')
        wait_text('profile-details', ['Last updated','Extend mode'])
        press_key(b'Tab')
        press_key(b'Return')
        wait_text('profile-runtime-preview', ['allow-lan','dns'])
        escape()
        wait_text('profile-preview-back', ['Last updated','Extend mode'])
        click(758,68)
        wait_text('profile-details-dismissed', ['Total traffic'], absent=['Last updated','Extend mode'])
        click(width-23,16)
        deadline = time.monotonic()+15
        while app.poll() is None and time.monotonic()<deadline:
            time.sleep(.1)
        if app.poll() is None:
            raise RuntimeError('Caption close did not close the owned window')
        print('caption-close passed',flush=True)
    finally:
        if xdisplay:
            x11.XCloseDisplay(xdisplay)
        stop(app)
        stop(xvfb)
        os.close(read_fd)
