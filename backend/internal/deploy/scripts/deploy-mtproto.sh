#!/bin/bash
set -euo pipefail

SNI_DOMAIN="${1:?SNI domain required}"
CONTAINER_NAME="${2:-mtproto-mtg}"
PORT="${3:-${MTPROTO_PORT:-443}}"
MTG_IMAGE="${MTG_IMAGE:-nineseconds/mtg:2}"

log() {
  echo "[deploy] $*" >&2
}

install_docker() {
  if command -v docker >/dev/null 2>&1; then
    log "Docker already installed"
    return
  fi
  log "Installing Docker..."
  curl -fsSL https://get.docker.com | sh
  if command -v systemctl >/dev/null 2>&1; then
    systemctl enable --now docker || true
  fi
}

generate_secret() {
  docker run --rm "$MTG_IMAGE" generate-secret --hex "$SNI_DOMAIN" | tr -d '[:space:]'
}

run_proxy() {
  local secret="$1"
  docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
  docker run -d \
    --name "$CONTAINER_NAME" \
    --restart unless-stopped \
    -p "${PORT}:443" \
    "$MTG_IMAGE" \
    simple-run -n 1.1.1.1 -i prefer-ipv4 "0.0.0.0:443" "$secret"
}

log "Starting MTProto deployment for SNI=$SNI_DOMAIN"
install_docker

SECRET=$(generate_secret)
if [[ -z "$SECRET" ]]; then
  log "Failed to generate secret"
  exit 1
fi

run_proxy "$SECRET"

PUBLIC_IP=$(curl -fsS --max-time 5 ifconfig.me 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}')
if [[ -z "$PUBLIC_IP" ]]; then
  PUBLIC_IP="127.0.0.1"
fi
PROXY_LINK="tg://proxy?server=${PUBLIC_IP}&port=${PORT}&secret=${SECRET}"

printf '{"port":%s,"secret":"%s","sni":"%s","proxy_link":"%s"}\n' \
  "$PORT" "$SECRET" "$SNI_DOMAIN" "$PROXY_LINK"
