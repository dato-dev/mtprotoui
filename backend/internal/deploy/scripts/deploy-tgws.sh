#!/bin/bash
set -euo pipefail

CONTAINER_NAME="${1:-tg-ws-proxy}"
PORT="${2:-1443}"
FAKE_TLS_DOMAIN="${3:-}"
TGWS_IMAGE="${TGWS_IMAGE:-dato1/tg-ws-proxy:latest}"

# Internal port the proxy listens on inside the container (image default).
INTERNAL_PORT=1443

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

# 16 random bytes as 32 hex chars, without depending on openssl/xxd.
generate_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16 | tr -d '[:space:]'
    return
  fi
  head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n'
}

# Lowercase hex encoding of the SNI domain (ASCII), for ee-secret links.
hex_encode() {
  printf '%s' "$1" | od -An -tx1 | tr -d ' \n'
}

run_proxy() {
  local raw_secret="$1"
  docker rm -f "$CONTAINER_NAME" 2>/dev/null || true

  # Override the image entrypoint so we can pass --fake-tls-domain, which the
  # default entrypoint does not expose as an environment variable.
  local args="--host 0.0.0.0 --port ${INTERNAL_PORT} --secret ${raw_secret}"
  if [[ -n "$FAKE_TLS_DOMAIN" ]]; then
    args="$args --fake-tls-domain ${FAKE_TLS_DOMAIN}"
  fi

  # shellcheck disable=SC2086
  docker run -d \
    --name "$CONTAINER_NAME" \
    --restart unless-stopped \
    -p "${PORT}:${INTERNAL_PORT}" \
    --entrypoint /opt/venv/bin/python \
    "$TGWS_IMAGE" \
    -u proxy/tg_ws_proxy.py $args
}

log "Starting tg-ws-proxy deployment (fake_tls=${FAKE_TLS_DOMAIN:-none})"
install_docker

RAW_SECRET=$(generate_secret)
if [[ ${#RAW_SECRET} -ne 32 ]]; then
  log "Failed to generate 32-hex secret (got '${RAW_SECRET}')"
  exit 1
fi

run_proxy "$RAW_SECRET"

# Build the secret exactly as it appears in the tg://proxy link:
#   dd<secret>                 — plain MTProto
#   ee<secret><domain-hex>     — Fake TLS masking
if [[ -n "$FAKE_TLS_DOMAIN" ]]; then
  DOMAIN_HEX=$(hex_encode "$FAKE_TLS_DOMAIN")
  LINK_SECRET="ee${RAW_SECRET}${DOMAIN_HEX}"
else
  LINK_SECRET="dd${RAW_SECRET}"
fi

PUBLIC_IP=$(curl -fsS --max-time 5 ifconfig.me 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}')
if [[ -z "$PUBLIC_IP" ]]; then
  PUBLIC_IP="127.0.0.1"
fi
PROXY_LINK="tg://proxy?server=${PUBLIC_IP}&port=${PORT}&secret=${LINK_SECRET}"

printf '{"port":%s,"secret":"%s","sni":"%s","proxy_link":"%s"}\n' \
  "$PORT" "$LINK_SECRET" "$FAKE_TLS_DOMAIN" "$PROXY_LINK"
