# -*- coding: utf-8 -*-
"""终极定位：用 $ 直接操作元素 vs switchTab"""
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
          // 直接用 $ 操作
          res.a = (() => { try { const el = $('#rg-form'); return 'get $rg-form = ' + (el ? 'element' : 'NULL'); } catch(e) { return 'ERR ' + e.message; } })();
          res.b = (() => { try { $('#rg-form').classList.remove('hidden'); return 'OK removed hidden'; } catch(e) { return 'ERR ' + e.message; } })();
          res.c = (() => { try { const el = document.querySelector('#rg-form'); return 'qs = ' + (el ? 'element' : 'NULL'); } catch(e) { return 'ERR ' + e.message; } })();
          // 用普通 function 包装 $ 调用（模拟 switchTab 的调用方式）
          res.d = (() => {
            function wrap(showId) {
              $(showId).classList.remove('hidden');
              return 'wrap OK';
            }
            try { return wrap('rg-form'); } catch(e) { return 'ERR ' + e.message; }
          })();
          // 重新定义 switchTab 并调用（用同一份源码）
          res.e = (() => {
            function switchTab2(showId, hideId, activeBtn, ghostBtn){
              $(showId).classList.remove('hidden'); $(hideId).classList.add('hidden');
              $(activeBtn).classList.remove('ghost'); $(ghostBtn).classList.add('ghost');
            }
            try { switchTab2('rg-form','lg-form','rg-tab','lg-tab'); return 'switchTab2 OK'; }
            catch(e) { return 'ERR ' + e.message; }
          })();
          // 全局 switchTab 再次调用（复位 class 后）
          res.f = (() => {
            try {
              document.getElementById('rg-form').classList.add('hidden');
              document.getElementById('lg-form').classList.remove('hidden');
              switchTab('rg-form','lg-form','rg-tab','lg-tab');
              return 'global switchTab OK';
            } catch(e) { return 'ERR ' + e.message; }
          })();
          return res;
        }""")
        for k, v in out.items():
            print("%s = %s" % (k, v))
        await browser.close()
    print("DONE")

asyncio.run(main())
