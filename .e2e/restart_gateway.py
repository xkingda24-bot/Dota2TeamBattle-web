# -*- coding: utf-8 -*-
"""重启 gateway 并验证新页面生效"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("106.12.71.33", 22, username="root", password="Smily123", timeout=20)
cmds = """
systemctl restart dota2-gateway
sleep 2
systemctl is-active dota2-auth dota2-tournament dota2-gateway
echo ---
curl -s -o /dev/null -w "HTTP %{http_code} (%{size_download} bytes)\n" http://127.0.0.1:8080/
"""
stdin, stdout, stderr = ssh.exec_command(cmds, timeout=60)
print(stdout.read().decode("utf-8", "replace"))
err = stderr.read().decode("utf-8", "replace")
if err.strip():
    print("stderr:", err)
ssh.close()
