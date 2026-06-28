# Changelog

## [1.3.0](https://github.com/dato-dev/mtprotoui/compare/v1.2.0...v1.3.0) (2026-06-26)


### ✨ Features

* scheduled SNI rotation ([070fc69](https://github.com/dato-dev/mtprotoui/commit/070fc69107f758729faeb46dd61306f4e1aef3a3))


### 📚 Documentation

* refresh README/ROADMAP and add log viewer enhancements task ([2191aae](https://github.com/dato-dev/mtprotoui/commit/2191aaeca6e8f9654fb31da850a56e8274b2aa2a))

## [1.2.0](https://github.com/dato-dev/mtprotoui/compare/v1.1.0...v1.2.0) (2026-06-26)


### ✨ Features

* **api:** audit log of server actions ([de127bd](https://github.com/dato-dev/mtprotoui/commit/de127bdd93ca9a05c32e6b39a9803e7fd62863ad))
* **ssh:** TOFU host key verification ([cccb87a](https://github.com/dato-dev/mtprotoui/commit/cccb87a9f8a6a62773c8684c080871fa995ec090))


### 🐛 Bug Fixes

* **deps:** align react-dom and types back to v18 ([cdd88f1](https://github.com/dato-dev/mtprotoui/commit/cdd88f1d189fa2940835e6682a49f05f63afd800))

## [1.1.0](https://github.com/dato-dev/mtprotoui/compare/v1.0.1...v1.1.0) (2026-06-25)


### ✨ Features

* **ui:** manual server ordering and persisted filters ([51bd582](https://github.com/dato-dev/mtprotoui/commit/51bd582bf3612427f53c98ed6a965a8843a57231))

## [1.0.1](https://github.com/dato-dev/mtprotoui/compare/v1.0.0...v1.0.1) (2026-06-25)

### 📚 Documentation

* Переработан README: витринная шапка с бейджами, hero-скриншот, галерея UI, диаграмма архитектуры (mermaid), сравнение типов прокси, раздел production-деплоя.

## 1.0.0 (2026-06-25)

### ✨ Features

* Веб-консоль развёртывания и мониторинга MTProto-прокси на VPS по SSH.
* Два типа прокси: mtg v2 и tg-ws-proxy (порт, опциональный Fake TLS).
* Деплой по SSH с ретраями/таймаутами и живым прогрессом.
* Health-check (ICMP ping + TCP), метрики контейнера (`docker stats`).
* Редактирование сервера, теги, поиск/фильтр/сортировка, массовые операции.
* QR-коды proxy-ссылок и просмотр логов контейнера.
* Авторизация админа с обязательной сменой пароля и аварийным сбросом.
* Caddy + Let's Encrypt; оркестрация через Docker Compose.
