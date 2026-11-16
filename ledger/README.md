# Ledger

**Группа:** BHEMBD-25  
**Студент:** Арсений Паниклов

Ledger хранит бюджеты и транзакции в PostgreSQL, проверяет лимиты и
кэширует сводные отчёты в Redis на 30 секунд.

```powershell
goose -dir ./ledger/migrations postgres $env:DATABASE_URL up
goose -dir ./ledger/migrations postgres $env:DATABASE_URL down
```

Переменные окружения: `DATABASE_URL`, `REDIS_ADDR`, `REDIS_DB`, `REDIS_PASSWORD`.
