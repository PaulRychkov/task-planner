# tasks desktop

Десктоп-клиент задач под Windows 10/11: Wails v2 (WebView2) + React 18 + TypeScript + Tailwind CSS. Тонкий клиент REST API бэкенда (`http://localhost:8081`), витрина проекта в стиле «голубой гошный» (палитра gopher blue `#00ADD8`).

## Возможности

- **Календарь**: месяц / неделя / день, навигация и «Сегодня». День — вертикальная шкала 06:00–24:00: якорные задачи позиционируются по `start_time_minutes` с высотой по длительности, гибкие — списком сверху; линия текущего времени.
- Отметки прямо с календаря: ✓ выполнено, ✕ осознанный пропуск; missed и просроченные pending подсвечены красным; кнопка «Перенести невыполненные» сдвигает цепочки всех просроченных задач (reschedule-missed по каждой).
- **Задачи**: многоуровневое дерево тем (parent_id) с задачами внутри и группой «Без темы»; форма со всеми видами повторений — once, daily, будни, выходные, дни недели, каждые N дней/недель, monthly (с днём месяца), spaced repetition с 4 пресетами из планера (базовый, Ebbinghaus, интенсив, Anki) и своим массивом интервалов; тема, приоритет 0–9, дедлайн (для once), весь день. Время у задачи одно из двух: фиксированное время непрерывного события (созвон, зал) с длительностью ИЛИ трудозатраты `effort_minutes` (пересчёт в помидоры показывается в форме и списке); флаг «Помидоры» (`requires_pomodoro`).
- **Темы**: справочник с описанием, архивирование, удаление.

## Структура

```
main.go               окно Wails, embed frontend/dist
app.go                биндинг GetBackendURL (env TASKS_API_URL)
wails.json            конфиг Wails CLI
frontend/
├── src/App.tsx       каркас: сайдбар + вьюхи + индикатор доступности бэкенда
├── src/api.ts        REST-клиент (fetch, ApiError)
├── src/types.ts      типы модели данных
├── src/lib/          dates (ISO-даты, сетка месяца), recurrence (лейблы, SR-пресеты)
└── src/components/   CalendarView (месяц/неделя/день), TaskForm, TasksView,
                      TopicsView, OccurrenceChip
```

## Запуск

Нужен запущенный бэкенд (см. [../backend/README.md](../backend/README.md)).

```powershell
& "$env:USERPROFILE\go\bin\wails.exe" dev     # горячая разработка
& "$env:USERPROFILE\go\bin\wails.exe" build   # сборка → build\bin\tasks-desktop.exe
```

Если Wails CLI не установлен: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.

Только фронтенд в браузере (без WebView2):

```powershell
cd frontend
npm install
npm run dev
```

## Конфигурация

`TASKS_API_URL` — адрес бэкенда (default `http://localhost:8081`); читается Go-частью и отдаётся фронтенду через биндинг `GetBackendURL`.
