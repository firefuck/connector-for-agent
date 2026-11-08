#!/usr/bin/env python3
"""Real GTK local controls + gateway + subprocess, isolated Xvfb display.
Requires Xvfb, xdotool, /usr/bin/python3 with Gtk3. No GNOME compositor test.
"""
import concurrent.futures
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.request

ROOT=Path(__file__).resolve().parents[1]
BINARY=ROOT/'dist/relaydesk-linux-amd64'
def wait(check, label, seconds=10):
    until=time.monotonic()+seconds
    while time.monotonic()<until:
        result=check()
        if result:return result
        time.sleep(.1)
    raise AssertionError('Timeout: '+label)

def main():
    children=[]
    with tempfile.TemporaryDirectory(prefix='relaydesk-local-') as directory:
        tmp=Path(directory)
        with (tmp/'process.log').open('wb') as log:
            def start(argv,env=None):
                p=subprocess.Popen(argv,stdout=log,stderr=log,env=env);children.append(p);return p
            listener=socket.socket();listener.bind(('127.0.0.1',0));port=listener.getsockname()[1];listener.close()
            base='http://127.0.0.1:'+str(port)
            admin=secrets.token_hex(32)
            env=dict(os.environ,RELAYDESK_ADMIN_TOKEN=admin,XDG_CACHE_HOME=str(tmp/'cache'))
            # Xvfb chooses a free display, avoiding shared user sessions.
            with (tmp/'display').open('wb') as display:
                xvfb=subprocess.Popen(['Xvfb','-displayfd',str(display.fileno()),'-screen','0','900x650x24','-nolisten','tcp'],pass_fds=(display.fileno(),),stdout=log,stderr=log)
                children.append(xvfb)
            number=wait(lambda:(tmp/'display').read_text().strip(),'Xvfb display')
            env.update(DISPLAY=':'+number,XDG_SESSION_TYPE='x11',GDK_BACKEND='x11')
            def post(path,data,token=admin):
                request=urllib.request.Request(base+path,data=json.dumps(data).encode(),headers={'Content-Type':'application/json','Authorization':'Bearer '+token,'Accept':'application/json, text/event-stream'})
                with urllib.request.urlopen(request,timeout=12) as response:return json.load(response)
            def alive():
                try:return post('/admin/list',{})
                except (OSError,ValueError):return None
            def click(window,x,y):subprocess.run(['xdotool','mousemove','--window',window,str(x),str(y),'click','1'],env=env,check=True)
            try:
                gateway=start([str(BINARY),'gateway','--listen','127.0.0.1:'+str(port),'--state',str(tmp/'state.json')],env)
                wait(alive,'gateway')
                code=post('/admin/pair',{'name':'Local test'})['code']
                paired=post('/pair',{'code':code},'')
                config=tmp/'connector.json';config.write_text(json.dumps(dict(gateway=base,token=paired['token'],id=paired['id'],wireguard_http=False)));config.chmod(0o600)
                token=post('/admin/agent',{'name':'Test agent','devices':[paired['id']],'tools':['*']})['token']
                connector=start([str(BINARY),'connector','--config',str(config)],env)
                def window_id():
                    r=subprocess.run(['xdotool','search','--onlyvisible','--name','RelayDesk — kontrol lokal'],env=env,text=True,capture_output=True)
                    return r.stdout.strip().splitlines()[0] if r.returncode==0 else None
                window=wait(window_id,'local GTK window')
                def state():return post('/admin/list',{})['devices'][0]
                wait(lambda:state()['paused'],'initial local pause')
                geometry=subprocess.check_output(['xdotool','getwindowgeometry','--shell',window],env=env,text=True)
                dimensions=dict(line.split('=',1) for line in geometry.splitlines() if '=' in line)
                width,height=int(dimensions['WIDTH']),int(dimensions['HEIGHT'])
                click(window,width//2,height-36)
                wait(lambda:not state()['paused'],'Resume button')
                def tool(name,args,request_id):
                    return post('/mcp',{'jsonrpc':'2.0','id':request_id,'method':'tools/call','params':{'name':name,'arguments':args}},token)
                tool('session_acquire',{'device_id':paired['id']},1)
                shot=tool('desktop_screenshot',{'device_id':paired['id']},10)
                metadata=json.loads(shot['result']['content'][1]['text'])
                assert metadata['frame_id'] and metadata['width']==900 and metadata['height']==650
                move=tool('desktop_move',{'device_id':paired['id'],'x':10,'y':10,'frame_id':metadata['frame_id']},11)
                assert not move['result'].get('isError'),move
                reused=tool('desktop_move',{'device_id':paired['id'],'x':20,'y':20,'frame_id':metadata['frame_id']},12)
                assert reused['result'].get('isError'),'frame was reused'

                with concurrent.futures.ThreadPoolExecutor() as pool:
                    future=pool.submit(tool,'system_exec',{'device_id':paired['id'],'command':"touch '"+str(tmp/'started')+"'; sleep 30; touch '"+str(tmp/'late')+"'"},2)
                    wait(lambda:(tmp/'started').exists(),'running subprocess')
                    output=subprocess.check_output([str(BINARY),'control','--config',str(config),'--op','pause'],env=env,text=True)
                    assert json.loads(output)['paused']
                    result=future.result(timeout=10)
                    assert result.get('result',{}).get('isError'),result
                wait(lambda:state()['paused'],'gateway pause synchronization')
                assert not (tmp/'late').exists(),'cancelled shell finished later'
                # Gateway down must not break local Stop, or trigger automatic resume.
                gateway.terminate();gateway.wait(timeout=8)
                subprocess.run([str(BINARY),'control','--config',str(config),'--op','stop'],env=env,check=True,stdout=log)
                connector.wait(timeout=8)
                assert config.exists() and Path(str(config)+'.stopped').exists()
                stopped=subprocess.run([str(BINARY),'connector','--autostart','--config',str(config)],env=env,stdout=log,stderr=log,timeout=5)
                assert stopped.returncode==0
                print('PASS real GTK Resume, local Pause cancels shell, gateway sync, offline Stop, pairing preservation and autostart Stop marker.')
                print('Xvfb test only; GNOME portal and Windows runtime require manual acceptance.')
            except Exception:
                print((tmp/'process.log').read_text(errors='replace')[-4000:])
                raise
            finally:
                for p in reversed(children):
                    if p.poll() is None:p.terminate()
                for p in reversed(children):
                    try:p.wait(timeout=5)
                    except subprocess.TimeoutExpired:p.kill();p.wait()

if __name__=='__main__':main()
