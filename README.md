# tasks — приложение задач

Приложение задач под Windows: headless-бэкенд на Go + десктоп-клиент на Wails. Любые задачи — от «убраться дома» до цепочек закрепления знаний (spaced repetition). Часть экосистемы activization: владеет данными о задачах, публикует события в Kafka для бота, отдаёт ICS-фид для внешних календарей.

Прототип и источник идей модели — планер в Google Таблицах: [planer.md](planer.md) (темы, форматы повторений, перенос невыполненных со сдвигом цепочки).

## Структура

```
tasks/
├── backend/            Go-сервис: REST + MCP + генератор вхождений + outbox-релей
│   ├── cmd/tasks/      точка входа
│   ├── internal/       config, handler, service, repository, models, kafka
│   ├── migrations/     SQL-миграции (golang-migrate, встроены в бинарник)
│   └── Dockerfile      multi-stage сборка
├── desktop/            Wails v2 приложение (React + TS + Tailwind)
│   ├── frontend/       UI: календарь, задачи, темы, план дня
│   └── main.go         окно + биндинг адреса бэкенда
├── docker-compose.yml  postgres:16 (порт 5433) + backend (порт 8081)
└── planer.md           UX-прототип (Google Apps Script)
```

## Запуск

### Docker (через WSL)

Команды выполняются из каталога `tasks/` (WSL наследует текущий каталог). Compose подключается к внешней сети `ecosystem` — на чистой машине её нужно создать один раз:

```powershell
wsl -d Ubuntu-22.04 -- docker network create ecosystem   # однократно
wsl -d Ubuntu-22.04 -- docker compose up -d --build
```

Поднимает Postgres на `localhost:5433` и бэкенд на `http://localhost:8081`. Kafka не обязательна: события копятся в outbox и уходят, когда брокер появится (общая Kafka — `infra/docker-compose.yml` экосистемы).

### Локально без Docker

```powershell
wsl -d Ubuntu-22.04 -- docker compose up -d postgres
cd backend
go run ./cmd/tasks
```

### Десктоп

```powershell
cd desktop
& "$env:USERPROFILE\go\bin\wails.exe" dev     # разработка
& "$env:USERPROFILE\go\bin\wails.exe" build   # сборка → build\bin\tasks-desktop.exe
```

Клиент ходит на `http://localhost:8081` (переопределяется переменной `TASKS_API_URL`).

## Конфигурация

`.env` в каталоге `backend/` (или корне tasks/), переменные окружения поверх. Префикс `TASKS_`:

| Переменная | Default | Смысл |
|---|---|---|
| TASKS_HTTP_PORT | 8081 | порт REST + MCP |
| TASKS_DB_HOST / PORT / USER / PASSWORD / NAME | localhost / 5433 / tasks / tasks / tasks | Postgres |
| TASKS_KAFKA_BROKERS | localhost:9094 | брокеры через запятую |
| TASKS_KAFKA_TOPIC | tasks.events | топик CloudEvents |
| TASKS_TIMEZONE | Europe/Moscow | локальная TZ пользователя |
| TASKS_WINDOW_DAYS | 60 | окно материализации вхождений |
| TASKS_OUTBOX_INTERVAL_SECONDS | 5 | период outbox-релея |

## API (кратко)

REST `/api/v1` (JSON snake_case, ошибки `{"error":{"code","message"}}`):
CRUD `/topics`, `/tasks`; `GET /occurrences?from&to&status`; `POST /occurrences/{id}/complete|skip`; `POST /tasks/{id}/reschedule-missed`; `GET /plans/{date}`; `PUT /plans/{date}/items`; `POST /plans/{date}/commit`; `GET /calendar.ics`; `GET /healthz`.

MCP (streamable HTTP `/mcp`): `list_tasks`, `create_task`, `complete_occurrence`, `skip_occurrence`, `list_due`, `get_day_plan`, `commit_day_plan`, `reschedule_missed`.

Подробности — в [backend/README.md](backend/README.md) и `docs/contracts.md` экосистемы. Модель данных — `docs/data-model.md`.

## Тесты

```powershell
cd backend
go test ./...
```

Покрыто: генератор повторений (все 9 видов, monthly-прижатие, SR c series_step), перенос невыполненных, план дня, outbox-релей с mock-продюсером, REST-хендлеры (httptest).
