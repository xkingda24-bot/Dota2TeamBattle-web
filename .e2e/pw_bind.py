# -*- coding: utf-8 -*-
"""playwright 复现：捕获绑定阶段错误"""
import asyncio

from playwright.async_api import async_playwright

async def main():
    logs, errors = [], []
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True, args=["--no-sandbox"])
        page = await browser.new_page()
        page.on("console", lambda m: logs.append("[%s] %s" % (m.type, m.text)))
        page.on("pageerror", lambda e: errors.append(str(e)))
        await page.goto("http://106.12.71.33:8080/#/login", timeout=20000)
        await page.wait_for_timeout(3000)

        # 检查绑定状态
        info = await page.evaluate("""() => {
          const q = id => document.getElementById(id);
          const out = {
            hasView: !!q('view'),
            lgTab: !!q('lg-tab'), rgTab: !!q('rg-tab'),
            lgSubmit: !!q('lg-submit'), rgSubmit: !!q('rg-submit'),
            rgPass: !!q('rg-pass'),
            rgTabOnclick: q('rg-tab') ? (q('rg-tab').onclick ? 'bound' : 'NULL') : 'n/a',
            lgTabOnclick: q('lg-tab') ? (q('lg-tab').onclick ? 'bound' : 'NULL') : 'n/a',
            switchTabType: typeof switchTab,
            doRegisterType: typeof doRegister,
            doLoginType: typeof doLogin,
            valType: typeof val,
          };
          // 尝试手动触发绑定
          try {
            if (q('rg-tab') && q('rg-tab').onclick) { q('rg-tab').onclick(); out.afterCall = 'rg-form hidden? ' + q('rg-form').classList.contains('hidden'); }
            else out.afterCall = 'no onclick to call';
          } catch(e) { out.afterCall = 'ERR: ' + e.message; }
          return out;
        }""")
        for k, v in info.items():
            print("%s = %s" % (k, v))

        # 手动重新绑定并测试
        rebind = await page.evaluate("""() => {
          try {
            const q = id => document.getElementById(id);
            q('rg-tab').onclick = () => { q('rg-form').classList.remove('hidden'); q('lg-form').classList.add('hidden'); };
            q('rg-tab').onclick();
            return 'rebind OK, rg-form hidden? ' + q('rg-form').classList.contains('hidden');
          } catch(e) { return 'ERR: ' + e.message; }
        }""")
        print("rebind test:", rebind)

        await browser.close()

    print("=== CONSOLE ===")
    for l in logs:
        print(l)
    print("=== PAGE ERRORS ===")
    for e in errors:
        print(e)
    print("DONE")

asyncio.run(main())
