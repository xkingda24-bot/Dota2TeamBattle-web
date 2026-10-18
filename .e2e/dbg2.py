# -*- coding: utf-8 -*-
import json, os, urllib.request, urllib.error

def req(method, url, body=None, token=None, uid=None):
    data = json.dumps(body).encode('utf-8') if body is not None else None
    r = urllib.request.Request(url, data=data, method=method)
    r.add_header('Content-Type', 'application/json')
    if token: r.add_header('Authorization', 'Bearer ' + token)
    if uid: r.add_header('X-User-Id', str(uid))
    print('REQ', url)
    for k, v in r.header_items():
        print('   HDR', repr(k), repr(v), 'ascii?', v.isascii())
    try:
        resp = urllib.request.urlopen(r, timeout=10)
        return resp.status, resp.read().decode('utf-8')
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode('utf-8')

A = 'http://127.0.0.1:18081'
T = 'http://127.0.0.1:18082'
s, r = req('POST', A + '/api/auth/register', {'username': 'noprofile', 'password': 'Passw0rd!', 'nickname': '没资料'})
print('register noprofile:', s, repr(r[:60]))
s, r = req('POST', A + '/api/auth/login', {'username': 'noprofile', 'password': 'Passw0rd!'})
print('login:', s)
u = json.loads(r)
tk = u['token']
uid = u['user']['userID']
print('tk len', len(tk), 'ascii?', tk.isascii(), 'uid', uid)
s, r = req('POST', T + '/api/tournament/3/register', {'role': 'player', 'secret': 'bd99'}, token=tk, uid=uid)
print('final:', s, repr(r[:80]))
