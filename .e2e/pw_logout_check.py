# -*- coding: utf-8 -*-
"""验证：退出后顶部导航按钮消失"""
from playwright.sync_api import sync_playwright

BASE = 'http://106.12.71.33:8080'
with sync_playwright() as p:
    b = p.chromium.launch(channel='chrome', headless=True)
    pg = b.new_page()
    errs = []
    pg.on('pageerror', lambda e: errs.append(str(e)))
    pg.goto(BASE + '/#/login')
    pg.wait_for_selector('#lg-user')
    pg.fill('#lg-user', 'admin01')
    pg.fill('#lg-pass', 'Passw0rd!')
    pg.click('#lg-submit')
    pg.wait_for_selector('#view h3:has-text("今日比赛")', timeout=15000)
    topbar_visible_login = pg.evaluate("!document.querySelector('#topbar').classList.contains('hidden')")
    print('STEP1 登录后顶部导航显示:', 'OK' if topbar_visible_login else 'FAIL')
    # 点退出
    pg.click('#userinfo button:has-text("退出")')
    pg.wait_for_selector('#lg-user', timeout=10000)
    pg.wait_for_timeout(500)
    hidden = pg.evaluate("document.querySelector('#topbar').classList.contains('hidden')")
    print('STEP2 退出后顶部导航隐藏:', 'OK' if hidden else 'FAIL')
    print('STEP3 回到登录页:', 'OK' if pg.evaluate("location.hash") == '#/login' else 'FAIL')
    print('JS errors:', errs if errs else 'none')
    b.close()
