"""Explicitly authorized real-device PoC. Loopback services over SSH, no product changes.
Requires RMP_POC_SSH_PASSWORD + an AskPass executable that reads that variable.
Exit 2 means at least one requirement check failed; observations are not checks.
"""
import argparse, base64, concurrent.futures, gzip, json, os, pathlib, re, shlex
import socket, subprocess, time, urllib.request, urllib.error, secrets


def main():
    ap=argparse.ArgumentParser()
    for key in ('host','remote-dir','helper','gost','askpass','output'): ap.add_argument('--'+key,required=True)
    ap.add_argument('--port',type=int,required=True)
    ap.add_argument('--full-udp',action='store_true',help='configure 65535-byte UDP buffers on both peers')
    ap.add_argument('--registration',help='optional first-packet gateway executable; loopback only')
    a=ap.parse_args(); out=pathlib.Path(a.output).resolve(); out.mkdir(parents=True,exist_ok=False)
    if not os.environ.get('RMP_POC_SSH_PASSWORD'): raise RuntimeError('Missing password environment variable')
    if not re.fullmatch(r'/tmp/root/gost-poc-[A-Za-z0-9_-]+',a.remote_dir): raise ValueError('Use an isolated /tmp/root/gost-poc-* directory')
    env=os.environ.copy(); env.update(SSH_ASKPASS=str(pathlib.Path(a.askpass).resolve()),SSH_ASKPASS_REQUIRE='force',DISPLAY='rmp:0')
    ssh=['ssh','-T','-p',str(a.port),'-o','ConnectTimeout=12','-o','StrictHostKeyChecking=accept-new','-o','PreferredAuthentications=password','-o','PubkeyAuthentication=no','-o','NumberOfPasswordPrompts=1']
    opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
    q=shlex.quote; d=a.remote_dir; procs=[]; files=[]; results=[]; remote_started=False
    def record(name, passed=None, **detail):
        row=dict(name=name,kind='observation' if passed is None else 'check',detail=detail)
        if passed is not None: row['passed']=bool(passed)
        results.append(row); (out/'results.json').write_text(json.dumps(results,indent=2),encoding='utf-8'); print(json.dumps(row),flush=True)
    def remote(cmd,data=None,timeout=35):
        p=subprocess.run(ssh+[a.host,cmd],input=data,stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=env,timeout=timeout)
        if p.returncode: raise RuntimeError(f'SSH {p.returncode}: {p.stderr.decode(errors="replace")} {p.stdout.decode(errors="replace")}')
        return p.stdout
    def save_remote(cmd,name):
        b=remote(cmd); (out/name).write_bytes(b); return b.decode(errors='replace')
    def launch(name,args):
        f=(out/(name+'.log')).open('wb'); files.append(f)
        p=subprocess.Popen(args,stdout=f,stderr=f,env=env,creationflags=subprocess.CREATE_NO_WINDOW);procs.append(p);return p
    def port():
        with socket.socket() as s:s.bind(('127.0.0.1',0));return s.getsockname()[1]
    relay,api,fixture,tcpport,udpport,serialport,cycleport=[port() for _ in range(7)]
    def request(base,path,method='GET',data=None):
        r=urllib.request.Request(f'http://127.0.0.1:{base}'+path,method=method,data=None if data is None else json.dumps(data).encode(),headers={'Content-Type':'application/json'})
        with opener.open(r,timeout=12) as res:return json.load(res)
    def wait_http(base,path):
        end=time.monotonic()+20
        while time.monotonic()<end:
            try:return request(base,path)
            except (OSError,ValueError):time.sleep(.25)
        raise TimeoutError(f'HTTP {base}{path} unavailable')
    def add(s): request(api,'/config/services','POST',s);time.sleep(.7)
    def delete(name):request(api,'/config/services/'+name,'DELETE');time.sleep(.25)
    def service(name,p,proto,target,chain=None):
        h={'type':'r'+proto}
        if chain:h['chain']=chain
        listener={'type':'r'+proto,'chain':'reverse'}
        if a.full_udp and proto=='udp': listener['metadata']={'readBufferSize':'65535'}  # GOST API metadata numeric JSON becomes float64, ignored by GetInt.
        return dict(name=name,addr=f'127.0.0.1:{p}',listener=listener,handler=h,forwarder={'nodes':[{'name':'target','addr':target}]})
    def line_bytes(c,n):
        got=b''
        while len(got)<n:
            v=c.recv(n-len(got))
            if not v:break
            got+=v
        return got
    def exchange(c,b):
        c.settimeout(8);c.sendall(b);got=b''
        while len(got)<len(b):
            v=c.recv(len(b)-len(got))
            if not v:break
            got+=v
        return got
    def tcp(b,p=tcpport):
        try:
            with socket.create_connection(('127.0.0.1',p),8) as c:got=exchange(c,b)
            return got==b,len(got),None
        except OSError as e:return False,0,str(e)
    def udp(b,p=udpport):
        try:
            with socket.socket(type=socket.SOCK_DGRAM) as c:
                c.settimeout(3);c.connect(('127.0.0.1',p));c.send(b);got=c.recv(65536)
            return got==b,len(got),None
        except OSError as e:return False,0,str(e)
    slot=0
    def readpty(n,ms=2000):return base64.b64decode(request(fixture,f'/pty/read?n={n}&ms={ms}&slot={slot}')['data'] or '')
    def writepty(b):return request(fixture,f'/pty/write?slot={slot}','POST',{'data':base64.b64encode(b).decode()})
    def sample(label):
        text=save_remote(f'p=$(cat {q(d+"/gost.pid")}); echo PID=$p; cat /proc/$p/status; echo ===STAT===; cat /proc/$p/stat; echo ===FD===; ls /proc/$p/fd | wc -l; echo ===MEMORY===; head -5 /proc/meminfo',label+'.txt')
        vals={}
        for line in text.splitlines():
            if line.startswith(('VmRSS:','VmHWM:','VmSize:','Threads:','MemAvailable:')): vals[line.split(':')[0]]=line.split(':')[1].strip()
        stat=text.split('===STAT===\n')[1].splitlines()[0].split(') ',1)[1].split()
        vals.update(utime_ticks=int(stat[11]),stime_ticks=int(stat[12]),fd=int(text.split('===FD===\n')[1].splitlines()[0]))
        record(label,**vals);return vals
    try:
        # Preflight is read-only; do not terminate processes or claim ports already in use.
        ports=remote('cat /proc/net/tcp /proc/net/tcp6 /proc/net/udp /proc/net/udp6').decode()
        for line in ports.splitlines():
            fields=line.split()
            if len(fields)>3 and ':' in fields[1]:
                try: local=int(fields[1].split(':')[-1],16)
                except ValueError:continue
                if 19400<=local<=19405:raise RuntimeError(f'Remote port {local} already present')
        helper_bytes=pathlib.Path(a.helper).read_bytes()
        remote(f'set -e; test -x {q(d+"/gost")}; test ! -e {q(d+"/device-helper")}; test ! -e {q(d+"/helper.gz")}; cat > {q(d+"/helper.gz")}',gzip.compress(helper_bytes),timeout=90)
        remote(f'set -e; gzip -dc {q(d+"/helper.gz")} > {q(d+"/device-helper")}; test "$(wc -c < {q(d+"/device-helper")})" -eq {len(helper_bytes)}')
        remote_started=True
        remote(f'chmod 700 {q(d+"/device-helper")}; {q(d+"/device-helper")} >{q(d+"/helper.log")} 2>&1 </dev/null & echo $! >{q(d+"/helper.pid")}')
        launch('relay',[str(pathlib.Path(a.gost).resolve()),'-L',f'relay://127.0.0.1:{relay}?bind=true'+('&udp.bufferSize=65535' if a.full_udp else '')])
        launch('ssh-forward',ssh+['-N','-o','ExitOnForwardFailure=yes','-o','ServerAliveInterval=10','-o','ServerAliveCountMax=2','-R',f'127.0.0.1:19400:127.0.0.1:{relay}','-L',f'127.0.0.1:{api}:127.0.0.1:19401','-L',f'127.0.0.1:{fixture}:127.0.0.1:19402',a.host])
        state=wait_http(fixture,'/state');record('fixture_running',True,**state)
        reverse={'name':'reverse','hops':[{'name':'relay-hop','nodes':[{'name':'relay','addr':'127.0.0.1:19400','connector':{'type':'relay'},'dialer':{'type':'tcp'}}]}]}
        chains=[reverse]
        for i,(label,baud) in enumerate((('9600',9600),('115200',115200),('tcp',115200))):chains.append({'name':'serial'+label,'hops':[{'name':'serial-hop','nodes':[{'name':'serial','addr':state['ptys'][i]+','+str(baud),'connector':{'type':'forward'},'dialer':{'type':'serial'}}]}]})
        cfg={'api':{'addr':'127.0.0.1:19401'},'chains':chains,'climiters':[{'name':'exclusive','limits':['$ 1']}],'services':[]}
        b=json.dumps(cfg).encode();(out/'initial-config.json').write_bytes(b);remote(f'cat > {q(d+"/config.json")}',b)
        # Hard time limit bounds orphan lifetime even if the existing maintenance SSH channel drops.
        remote(f'/tmp/root/busybox-ipq timeout -s TERM 600 {q(d+"/gost")} -C {q(d+"/config.json")} >{q(d+"/gost.log")} 2>&1 </dev/null & echo $! >{q(d+"/watchdog.pid")}')
        wait_http(api,'/config')
        remote(f'for p in /proc/[0-9]*; do if [ "$(readlink "$p/exe")" = {q(d+"/gost")} ]; then echo "${{p##*/}}" >{q(d+"/gost.pid")}; fi; done; test -s {q(d+"/gost.pid")}')
        sample('resources_no_mapping')
        ts=service('tcp',tcpport,'tcp','127.0.0.1:19403');us=service('udp',udpport,'udp','127.0.0.1:19404');add(ts);add(us)
        b=bytes(range(256))*224
        ok,n,e=tcp(b);record('tcp_binary_57344',ok,received=n,error=e)
        with concurrent.futures.ThreadPoolExecutor(8) as pool:
            r=list(pool.map(lambda i:udp(bytes([i])*1200),range(8)))
        record('udp_8_sources_before_large',all(x[0] for x in r),results=r)
        for n in (1,1200,8192,32000,65000,0):
            ok,got,e=udp(bytes(i%251 for i in range(n)));record('udp_datagram_'+str(n),ok,received=got,error=e)
        with concurrent.futures.ThreadPoolExecutor(8) as pool:
            r=list(pool.map(lambda i:tcp(bytes([i])*16384),range(8)))
        record('tcp_8_clients',all(x[0] for x in r),results=r)
        with concurrent.futures.ThreadPoolExecutor(8) as pool:
            r=list(pool.map(lambda i:udp(bytes([i])*1200),range(8)))
        record('udp_8_sources',all(x[0] for x in r),results=r)
        if a.full_udp:
            with concurrent.futures.ThreadPoolExecutor(8) as pool:
                r=list(pool.map(lambda i:udp(bytes([i])*32000),range(8)))
            record('udp_8_large_sources',all(x[0] for x in r),results=r)
        delete('udp');add(us)
        ok,n,e=udp(b'reset-small');record('udp_after_explicit_recreate',ok,received=n,error=e)
        with concurrent.futures.ThreadPoolExecutor(8) as pool:
            r=list(pool.map(lambda i:udp(bytes([i])*1200),range(8)))
        record('udp_8_sources_after_recreate',all(x[0] for x in r),results=r)
        connections=[socket.create_connection(('127.0.0.1',tcpport),8) for _ in range(8)]
        try:
            record('tcp_8_live_roundtrip',all(exchange(c,b'hold')==b'hold' for c in connections));sample('resources_8_live')
        finally:
            for c in connections:c.close()
        with socket.create_connection(('127.0.0.1',tcpport),8) as active:
            record('delete_setup',exchange(active,b'before')==b'before')
            delete('tcp')
            try:gone=exchange(active,b'after')!=b'after'
            except OSError:gone=True
            record('delete_revokes_active_tcp',gone)
        ok,n,e=tcp(b'after');record('delete_removes_tcp_listener',not ok,error=e)
        add(ts)
        good=0
        for i in range(20):
            c=service('cycle',cycleport,'tcp','127.0.0.1:19403');add(c);ok,_,_=tcp(b'cycle',cycleport);good+=ok;delete('cycle')
        record('20_add_delete_cycles',good==20,successful=good);sample('resources_after_cycles')
        before=sample('idle_before');started=time.monotonic();time.sleep(30);after=sample('idle_after');record('idle_cpu_ticks',wall_seconds=time.monotonic()-started,user_ticks=after['utime_ticks']-before['utime_ticks'],system_ticks=after['stime_ticks']-before['stime_ticks'])
        for slot,baud in enumerate((9600,115200)):
            s=service('serial',serialport,'tcp','127.0.0.1:1','serial'+str(baud));add(s)
            # No active readiness probe: it would create another UART reader.
            with socket.create_connection(('127.0.0.1',serialport),8) as c:
                data=bytes(range(256))*4;c.sendall(data);got=readpty(len(data));record('pty_'+str(baud)+'_network_to_serial',got==data,received=len(got))
                writepty(data);got=b'';c.settimeout(8)
                while len(got)<len(data):
                    chunk=c.recv(len(data)-len(got))
                    if not chunk:break
                    got+=chunk
                record('pty_'+str(baud)+'_serial_to_network',got==data,received=len(got))
                t=request(fixture,f'/pty/termios?slot={slot}');record('pty_'+str(baud)+'_termios',t['speed_bits']==(13 if baud==9600 else 4098),**t)
            delete('serial');time.sleep(.5)
        for slot,proto in enumerate(('tcp',),start=2):
            # TCP-only serial ownership. LAN UDP tests above remain required.
            bridge={'name':'bridge','addr':'127.0.0.1:19405','listener':{'type':proto,'metadata':{'ttl':'60s'}},'handler':{'type':proto,'chain':'serial'+proto},'forwarder':{'nodes':[{'name':'uart','addr':'127.0.0.1:1'}]},'climiter':'exclusive'}
            add(bridge);add(service('serial',serialport,proto,'127.0.0.1:19405'));readpty(4096,100)
            with socket.socket(type=socket.SOCK_STREAM if proto=='tcp' else socket.SOCK_DGRAM) as owner:
                owner.settimeout(6);owner.connect(('127.0.0.1',serialport));owner.sendall(b'owner');got=readpty(5);record(proto+'_serial_owner_write',got==b'owner',received=got.hex())
                with socket.socket(type=socket.SOCK_STREAM if proto=='tcp' else socket.SOCK_DGRAM) as other:
                    other.settimeout(3);other.connect(('127.0.0.1',serialport))
                    try:other.sendall(b'intruder')
                    except OSError:pass
                    got=readpty(8,1200);record(proto+'_serial_excludes_second_writer',not got,received=got.hex())
                writepty(b'reply')
                try:got=owner.recv(5)
                except OSError:got=b''
                record(proto+'_serial_owner_keeps_reply',got==b'reply',received=got.hex())
            if a.registration:
                # Generic TCP sockets send a text registration packet. No Relay client.
                authport=port();token=secrets.token_hex(32)
                env['RMP_SERIAL_REGISTRATION_TOKEN']=token
                gate=launch('registration',[str(pathlib.Path(a.registration).resolve()),'-listen',f'127.0.0.1:{authport}','-backend',f'127.0.0.1:{serialport}','-minutes','240'])
                del env['RMP_SERIAL_REGISTRATION_TOKEN']
                time.sleep(.8)
                def authconnect():
                    c=socket.create_connection(('127.0.0.1',authport),5);c.settimeout(6);return c
                def line(c):
                    b=b''
                    while not b.endswith(b'\n') and len(b)<512:
                        v=c.recv(1)
                        if not v:break
                        b+=v
                    return b
                readpty(4096,200)
                with authconnect() as bad:
                    bad.sendall(b'AUTH '+b'0'*64+b'\r\nGARBAGE')
                    record('registration_wrong_token_rejected',line(bad)==b'ERR AUTH\r\n')
                    record('registration_wrong_token_zero_pty_bytes',readpty(4096,250)==b'')
                with authconnect() as idle, authconnect() as owner:
                    owner.sendall(b'AUTH '+token[:20].encode())
                    record('registration_fragment_zero_pty_bytes',readpty(4096,250)==b'')
                    owner.sendall(token[20:].encode()+b'\r\n'+b'registered-owner')
                    record('registration_success_ack',line(owner)==b'OK\r\n')
                    record('registration_header_stripped_payload_intact',readpty(16)==b'registered-owner')
                    with authconnect() as second:
                        second.sendall(b'AUTH '+token.encode()+b'\nINTRUDER')
                        record('registration_busy_rejected',line(second)==b'ERR BUSY\r\n')
                        record('registration_busy_zero_pty_bytes',readpty(4096,250)==b'')
                    writepty(b'registered-reply')
                    record('registration_pty_reply',line_bytes(owner,16)==b'registered-reply')
                time.sleep(.5)
                with authconnect() as again:
                    again.sendall(b'AUTH '+token.encode()+b'\nnext-owner')
                    record('registration_reopen_ack',line(again)==b'OK\r\n')
                    record('registration_reopen_pty_write',readpty(10)==b'next-owner')
                    writepty(b'next-reply')
                    record('registration_reopen_pty_reply',line_bytes(again,10)==b'next-reply')
                for repeat in range(5):
                    time.sleep(.1)
                    with authconnect() as again:
                        payload=b'cycle-'+bytes([repeat])
                        again.sendall(b'AUTH '+token.encode()+b'\n'+payload)
                        ack=line(again)
                        forward=readpty(len(payload))
                        writepty(payload)
                        try:back=line_bytes(again,len(payload))
                        except OSError:back=b''
                        record('registration_fast_reopen_'+str(repeat),ack==b'OK\r\n' and forward==payload and back==payload,ack=ack.decode(errors='replace'),forward=forward.hex(),reply=back.hex())
                gate.terminate();gate.wait(timeout=5)
            delete('serial');delete('bridge');time.sleep(.5)
        (out/'final-config.json').write_text(json.dumps(request(api,'/config'),indent=2))
        sample('resources_final')
    except Exception as exc:
        record('infrastructure_error',False,error=repr(exc))
    finally:
        if remote_started:
            try:
                # Match the exact owned executable before signalling; never blanket-kill gost/Probe.
                cmd=f'for name in gost device-helper; do for p in /proc/[0-9]*; do if [ "$(readlink "$p/exe")" = "{d}/$name" ]; then kill "${{p##*/}}"; fi; done; done; sleep 1; '
                cmd+=f'echo ===REMAINING===; for p in /proc/[0-9]*; do e=$(readlink "$p/exe"); case "$e" in {d}/*) echo "$p $e";; esac; done; echo ===PROBE===; for p in /proc/[0-9]*; do e=$(readlink "$p/exe"); if [ "$e" = /tmp/root/router-agent ]; then echo "$p $e"; fi; done; echo ===ROUTES===; ip route; echo ===MEMORY===; head -5 /proc/meminfo'
                text=save_remote(cmd,'cleanup.txt');record('cleanup_owned_processes',not text.split('===REMAINING===\n')[1].split('===PROBE===')[0].strip())
                for name in ('gost.log','helper.log'):save_remote(f'cat {q(d+"/"+name)}',name)
            except Exception as exc:record('cleanup_unconfirmed',False,error=str(exc),watchdog_seconds=600)
        for p in reversed(procs):
            if p.poll() is None:
                p.terminate()
                try:p.wait(timeout=8)
                except subprocess.TimeoutExpired:p.kill();p.wait()
        for f in files:f.close()
    return 2 if any(r.get('passed') is False for r in results) else 0
if __name__=='__main__':raise SystemExit(main())
