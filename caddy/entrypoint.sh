#!/bin/sh
set -e

DOMAIN="${DOMAIN:-localhost}"
ACME_EMAIL="${ACME_EMAIL:-}"

if [ "$DOMAIN" = "localhost" ] || [ "$DOMAIN" = "127.0.0.1" ]; then
  SITE=":80"
  {
    echo "$SITE {"
    echo "    encode gzip"
    echo ""
    echo "    handle /api/* {"
    echo "        reverse_proxy api:8080"
    echo "    }"
    echo ""
    echo "    handle {"
    echo "        root * /srv"
    echo "        try_files {path} /index.html"
    echo "        file_server"
    echo "    }"
    echo "}"
  } > /etc/caddy/Caddyfile
else
  {
    echo "{"
    if [ -n "$ACME_EMAIL" ]; then
      echo "    email $ACME_EMAIL"
    fi
    echo "}"
    echo ""
    echo "$DOMAIN {"
    echo "    encode gzip"
    echo ""
    echo "    handle /api/* {"
    echo "        reverse_proxy api:8080"
    echo "    }"
    echo ""
    echo "    handle {"
    echo "        root * /srv"
    echo "        try_files {path} /index.html"
    echo "        file_server"
    echo "    }"
    echo "}"
  } > /etc/caddy/Caddyfile
fi

exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
