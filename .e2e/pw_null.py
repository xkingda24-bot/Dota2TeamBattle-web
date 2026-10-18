# -*- coding: utf-8 -*-
"""精确定位 switchTab 中 null 的来源"""
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
          const q = id => document.getElementById(id);
          ['lg-form','rg-form','lg-tab','rg-tab','lg-submit','rg-submit'].forEach(id => {
            res[id] = {
              getElement: !!q(id),
              query: !!document.querySelector('#' + id),
              qsCount: document.querySelectorAll('#' + id).length,
            };
          });
          // $ 函数到底是什么
          res.dollarType = typeof $;
          res.dollarSrc = String($).slice(0, 80);
          // 逐段调用 switchTab 定位 null
          try {
            document.getElementById('rg-form').classList.remove('hidden');
            res.step1 = 'rg-form ok';
          } catch(e) { res.step1 = 'ERR ' + e.message; }
          try {
            document.getElementById('lg-form').classList.add('hidden');
            res.step2 = 'lg-form ok';
          } catch(e) { res.step2 = 'ERR ' + e.message; }
          try {
            document.getElementById('rg-tab').classList.remove('ghost');
            res.step3 = 'rg-tab ok';
          } catch(e) { res.step3 = 'ERR ' + e.message; }
          try {
            document.getElementById('lg-tab').classList.add('ghost');
            res.step4 = 'lg-tab ok';
          } catch(e) { res.step4 = 'ERR ' + e.message; }
          // 调用 switchTab 确认
          try { switchTab('rg-form','lg-form','rg-tab','lg-tab'); res.switchTab = 'OK'; }
          catch(e) { res.switchTab = 'ERR: ' + e.message; }
          return res;
        }""")
        for k, v in out.items():
            print("%s = %s" % (k, v))
        await browser.close()
    print("DONE")

asyncio.run(main())
