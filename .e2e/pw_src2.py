# -*- coding: utf-8 -*-
"""打印服务器页面 doRegister / val / api.call 源码"""
import asyncio

from playwright.async_api import async_playwright

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True, args=["--no-sandbox"])
        page = await browser.new_page()
        await page.goto("http://106.12.71.33:8080/#/login", timeout=20000)
        await page.wait_for_timeout(2500)
        out = await page.evaluate("""() => {
          const g = name => (typeof eval(name) === 'function') ? String(eval(name)) : 'NOT FUNCTION: ' + typeof eval(name);
          return {
            doRegister: g('doRegister'),
            val: g('val'),
            switchTab: g('switchTab'),
            doLogin: g('doLogin'),
          };
        }""")
        for k, v in out.items():
            print("=== %s ===" % k)
            print(v)
            print()
        await browser.close()
    print("DONE")

asyncio.run(main())
