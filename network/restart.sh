#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# 选择 compose 命令：优先 docker compose（v2），否则回退 docker-compose（v1）
detect_compose() {
  if docker compose version >/dev/null 2>&1; then
    echo "docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    echo "docker-compose"
  else
    echo "ERROR: docker compose/docker-compose 未安装" >&2
    exit 1
  fi
}
COMPOSE_CMD="$(detect_compose)"

echo "[INFO] removing cli.togettoyou.com to avoid v1 'ContainerConfig' recreate bug..."
docker rm -f cli.togettoyou.com >/dev/null 2>&1 || true

echo "[INFO] bringing Fabric services up-to-date..."
pushd "$SCRIPT_DIR" >/dev/null
$COMPOSE_CMD up -d
popd >/dev/null

echo "[OK] network is up-to-date."
