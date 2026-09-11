"""Local-only GOST v3 authentication evaluation; never opens a physical UART.
Serial is represented by an observed TCP echo sink. Synthetic credentials only.
Wire framing follows github.com/go-gost/relay v0.7.0, not a new product protocol.
"""
import argparse, json, pathlib, secrets, socket, struct, subprocess, threading, time, urllib.request


def recvn(c, n):
    b = b''
    while len(b) < n:
        v = c.recv(n - len(b))
        if not v: break
        b += v
    return b


def feature(kind, data):
    return struct.pack('!BH', kind, len(data)) + data


def request_frame(user, password, target=None):
    f = b''
    if user is not None:
        u, p = user.encode(), password.encode()
        f += feature(1, bytes([len(u)]) + u + bytes([len(p)]) + p)
    if target is not None:
        f += feature(2, b'\x01' + socket.inet_aton('127.0.0.1') + struct.pack('!H', target))
    return struct.pack('!BBH', 1, 1, len(f)) + f


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--gost', required=True)
    ap.add_argument('--output', required=True)
    a = ap.parse_args()
    out = pathlib.Path(a.output).resolve(); out.mkdir(parents=True, exist_ok=False)
    lock = threading.Lock(); accepted = []; active = set(); results = []; sockets = []
    proc = None; log = None
    greeting = b'UART-READY\x00'
    user, password = 'poc-user', secrets.token_hex(32)
    def record(name, passed=None, **detail):
        r = dict(name=name, kind='observation' if passed is None else 'check', detail=detail)
        if passed is not None: r['passed'] = bool(passed)
        results.append(r); (out/'results.json').write_text(json.dumps(results, indent=2), encoding='utf-8')
        print(json.dumps(r), flush=True)
    def count():
        with lock: return len(accepted)
    def wait_closed():
        end = time.monotonic() + 3
        while time.monotonic() < end:
            with lock:
                if not active: return
            time.sleep(.03)
        raise TimeoutError('Observed target connections did not close')
    def echo(c, idx):
        try:
            c.settimeout(10); c.sendall(greeting)
            while b := c.recv(65536):
                with lock: accepted[idx].extend(b)
                c.sendall(b)
        except OSError: pass
        finally:
            c.close()
            with lock: active.discard(c)
    def accept(s):
        while True:
            try: c, _ = s.accept()
            except OSError: return
            with lock:
                idx = len(accepted); accepted.append(bytearray()); active.add(c)
            threading.Thread(target=echo, args=(c, idx), daemon=True).start()
    def port():
        with socket.socket() as s: s.bind(('127.0.0.1', 0)); return s.getsockname()[1]
    api, raw, relay, limited, inner, decoy = [port() for _ in range(6)]
    def connect(p):
        c = socket.create_connection(('127.0.0.1', p), timeout=3); sockets.append(c); return c
    def response(c):
        b = recvn(c, 4)
        if len(b) != 4: return None
        v, status, n = struct.unpack('!BBH', b)
        if len(recvn(c, n)) != n or v != 1: raise ValueError('Bad response')
        return status
    def authenticated(p, payload=b''):
        c = connect(p); c.sendall(request_frame(user, password) + payload)
        if response(c) != 0: raise RuntimeError('Authentication rejected')
        return c
    def no_read(c):
        try: return c.recv(256)
        except ConnectionResetError: return b''
    try:
        sink = socket.socket(); sockets.append(sink); sink.bind(('127.0.0.1', 0)); sink.listen()
        target = sink.getsockname()[1]
        threading.Thread(target=accept, args=(sink,), daemon=True).start()
        bait = socket.socket(); sockets.append(bait); bait.bind(('127.0.0.1', decoy)); bait.listen(); bait.settimeout(.1)
        def service(name, p, kind, dest, auth=False, exclusive=False):
            h = dict(type=kind)
            if auth: h['auth'] = dict(username=user, password=password)
            if kind == 'relay': h['metadata'] = dict(readTimeout='1s', nodelay=True)
            s = dict(name=name, addr=f'127.0.0.1:{p}', listener=dict(type='tcp'), handler=h,
                     forwarder=dict(nodes=[dict(name='fixed', addr=f'127.0.0.1:{dest}')]))
            if exclusive: s['climiter'] = 'one'
            return s
        cfg = dict(api=dict(addr=f'127.0.0.1:{api}'), climiters=[dict(name='one', limits=['$ 1'])], services=[
            service('raw', raw, 'tcp', target, True), service('relay', relay, 'relay', target, True),
            service('inner', inner, 'tcp', target, exclusive=True), service('limited', limited, 'relay', inner, True)])
        config = out/'config.json'; config.write_text(json.dumps(cfg, indent=2), encoding='utf-8')
        log = (out/'gost.log').open('wb')
        proc = subprocess.Popen([str(pathlib.Path(a.gost).resolve()), '-C', str(config)], stdout=log, stderr=log,
                                creationflags=subprocess.CREATE_NO_WINDOW)
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        end = time.monotonic()+8
        while True:
            try:
                with opener.open(f'http://127.0.0.1:{api}/config', timeout=.5) as r: json.load(r)
                break
            except OSError:
                if time.monotonic() > end: raise
                time.sleep(.1)
        with connect(raw) as c:
            got = recvn(c, len(greeting)); c.sendall(b'UNAUTHENTICATED'); echoed = recvn(c, 15)
            record('plain_tcp_auth_is_not_gate', greeting_received=got == greeting, unauthenticated_echo=echoed == b'UNAUTHENTICATED')
        wait_closed()
        for name, frame in [('missing', request_frame(None, '')), ('wrong', request_frame(user, 'wrong'))]:
            before = count()
            with connect(relay) as c:
                c.sendall(frame+b'MUST-NOT-REACH-UART'); status=response(c); tail=no_read(c)
            record(name+'_auth_no_backend', status == 2 and count() == before and tail == b'', status=status, new_connections=count()-before)
        for name, frame in [('garbage', b'GET / HTTP/1.0\r\n\r\n'), ('partial', request_frame(user,password)[:7]), ('idle', b'')]:
            before=count(); start=time.monotonic()
            with connect(relay) as c:
                if frame: c.sendall(frame)
                closed=no_read(c)==b''
            elapsed=time.monotonic()-start
            record(name+'_rejected_without_backend', closed and count()==before and elapsed<2.5, seconds=elapsed, new_connections=count()-before)
        # Same TCP socket, fragmented auth, final auth bytes coalesced with arbitrary payload.
        before=count(); payload=bytes(range(256))*4; frame=request_frame(user,password,decoy)
        with connect(relay) as c:
            c.sendall(frame[:7]); time.sleep(.15)
            record('incomplete_auth_does_not_dial', count()==before)
            c.sendall(frame[7:]+payload); status=response(c); got=recvn(c,len(greeting)+len(payload))
            record('valid_fragmented_auth_exact_binary', status==0 and got==greeting+payload, bytes=len(got)-len(greeting))
        wait_closed()
        with lock: captured=bytes(accepted[-1])
        record('auth_frame_not_forwarded', captured==payload, target_bytes=len(captured))
        try:
            c,_=bait.accept(); c.close(); redirected=True
        except socket.timeout: redirected=False
        record('client_cannot_override_fixed_target', not redirected)
        # Idle/invalid unauthenticated sockets must not reserve the UART's sole writer slot.
        before=count()
        with connect(limited) as idle, authenticated(limited) as owner:
            record('authenticated_owner_not_blocked_by_idle', recvn(owner,len(greeting))==greeting and count()==before+1)
            with connect(limited) as bad:
                bad.sendall(request_frame(user,'wrong')+b'junk'); status=response(bad)
            record('invalid_auth_during_owner_no_backend', status==2 and count()==before+1)
            with authenticated(limited,b'intruder') as second:
                closed=no_read(second)==b''
            owner.sendall(b'owner-survives'); got=recvn(owner,14)
            record('post_auth_exclusive_blocks_second', closed and count()==before+1)
            record('owner_survives_other_connections', got==b'owner-survives')
        wait_closed()
        try:
            with authenticated(limited) as c:
                reopened = recvn(c, len(greeting)) == greeting
            record('slot_released_for_new_authenticated_owner', reopened)
        except OSError as exc:
            record('slot_released_for_new_authenticated_owner', False, error=repr(exc))
        wait_closed()
    except Exception as exc: record('harness_failure',False,error=repr(exc))
    finally:
        if proc is not None:
            if proc.poll() is None: proc.terminate()
            try: proc.wait(timeout=5)
            except subprocess.TimeoutExpired: proc.kill(); proc.wait()
        for c in sockets:
            try: c.close()
            except OSError: pass
        with lock:
            for c in active: c.close()
        if log is not None: log.close()
    return 2 if any(r.get('passed') is False for r in results) else 0

if __name__ == '__main__': raise SystemExit(main())
