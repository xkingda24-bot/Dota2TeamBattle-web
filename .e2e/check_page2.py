# -*- coding: utf-8 -*-
"""验证远程页面修复内容与缓存响应头"""
import urllib.request, sys

opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
r = opener.open("http://106.12.71.33:8080/", timeout=10)
c = r.read().decode("utf-8", "replace")
print("status:", r.status)
print("Cache-Control:", r.headers.get("Cache-Control"))
print("Pragma:", r.headers.get("Pragma"))
print("len:", len(c))
checks = {
    "switchTab 函数": "function switchTab" in c,
    "val() 安全取值": "function val(" in c,
    "rg-submit 按钮 id": 'id="rg-submit"' in c,
    "lg-submit 按钮 id": 'id="lg-submit"' in c,
    "无 onclick doRegister(期望False)": 'onclick="doRegister()"' in c,
    "无 onclick doLogin(期望False)": 'onclick="doLogin()"' in c,
    "事件绑定 onclick=doRegister": "onclick=doRegister" in c,
}
for k, v in checks.items():
    print("  %-34s -> %s" % (k, v))
