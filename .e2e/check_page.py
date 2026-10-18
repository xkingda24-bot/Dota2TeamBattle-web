# -*- coding: utf-8 -*-
"""检查远程/本地页面注册表单完整性"""
import urllib.request, urllib.error, sys

def get(url):
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        r = opener.open(url, timeout=10)
        return r.status, r.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", "replace")
    except Exception as e:
        return 0, str(e)

url = sys.argv[1] if len(sys.argv) > 1 else "http://106.12.71.33:8080/"
s, c = get(url)
print("page:", url, "status=", s, "len=", len(c))
for key in ['id="rg-user"', 'id="rg-pass"', 'id="rg-nick"', 'onclick="doRegister()"',
            'id="rg-tab"', 'function doRegister', 'id="lg-user"']:
    print("  contains %-28s -> %s" % (key, key in c))
i = c.find('<div id="rg-form"')
if i >= 0:
    print("rg-form html:", c[i:i+450])
