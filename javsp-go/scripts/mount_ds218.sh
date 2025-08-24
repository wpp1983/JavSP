#!/bin/bash
set -euo pipefail

NAS=192.168.50.4
# NFSv4 通常导出的是“伪根”，Synology 多数场景仍需写完整卷路径
EXPORT_V4="/volume1/媒体库"
EXPORT_V3="/volume1/媒体库"
MNT="/mnt/nas"

# 依赖工具
if ! command -v mount.nfs >/dev/null 2>&1; then
  echo "🔧 安装 nfs-utils ..."
  sudo pacman -Sy --needed nfs-utils
fi

# 检查内核是否支持 nfs
if ! grep -qE '^nodev\s+nfs4?$' /proc/filesystems; then
  echo "⚠️ 当前内核未声明 nfs 文件系统（/proc/filesystems）。尝试加载模块..."
  if ! sudo modprobe nfs 2>/dev/null; then
    echo "❌ 无法加载 nfs 模块。当前内核可能未编译 NFS 客户端。"
    echo "👉 方案：安装官方内核（例如 linux-lts + linux-lts-headers）后重启再试。"
    exit 1
  fi
fi

sudo mkdir -p "$MNT"

echo "🔗 尝试 NFSv4.1 挂载: $NAS:$EXPORT_V4 -> $MNT"
if sudo mount -t nfs4 -o vers=4.1,timeo=600,retrans=2,noatime "$NAS:$EXPORT_V4" "$MNT"; then
  echo "✅ NFSv4.1 挂载成功：$MNT"
  exit 0
fi

echo "⚠️ NFSv4.1 失败，回退尝试 NFSv3（需要 NAS 端开启 NFSv3 与 rpcbind）"
# 可选优化：rsize/wsize 64k、nolock 避免某些锁问题
if sudo mount -t nfs -o vers=3,proto=tcp,timeo=600,retrans=2,rsize=65536,wsize=65536,noatime "$NAS:$EXPORT_V3" "$MNT"; then
  echo "✅ NFSv3 挂载成功：$MNT"
  exit 0
fi

echo "❌ NFS 挂载仍失败。请检查："
echo "  1) DSM > 文件服务 > NFS 已启用；共享“媒体库”的 NFS 权限包含你的客户端 IP（如 192.168.50.0/24）"
echo "  2) 若走 NFSv3：DSM 端已允许 rpcbind/portmap；客户端能 showmount -e 192.168.50.4"
echo "  3) 防火墙未拦截 2049/tcp（NFSv4）及相关端口（NFSv3 的 mountd/lockd/rpcb）"
exit 2
