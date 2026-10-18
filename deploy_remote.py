# -*- coding: utf-8 -*-
"""
Dota2 战队赛 Web 版 - 远程部署脚本（本机执行）
前置：pip install paramiko（已装）
用法：设置以下环境变量后运行 python deploy_remote.py
  DOTA2_SSH_HOST   服务器 IP（默认 106.12.71.33）
  DOTA2_SSH_USER   登录用户（默认 root）
  DOTA2_SSH_PASS   登录密码
流程：上传二进制与 systemd 单元 → 服务器生成密钥/env → 安装并启动服务 → 放行 8080 → 验证
"""
import os
import sys
import stat
import time

import paramiko

HOST = os.environ.get("DOTA2_SSH_HOST", "106.12.71.33")
USER = os.environ.get("DOTA2_SSH_USER", "root")
PASS = os.environ.get("DOTA2_SSH_PASS", "")
ROOT = os.path.dirname(os.path.abspath(__file__))
BINS = [
    os.path.join(ROOT, "release", "linux-amd64", "dota2-auth"),
    os.path.join(ROOT, "release", "linux-amd64", "dota2-tournament"),
    os.path.join(ROOT, "release", "linux-amd64", "dota2-gateway"),
]
UNITS = [
    os.path.join(ROOT, "deploy", "dota2-auth.service"),
    os.path.join(ROOT, "deploy", "dota2-tournament.service"),
    os.path.join(ROOT, "deploy", "dota2-gateway.service"),
]

SERVER_SETUP = r"""
set -e
mkdir -p /opt/dota2/bin /opt/dota2/data /opt/dota2/etc
# 用 mv 原子替换上传的 .new 临时文件（避免覆盖运行中二进制被文件保护拦截）
for b in dota2-auth dota2-tournament dota2-gateway; do
  if [ -f /opt/dota2/bin/$b.new ]; then mv -f /opt/dota2/bin/$b.new /opt/dota2/bin/$b; fi
done
chmod +x /opt/dota2/bin/* 2>/dev/null || true

# ---- 生成密钥与环境变量（首次部署生成；重复部署保留已有密钥，避免旧令牌失效） ----
if [ ! -s /opt/dota2/etc/auth.env ]; then
  python3 - <<'PYEOF'
import secrets
with open('/opt/dota2/etc/auth.env', 'w') as f:
    f.write('JWT_SECRET=%s\nINTERNAL_KEY=%s\nAUTH_ADDR=127.0.0.1:8081\nAUTH_DB=/opt/dota2/data/auth.db\n' %
            (secrets.token_hex(32), secrets.token_hex(32)))
with open('/opt/dota2/etc/tournament.env', 'w') as f:
    f.write('TOURNAMENT_ADDR=127.0.0.1:8082\nTOURNAMENT_DB=/opt/dota2/data/tournament.db\nAUTH_INTERNAL_URL=http://127.0.0.1:8081\n')
with open('/opt/dota2/etc/gateway.env', 'w') as f:
    f.write('GATEWAY_ADDR=0.0.0.0:8080\nGATEWAY_AUTH_URL=http://127.0.0.1:8081\nGATEWAY_TOUR_URL=http://127.0.0.1:8082\n')
PYEOF
  # 同步共享密钥到 tournament / gateway
  grep '^JWT_SECRET=' /opt/dota2/etc/auth.env >> /opt/dota2/etc/gateway.env
  grep '^INTERNAL_KEY=' /opt/dota2/etc/auth.env >> /opt/dota2/etc/tournament.env
  echo "generated fresh secrets"
else
  echo "auth.env exists, keeping secrets"
fi

# ---- 安装 systemd 单元 ----
cp /opt/dota2/etc/dota2-*.service /etc/systemd/system/ 2>/dev/null || true
systemctl daemon-reload
systemctl enable dota2-auth dota2-tournament dota2-gateway 2>/dev/null || true
systemctl restart dota2-auth dota2-tournament dota2-gateway

# ---- 防火墙 ----
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
  ufw allow 8080/tcp >/dev/null 2>&1 && echo "ufw: 8080 allowed"
fi

# ---- 验证 ----
sleep 2
echo "--- service status ---"
systemctl is-active dota2-auth dota2-tournament dota2-gateway
echo "--- gateway http ---"
curl -s -o /dev/null -w "HTTP %{http_code} (%{size_download} bytes)\n" http://127.0.0.1:8080/ || echo "curl failed"
"""


def main():
    if not PASS:
        print("请先设置环境变量 DOTA2_SSH_PASS（服务器 root 密码）")
        return 1
    for p in BINS + UNITS:
        if not os.path.exists(p):
            print("缺少文件:", p)
            return 1

    print("连接 %s@%s ..." % (USER, HOST))
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    try:
        ssh.connect(HOST, port=22, username=USER, password=PASS, timeout=20)
    except Exception as e:
        print("SSH 连接失败:", e)
        return 1
    sftp = ssh.open_sftp()

    # 先确保远程目录存在
    stdin, stdout, stderr = ssh.exec_command("mkdir -p /opt/dota2/bin /opt/dota2/etc /opt/dota2/data", timeout=30)
    stdout.channel.recv_exit_status()

    # 上传二进制与单元文件（二进制先传 .new，由服务器端 mv 替换）
    for local, remote in zip(BINS, ["/opt/dota2/bin/dota2-auth.new", "/opt/dota2/bin/dota2-tournament.new", "/opt/dota2/bin/dota2-gateway.new"]):
        print("上传", os.path.basename(local), "->", remote)
        sftp.put(local, remote)
    for local, remote in zip(UNITS, ["/opt/dota2/etc/dota2-auth.service", "/opt/dota2/etc/dota2-tournament.service", "/opt/dota2/etc/dota2-gateway.service"]):
        print("上传", os.path.basename(local), "->", remote)
        sftp.put(local, remote)
    sftp.close()

    print("执行服务器配置...")
    stdin, stdout, stderr = ssh.exec_command(SERVER_SETUP, timeout=120)
    out = stdout.read().decode("utf-8", "replace")
    err = stderr.read().decode("utf-8", "replace")
    print(out)
    if err.strip():
        print("--- stderr ---")
        print(err)
    code = stdout.channel.recv_exit_status()
    ssh.close()
    print("部署脚本退出码:", code)
    return code


if __name__ == "__main__":
    sys.exit(main())
