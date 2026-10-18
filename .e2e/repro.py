import json, urllib.request, urllib.error
def req(method, url, body=None, token=None, uid=None):
    data = json.dumps(body).encode("utf-8") if body is not None else None
    r = urllib.request.Request(url, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token: r.add_header("Authorization", "Bearer " + token)
    if uid: r.add_header("X-User-Id", str(uid))
    try:
        resp = urllib.request.urlopen(r, timeout=10)
        return resp.status, resp.read().decode("utf-8")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8")
    except Exception as e:
        return -1, repr(e)
T="http://127.0.0.1:18082"; A="http://127.0.0.1:18081"
s,r = req("POST", A+"/api/auth/login", {"username":"auto01","password":"Passw0rd!"})
u=json.loads(r); tk=u["token"]; uid=u["user"]["userID"]
print("login", s, uid)
for i in range(3):
    s,r = req("POST", T+"/api/tournament/3/cancel", {}, token=tk, uid=uid)
    print("cancel try%d"%(i+1), s, repr(r))
