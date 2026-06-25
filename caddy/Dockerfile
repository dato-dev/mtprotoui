FROM node:22-alpine AS ui-builder

WORKDIR /app
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ .
RUN npm run build

FROM caddy:2.8-alpine

COPY caddy/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
COPY --from=ui-builder /app/dist /srv

EXPOSE 80 443

ENTRYPOINT ["/entrypoint.sh"]
