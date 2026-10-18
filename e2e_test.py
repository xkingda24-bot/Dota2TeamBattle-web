# -*- coding: utf-8 -*-
"""
Dota2 战队赛 Web 版 - 端到端验证脚本
前置：本机已启动 auth-service(18081) 与 tournament-service(18082)
身份：直接调用服务时，auth 用 Bearer token，tournament 用 X-User-Id 头（模拟网关注入）
运行：python e2e_test.py
"""
import http.client
import json
import time
import urllib.request
import urllib.error
import sys

AUTH = "http://127.0.0.1:18081"
TOUR = "http://127.0.0.1:18082"

PASS = []
FAIL = []


def check(name, cond, extra=""):
    (PASS if cond else FAIL).append((name, extra))
    print(("  PASS  " if cond else "  FAIL  ") + name + ("  | " + extra if extra else ""))


def req(method, url, body=None, token=None, uid=None):
    data = json.dumps(body).encode("utf-8") if body is not None else None
    r = urllib.request.Request(url, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    if uid:
        r.add_header("X-User-Id", str(uid))
    for attempt in range(2):
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
        except (ConnectionResetError, http.client.RemoteDisconnected) as e:
            if attempt == 1:
                return -1, {"error": repr(e)}
            time.sleep(0.5)
    return -1, {"error": "unreachable"}


def register(username, password, nickname):
    s, r = req("POST", AUTH + "/api/auth/register", {"username": username, "password": password, "nickname": nickname})
    return s, r


def login(username, password):
    s, r = req("POST", AUTH + "/api/auth/login", {"username": username, "password": password})
    if s != 200:
        return None, None
    return r["token"], r["user"]


def profile(token, nickname, mmr, roles, signature):
    s, r = req("POST", AUTH + "/api/auth/profile",
               {"nickname": nickname, "mmr": mmr, "roles": roles, "signature": signature}, token=token)
    return s, r


# ============ 0. 用户体系 ============
print("== 0. 用户体系 ==")
s, r = register("admin01", "Passw0rd!", "总舵主")
check("注册管理员 admin01", s == 200, str(r))
token_admin, user_admin = login("admin01", "Passw0rd!")
admin_id = user_admin.get("userID")
check("登录 admin01 并拿到 userID", token_admin is not None and admin_id, str(user_admin))

s, r = req("POST", AUTH + "/api/auth/init-super", {}, token=token_admin)
check("初始化最高管理员", s == 200 and r.get("isSuper") is True, str(r))
s, r = req("POST", AUTH + "/api/auth/init-super", {}, token=token_admin)
check("重复初始化被拒", s == 409, str(r))

auto_users = []
for i in range(4):
    s, r = register("auto0%d" % (i + 1), "Passw0rd!", "自动机%d" % (i + 1))
    tk, u = login("auto0%d" % (i + 1), "Passw0rd!")
    s2, r2 = profile(tk, "自动机%d" % (i + 1), 5000 + i * 100, [1 + i % 5], "自动测试")
    auto_users.append((tk, u["userID"]))
check("注册并完善 4 个普通用户资料", len(auto_users) == 4 and all(u for _, u in auto_users))


# ============ 1. 队长选人模式全流程 ============
print("== 1. 队长选人模式（N=5 K=5） ==")
s, r = req("POST", TOUR + "/api/tournament",
           {"name": "端到端测试赛", "time": "2026-10-18 20:00", "secret": "abc123",
            "mode": "captain", "N": 5, "K": 5, "waitLimit": 3}, token=token_admin, uid=admin_id)
check("管理员创建比赛", s == 200, str(r))
tid = r.get("id")

s, r = req("POST", TOUR + "/api/tournament/%s/seed-test" % tid, {}, token=token_admin, uid=admin_id)
check("注入虚拟测试数据", s == 200, str(r))

s, r = req("POST", TOUR + "/api/tournament/%s/start-draft" % tid, {}, token=token_admin, uid=admin_id)
check("开始选人", s == 200, str(r))
s, r = req("GET", TOUR + "/api/tournament/%s" % tid, token=token_admin, uid=admin_id)
check("选人阶段详情 draft 状态", r.get("status") == "draft" and r.get("draftInfo") is not None)

# 蛇形轮次验证：25 次代选，teamIndex 应遵循 0..4,4..0,0..4,4..0,0..4
expected_seq = [0, 1, 2, 3, 4, 4, 3, 2, 1, 0, 0, 1, 2, 3, 4, 4, 3, 2, 1, 0, 0, 1, 2, 3, 4]
got_seq = []
remaining = [x["userID"] for x in r["draftInfo"]["remainingPlayers"]]
turn = 0
ok_snake = True
while turn < 25 and len(remaining) > 0:
    s, r2 = req("POST", TOUR + "/api/tournament/%s/pick" % tid,
                {"playerUserID": remaining[0]}, token=token_admin, uid=admin_id)
    if s != 200:
        check("代选第 %d 次" % (turn + 1), False, str(r2))
        ok_snake = False
        break
    got_seq.append(r2.get("teamIndex"))
    turn += 1
    s, r = req("GET", TOUR + "/api/tournament/%s" % tid, token=token_admin, uid=admin_id)
    remaining = [x["userID"] for x in r["draftInfo"]["remainingPlayers"]]
check("25 次代选全部成功", ok_snake and turn == 25)
check("蛇形轮次落位正确", got_seq == expected_seq, str(got_seq))

player_ids = [x["userID"] for x in r["registrations"] if x["role"] == "player"]
s, r = req("POST", TOUR + "/api/tournament/%s/pick" % tid,
           {"playerUserID": player_ids[0]}, token=token_admin, uid=admin_id)
check("选完后再选被拒", s == 400, str(r))

s, r = req("POST", TOUR + "/api/tournament/%s/seeding" % tid, {"seeding": [4, 3, 2, 1, 0]}, token=token_admin, uid=admin_id)
check("设置对阵落位", s == 200, str(r))
s, r = req("POST", TOUR + "/api/tournament/%s/start-bracket" % tid, {}, token=token_admin, uid=admin_id)
check("生成 TI 双败对阵", s == 200, str(r))
s, r = req("GET", TOUR + "/api/tournament/%s" % tid, token=token_admin, uid=admin_id)
matches = r.get("matches", [])
check("场次数 = 2N-2 = 8", len(matches) == 8, "got %d" % len(matches))
champ_match = [m for m in matches if m.get("isChampionMatch")]
check("存在总决赛（isChampionMatch）", len(champ_match) == 1)
check("总决赛 BO 默认 BO1", champ_match[0].get("bo") == "BO1", str(champ_match[0]))

# 逐场记录结果（按可记场次推进，直到全部完成）
steps = 0
s, r = req("GET", TOUR + "/api/tournament/%s" % tid, token=token_admin, uid=admin_id)
mm = r["matches"]
while True:
    pending = [m for m in mm if m["status"] == "pending" and m.get("teamAInfo") and m.get("teamBInfo")]
    if not pending:
        break
    m = pending[0]
    winner = "U%d" % m["teamAInfo"]["userID"]
    s, r2 = req("POST", TOUR + "/api/tournament/%s/result" % tid,
                {"matchId": m["id"], "winner": winner}, token=token_admin, uid=admin_id)
    if s != 200:
        check("记录第 %d 场结果" % (steps + 1), False, str(r2))
        break
    steps += 1
    s, r = req("GET", TOUR + "/api/tournament/%s" % tid, token=token_admin, uid=admin_id)
    mm = r["matches"]
check("8 场比赛全部记录完成", steps == 8, "steps=%d" % steps)
s, r = req("GET", TOUR + "/api/tournament/%s" % tid, token=token_admin, uid=admin_id)
check("产生冠军且比赛结束", r.get("status") == "finished" and r.get("champion") is not None, str(r.get("champion")))

# 级联重置测试
s, r = req("POST", TOUR + "/api/tournament/%s/reset-result" % tid, {"matchId": 1}, token=token_admin, uid=admin_id)
check("重置第 1 场结果", s == 200, str(r))
s, r = req("GET", TOUR + "/api/tournament/%s" % tid, token=token_admin, uid=admin_id)
done_cnt = sum(1 for m in r["matches"] if m["status"] == "done")
check("级联重置后剩余已完赛数 < 8", done_cnt < 8, "done=%d" % done_cnt)
check("重置后冠军被清空、回到对战", r.get("champion") is None and r.get("status") == "battle")

# 重记到冠军
s, r = req("GET", TOUR + "/api/tournament/%s" % tid, token=token_admin, uid=admin_id)
mm = r["matches"]
steps = 0
while True:
    pending = [m for m in mm if m["status"] == "pending" and m.get("teamAInfo") and m.get("teamBInfo")]
    if not pending:
        break
    m = pending[0]
    winner = "U%d" % m["teamAInfo"]["userID"]
    s, r2 = req("POST", TOUR + "/api/tournament/%s/result" % tid,
                {"matchId": m["id"], "winner": winner}, token=token_admin, uid=admin_id)
    if s != 200:
        check("重记第 %d 场" % (steps + 1), False, str(r2))
        break
    steps += 1
    s, r = req("GET", TOUR + "/api/tournament/%s" % tid, token=token_admin, uid=admin_id)
    mm = r["matches"]
check("重记到全部完成（%d 场）" % steps, r.get("status") == "finished", str(r.get("champion")))

s, r = req("POST", TOUR + "/api/tournament/%s/finish" % tid, {}, token=token_admin, uid=admin_id)
check("结束归档比赛", s == 200, str(r))
s, r = req("GET", TOUR + "/api/tournament/%s" % tid, token=token_admin, uid=admin_id)
check("归档后 archivedAt 存在", r.get("archivedAt") is not None)


# ============ 2. 系统分组模式全流程 ============
print("== 2. 系统分组模式（N=2 K=2） ==")
s, r = req("POST", TOUR + "/api/tournament",
           {"name": "自动分组赛", "time": "2026-10-18 21:00", "secret": "grp456",
            "mode": "auto", "N": 2, "K": 2, "waitLimit": 1}, token=token_admin, uid=admin_id)
check("创建 auto 模式比赛", s == 200, str(r))
tid2 = r.get("id")

s, r = req("POST", TOUR + "/api/tournament/%s/register" % tid2,
           {"role": "player", "secret": "wrong"}, token=auto_users[0][0], uid=auto_users[0][1])
check("暗号错误被拒", s == 400 and "暗号" in str(r), str(r))

for tk, u in auto_users:
    s, r = req("POST", TOUR + "/api/tournament/%s/register" % tid2,
               {"role": "player", "secret": "grp456"}, token=tk, uid=u)
    check("4 名用户报名成功", s == 200, str(r))

s, r = req("POST", TOUR + "/api/tournament/%s/auto-group" % tid2, {}, token=token_admin, uid=admin_id)
check("系统分组预览", s == 200 and len(r.get("groups", [])) == 2, "maxDiff=%s" % r.get("maxDiff"))
check("每组恰好 2 人", all(len(g["members"]) == 2 for g in r.get("groups", [])))

s, r = req("POST", TOUR + "/api/tournament/%s/confirm-group" % tid2, {}, token=token_admin, uid=admin_id)
check("确认分组", s == 200, str(r))
s, r = req("GET", TOUR + "/api/tournament/%s" % tid2, token=token_admin, uid=admin_id)
teams = r.get("teams", [])
check("分组后生成 2 支队伍", len(teams) == 2)
cap0 = teams[0]["captain"]["userID"]
cap0_tk = cap0_uid = None
for tk, u in auto_users:
    if u == cap0:
        cap0_tk, cap0_uid = tk, u
        break
s, r = req("POST", TOUR + "/api/tournament/%s/team-name" % tid2,
           {"teamName": "银河战舰"}, token=cap0_tk, uid=cap0_uid)
check("组长设置队名", s == 200, str(r))
other_tk = other_uid = None
for tk, u in auto_users:
    if u != cap0:
        other_tk, other_uid = tk, u
        break
s, r = req("POST", TOUR + "/api/tournament/%s/team-name" % tid2,
           {"teamName": "乱起名"}, token=other_tk, uid=other_uid)
check("非组长设队名被拒", s == 400, str(r))

s, r = req("POST", TOUR + "/api/tournament/%s/start-bracket" % tid2, {}, token=token_admin, uid=admin_id)
check("auto 模式生成对阵", s == 200, str(r))
s, r = req("GET", TOUR + "/api/tournament/%s" % tid2, token=token_admin, uid=admin_id)
matches2 = r.get("matches", [])
check("N=2 时场次数 = 1（双败即单场冠军赛）", len(matches2) == 1, "got %d" % len(matches2))

mm2 = r["matches"]
for m in mm2:
    if m["status"] != "pending" or not m.get("teamAInfo"):
        continue
    winner = "U%d" % m["teamAInfo"]["userID"]
    s, r2 = req("POST", TOUR + "/api/tournament/%s/result" % tid2,
                {"matchId": m["id"], "winner": winner}, token=token_admin, uid=admin_id)
    check("auto 模式记录第 %d 场" % m["id"], s == 200, str(r2))
s, r = req("GET", TOUR + "/api/tournament/%s" % tid2, token=token_admin, uid=admin_id)
check("auto 模式产生冠军", r.get("status") == "finished" and r.get("champion") is not None, str(r.get("champion")))


# ============ 3. 报名阶段边界 ============
print("== 3. 报名阶段边界 ==")
s, r = req("POST", TOUR + "/api/tournament",
           {"name": "边界测试赛", "time": "2026-10-18 22:00", "secret": "bd99",
            "mode": "captain", "N": 2, "K": 2, "waitLimit": 1}, token=token_admin, uid=admin_id)
check("创建边界测试赛", s == 200, str(r))
tid3 = r.get("id")

# 未完善资料的用户（只注册不设资料）
s, r = register("noprofile", "Passw0rd!", "没资料")
tk_no, user_no = login("noprofile", "Passw0rd!")
u_no = user_no["userID"]
s, r = req("POST", TOUR + "/api/tournament/%s/register" % tid3,
           {"role": "player", "secret": "bd99"}, token=tk_no, uid=u_no)
check("未设置资料报名被拒", s == 400 and "资料" in str(r), str(r))

for tk, u in auto_users:
    s, r = req("POST", TOUR + "/api/tournament/%s/register" % tid3,
               {"role": "player", "secret": "bd99"}, token=tk, uid=u)
    check("边界赛 4 人报名", s == 200, str(r))

# 第 5 个用户：队员满 → 自动转替补
s, r = register("over01", "Passw0rd!", "替补员")
tk_over, user_over = login("over01", "Passw0rd!")
u_over = user_over["userID"]
profile(tk_over, "替补员", 6000, [2, 3], "替补的自我修养")
s, r = req("POST", TOUR + "/api/tournament/%s/register" % tid3,
           {"role": "player", "secret": "bd99"}, token=tk_over, uid=u_over)
check("队员满后自动转替补", s == 200 and r.get("role") == "waitlist", str(r))

s, r = req("POST", TOUR + "/api/tournament/%s/register" % tid3,
           {"role": "player", "secret": "bd99"}, token=tk_over, uid=u_over)
check("重复报名被拒", s == 400, str(r))

s, r = req("POST", TOUR + "/api/tournament",
           {"name": "x", "secret": "y", "N": 2, "K": 1, "waitLimit": 0}, token=auto_users[0][0], uid=auto_users[0][1])
check("普通用户创建比赛被拒", s == 403, str(r))

s, r = req("POST", TOUR + "/api/tournament/%s/cancel" % tid, {}, token=auto_users[0][0], uid=auto_users[0][1])
check("非报名阶段不能退报", s == 400, str(r))

s, r = req("POST", TOUR + "/api/tournament/%s/cancel" % tid3, {}, token=auto_users[0][0], uid=auto_users[0][1])
check("报名阶段退报成功", s == 200, str(r))
s, r = req("POST", TOUR + "/api/tournament/%s/cancel" % tid3, {}, token=auto_users[0][0], uid=auto_users[0][1])
check("重复退报被拒", s == 400, str(r))


# ============ 结果 ============
print("\n===== 汇总：%d 通过 / %d 失败 =====" % (len(PASS), len(FAIL)))
for name, extra in FAIL:
    print("  FAILED: %s | %s" % (name, extra))
sys.exit(1 if FAIL else 0)
