# -*- coding: utf-8 -*-
"""下载远程页面到本地，供语法与完整性检查"""
import urllib.request

opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
r = opener.open("http://106.12.71.33:8080/", timeout=15)
data = r.read()
print("status:", r.status)
print("bytes:", len(data))
print("headers:", dict(r.headers))
with open(r"C:\Users\11419\Doubao\chats\2026-10-18\new-chat\dota2-web\.e2e\remote_page.html", "wb") as f:
    f.write(data)
text = data.decode("utf-8", "replace")
print("ends with </html>:", text.rstrip().endswith("</html>"))
print("script tags:", text.count("<script"), text.count("</script>"))
print("has switchTab:", "function switchTab" in text)
