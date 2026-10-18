# -*- coding: utf-8 -*-
"""核心链路回归 v5：captain 选人 → 生成对阵 → BO3 带比分 → 归档只读（urllib 直连）"""
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

_, d = call('POST', '/api/auth/login', {'username': 'admin01', 'password': 'Passw0rd!'})
tok = d['token']
R = []

# 建 captain 赛 N=2 K=2
code, t = call('POST', '/api/tournament', {'name': '回归-v5', 'time': '2026-10-18 21:00', 'secret': 'reg5', 'mode': 'captain', 'N': 2, 'K': 2, 'waitLimit': 0}, token=tok)
tid = t['id']
call('POST', '/api/tournament/%d/seed-test' % tid, {}, token=tok)
code, resp = call('POST', '/api/tournament/%d/start-draft' % tid, {}, token=tok)
R.append(('PASS' if code == 200 else 'FAIL') + ' start-draft')

# 蛇形选人（管理员代选，轮流校验由服务端强制）
code, t = call('GET', '/api/tournament/%d' % tid, token=tok)
draft = t['draftInfo']
print('draft turn total:', draft['turn'], '/', draft['totalPicks'])
picks = 0
while picks < draft['totalPicks']:
    code, t = call('GET', '/api/tournament/%d' % tid, token=tok)
    d2 = t['draftInfo']
    if d2['done']:
        break
    rem = d2['remainingPlayers']
    if not rem:
        print('no remaining players!'); break
    pid = rem[0]['userID']
    code, resp = call('POST', '/api/tournament/%d/pick' % tid, {'playerUserID': pid}, token=tok)
    if code != 200:
        print('pick fail:', code, resp); break
    picks += 1
R.append(('PASS' if picks == 4 else 'FAIL') + ' 蛇形选人 4 次代选完成 (picks=%d)' % picks)

# 生成对阵（N=2 → 1 场总决赛）
code, resp = call('POST', '/api/tournament/%d/start-bracket' % tid, {}, token=tok)
R.append(('PASS' if code == 200 else 'FAIL') + ' start-bracket')
code, t = call('GET', '/api/tournament/%d' % tid, token=tok)
print('matches:', [(m['id'], m['label'], m['bo'], m['stage']) for m in t['matches']])
gf = t['matches'][0]

# 设 BO3 + 记录带比分 2:0
code, resp = call('POST', '/api/tournament/%d/bo' % tid, {'matchId': gf['id'], 'bo': 'BO3'}, token=tok)
R.append(('PASS' if code == 200 else 'FAIL') + ' 设 BO3')
winner = gf.get('winner') or ('U' + str(t['teams'][0]['captain']['userID']))
print('winner arg:', winner, '| teamA info:', gf.get('teamA'), gf.get('displayA'))
code, resp = call('POST', '/api/tournament/%d/result' % tid, {'matchId': gf['id'], 'winner': winner, 'scoreA': 2, 'scoreB': 0}, token=tok)
print('result ->', code, resp)
R.append(('PASS' if code == 200 else 'FAIL') + ' 带比分记录结果')
code, t = call('GET', '/api/tournament/%d' % tid, token=tok)
m = t['matches'][0]
print('after result:', m['status'], 'scoreA=', m.get('scoreA'), 'scoreB=', m.get('scoreB'), 'winner=', m['winner'])
R.append(('PASS' if m['status'] == 'done' and m.get('scoreA') == 2 and m.get('scoreB') == 0 else 'FAIL') + ' 比分 2:0 落库')
R.append(('PASS' if t.get('champion') else 'FAIL') + ' 冠军产生')

# 归档
code, resp = call('POST', '/api/tournament/%d/finish' % tid, {}, token=tok)
R.append(('PASS' if code == 200 else 'FAIL') + ' 归档')
# 归档后再修正 → 400
code, resp = call('POST', '/api/tournament/%d/reset-result' % tid, {'matchId': gf['id']}, token=tok)
print('reset after archive:', code, resp)
R.append(('PASS' if code == 400 else 'FAIL') + ' 归档后修正被拒')

# 清理
call('DELETE', '/api/tournament/%d' % tid, token=tok)
print('cleaned:', tid)

print('\n===== SUMMARY =====')
for x in R:
    print(x)
