# -*- coding: utf-8 -*-
"""深挖：test 的 createdAt 实际值 vs todayStart 各取值"""
import json, urllib.request, time

B = 'http://106.12.71.33:8080'
op = urllib.request.build_opener(urllib.request.ProxyHandler({}))

def call(m, p, b=None, tok=''):
    r = urllib.request.Request(B + p, method=m, data=(json.dumps(b).encode() if b is not None else None),
                               headers={'Content-Type': 'application/json', 'Authorization': 'Bearer ' + tok})
    try:
        with op.open(r, timeout=15) as x:
            return x.status, json.loads(x.read().decode() or '{}')
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read().decode() or '{}')

_, d = call('POST', '/api/auth/login', {'username': 'admin01', 'password': 'Passw0rd!'})
tok = d['token']

now_ms = int(time.time() * 1000)
now_s = int(time.time())
today0_s = now_s // 86400 * 86400 - 8 * 3600
today0_ms = today0_s * 1000
print('now_s=%d now_ms=%d today0_s=%d today0_ms=%d' % (now_s, now_ms, today0_s, today0_ms))

for name, ts in [('sec', today0_s), ('ms', today0_ms), ('zero', 0), ('now_ms', now_ms)]:
    c, h = call('GET', '/api/tournament/home?todayStart=%d' % ts, tok=tok)
    t_names = [(t.get('name'), t.get('status'), t.get('archivedAt')) for t in h['today']]
    o_names = [(t.get('name'), t.get('status'), t.get('archivedAt')) for t in h['older']]
    print('todayStart=%s(%d) -> today=%s older=%s' % (name, ts, t_names, o_names))
