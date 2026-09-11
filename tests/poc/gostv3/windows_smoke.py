"""Windows GOST v3 smoke test; loopback only, no product processes or settings."""
import argparse, concurrent.futures, json, os, pathlib, secrets, socket, subprocess, threading, time, urllib.request


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--gost', required=True)
    ap.add_argument('--output', required=True)
    ap.add_argument('--full-udp', action='store_true')
    ap.add_argument('--registration', help='optional first-packet gateway binary')
    args = ap.parse_args()
    out = pathlib.Path(args.output).resolve()
    out.mkdir(parents=True, exist_ok=False)
    results, procs, logs, sockets = [], [], [], []
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def record(name, passed, detail):
        r = dict(name=name, passed=passed, detail=detail)
        results.append(r)
        print(json.dumps(r), flush=True)
        (out / 'results.json').write_text(json.dumps(results, indent=2))

    def free_port(proto=socket.SOCK_STREAM):
        with socket.socket(type=proto) as s:
            s.bind(('127.0.0.1', 0))
            return s.getsockname()[1]

    def launch(name, *cli):
        f = (out / (name+'.log')).open('wb')
        logs.append(f)
        p = subprocess.Popen([str(pathlib.Path(args.gost).resolve()), *cli], stdout=f, stderr=f,
                             creationflags=subprocess.CREATE_NO_WINDOW)
        procs.append(p)
        return p

    def wait(port):
        until = time.monotonic()+8
        while time.monotonic()<until:
            try:
                with socket.create_connection(('127.0.0.1',port), timeout=.1): return
            except OSError: time.sleep(.05)
        raise TimeoutError(f'listener {port} not ready')

    def tcp_echo(s):
        try:
            while True:
                c, _ = s.accept()
                def run(c):
                    with c:
                        try:
                            while b:=c.recv(65536): c.sendall(b)
                        except OSError: pass
                threading.Thread(target=run, args=(c,),daemon=True).start()
        except OSError: pass

    def udp_echo(s):
        try:
            while True:
                b,a=s.recvfrom(65536)
                s.sendto(b,a)
        except OSError: pass

    def exchange(c,b):
        c.settimeout(3)
        c.sendall(b)
        data=b''
        while len(data)<len(b):
            v=c.recv(len(b)-len(data))
            if not v: break
            data+=v
        return data==b

    try:
        echo_ports=[]
        for proto,fn in [(socket.SOCK_STREAM,tcp_echo),(socket.SOCK_DGRAM,udp_echo)]:
            s=socket.socket(type=proto);s.bind(('127.0.0.1',0));sockets.append(s)
            if proto==socket.SOCK_STREAM:s.listen()
            echo_ports.append(s.getsockname()[1])
            threading.Thread(target=fn,args=(s,),daemon=True).start()
        relay,api,rtcp,rudp=free_port(),free_port(),free_port(),free_port(socket.SOCK_DGRAM)
        server=launch('server','-L',f'relay://127.0.0.1:{relay}?bind=true'+('&udp.bufferSize=65535' if args.full_udp else ''))
        wait(relay)
        client=launch('client','-L',f'rtcp://127.0.0.1:{rtcp}/127.0.0.1:{echo_ports[0]}',
                      '-L',f'rudp://127.0.0.1:{rudp}/127.0.0.1:{echo_ports[1]}'+('?readBufferSize=65535' if args.full_udp else ''),
                      '-F',f'relay://127.0.0.1:{relay}','-api',f'127.0.0.1:{api}')
        wait(rtcp);wait(api)
        c=socket.create_connection(('127.0.0.1',rtcp),timeout=3);sockets.append(c)
        record('windows_reverse_tcp_binary',exchange(c,bytes(range(256))*128),'32768 exact binary bytes')
        with socket.socket(type=socket.SOCK_DGRAM) as u:
            u.settimeout(3);b=bytes(range(256))*4;u.sendto(b,('127.0.0.1',rudp));data,_=u.recvfrom(65536)
            record('windows_reverse_udp_binary',data==b,'1024 exact binary bytes')
        if args.full_udp:
            def datagram(size,seed=0):
                b=bytes((i+seed)%251 for i in range(size))
                try:
                    with socket.socket(type=socket.SOCK_DGRAM) as u:
                        u.settimeout(3);u.sendto(b,('127.0.0.1',rudp));data,_=u.recvfrom(65536)
                    return data==b, len(data), None
                except OSError as e:return False,0,str(e)
            for size in (0,1,1200,8192,32000,65000,0,19):
                ok,n,e=datagram(size);record('windows_udp_'+str(size),ok,dict(received=n,error=e))
            with concurrent.futures.ThreadPoolExecutor(8) as pool:
                rs=list(pool.map(lambda i:datagram(32000,i),range(8)))
            record('windows_udp_concurrent_large',all(r[0] for r in rs),rs)
            ok,n,e=datagram(19);record('windows_udp_small_after_large',ok,dict(received=n,error=e))
        if args.registration:
            authport=free_port(); token=secrets.token_hex(32)
            env=os.environ.copy();env['RMP_SERIAL_REGISTRATION_TOKEN']=token
            log=(out/'registration.log').open('wb');logs.append(log)
            g=subprocess.Popen([str(pathlib.Path(args.registration).resolve()),'-listen',f'127.0.0.1:{authport}','-backend',f'127.0.0.1:{rtcp}'],env=env,stdout=log,stderr=log,creationflags=subprocess.CREATE_NO_WINDOW)
            procs.append(g);wait(authport)
            def line(c):
                b=b''
                while not b.endswith(b'\n') and len(b)<512:
                    v=c.recv(1)
                    if not v:break
                    b+=v
                return b
            with socket.create_connection(('127.0.0.1',authport),3) as denied:
                denied.sendall(b'GET / HTTP/1.0\r\n');record('registration_garbage_rejected',line(denied)==b'ERR AUTH\r\n','generic socket')
            for i in range(3):
                with socket.create_connection(('127.0.0.1',authport),3) as owner:
                    owner.sendall(b'AUTH '+token.encode()+b'\r\n')
                    ok=line(owner)==b'OK\r\n' and exchange(owner,bytes(range(256))*64)
                    record('registration_gost_tcp_roundtrip_'+str(i),ok,'16384 exact binary bytes; no adapter')
                    with socket.create_connection(('127.0.0.1',authport),3) as busy:
                        busy.sendall(b'AUTH '+token.encode()+b'\nJUNK')
                        record('registration_exclusive_'+str(i),line(busy)==b'ERR BUSY\r\n' and exchange(owner,b'owner alive'),'existing owner still works')
                time.sleep(.1)
        cfg=json.load(opener.open(f'http://127.0.0.1:{api}/config',timeout=3))
        (out/'client-config.json').write_text(json.dumps(cfg,indent=2))
        name=cfg['services'][0]['name']
        req=urllib.request.Request(f'http://127.0.0.1:{api}/config/services/{name}',method='DELETE')
        with opener.open(req,timeout=3) as r: status=r.status
        time.sleep(.3)
        try: live=exchange(c,b'after-delete')
        except OSError: live=False
        try:
            with socket.create_connection(('127.0.0.1',rtcp),timeout=.5): new=True
        except OSError:new=False
        record('windows_delete_closes_active_and_listener',status==200 and not live and not new,
               dict(api_status=status,active_echo=live,new_connect=new))
    except Exception as e:
        record('harness_failure',False,repr(e))
    finally:
        for p in reversed(procs):
            if p.poll() is None:p.kill()
            p.wait(timeout=5)
        for s in sockets:s.close()
        for f in logs:f.close()
    return 0 if all(r['passed'] for r in results) else 2

if __name__=='__main__':raise SystemExit(main())
