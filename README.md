# MTProto UI

Web-консоль для развёртывания и мониторинга MTProto-прокси Telegram на удалённых VPS.

Пользователь добавляет сервер по SSH, а система автоматически устанавливает Docker, поднимает [mtg v2](https://github.com/9seconds/mtg) с SNI из whitelist и выдаёт готовую ссылку `tg://proxy?...`.

## Возможности

- Список MTProto-серверов в виде таблицы с раскрываемыми деталями
- Добавление серверов по SSH (пароль или ключ)
- Выбор типа прокси при добавлении: **mtg v2** или **[tg-ws-proxy](https://github.com/Flowseal/tg-ws-proxy)**
- Настраиваемый порт прокси (по умолчанию 443 для mtg, 1443 для tg-ws-proxy)
- Опциональный Fake TLS (ee-секрет с SNI) для tg-ws-proxy
- Автоматический деплой через bash-скрипт на удалённой машине
- SNI-домен случайно выбирается из [whitelist](https://github.com/hxehex/russia-mobile-internet-whitelist/blob/main/whitelist.txt)
- Копирование Telegram proxy-ссылки
- Пересоздание прокси (новый secret и SNI)
- Удаление с остановкой контейнера и образа на VPS
- Прогресс-бар при деплое и удалении
- Health-check: ICMP ping + TCP-проверка порта
- Авторизация WebUI, обязательная смена пароля при первом входе
- HTTPS через Caddy + Let's Encrypt

## Стек

| Компонент | Технология |
|-----------|------------|
| Backend | Go, SQLite, JWT |
| Frontend | React, TypeScript, Vite |
| Reverse proxy / TLS | Caddy 2 |
| MTProto | `nineseconds/mtg:2`, `dato1/tg-ws-proxy:latest` |
| Оркестрация | Docker Compose |

## Архитектура

```
Пользователь
    │
    ▼
┌─────────┐     /api/*     ┌─────────┐
│  Caddy  │ ─────────────► │ Go API  │
│  + UI   │                │ SQLite  │
└─────────┘                └────┬────┘
                                │ SSH
                    ┌───────────┴───────────┐
                    ▼                       ▼
              VPS #1 (mtg)            VPS #2 (mtg)
```

## Быстрый старт

### Требования

- Docker и Docker Compose
- Для production с HTTPS: домен с A-записью на сервер, открытые порты 80/443

### 1. Клонировать и настроить `.env`

```bash
git clone <repo-url> mtprotoui
cd mtprotoui
cp .env.example .env
```

Сгенерировать ключ шифрования:

```bash
openssl rand -base64 32
```

Заполнить `.env`:

```env
ENCRYPTION_KEY=<результат openssl>
JWT_SECRET=<любая длинная случайная строка>
ADMIN_USER=admin
ADMIN_PASSWORD=<временный пароль>

# Production
DOMAIN=mtproto.example.com
ACME_EMAIL=admin@example.com

# Локально (без TLS)
# DOMAIN=localhost
# HTTP_PORT=8080
```

### 2. Запустить

```bash
docker compose up --build -d
```

### 3. Открыть WebUI

| Режим | URL |
|-------|-----|
| Production | `https://<DOMAIN>` |
| Локально | `http://localhost:8080` (если `HTTP_PORT=8080`) |

При первом входе система попросит сменить пароль на надёжный (мин. 12 символов, заглавные/строчные, цифра, спецсимвол).

## Добавление MTProto-сервера

1. Нажать **«Добавить сервер»**
2. Указать IP, SSH-пользователя, пароль или приватный ключ
3. Опционально проверить SSH-соединение
4. Подтвердить — начнётся фоновый деплой

### Требования к удалённому VPS

- Linux с SSH-доступом (root или пользователь в группе `docker`)
- Открытый порт **443** (или тот, что используется для MTProto)
- Исходящий интернет (Docker Hub, get.docker.com)

### Что происходит при деплое

1. Подключение по SSH
2. Установка Docker (если не установлен)
3. Генерация secret через mtg с SNI-доменом из whitelist
4. Запуск контейнера `mtproto-mtg` на порту 443
5. Сохранение proxy-ссылки в базе

## Переменные окружения

### Обязательные

| Переменная | Описание |
|------------|----------|
| `ENCRYPTION_KEY` | Base64, 32 байта — шифрование SSH-credentials |
| `JWT_SECRET` | Секрет для JWT-сессий |
| `ADMIN_USER` | Логин администратора |
| `ADMIN_PASSWORD` | Начальный пароль (сменится при первом входе) |

### Caddy / HTTPS

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `DOMAIN` | `localhost` | Домен WebUI. Реальный домен → авто LE |
| `ACME_EMAIL` | — | Email для Let's Encrypt |
| `HTTP_PORT` | `80` | Порт HTTP на хосте |
| `HTTPS_PORT` | `443` | Порт HTTPS на хосте |

### Health-check

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `HEALTH_INTERVAL` | `5m` | Интервал фоновых проверок |
| `PING_ENABLED` | `true` | ICMP ping (нужен `NET_RAW`) |
| `PING_COUNT` | `3` | Число ping-пакетов |
| `PING_TIMEOUT` | `4s` | Таймаут ping |
| `TCP_TIMEOUT` | `4s` | Таймаут TCP-проверки |

### Прочие

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `WHITELIST_URL` | GitHub whitelist | URL списка SNI-доменов |
| `ADMIN_PASSWORD_RESET` | `false` | `true` → при старте сбросить пароль админа на `ADMIN_PASSWORD` и потребовать смену. После входа вернуть `false`. |

## Статусы серверов

| Статус | Значение |
|--------|----------|
| **Онлайн** | TCP-порт прокси доступен |
| **Деградация** | Ping OK, TCP закрыт |
| **Офлайн** | Ping и TCP недоступны |

## Структура проекта

```
mtprotoui/
├── backend/           # Go API
│   ├── cmd/api/
│   └── internal/
│       ├── api/
│       ├── auth/
│       ├── deploy/
│       │   └── scripts/deploy-mtproto.sh
│       ├── health/
│       └── store/
├── frontend/          # React UI
├── caddy/             # Caddy + сборка UI
├── docker-compose.yml
└── .env.example
```

## API (кратко)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/auth/login` | Вход |
| POST | `/api/auth/change-password` | Смена пароля |
| GET | `/api/servers` | Список серверов |
| POST | `/api/servers` | Добавить + деплой |
| DELETE | `/api/servers/:id` | Удалить (async) |
| POST | `/api/servers/:id/recreate` | Пересоздать MTProto |
| POST | `/api/servers/:id/health` | Ручная проверка |
| GET | `/api/servers/:id/qr` | QR-код proxy-ссылки (PNG) |
| GET | `/api/servers/:id/logs` | Логи контейнера (`?tail=N`) |
| POST | `/api/servers/test-ssh` | Тест SSH |

## Безопасность

- SSH-credentials шифруются AES-256-GCM (`ENCRYPTION_KEY`)
- JWT в httpOnly cookie
- Обязательная смена слабого начального пароля
- WebUI рекомендуется держать за HTTPS (Caddy + LE)
- Не коммитьте `.env` в git

## Разработка

### Frontend (локально)

```bash
cd frontend
npm install
npm run dev
```

Vite проксирует `/api` на `http://localhost:8080`.

### Backend (локально)

```bash
cd backend
cp ../.env .env   # или экспортировать переменные
go run ./cmd/api
```

### Сборка

```bash
docker compose build
docker compose up -d
```

## Лицензия

MIT (или укажите свою лицензию при публикации)
