# -*- coding: utf-8 -*-
"""UI 完整保存：选位置 + 合法天梯分"""
import asyncio

from playwright.async_api import async_playwright

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True, args=["--no-sandbox"])
        page = await browser.new_page()
        await page.goto("http://106.12.71.33:8080/#/login", timeout=20000)
        await page.wait_for_timeout(2000)
        await page.fill("#lg-user", "admin01")
        await page.fill("#lg-pass", "Passw0rd!")
        await page.click("#lg-submit")
        await page.wait_for_timeout(2500)
        await page.goto("http://106.12.71.33:8080/#/profile", timeout=15000)
        await page.wait_for_timeout(1500)

        # 选一号位
        await page.click(".chip[data-r='1']")
        await page.fill("#p-mmr", "5000")
        await page.evaluate("saveProfile()")
        await page.wait_for_timeout(900)
        modal = await page.evaluate("document.getElementById('modal').textContent")
        print("UI 保存(位置+mmr=5000):", modal[:40])
        print("PASS" if "资料已保存" in modal else "FAIL")

        # 回读资料页确认 mmr 已保存
        await page.goto("http://106.12.71.33:8080/#/profile", timeout=15000)
        await page.wait_for_timeout(1500)
        mmr_val = await page.evaluate("document.getElementById('p-mmr').value")
        print("回读 mmr:", mmr_val)
        print("PASS" if mmr_val == "5000" else "FAIL")
        await browser.close()
    print("DONE")

asyncio.run(main())
