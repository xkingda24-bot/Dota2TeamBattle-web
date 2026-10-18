# -*- coding: utf-8 -*-
"""页面验证：主页今日/历史划分（channel=chrome）"""
from playwright.sync_api import sync_playwright

BASE = 'http://106.12.71.33:8080'
with sync_playwright() as p:
    b = p.chromium.launch(channel='chrome', headless=True)
    pg = b.new_page()
    pg.goto(BASE + '/#/login')
    pg.wait_for_selector('#lg-user')
    pg.fill('#lg-user', 'admin01')
    pg.fill('#lg-pass', 'Passw0rd!')
    pg.click('#lg-submit')
    pg.wait_for_selector('#view h3:has-text("今日比赛")', timeout=15000)
    html = pg.inner_text('#view')
    # 提取今日/历史卡片内容
    import re
    today_section = html.split('今日比赛')[1].split('历史比赛')[0] if '历史比赛' in html else ''
    print('today section contains test:', 'test' in today_section)
    print('today section:', today_section.replace('\n', ' | ')[:200])
    print('history section contains test:', 'test' in html.split('历史比赛')[1] if '历史比赛' in html else False)
    b.close()
