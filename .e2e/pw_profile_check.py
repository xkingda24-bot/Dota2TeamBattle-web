# -*- coding: utf-8 -*-
"""验证资料页：天梯分必填 + 1-20000 范围（UI + API）"""
import asyncio, json, urllib.request

from playwright.async_api import async_playwright

def api_call(method, path, body=None, token=None):
    """socket 直连（绕过代理）调用 API"""
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request("http://106.12.71.33:8080" + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        r = urllib.request.build_opener(urllib.request.ProxyHandler({})).open(req, timeout=15)
        return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True, args=["--no-sandbox"])
        page = await browser.new_page()
        await page.goto("http://106.12.71.33:8080/#/login", timeout=20000)
        await page.wait_for_timeout(2000)
        # 登录 admin01
        await page.fill("#lg-user", "admin01")
        await page.fill("#lg-pass", "Passw0rd!")
        await page.click("#lg-submit")
        await page.wait_for_timeout(2500)
        await page.goto("http://106.12.71.33:8080/#/profile", timeout=15000)
        await page.wait_for_timeout(1500)

        label = await page.evaluate("document.querySelector('label') ? Array.from(document.querySelectorAll('label')).map(l=>l.textContent).join('|') : ''")
        print("1 标签:", label)
        print("   PASS" if "必填" in label and "1-20000" in label else "   FAIL")

        mmr_attr = await page.evaluate("document.getElementById('p-mmr').getAttribute('min') + '/' + document.getElementById('p-mmr').getAttribute('max')")
        print("2 min/max:", mmr_attr)

        # 清空天梯分提交 -> 应提示请填写天梯分
        await page.fill("#p-mmr", "")
        await page.click("#p-mmr")
        await page.evaluate("""() => {
          const el = document.getElementById('p-mmr');
          el.value = ''; el.dispatchEvent(new Event('input', {bubbles:true}));
        }""")
        await page.evaluate("saveProfile()")
        await page.wait_for_timeout(600)
        modal = await page.evaluate("document.getElementById('modal').textContent")
        print("3 空值提交:", modal[:40])
        print("   PASS" if "请填写天梯分" in modal else "   FAIL")

        # 填 0 提交 -> 范围提示
        await page.fill("#p-mmr", "0")
        await page.evaluate("saveProfile()")
        await page.wait_for_timeout(600)
        modal = await page.evaluate("document.getElementById('modal').textContent")
        print("4 mmr=0:", modal[:40])
        print("   PASS" if "1-20000" in modal else "   FAIL")

        # 填 20001 -> 范围提示
        await page.fill("#p-mmr", "20001")
        await page.evaluate("saveProfile()")
        await page.wait_for_timeout(600)
        modal = await page.evaluate("document.getElementById('modal').textContent")
        print("5 mmr=20001:", modal[:40])
        print("   PASS" if "1-20000" in modal else "   FAIL")

        # 填合法值 5000 保存
        await page.fill("#p-mmr", "5000")
        await page.evaluate("saveProfile()")
        await page.wait_for_timeout(800)
        modal = await page.evaluate("document.getElementById('modal').textContent")
        print("6 mmr=5000:", modal[:40])
        print("   PASS" if "资料已保存" in modal else "   FAIL")

        await browser.close()

    # API 层验证
    token = None
    st, body = api_call("POST", "/api/auth/login", {"username": "admin01", "password": "Passw0rd!"})
    token = json.loads(body)["token"]
    st1, b1 = api_call("POST", "/api/auth/profile", {"nickname": "admin01", "roles": [1, 2], "signature": "x"}, token)
    print("7 API 不带 mmr: status=%s msg=%s" % (st1, json.loads(b1).get("error", "")))
    st2, b2 = api_call("POST", "/api/auth/profile", {"nickname": "admin01", "mmr": 0, "roles": [1, 2], "signature": "x"}, token)
    print("8 API mmr=0: status=%s msg=%s" % (st2, json.loads(b2).get("error", "")))
    st3, b3 = api_call("POST", "/api/auth/profile", {"nickname": "admin01", "mmr": 20001, "roles": [1, 2], "signature": "x"}, token)
    print("9 API mmr=20001: status=%s msg=%s" % (st3, json.loads(b3).get("error", "")))
    st4, b4 = api_call("POST", "/api/auth/profile", {"nickname": "admin01", "mmr": 5000, "roles": [1, 2], "signature": "x"}, token)
    print("10 API mmr=5000: status=%s" % st4)
    print("DONE")

asyncio.run(main())
