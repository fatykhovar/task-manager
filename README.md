# Task Manager Service

REST API сервис для управления задачами в командах — Go + PostgreSQL + Redis + Docker + Prometheus/Grafana.

## Стек технологий

| Компонент | Технология |
|-----------|------------|
| Язык | Go 1.22 |
| HTTP router | chi v5 |
| БД | PostgreSQL 16 |
| Кеш | Redis 7 |
| Аутентификация | JWT (HS256) |
| Логирование | zap |
| Метрики | Prometheus + Grafana |
| Контейнеры | Docker + Docker Compose |
| Тесты | testcontainers-go |

## Быстрый старт

```bash
# Запустить весь стек
make docker-up

# API доступен на http://localhost:8080
# Prometheus: http://localhost:9090
# Grafana: http://localhost:3000 (admin/admin)
```

## API Endpoints

### Аутентификация
```
POST /api/v1/register   — регистрация
POST /api/v1/login      — вход, получение JWT
```

### Команды (требуют Authorization: Bearer <token>)
```
POST /api/v1/teams               — создать команду (стать owner)
GET  /api/v1/teams               — список команд пользователя
POST /api/v1/teams/{id}/invite   — пригласить пользователя (owner/admin)
```

### Задачи
```
POST /api/v1/tasks                          — создать задачу
GET  /api/v1/tasks?team_id=1&status=todo   — список с фильтрацией и пагинацией
PUT  /api/v1/tasks/{id}                    — обновить задачу
GET  /api/v1/tasks/{id}/history            — история изменений
POST /api/v1/tasks/{id}/comments           — добавить комментарий
```

### Аналитика
```
GET /api/v1/analytics/team-stats      — статистика команд (JOIN 3+ таблиц)
GET /api/v1/analytics/top-users       — топ-3 по задачам в каждой команде (оконная функция)
GET /api/v1/analytics/integrity-check — задачи с исполнителем вне команды (подзапрос)
```

### Системные
```
GET /health    — healthcheck
GET /metrics   — Prometheus метрики
```

## Архитектура

```
cmd/server/          — точка входа, wire-up
internal/
  config/            — конфигурация (YAML + ENV)
  model/             — доменные модели
  repository/        — слой БД (PostgreSQL)
  cache/             — Redis кеш
  service/           — бизнес-логика
  handler/           — HTTP handlers
  middleware/        — JWT auth, rate limit, metrics, logger
migrations/          — SQL схема
tests/
  unit/              — unit-тесты с моками
  integration/       — интеграционные тесты (testcontainers)
monitoring/          — конфигурация Prometheus/Grafana
```

## Ключевые особенности

### Безопасность
- JWT аутентификация с проверкой прав на каждый эндпоинт
- Role-based access control (owner / admin / member)
- Rate limiting: 100 req/min на пользователя (token bucket)

### Производительность
- Redis кеширование списка задач команды (TTL 5 минут)
- Инвалидация кеша при изменении задачи
- PostgreSQL connection pool (25 max open, 10 idle)
- Составные индексы для частых фильтров (team_id + status)
- Пагинация LIMIT/OFFSET на уровне БД

### Надёжность
- Circuit breaker для email-сервиса (5 сбоев → open → 60s reset)
- Graceful shutdown (30-секундный таймаут)
- Транзакции при создании команды и обновлении задачи

### Наблюдаемость
- Prometheus: http_requests_total, http_request_duration_seconds (по методу, пути, статусу)
- Структурированные логи (zap)
- История изменений задач (audit log)

### Сложные SQL-запросы
- **JOIN 3+ таблиц + агрегация**: teams + team_members + tasks — количество участников и выполненных задач за 7 дней
- **Оконная функция**: `ROW_NUMBER() OVER (PARTITION BY team_id ORDER BY task_count DESC)` — топ-3 по каждой команде за месяц
- **Коррелированный подзапрос**: задачи с исполнителем, не являющимся членом команды

## Разработка

```bash
make test-unit          # unit-тесты
make test-integration   # интеграционные тесты (нужен Docker)
make test               # все тесты
make coverage-html      # HTML-отчёт о покрытии
make lint               # golangci-lint
```
