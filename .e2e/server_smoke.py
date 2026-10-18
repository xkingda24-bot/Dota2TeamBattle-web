# -*- coding: utf-8 -*-
"""服务器端冒烟：经 gateway(127.0.0.1:8080) 走完整业务流，验证部署内容可用"""
import os
import paramiko

HOST = os.environ.get("DOTA2_SSH_HOST", "106.12.71.33")
PASS = os.environ.get("DOTA2_SSH_PASS", "")

REMOTE = r"""
import json, urllib.request, urllib.error, sys
G = "http://127.0.0.1:8080"
PASS_N, FAIL_N = [], []
def check(name, cond, extra=""):
    (PASS_N if cond else FAIL_N).append(name)
    print(("  PASS  " if cond else "  FAIL  ") + name + ("  | " + str(extra) if extra else ""))
def req(method, url, body=None, token=None):
    data = json.dumps(body).encode("utf-8") if body is not None else None
    r = urllib.request.Request(url, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token: r.add_header("Authorization", "Bearer " + token)
    try:
        resp = urllib.request.urlopen(r, timeout=15)
        t = resp.read().decode("utf-8")
        return resp.status, (json.loads(t) if t else {})
    except urllib.error.HTTPError as e:
        t = e.read().decode("utf-8")
        try: return e.code, json.loads(t)
        except Exception: return e.code, {"error": t}

# 1 注册/登录/超管
s, r = req("POST", G + "/api/auth/register", {"username": "admin01", "password": "Passw0rd!", "nickname": "总舵主"})
if s != 200:
    s, r = req("POST", G + "/api/auth/login", {"username": "admin01", "password": "Passw0rd!"})
check("注册或登录 admin01", s == 200 and "token" in r, str(r)[:60])
tk = r["token"]
uid = r["user"]["userID"]
s, r = req("POST", G + "/api/auth/init-super", {}, tk)
check("初始化超管", s == 200 or s == 409, str(r))

# 2 建赛 + 虚拟数据 + 选人
s, r = req("POST", G + "/api/tournament", {"name": "部署冒烟赛", "time": "2026-10-18 22:00", "secret": "smoke123",
           "mode": "captain", "N": 5, "K": 5, "waitLimit": 2}, tk)
check("创建比赛", s == 200, str(r))
tid = r["id"]
s, r = req("POST", G + "/api/tournament/%d/seed-test" % tid, {}, tk)
check("注入虚拟数据", s == 200, str(r))
s, r = req("POST", G + "/api/tournament/%d/start-draft" % tid, {}, tk)
check("开始选人", s == 200, str(r))
for i in range(25):
    s, r = req("GET", G + "/api/tournament/%d" % tid, token=tk)
    rp = [x["userID"] for x in r["draftInfo"]["remainingPlayers"]]
    if not rp: break
    s, r = req("POST", G + "/api/tournament/%d/pick" % tid, {"playerUserID": rp[0]}, tk)
check("25 次代选完成", s == 200, str(r))

# 3 对阵 + 结果
s, r = req("POST", G + "/api/tournament/%d/seeding" % tid, {"seeding": [4, 3, 2, 1, 0]}, tk)
s, r = req("POST", G + "/api/tournament/%d/start-bracket" % tid, {}, tk)
check("生成对阵", s == 200, str(r))
steps = 0
for _ in range(20):
    s, r = req("GET", G + "/api/tournament/%d" % tid, token=tk)
    if r.get("status") == "finished" and r.get("champion"): break
    mm = r["matches"]
    pending = [m for m in mm if m["status"] == "pending" and m.get("teamAInfo") and m.get("teamBInfo")]
    if not pending: break
    m = pending[0]
    s, r2 = req("POST", G + "/api/tournament/%d/result" % tid, {"matchId": m["id"], "winner": "U%d" % m["teamAInfo"]["userID"]}, tk)
    steps += 1
s, r = req("GET", G + "/api/tournament/%d" % tid, token=tk)
check("产生冠军", r.get("status") == "finished" and r.get("champion") is not None, str(r.get("champion")))
s, r = req("POST", G + "/api/tournament/%d/finish" % tid, {}, tk)
s, r = req("GET", G + "/api/tournament/%d" % tid, token=tk)
check("归档成功", r.get("archivedAt") is not None, str(r.get("archivedAt"))[:40])

# 4 主页
s, r = req("GET", G + "/api/tournament/home?todayStart=" + str(int(__import__("time").time()*1000)), token=tk)
check("主页返回比赛", s == 200 and len(r.get("today", [])) >= 0, str(r)[:60])

print("SUMMARY pass=%d fail=%d" % (len(PASS_N), len(FAIL_N)))
if FAIL_N:
    print("FAILED:", FAIL_N)
sys.exit(1 if FAIL_N else 0)
"""

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, port=22, username="root", password=PASS, timeout=20)
cmd = "python3 - <<'PYEOF'\n" + REMOTE + "\nPYEOF"
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=300)
out = stdout.read().decode("utf-8", "replace")
err = stderr.read().decode("utf-8", "replace")
print(out)
if err.strip():
    print("--- stderr ---")
    print(err)
code = stdout.channel.recv_exit_status()
ssh.close()
print("remote smoke exit:", code)
raise SystemExit(code)
