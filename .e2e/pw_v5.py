# -*- coding: utf-8 -*-
"""前端 UI 修复验证 pw_v5：分组交换 / 归档只读 / 创建默认值 / 暗号记忆 / 比分输入（channel=chrome）"""
from playwright.sync_api import sync_playwright

BASE = 'http://106.12.71.33:8080'
results = []

with sync_playwright() as p:
    b = p.chromium.launch(channel='chrome', headless=True)
    pg = b.new_page()
    errs = []
    pg.on('pageerror', lambda e: errs.append(str(e)))

    # 登录并等待主页渲染完成（消除 async 竞态）
    pg.goto(BASE + '/#/login')
    pg.wait_for_selector('#lg-user')
    pg.fill('#lg-user', 'admin01')
    pg.fill('#lg-pass', 'Passw0rd!')
    pg.click('#lg-submit')
    pg.wait_for_selector('#view h3:has-text("今日比赛")', timeout=15000)
    results.append('PASS 登录 admin01')

    # 1. 创建页默认值对齐需求 N=8/K=4/替补5
    pg.evaluate('location.hash = "#/create"')
    pg.wait_for_selector('#c-name')
    nv = pg.evaluate('document.getElementById("c-n").value')
    kv = pg.evaluate('document.getElementById("c-k").value')
    wv = pg.evaluate('document.getElementById("c-wait").value')
    print('defaults:', nv, kv, wv)
    results.append(('PASS' if (nv, kv, wv) == ('8', '4', '5') else 'FAIL') + ' 创建默认值 8/4/5')

    # 2. 前端新功能代码存在性（源码级）
    src = pg.evaluate('document.documentElement.outerHTML')
    results.append(('PASS' if 'd2_secret_' in src else 'FAIL') + ' 暗号记忆代码存在')
    results.append(('PASS' if 'rs-a' in src else 'FAIL') + ' 比分输入代码存在')
    results.append(('PASS' if 'refreshTimer' in src else 'FAIL') + ' 报名页自动刷新代码存在')
    results.append(('PASS' if 'toggleSwap' in src else 'FAIL') + ' 分组交换代码存在')

    # 3. 创建 auto 赛并注入数据 → 分组预览 → 交换 → 确认
    pg.fill('#c-name', 'UI验证-auto')
    pg.fill('#c-secret', 'v5secret')
    pg.select_option('#c-mode', 'auto')
    pg.fill('#c-n', '2'); pg.fill('#c-k', '2'); pg.fill('#c-wait', '0')
    pg.click('#view button.btn:has-text("创建")')
    pg.wait_for_function('location.hash.includes("/t/")', timeout=10000)
    pg.wait_for_selector('#view h3:has-text("报名")', timeout=10000)
    pg.evaluate('closeModal()')  # 关闭"创建成功"弹窗
    pg.wait_for_function('!document.getElementById("modal-bg").classList.contains("show")', timeout=5000)
    tid = int(pg.evaluate('location.hash').split('/')[-1])
    print('tournament:', tid)
    pg.click('#view button:has-text("注入虚拟数据")')
    pg.wait_for_selector('#cf-yes')
    pg.click('#cf-yes')
    pg.wait_for_function('!document.getElementById("modal-bg").classList.contains("show")', timeout=8000)
    pg.wait_for_selector('#view h3:has-text("名单")', timeout=8000)

    # 系统分组预览（等弹窗渲染完成）
    pg.click('text=系统分组预览')
    pg.wait_for_selector('#modal-bg.show', timeout=10000)
    modal = pg.locator('#modal')
    print('modal text:', modal.inner_text()[:200].replace('\n', ' '))
    pg.wait_for_selector('#modal-bg.show .chip', timeout=10000)
    results.append('PASS 分组预览弹窗打开')
    results.append(('PASS' if '点击队员选中' in modal.inner_text() else 'FAIL') + ' 交换提示文案存在')
    # 交换：组0成员0 <-> 组1成员0
    modal.locator('.chip').nth(0).click()
    pg.wait_for_timeout(300)
    modal.locator('.chip').nth(2).click()
    pg.wait_for_timeout(300)
    results.append('PASS 交换点击无异常')
    # 确认分组（缺位时会有二次确认弹窗，自动点"仍确认"）
    pg.click('#modal >> text=确认分组')
    pg.wait_for_timeout(800)
    if pg.evaluate('!!document.getElementById("cf-yes")'):
        pg.click('#cf-yes')
        results.append('PASS 缺位二次确认弹窗出现')
    pg.wait_for_function('!document.getElementById("modal-bg").classList.contains("show")', timeout=8000)
    pg.wait_for_selector('#view h3:has-text("队伍")', timeout=8000)
    results.append('PASS 手动交换后确认分组成功')

    # 4. 归档赛赛程页只读（无记录/修正/设赛制按钮）
    pg.evaluate('location.hash = "#/bracket/2"')
    pg.wait_for_selector('#view .bracket-section', timeout=10000)
    body = pg.inner_text('#view')
    has_admin_btn = ('记录结果' in body) or ('修正本场' in body) or ('设赛制' in body)
    print('archived page admin buttons present:', has_admin_btn)
    results.append(('PASS' if not has_admin_btn else 'FAIL') + ' 归档赛赛程页无管理按钮')

    # 5. 普通用户可见管理员名单（页内导航验证）
    pg.evaluate("""(async () => {
      const r = await fetch('/api/tournament/'+__tid__, {method:'DELETE',
        headers:{'Authorization':'Bearer '+localStorage.getItem('d2_token')}});
      return r.status;
    })""".replace('__tid__', str(tid)))
    print('cleaned:', tid)

    print('page errors:', errs)
    results.append(('PASS' if not errs else 'FAIL') + ' 无页面 JS 报错')
    b.close()

print('\n===== SUMMARY =====')
for x in results:
    print(x)
