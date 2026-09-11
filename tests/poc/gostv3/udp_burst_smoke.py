"""Bounded UDP burst diagnosis on the authorized router; no physical UART writes.
Records Windows IPv4 UDP counters, per-packet relay trace and observed target IDs.
This does not retry datagrams or change machine-wide socket/network settings.
"""
import argparse, concurrent.futures, ctypes, gzip, json, os, pathlib, re, shlex
import socket, struct, subprocess, threading, time, urllib.request


def win_udp_stats():
    class Stats(ctypes.Structure):
        _fields_=[(n,ctypes.c_uint32) for n in ('in_datagrams','no_ports','in_errors','out_datagrams','num_addrs')]
    fn=ctypes.WinDLL('iphlpapi.dll').GetUdpStatisticsEx
    fn.argtypes=[ctypes.POINTER(Stats),ctypes.c_uint32];fn.restype=ctypes.c_uint32
    v=Stats();code=fn(ctypes.byref(v),socket.AF_INET)
    if code:raise OSError(code,'GetUdpStatisticsEx')
    return {n:getattr(v,n) for n,_ in v._fields_}


def main():
    ap=argparse.ArgumentParser()
    for n in ('gost','output'):ap.add_argument('--'+n,required=True)
    for n in ('host','remote-dir','helper','askpass'):ap.add_argument('--'+n)
    ap.add_argument('--local',action='store_true',help='Windows loopback diagnostic only; not real-device evidence')
    ap.add_argument('--port',type=int)
    ap.add_argument('--rounds',type=int,default=5)
    ap.add_argument('--size',type=int,default=32000)
    ap.add_argument('--clients',type=int,default=8)
    ap.add_argument('--late-window',type=float,default=0,help='observe late replies after 3s failure without retrying or changing pass criteria (max 12s)')
    a=ap.parse_args()
    if not 0<=a.late_window<=12:raise ValueError('late observation must be bounded to 12 seconds')
    if not 1<=a.rounds<=20 or not 12<=a.size<=65000 or not 1<=a.clients<=16:raise ValueError('bounded test arguments required')
    if not a.local and not all((a.host,a.port,a.remote_dir,a.helper,a.askpass)):raise ValueError('remote SSH/fixture arguments required')
    if not a.local and not re.fullmatch(r'/tmp/root/gost-poc-[A-Za-z0-9_-]+',a.remote_dir):raise ValueError('isolated directory required')
    if not a.local and not os.environ.get('RMP_POC_SSH_PASSWORD'):raise ValueError('password environment missing')
    out=pathlib.Path(a.output).resolve();out.mkdir(parents=True,exist_ok=False)
    env=os.environ.copy()
    if not a.local:env.update(SSH_ASKPASS=str(pathlib.Path(a.askpass).resolve()),SSH_ASKPASS_REQUIRE='force',DISPLAY='rmp:0')
    ssh=['ssh','-T','-p',str(a.port),'-o','ConnectTimeout=12','-o','StrictHostKeyChecking=accept-new','-o','PreferredAuthentications=password','-o','PubkeyAuthentication=no','-o','NumberOfPasswordPrompts=1']
    q=shlex.quote;d=a.remote_dir;procs=[];logs=[];results=[];started=False;target=None;target_thread=None
    target_lock=threading.Lock();target_state={'packets':0,'bytes':0,'recent':[]};stop=threading.Event()
    def remote(cmd,data=None,timeout=40):
        p=subprocess.run(ssh+[a.host,cmd],input=data,capture_output=True,timeout=timeout,env=env)
        if p.returncode:raise RuntimeError(p.stderr.decode(errors='replace')+p.stdout.decode(errors='replace'))
        return p.stdout
    def record(name,passed=None,**detail):
        row=dict(name=name,detail=detail)
        if passed is not None:row['passed']=passed
        results.append(row);(out/'results.json').write_text(json.dumps(results,indent=2),encoding='utf-8');print(json.dumps(row),flush=True)
    def launch(name,args):
        f=(out/(name+'.log')).open('wb');logs.append(f)
        p=subprocess.Popen(args,stdout=f,stderr=f,env=env,creationflags=subprocess.CREATE_NO_WINDOW);procs.append(p);return p
    def port():
        with socket.socket() as s:s.bind(('127.0.0.1',0));return s.getsockname()[1]
    relay,api,fixture,entry=[port() for _ in range(4)]
    opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
    def http(port,path,method='GET',data=None):
        if a.local and port==fixture and path=='/udp/stats':
            with target_lock:return json.loads(json.dumps(target_state))
        req=urllib.request.Request(f'http://127.0.0.1:{port}'+path,method=method,data=None if data is None else json.dumps(data).encode(),headers={'Content-Type':'application/json'})
        with opener.open(req,timeout=10) as r:return json.load(r)
    def ready(port,path):
        end=time.monotonic()+20
        while time.monotonic()<end:
            try:return http(port,path)
            except (OSError,ValueError):time.sleep(.2)
        raise TimeoutError('HTTP not ready')
    def trace_snapshot():
        events=[]
        for line in (out/'relay.log').read_text(encoding='utf-8',errors='replace').splitlines():
            try:r=json.loads(line)
            except ValueError:continue
            m=re.search(r'127\.0\.0\.1:(\d+) (<<<|>>>) 127\.0\.0\.1:(\d+) data: (\d+)',r.get('msg',''))
            if m:events.append(dict(time=r.get('time'),listener_port=int(m[1]),direction=m[2],client_port=int(m[3]),size=int(m[4])))
        return events
    try:
        if a.local:
            target=socket.socket(type=socket.SOCK_DGRAM);target.bind(('127.0.0.1',0));target.settimeout(.2)
            target_port=target.getsockname()[1]
            target_state['recv_buffer']=target.getsockopt(socket.SOL_SOCKET,socket.SO_RCVBUF)
            def echo():
                while not stop.is_set():
                    try:data,addr=target.recvfrom(65536)
                    except socket.timeout:continue
                    except OSError:return
                    written=target.sendto(data,addr)
                    with target_lock:
                        target_state['packets']+=1;target_state['bytes']+=len(data)
                        target_state['recent']=(target_state['recent']+[dict(prefix=data[:12].hex(),length=len(data),written=written)])[-128:]
            target_thread=threading.Thread(target=echo,daemon=True);target_thread.start()
            record('environment',True,mode='Windows loopback only',target_port=target_port)
        else:
            rows=remote('cat /proc/net/tcp /proc/net/tcp6 /proc/net/udp /proc/net/udp6').decode()
            for line in rows.splitlines():
                f=line.split()
                if len(f)>3 and ':'in f[1] and f[3]!='06':  # TCP TIME_WAIT is not a live listener
                    try:p=int(f[1].split(':')[-1],16)
                    except ValueError:continue
                    if 19400<=p<=19404:raise RuntimeError('test port occupied: '+str(p))
            b=pathlib.Path(a.helper).read_bytes();z=gzip.compress(b)
            remote(f'set -e; test -x {q(d+"/gost")}; test ! -e {q(d+"/device-helper")}; test ! -e {q(d+"/helper.gz")}; cat > {q(d+"/helper.gz")}',z,90)
            remote(f'set -e; gzip -dc {q(d+"/helper.gz")} > {q(d+"/device-helper")}; test "$(wc -c < {q(d+"/device-helper")})" -eq {len(b)}; chmod 700 {q(d+"/device-helper")}')
            started=True
            remote(f'{q(d+"/device-helper")} >{q(d+"/helper.log")} 2>&1 </dev/null &')
        md={'bind':True,'udp.bufferSize':'65535'}
        cfg={'log':{'level':'trace'},'services':[{'name':'relay','addr':f'127.0.0.1:{relay}','listener':{'type':'tcp'},'handler':{'type':'relay','metadata':md}}]}
        p=out/'relay.json';p.write_text(json.dumps(cfg));launch('relay',[str(pathlib.Path(a.gost).resolve()),'-C',str(p)])
        if not a.local:
            launch('ssh-forward',ssh+['-N','-o','ExitOnForwardFailure=yes','-o','ServerAliveInterval=10','-o','ServerAliveCountMax=2','-R',f'127.0.0.1:19400:127.0.0.1:{relay}','-L',f'127.0.0.1:{api}:127.0.0.1:19401','-L',f'127.0.0.1:{fixture}:127.0.0.1:19402',a.host])
        record('fixture_ready',True,stats=ready(fixture,'/udp/stats'))
        cfg={'api':{'addr':'127.0.0.1:19401'},'log':{'level':'trace'},'chains':[{'name':'reverse','hops':[{'name':'hop','nodes':[{'name':'node','addr':'127.0.0.1:19400','connector':{'type':'relay'},'dialer':{'type':'tcp'}}]}]}],'services':[{'name':'udp','addr':f'127.0.0.1:{entry}','listener':{'type':'rudp','chain':'reverse','metadata':{'readBufferSize':'65535'}},'handler':{'type':'rudp'},'forwarder':{'nodes':[{'name':'target','addr':'127.0.0.1:19404'}]}}]}
        if a.local:
            cfg['api']['addr']=f'127.0.0.1:{api}'
            cfg['chains'][0]['hops'][0]['nodes'][0]['addr']=f'127.0.0.1:{relay}'
            cfg['services'][0]['forwarder']['nodes'][0]['addr']=f'127.0.0.1:{target_port}'
            p=out/'client.json';p.write_text(json.dumps(cfg));launch('client',[str(pathlib.Path(a.gost).resolve()),'-C',str(p)])
        else:
            remote(f'cat > {q(d+"/config.json")}',json.dumps(cfg).encode())
            remote(f'/tmp/root/busybox-ipq timeout -s TERM 600 {q(d+"/gost")} -C {q(d+"/config.json")} >{q(d+"/gost.log")} 2>&1 </dev/null &')
        ready(api,'/config');time.sleep(1)
        for round_id in range(a.rounds):
            before_win=win_udp_stats();before_target=http(fixture,'/udp/stats')
            def one(i):
                data=struct.pack('!4sII',b'UBP1',round_id,i)+bytes([i])*(a.size-12)
                with socket.socket(type=socket.SOCK_DGRAM) as c:
                    c.bind(('127.0.0.1',0));c.connect(('127.0.0.1',entry));c.settimeout(3)
                    p=c.getsockname()[1];start=time.monotonic();c.send(data)
                    try:got=c.recv(65536);return dict(client=i,port=p,passed=got==data,received=len(got),seconds=time.monotonic()-start,recv_buffer=c.getsockopt(socket.SOL_SOCKET,socket.SO_RCVBUF))
                    except OSError as e:
                        result=dict(client=i,port=p,passed=False,error=str(e),seconds=time.monotonic()-start)
                        if isinstance(e,socket.timeout) and a.late_window:
                            c.settimeout(a.late_window)
                            try:
                                got=c.recv(65536)
                                result.update(late_received=len(got),late_exact=got==data,late_seconds=time.monotonic()-start)
                            except OSError as late:result['late_error']=str(late)
                        return result
            with concurrent.futures.ThreadPoolExecutor(a.clients) as pool:r=list(pool.map(one,range(a.clients)))
            after_target=http(fixture,'/udp/stats');after_win=win_udp_stats();events=trace_snapshot();ports={x['port'] for x in r}
            record('burst_'+str(round_id),all(x['passed'] for x in r),clients=r,target_before=before_target,target_after=after_target,windows_delta={k:after_win[k]-before_win[k] for k in after_win if k!='num_addrs'},relay_events=[e for e in events if e['client_port']in ports])
            time.sleep(.25)
        # Verify that a burst failure did not corrupt framing for the next datagram.
        with socket.socket(type=socket.SOCK_DGRAM) as c:
            c.settimeout(3);c.sendto(b'following-small',('127.0.0.1',entry));got,_=c.recvfrom(65536)
            record('following_small',got==b'following-small')
    except Exception as e:record('infrastructure_error',False,error=repr(e))
    finally:
        if started:
            try:
                cmd=f'for p in /proc/[0-9]*; do e=$(readlink "$p/exe"); case "$e" in {d}/gost|{d}/device-helper) kill "${{p##*/}}";; esac; done; sleep 1; '
                cmd+=f'for p in /proc/[0-9]*; do e=$(readlink "$p/exe"); case "$e" in {d}/*) echo "$p $e";; esac; done'
                remaining=remote(cmd).decode();record('cleanup',not remaining.strip(),remaining=remaining)
                for n in ('gost.log','helper.log'):(out/n).write_bytes(remote(f'cat {q(d+"/"+n)}'))
            except Exception as e:record('cleanup_unconfirmed',False,error=str(e))
        for p in reversed(procs):
            if p.poll() is None:p.terminate()
            try:p.wait(timeout=5)
            except subprocess.TimeoutExpired:p.kill();p.wait()
        stop.set()
        if target_thread:target_thread.join(timeout=1)
        if target:target.close()
        for f in logs:f.close()
    return 2 if any(x.get('passed') is False for x in results) else 0
if __name__=='__main__':raise SystemExit(main())
