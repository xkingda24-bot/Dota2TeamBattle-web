# -*- coding: utf-8 -*-
"""需求核对修复项外网验证 v5：归档只读 / 手动分组交换 / 普通用户看名单（urllib 直连）"""
import json, urllib.request

BASE = 'http://106.12.71.33:8080'
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

def call(method, path, body=None, token=''):
    req = urllib.request.Request(BASE + path, method=method)
    req.add_header('Content-Type', 'application/json')
    if token:
        req.add_header('Authorization', 'Bearer ' + token)
    data = json.dumps(body).encode() if body is not None else None
    try:
        with opener.open(req, data, timeout=15) as r:
            raw = r.read().decode()
            return r.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {'error': raw}

results = []

# --- 登录超管 ---
_, d = call('POST', '/api/auth/login', {'username': 'admin01', 'password': 'Passw0rd!'})
tok = d['token']

# --- 1. 归档比赛只读 ---
st, t2 = call('GET', '/api/tournament/2', token=tok)
print('archived status:', t2.get('status'), '| champion:', (t2.get('champion') or {}).get('name'))
results.append(('PASS' if t2.get('status') == 'finished' else 'FAIL') + ' 归档赛状态 finished')
mid = t2['matches'][0]['id']
code, resp = call('POST', '/api/tournament/2/reset-result', {'matchId': mid}, token=tok)
print('reset-result ->', code, resp)
results.append(('PASS' if code == 400 else 'FAIL') + ' reset-result 归档拦截')

# --- 2. 手动分组交换 ---
code, d2 = call('POST', '/api/tournament', {'name': '核对测试-auto', 'time': '2026-10-18 20:00', 'secret': 'test123', 'mode': 'auto', 'N': 2, 'K': 2, 'waitLimit': 0}, token=tok)
tid = d2['id']; print('created tournament id:', tid)
call('POST', '/api/tournament/%d/seed-test' % tid, {}, token=tok)
code, d3 = call('POST', '/api/tournament/%d/auto-group' % tid, {}, token=tok)
g0 = [m['userID'] for m in d3['groups'][0]['members']]
g1 = [m['userID'] for m in d3['groups'][1]['members']]
print('auto groups:', g0, '|', g1)
new0 = [g1[0]] + g0[1:]
new1 = [g0[0]] + g1[1:]
code, resp = call('POST', '/api/tournament/%d/confirm-group' % tid, {'groups': [new0, new1]}, token=tok)
print('confirm manual ->', code, resp)
code, t4 = call('GET', '/api/tournament/%d' % tid, token=tok)
actual = set()
for tm in t4['teams']:
    actual.add(tm['captain']['userID'])
    actual.update(m['userID'] for m in tm['players'])
expected = set(new0 + new1)
print('membership:', sorted(actual), '==', sorted(expected))
results.append(('PASS' if actual == expected else 'FAIL') + ' 手动交换分组落库一致')
call('DELETE', '/api/tournament/%d' % tid, token=tok)
print('cleaned:', tid)

# --- 3. 普通用户可见名单 / users 仅超管 ---
code, d5 = call('POST', '/api/auth/register', {'username': 'v5chk', 'password': 'v5chk123', 'nickname': 'v5核对'})
if code != 200:
    code, d5 = call('POST', '/api/auth/login', {'username': 'v5chk', 'password': 'v5chk123'})
tok2 = d5['token']
code, resp = call('GET', '/api/auth/staff', token=tok2)
print('staff as normal ->', code)
results.append(('PASS' if code == 200 else 'FAIL') + ' 普通用户可见名单')
code, resp = call('GET', '/api/auth/users', token=tok2)
print('users as normal ->', code)
results.append(('PASS' if code in (403, 401) else 'FAIL') + ' 用户列表仅超管')

print('\n===== SUMMARY =====')
for x in results:
    print(x)
