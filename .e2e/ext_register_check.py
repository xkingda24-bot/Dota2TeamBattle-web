# -*- coding: utf-8 -*-
"""外网快速验证：注册新用户 + 登录 + 读取资料"""
import json, urllib.request, urllib.error

opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
G = "http://106.12.71.33:8080"

def req(method, url, body=None, token=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(url, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    try:
        resp = opener.open(r, timeout=20)
        t = resp.read().decode()
        return resp.status, (json.loads(t) if t else {})
    except urllib.error.HTTPError as e:
        t = e.read().decode()
        try:
            return e.code, json.loads(t)
        except Exception:
            return e.code, {"error": t}

import time
uname = "regfix%d" % int(time.time() % 100000)
s, r = req("POST", G + "/api/auth/register",
           {"username": uname, "password": "Passw0rd!", "nickname": "修复验证"})
print("register:", s, r if s != 200 else "OK")
if s != 200:
    raise SystemExit(1)
tk = r["token"]
s, r = req("GET", G + "/api/auth/me", token=tk)
print("me:", s, "userID=%s nickname=%s" % (r.get("userID"), r.get("nickname")))
s, r = req("GET", G + "/api/tournament/home", token=tk)
print("home:", s, "ok")
print("RESULT: PASS")
