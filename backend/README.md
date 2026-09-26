# tasks backend

Go-сервис задач: REST API + MCP-сервер + генератор вхождений + transactional outbox с релеем в Kafka. Владелец БД `tasks` (Postgres 16, порт 5433). Реализует модель данных v1.0 (`docs/data-model.md` экосистемы) и контракты (`docs/contracts.md`).

## Назначение

- Задача — ПРАВИЛО (recurrence_kind + params по JSCalendar), вхождения материализуются в окно 60 дней.
- Идемпотентная генерация по `UNIQUE(task_id, date)`: при изменении правила pending-строки обновляются на месте, исчезнувшие из расписания удаляются с эмитом `plan.updated` в той же транзакции; история (completed/missed/skipped/rescheduled) не трогается.
- SR-прогресс живёт в `series_step`: «перенос невыполненных» сдвигает `start_date`, pending пересоздаются с теми же step по новым датам, выполненные этапы не генерируются повторно.
- Ночной джоб (00:05 локального времени + на старте): pending с прошедшей датой → missed + событие.
- Все события пишутся в `events_outbox` в одной транзакции с бизнес-изменением; релей публикует CloudEvents 1.0 в Kafka `tasks.events` (ключ партиции — aggregate_id). Kafka недоступна — события копятся. Осознанный компромисс: при ошибке публикации проход останавливается ради сохранения порядка событий (без dead-letter и лимита попыток) — «ядовитое» событие блокирует релей до ручного вмешательства, что для локальной экосистемы приемлемо.

## Структура

```
cmd/tasks/main.go        конфиг, миграции, DI, HTTP, релей, ночной тикер, graceful shutdown
internal/config/         viper + .env (префикс TASKS_)
internal/handler/        Gin: REST, ICS-фид, MCP-тулы (go-sdk, /mcp)
internal/service/        бизнес-логика: schedule (генератор дат), tasks, occurrences,
                         plans, topics, events (payload'ы), outbox (релей)
internal/repository/     интерфейсы Store + GORM-реализация
internal/repository/memory/  in-memory Store для тестов
internal/models/         GORM-модели, enum-ы, тип Date
internal/kafka/          sarama SyncProducer с ленивым подключением
migrations/              golang-migrate SQL, embed в бинарник
```

## Запуск

Из каталога `tasks/` (WSL наследует текущий каталог):

```powershell
wsl -d Ubuntu-22.04 -- docker compose up -d postgres
cd backend
go run ./cmd/tasks
```

Миграции применяются автоматически при старте. Полный стек — `docker compose up -d --build` из каталога `tasks/` (в WSL); перед первым запуском создать общую сеть: `docker network create ecosystem`.

## Конфигурация

`.env` рядом с бинарником или в родительском каталоге; env-переменные поверх. См. таблицу в корневом [README](../README.md): `TASKS_HTTP_PORT`, `TASKS_DB_*`, `TASKS_KAFKA_BROKERS`, `TASKS_KAFKA_TOPIC`, `TASKS_TIMEZONE`, `TASKS_WINDOW_DAYS`, `TASKS_OUTBOX_INTERVAL_SECONDS`.

## API

### REST `/api/v1`

| Метод и путь | Что делает |
|---|---|
| GET/POST `/topics`, GET/PUT/DELETE `/topics/{id}` | справочник тем |
| GET/POST `/tasks`, GET/PUT/DELETE `/tasks/{id}` | CRUD правил задач (создание/правка перегенерирует окно) |
| POST `/tasks/{id}/reschedule-missed` | сдвиг невыполненных: missed/просроченные pending → rescheduled, start_date сдвигается, серия сохраняет series_step |
| GET `/occurrences?from&to&status&task_id` | вхождения с задачей и темой (`task_id` — необязательный фильтр по одной задаче) |
| POST `/occurrences/{id}/complete` | выполнить (+ `occurrence.completed`; закрытие once/последнего SR-этапа → `task.completed`) |
| POST `/occurrences/{id}/skip` | осознанный пропуск (+ `occurrence.skipped`) |
| GET `/plans/{date}` | план дня с элементами |
| PUT `/plans/{date}/items` | заменить состав `{items:[{occurrence_id, planned_start_minutes}]}`; на закоммиченном плане эмитит `plan.updated` |
| POST `/plans/{date}/commit` | `{committed_by: app\|bot\|agent}` + `plan.committed` |
| GET `/calendar.ics` | ICS-фид вхождений (−30…+60 дней) |
| GET `/healthz` | статус БД |

Ошибки: `{"error":{"code":"validation|not_found|conflict|internal","message":"..."}}`. Даты `YYYY-MM-DD`, моменты RFC3339.

Семантика `PUT /tasks/{id}`: nullable-поля (`description`, `topic_id`, `due`, `start_time_minutes`, `estimated_duration_minutes`, `recurrence_params`) заменяются целиком — отсутствие поля означает NULL; не-nullable поля (`start_date`, `all_day`, `priority`, `is_active`, `progress`) при отсутствии в теле сохраняют текущее значение. `source`/`external_id` задаются только при создании и через PUT не меняются — связь с внешним источником нельзя потерять при редактировании.

API рассчитан на локальный запуск (localhost, единственный пользователь): аутентификации нет, CORS открыт (`Access-Control-Allow-Origin: *`). Для публичного развёртывания понадобится auth-слой и ограничение origin'ов.

### MCP `/mcp` (streamable HTTP)

`list_tasks`, `create_task` (плоские поля: kind + days/n/day_of_month/intervals, topic по имени), `complete_occurrence`, `skip_occurrence`, `list_due` (pending/missed вхождения + open-задачи с дедлайном), `get_day_plan`, `commit_day_plan`, `reschedule_missed`.

### События (CloudEvents 1.0, топик `tasks.events`)

`task.created|updated|completed|cancelled` (снапшот задачи: окно/время, `effort_minutes`, `requires_pomodoro`, тема), `occurrence.completed|missed|skipped|rescheduled`, `plan.committed|updated`. `subject` — UUID задачи для task-скоупных событий; payload несёт `task_id` (+`date` у occurrence.*).

## Тесты

```powershell
go test ./...
```

- `schedule_test.go` — все 9 видов повторений table-driven, monthly-прижатие к концу короткого месяца, SR-интервалы и series_step;
- `tasks_test.go` — генерация окна, идемпотентность, обновление pending на месте при смене правила, удаление из закоммиченного плана с `plan.updated`, reschedule-missed с сохранением series_step, missed-джоб, закрытие задачи последним SR-этапом, валидация;
- `plans_test.go` — черновик/коммит, `plan.updated` только для закоммиченного плана (added/removed/reordered), валидация элементов;
- `outbox_test.go` — релей с mock-продюсером: конверт CloudEvents, порядок, ключ партиции, накопление при недоступной Kafka;
- `handler_test.go` — httptest: CRUD, фильтры, конфликты, формат ошибок, план, ICS.
