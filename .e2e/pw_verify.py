# -*- coding: utf-8 -*-
"""完整验证：点击注册tab切换+高亮，填写注册表单并提交"""
import asyncio, time

from playwright.async_api import async_playwright

async def main():
    errors, logs = [], []
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True, args=["--no-sandbox"])
        page = await browser.new_page()
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.on("console", lambda m: (logs.append(m.text) if m.type == "error" else None))
        await page.goto("http://106.12.71.33:8080/#/login", timeout=20000)
        await page.wait_for_timeout(2500)

        # 1. 初始状态：登录表单显示
        lg_hidden0 = await page.evaluate("document.getElementById('lg-form').classList.contains('hidden')")
        print("1 初始登录表单 hidden =", lg_hidden0)

        # 2. 点击注册 tab
        await page.click("#rg-tab")
        await page.wait_for_timeout(400)
        st = await page.evaluate("""() => ({
          rgHidden: document.getElementById('rg-form').classList.contains('hidden'),
          lgHidden: document.getElementById('lg-form').classList.contains('hidden'),
          rgGhost: document.getElementById('rg-tab').classList.contains('ghost'),
          lgGhost: document.getElementById('lg-tab').classList.contains('ghost')
        })""")
        print("2 点注册后: rg-hidden=%s lg-hidden=%s rg-ghost=%s lg-ghost=%s" % (
            st["rgHidden"], st["lgHidden"], st["rgGhost"], st["lgGhost"]))
        print("   PASS" if (not st["rgHidden"] and st["lgHidden"] and not st["rgGhost"] and st["lgGhost"]) else "   FAIL")

        # 3. 填写并提交注册
        uname = "pww%d" % int(time.time() % 1000000)
        await page.fill("#rg-user", uname)
        await page.fill("#rg-pass", "Passw0rd!")
        await page.fill("#rg-nick", "pw验证")
        await page.click("#rg-submit")
        await page.wait_for_timeout(3000)

        url = page.url
        modal_visible = await page.evaluate("document.getElementById('modal-bg').classList.contains('show')")
        modal_text = await page.evaluate("document.getElementById('modal').textContent") if modal_visible else ""
        view_len = await page.evaluate("document.getElementById('view').innerHTML.length")
        print("3 提交后: url=%s modal=%s view_len=%s" % (url, modal_visible, view_len))
        print("   弹窗内容:", modal_text[:80])
        print("   PASS" if ("#/profile" in url or "资料" in modal_text) else "   FAIL")

        await browser.close()

    print("=== 页面错误 ===")
    for e in errors:
        print("  ERR:", e)
    print("=== console error ===")
    for l in logs:
        print("  LOG:", l)
    print("DONE")

asyncio.run(main())
