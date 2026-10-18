# -*- coding: utf-8 -*-
"""gateway 联调验证：静态前端 + 认证转发 + JWT 注入 X-User-Id"""
import json
import time
import urllib.request
import urllib.error

G = "http://127.0.0.1:8080"
PASS, FAIL = [], []


def check(name, cond, extra=""):
    (PASS if cond else FAIL).append((name, extra))
    print(("  PASS  " if cond else "  FAIL  ") + name + ("  | " + extra if extra else ""))


def req(method, url, body=None, token=None):
    data = json.dumps(body).encode("utf-8") if body is not None else None
    r = urllib.request.Request(url, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    for _ in range(2):
        try:
            resp = urllib.request.urlopen(r, timeout=10)
            text = resp.read().decode("utf-8")
            return resp.status, (json.loads(text) if text else {})
        except urllib.error.HTTPError as e:
            text = e.read().decode("utf-8")
            try:
                return e.code, json.loads(text)
            except Exception:
                return e.code, {"error": text}
        except (ConnectionResetError,) as e:
            time.sleep(0.5)
    return -1, {"error": "unreachable"}


print("== gateway 静态前端 ==")
import http.client
conn = http.client.HTTPConnection("127.0.0.1", 8080, timeout=10)
conn.request("GET", "/")
resp = conn.getresponse()
body = resp.read().decode("utf-8")
check("GET / 返回 HTML 且含标题", resp.status == 200 and "DOTA2 战队赛" in body and "<html" in body, "len=%d" % len(body))
conn.close()

print("== 认证经网关转发 ==")
s, r = req("POST", G + "/api/auth/register", {"username": "gw01", "password": "Passw0rd!", "nickname": "网关测试"})
check("经网关注册", s == 200, str(r))
s, r = req("POST", G + "/api/auth/login", {"username": "gw01", "password": "Passw0rd!"})
check("经网关登录", s == 200 and "token" in r, str(r)[:80])
token = r.get("token")

print("== JWT 注入链路 ==")
s, r = req("GET", G + "/api/tournament/home", token=token)
check("带 token 访问赛事主页", s == 200 and r.get("user") and r["user"].get("userID"), str(r)[:120])
s, r = req("GET", G + "/api/tournament/home")
check("无 token 访问赛事被拒", s == 401, str(r))
s, r = req("POST", G + "/api/tournament", {"name": "x", "secret": "y", "N": 2, "K": 1, "waitLimit": 0}, token=token)
check("经网关普通用户建赛被拒（注入生效）", s == 403, str(r))

print("\n===== 汇总：%d 通过 / %d 失败 =====" % (len(PASS), len(FAIL)))
for name, extra in FAIL:
    print("  FAILED: %s | %s" % (name, extra))
raise SystemExit(1 if FAIL else 0)
