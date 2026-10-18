# -*- coding: utf-8 -*-
"""服务器网络排查：监听端口、自环访问、防火墙"""
import os
import paramiko

HOST = os.environ.get("DOTA2_SSH_HOST", "106.12.71.33")
PASS = os.environ.get("DOTA2_SSH_PASS", "")

CMDS = r"""
echo "=== 8080/22 监听 ==="
ss -tlnp | grep -E ':8080|:22 ' || true
echo "=== 本机 127.0.0.1:8080 ==="
curl -s -o /dev/null -w "HTTP %{http_code}\n" --max-time 10 http://127.0.0.1:8080/ || echo fail
echo "=== 服务器自环公网IP:8080 ==="
curl -s -o /dev/null -w "HTTP %{http_code}\n" --max-time 10 http://106.12.71.33:8080/ || echo fail
echo "=== 公网IP:22 ==="
timeout 8 bash -c 'echo > /dev/tcp/106.12.71.33/22' 2>/dev/null && echo "22 reachable" || echo "22 NOT reachable"
echo "=== ufw ==="
ufw status 2>/dev/null || echo "ufw not installed"
echo "=== iptables INPUT ==="
iptables -L INPUT -n 2>/dev/null | head -20 || echo "no iptables perm"
echo "=== nginx/其他 8080 服务 ==="
ps aux | grep -E 'nginx|apache|frp' | grep -v grep || echo "none"
"""

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, port=22, username="root", password=PASS, timeout=20)
stdin, stdout, stderr = ssh.exec_command(CMDS, timeout=60)
print(stdout.read().decode("utf-8", "replace"))
err = stderr.read().decode("utf-8", "replace")
if err.strip():
    print("--- stderr ---")
    print(err)
ssh.close()
