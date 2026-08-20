#!/usr/bin/env bash
# install-systemd.sh —— 幂等安装 gpt2api-caddy-sync.{service,path}
#
# 机制:systemd.path 监听 /opt/claude-relay-service/caddy/Caddyfile,
# 任何 write/rename 立即触发 service(跑 sync-caddy.sh),把
# gpt2api-managed block 补回并 caddy reload。事件驱动,零轮询。
#
# 这个脚本每次 gpt2api 部署时跑一次:
#   - 源文件(deploy/systemd/*)和已安装的 /etc/systemd/system/* 一致 → no-op
#   - 不一致 / 不存在 → cp + daemon-reload + enable --now
#
# 迁移路径:如果机器上还有老版的 gpt2api-caddy-sync.timer(轮询版),
# 脚本会在安装 path 之前把 timer 停掉并删除,避免两套并行。

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SRC_DIR="$REPO_ROOT/deploy/systemd"
DST_DIR="/etc/systemd/system"

log() { echo "[install-systemd] $*"; }
warn() { echo "[install-systemd] WARN: $*" >&2; }

if [ ! -d "$SRC_DIR" ]; then
    warn "source dir not found: $SRC_DIR — 跳过"
    exit 0
fi
if ! command -v systemctl >/dev/null 2>&1; then
    warn "systemctl 不可用(非 systemd 系统?) — 跳过"
    exit 0
fi

# --- 清理老版 timer(从 polling 迁移到 path 事件驱动) ---
legacy_timer="$DST_DIR/gpt2api-caddy-sync.timer"
if [ -f "$legacy_timer" ]; then
    log "发现老版 timer,停用并删除(已切换到 path unit)"
    sudo systemctl disable --now gpt2api-caddy-sync.timer 2>/dev/null || true
    sudo rm -f "$legacy_timer"
fi

# --- 幂等写入 service + path ---
changed=0
for name in gpt2api-caddy-sync.service gpt2api-caddy-sync.path; do
    src="$SRC_DIR/$name"
    dst="$DST_DIR/$name"
    if [ ! -f "$src" ]; then
        warn "missing $src — 跳过"
        continue
    fi
    if [ -f "$dst" ] && cmp -s "$src" "$dst"; then
        log "$name 一致,no-op"
    else
        sudo cp "$src" "$dst"
        changed=1
        log "$name 已更新"
    fi
done

if [ "$changed" -eq 1 ]; then
    sudo systemctl daemon-reload
    log "daemon-reload ok"
fi

# enable --now 幂等:已 enabled + active 时不报错
if ! sudo systemctl is-enabled --quiet gpt2api-caddy-sync.path \
    || ! sudo systemctl is-active  --quiet gpt2api-caddy-sync.path \
    || [ "$changed" -eq 1 ]; then
    sudo systemctl enable --now gpt2api-caddy-sync.path
    log "path unit enabled+started(监听 /opt/claude-relay-service/caddy/Caddyfile)"
else
    log "path unit already enabled+active,no-op"
fi

# 立即跑一次 service 确保当前 Caddyfile 状态是对的(启动后并不会自动触发一次,
# 只有未来的文件变化才触发 —— 首次启动时手动兜底一次)
if [ "$changed" -eq 1 ]; then
    sudo systemctl start gpt2api-caddy-sync.service || true
    log "初始同步完成"
fi
