# Gateway

**Группа:** BHEMBD-25  
**Студент:** Арсений Паниклов

Gateway публикует HTTP-эндпойнты бюджетов, транзакций и сводного отчёта.
Данные хранятся в PostgreSQL через Ledger, а отчёты кэшируются в Redis.

```powershell
go run ./gateway/cmd/gateway
curl.exe "http://localhost:8080/api/reports/summary?from=2026-10-01&to=2026-10-31"
```
