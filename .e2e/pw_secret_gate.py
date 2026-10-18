# -*- coding: utf-8 -*-
"""暗号闸门端到端验证 v2：真实点击为主，按钮可用性一并验证"""
import time
from playwright.sync_api import sync_playwright

BASE = 'http://106.12.71.33:8080'
ts = int(time.time())
user = 'gate%s' % (ts % 100000000)
nick = '闸门测%s' % (ts % 100000)

with sync_playwright() as p:
    b = p.chromium.launch(channel='chrome', headless=True)
    ctx = b.new_context()
    pg = ctx.new_page()
    errs = []
    pg.on('pageerror', lambda e: errs.append(str(e)))

    def goto_home():
        pg.evaluate("location.hash='#/home'")
        pg.wait_for_selector('#view h3:has-text("今日比赛")', timeout=15000)

    def click_test_card():
        pg.click('#view .card[style*="cursor:pointer"]:has-text("test")')
        pg.wait_for_selector('#modal:has-text("输入报名暗号")', timeout=10000)

    def click_modal_btn(label):
        pg.click('#modal .btn:has-text("'+label+'")', timeout=5000)

    # 注册
    pg.goto(BASE + '/#/login')
    pg.wait_for_selector('#rg-tab')
    pg.click('#rg-tab')
    pg.fill('#rg-user', user)
    pg.fill('#rg-pass', 'Passw0rd!')
    pg.fill('#rg-nick', nick)
    pg.click('#rg-submit')
    pg.wait_for_selector('#p-mmr', timeout=15000)
    if pg.evaluate("document.querySelector('#modal-bg').classList.contains('show')"):
        pg.click('#modal .btn')
    pg.fill('#p-mmr', '3450')
    pg.click('#p-roles .chip[data-r="1"]')
    pg.click('#p-roles .chip[data-r="2"]')
    pg.fill('#p-sig', '测试签名')
    pg.click('text=保存资料')
    pg.wait_for_selector('#modal:has-text("资料已保存")', timeout=10000)
    pg.click('#modal .btn')
    goto_home()

    # STEP1 点击比赛 → 弹暗号框
    click_test_card()
    print('STEP1 点击比赛后弹出暗号框: OK')

    # STEP2 错误暗号（真实点击验证按钮）
    pg.fill('#gate-secret', 'wrongsecret')
    click_modal_btn('验证')
    pg.wait_for_selector('#modal:has-text("暗号错误")', timeout=8000)
    print('STEP2 错误暗号被拒(按钮点击): OK')
    # 关闭错误提示
    pg.click('#modal .btn:has-text("确定")')

    # 重新进入比赛，验证再次弹框 + 正确暗号（真实点击）
    goto_home()
    click_test_card()
    pg.fill('#gate-secret', 'test')
    click_modal_btn('验证')
    pg.wait_for_selector('#view h3:has-text("报名")', timeout=10000)
    print('STEP3 正确暗号进入报名页(按钮点击): OK')

    # STEP4 报名暗号自动填充 + 报名
    pg.wait_for_selector('#r-secret', timeout=8000)
    sec = pg.input_value('#r-secret')
    print('STEP4 报名暗号自动填充:', sec, ('OK' if sec == 'test' else 'FAIL'))
    pg.click('#view button:has-text("报名")', timeout=5000)
    pg.wait_for_selector('#view:has-text("你已报名")', timeout=10000)
    print('STEP5 报名成功: OK')

    # STEP6 本机已通过暗号 → 直接进入不再弹窗
    if pg.evaluate("document.querySelector('#modal-bg').classList.contains('show')"):
        pg.click('#modal .btn:has-text("确定")')
    pg.click('#view button:has-text("返回")', timeout=5000)
    goto_home()
    pg.click('#view .card[style*="cursor:pointer"]:has-text("test")')
    pg.wait_for_selector('#view h3:has-text("报名")', timeout=8000)
    gate_shown = pg.evaluate("document.querySelector('#modal-bg').classList.contains('show') && document.querySelector('#modal').innerText.includes('输入报名暗号')")
    print('STEP6 本机已通过暗号直接进入(不再弹窗):', 'OK' if not gate_shown else 'FAIL')
    print('JS errors:', errs if errs else 'none')
    b.close()
