# -*- coding: utf-8 -*-
"""原始 socket 直连公网 8080，确认真实服务器返回的页面内容与响应头"""
import socket, os, sys

print("env proxies: http_proxy=%r https_proxy=%r HTTP_PROXY=%r HTTPS_PROXY=%r" % (
    os.environ.get("http_proxy"), os.environ.get("https_proxy"),
    os.environ.get("HTTP_PROXY"), os.environ.get("HTTPS_PROXY")))

s = socket.create_connection(("106.12.71.33", 8080), timeout=15)
print("connected to:", s.getpeername())
s.sendall(b"GET / HTTP/1.1\r\nHost: 106.12.71.33\r\nConnection: close\r\n\r\n")
s.settimeout(15)
data = b""
try:
    while True:
        chunk = s.recv(8192)
        if not chunk:
            break
        data += chunk
except socket.timeout:
    print("recv timed out, got", len(data))
s.close()

head, _, body = data.partition(b"\r\n\r\n")
print("--- response head ---")
print(head.decode("utf-8", "replace")[:800])
print("--- body len:", len(body))
for key in [b'switchTab', b'function val(', b'rg-submit', b'onclick="doRegister()"']:
    print("  contains %-28s -> %s" % (key.decode(), key in body))
