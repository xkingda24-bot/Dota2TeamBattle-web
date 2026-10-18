# -*- coding: utf-8 -*-
"""打印服务器页面 switchTab 函数源码，对比本地"""
import asyncio

from playwright.async_api import async_playwright

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True, args=["--no-sandbox"])
        page = await browser.new_page()
        await page.goto("http://106.12.71.33:8080/#/login", timeout=20000)
        await page.wait_for_timeout(3000)
        out = await page.evaluate("""() => {
          const res = {};
          res.switchTabSrc = typeof switchTab === 'function' ? String(switchTab) : 'NOT FUNCTION';
          res.valSrc = typeof val === 'function' ? String(val) : 'NOT FUNCTION';
          res.scriptText = document.querySelector('script') ? document.querySelector('script').textContent.slice(0, 100) : 'no script';
          res.scriptLen = document.querySelector('script') ? document.querySelector('script').textContent.length : 0;
          return res;
        }""")
        for k, v in out.items():
            print("=== %s ===" % k)
            print(v if isinstance(v, str) else v)
        await browser.close()
    print("DONE")

asyncio.run(main())
