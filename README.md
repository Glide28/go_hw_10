# Домашнее задание №10 по Go

**Группа:** BHEMBD-25  
**Студент:** Арсений Паниклов

Тема: «Синхронизация и каналы».

Проект продолжает развитие CashCraft из ДЗ №9. В Ledger добавлен конкурентный
пакетный импорт транзакций с worker pool, каналами `jobs/results`,
`sync.WaitGroup`, корректной отменой через `context.Context` и сбором
частичного результата без гонок.

## Возможности

- регистрация и вход с выдачей JWT;
- защищённый HTTP API для транзакций, бюджетов и отчётов;
- PostgreSQL и миграции схемы;
- контроль превышения бюджета;
- Redis-кэш отчётов (TTL 30 секунд) и списка бюджетов (TTL 20 секунд);
- отдельный процесс Ledger с gRPC API и Gateway-клиентом;
- импорт и экспорт транзакций в CSV;
- пакетный импорт транзакций через настраиваемый пул воркеров;
- конкурентный расчёт отчёта по категориям;
- единые JSON-ошибки и таймауты запросов;
- Docker Compose для PostgreSQL, Redis и Gateway;
- OpenAPI-описание и пример Google Apps Script.

## Структура

```text
gateway/                    HTTP API, JWT middleware и gRPC-клиент
ledger/                     gRPC-сервер, бизнес-логика и доступ к данным
  internal/domain/          сущности и контракты
  internal/service/         бюджеты, транзакции, отчёты и кэш
  internal/repository/      PostgreSQL и Redis
  migrations/               схема БД
openapi.yaml                спецификация API
google-apps-script.js       интеграция с Google Таблицами
docker-compose.yml          локальное окружение
```

Направление зависимостей: `HTTP Gateway → gRPC Ledger → Service → Domain`. PostgreSQL и Redis
подключаются в слое приложения, поэтому бизнес-логика не зависит от деталей
хранения.

## Быстрый запуск

1. Создайте локальный файл окружения:

```powershell
Copy-Item .env.example .env
```

2. Замените `JWT_SECRET` в `.env` на длинную случайную строку.

3. Запустите проект:

```powershell
docker compose up --build -d
docker compose logs -f gateway
```

4. Проверьте доступность:

```powershell
curl.exe http://localhost:8080/ping
```

При первом запуске PostgreSQL выполняет миграцию из
`ledger/migrations/00001_create_ledger_tables.up.sql`.

## Регистрация и авторизация

```powershell
curl.exe -X POST http://localhost:8080/api/auth/register `
  -H "Content-Type: application/json" `
  -d '{"email":"student@example.com","password":"strong-password"}'

curl.exe -X POST http://localhost:8080/api/auth/login `
  -H "Content-Type: application/json" `
  -d '{"email":"student@example.com","password":"strong-password"}'
```

Скопируйте поле `token` из ответа:

```powershell
$token = '<JWT_TOKEN>'
```

## Основные запросы

Создание бюджета:

```powershell
curl.exe -X POST http://localhost:8080/api/budgets `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: application/json" `
  -d '{"category":"еда","limit":15000}'
```

Добавление расхода:

```powershell
curl.exe -X POST http://localhost:8080/api/transactions `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: application/json" `
  -d '{"amount":1250.50,"category":"еда","description":"продукты","date":"2026-11-14"}'
```

Отчёт:

```powershell
curl.exe "http://localhost:8080/api/reports/summary?from=2026-11-01&to=2026-11-30" `
  -H "Authorization: Bearer $token"
```

Повторный запрос в течение 30 секунд возвращается из Redis. Ключ имеет вид
`report:summary:YYYY-MM-DD:YYYY-MM-DD`. Список бюджетов хранится под ключом
`budgets:all` и удаляется после изменения бюджета.

## CSV

Экспорт:

```powershell
curl.exe http://localhost:8080/api/transactions/export.csv `
  -H "Authorization: Bearer $token" -o transactions.csv
```

Импорт принимает `text/csv` со столбцами `amount,category,description,date`:

```powershell
curl.exe -X POST http://localhost:8080/api/transactions/import.csv `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: text/csv" `
  --data-binary '@transactions.csv'
```

## Пакетный импорт

Эндпоинт `POST /api/transactions/bulk` принимает JSON-массив транзакций.
Число воркеров задаётся параметром `workers` от 1 до 64; значение по умолчанию
равно 4.

```powershell
curl.exe -X POST "http://localhost:8080/api/transactions/bulk?workers=4" `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: application/json" `
  -d '[{"amount":1250.50,"category":"еда","description":"продукты","date":"2026-11-17"},{"amount":450,"category":"транспорт","description":"такси","date":"2026-11-17"}]'
```

Ответ всегда содержит сводку по обработанным элементам:

```json
{
  "accepted": 1,
  "rejected": 1,
  "errors": [
    {"index": 1, "error": "budget exceeded"}
  ]
}
```

Частичные ошибки возвращаются с кодом `200 OK`. При отмене запроса или
истечении таймаута Gateway возвращает `504 Gateway Timeout`, сообщение об
ошибке и накопленную к моменту остановки сводку.

## Google Таблицы

1. Создайте листы `Transactions` и `Report`.
2. В `Transactions` используйте столбцы: сумма, категория, описание, дата.
3. В `Report!B1` и `Report!B2` укажите начало и конец периода.
4. Откройте **Расширения → Apps Script** и вставьте содержимое
   `google-apps-script.js`.
5. Укажите публичный адрес Gateway в `API_BASE`.
6. Один раз выполните `login(email, password)`, затем используйте
   `addTransactionFromActiveRow()` и `loadMonthlyReport()`.

## Документация и проверка

OpenAPI: [openapi.yaml](openapi.yaml).

```powershell
go test ./ledger/... ./gateway/...
go test -race ./ledger/... ./gateway/...
go vet ./ledger/... ./gateway/...
go test -coverprofile=cover.out ./ledger/... ./gateway/...
```

Остановка:

```powershell
docker compose down
```
