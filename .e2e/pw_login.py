# -*- coding: utf-8 -*-
"""UI 验证登录流程（admin01）"""
import asyncio

from playwright.async_api import async_playwright

async def main():
    errors = []
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True, args=["--no-sandbox"])
        page = await browser.new_page()
        page.on("pageerror", lambda e: errors.append(str(e)))
        await page.goto("http://106.12.71.33:8080/#/login", timeout=20000)
        await page.wait_for_timeout(2500)
        await page.fill("#lg-user", "admin01")
        await page.fill("#lg-pass", "Passw0rd!")
        await page.click("#lg-submit")
        await page.wait_for_timeout(3000)
        url = page.url
        view_len = await page.evaluate("document.getElementById('view').innerHTML.length")
        has_card = await page.evaluate("document.querySelector('.card') !== null")
        print("login url:", url, "| view_len:", view_len, "| has_card:", has_card)
        print("PASS" if ("#/home" in url and has_card) else "FAIL")
        await browser.close()
    for e in errors:
        print("ERR:", e)
    print("DONE")

asyncio.run(main())
