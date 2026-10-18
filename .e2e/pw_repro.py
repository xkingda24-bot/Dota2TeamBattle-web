# -*- coding: utf-8 -*-
"""playwright 驱动系统 Chrome，复现注册页操作并捕获 console 错误"""
import asyncio, json, sys

from playwright.async_api import async_playwright

async def main():
    logs = []
    errors = []
    async with async_playwright() as p:
        try:
            browser = await p.chromium.launch(channel="chrome", headless=True, args=["--no-sandbox"])
        except Exception as e:
            print("LAUNCH FAIL:", e)
            return 1
        page = await browser.new_page()
        page.on("console", lambda m: logs.append("[%s] %s" % (m.type, m.text)))
        page.on("pageerror", lambda e: errors.append(str(e)))

        await page.goto("http://106.12.71.33:8080/#/login", timeout=20000)
        await page.wait_for_timeout(3000)

        view_html = await page.evaluate("document.getElementById('view').innerHTML")
        print("view len:", len(view_html))
        print("view has lg-user:", 'id="lg-user"' in view_html)
        print("view has rg-user:", 'id="rg-user"' in view_html)

        # 点击注册标签
        await page.click("#rg-tab")
        await page.wait_for_timeout(500)
        rg_hidden = await page.evaluate("document.getElementById('rg-form').classList.contains('hidden')")
        lg_hidden = await page.evaluate("document.getElementById('lg-form').classList.contains('hidden')")
        rg_ghost = await page.evaluate("document.getElementById('rg-tab').classList.contains('ghost')")
        lg_ghost = await page.evaluate("document.getElementById('lg-tab').classList.contains('ghost')")
        print("after click rg-tab: rg-hidden=%s lg-hidden=%s rg-ghost=%s lg-ghost=%s" % (rg_hidden, lg_hidden, rg_ghost, lg_ghost))

        # 填写注册表单并提交
        await page.fill("#rg-user", "pwtest01")
        await page.fill("#rg-pass", "Passw0rd!")
        await page.fill("#rg-nick", "pw测试")
        await page.click("#rg-submit")
        await page.wait_for_timeout(2500)

        url = page.url
        modal_html = await page.evaluate("document.getElementById('modal').innerHTML") if await page.evaluate("document.getElementById('modal-bg').classList.contains('show')") else "(no modal)"
        print("url after submit:", url)
        print("modal:", modal_html[:200])
        print("view now len:", len(await page.evaluate("document.getElementById('view').innerHTML")))

        await browser.close()

    print("=== CONSOLE LOGS ===")
    for l in logs:
        print(l)
    print("=== PAGE ERRORS ===")
    for e in errors:
        print(e)
    print("DONE")

asyncio.run(main())
