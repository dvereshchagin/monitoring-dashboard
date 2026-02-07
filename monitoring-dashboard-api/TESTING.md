# Testing Guide

## Manual Testing Checklist

### 1. Build & Compilation

```bash
# Проверить что проект компилируется
go build ./...

# Собрать бинарь
make build

# Проверить бинарь
ls -lh bin/monitoring-dashboard
```

### 2. Database Setup

```bash
# Создать БД
createdb monitoring

# Проверить подключение
psql -U postgres -d monitoring -c "SELECT 1;"

# Запустить миграции
make migrate

# Проверить таблицы
psql -U postgres -d monitoring -c "\dt"

# Должна быть таблица metrics
```

### 3. Application Startup

```bash
# Запустить приложение
make run

# Или через бинарь
./bin/monitoring-dashboard

# Ожидаемый вывод:
# [INFO] Starting Monitoring Dashboard
# [INFO] Database connected successfully
# [INFO] WebSocket hub started
# [INFO] Metrics collector started interval=2s
# [INFO] HTTP server starting port=8080
# [INFO] Dashboard available at http://localhost:8080
```

### 4. Dashboard Access

Откройте в браузере:
```
http://localhost:8080
```

**Ожидаемый результат:**
- ✅ Видны 4 metric cards (CPU, Memory, Disk, Network)
- ✅ Значения отображаются корректно
- ✅ Статус "Connected" зеленый
- ✅ Графики Chart.js отображаются

### 5. Real-time Updates

Откройте DevTools → Network → WS:

```
ws://localhost:8080/ws [101 Switching Protocols]
```

Во вкладке Messages должны приходить JSON каждые 2 секунды:

```json
{
  "type": "snapshot",
  "data": {
    "timestamp": "2026-01-15T...",
    "cpu": { "value": 45.2, "unit": "%" },
    "memory": { "value": 62.5, "unit": "%" },
    ...
  }
}
```

### 6. Database Verification

```bash
# Подождите 1 минуту, затем:

# Проверьте количество записей
psql -U postgres -d monitoring -c "SELECT COUNT(*) FROM metrics;"
# Должно быть ~120 записей (4 типа × 30 записей)

# Проверьте распределение по типам
psql -U postgres -d monitoring -c "
  SELECT metric_type, COUNT(*) as count
  FROM metrics
  GROUP BY metric_type;
"

# Должно быть примерно поровну:
#  metric_type | count
# -------------+-------
#  cpu         |    30
#  memory      |    30
#  disk        |    30
#  network     |    30

# Проверьте последние метрики
psql -U postgres -d monitoring -c "
  SELECT metric_type, value, unit, collected_at
  FROM metrics
  ORDER BY collected_at DESC
  LIMIT 10;
"
```

### 7. API Testing

```bash
# Получить историю CPU за последний час
curl -s "http://localhost:8080/api/metrics/history?type=cpu&duration=1h" | jq '.metrics | length'
# Должно вернуть количество записей

# Получить детальную информацию
curl -s "http://localhost:8080/api/metrics/history?type=cpu&duration=1h" | jq '.average'
# Должно вернуть среднее значение

# Получить историю для разных типов
curl "http://localhost:8080/api/metrics/history?type=memory&duration=30m"
curl "http://localhost:8080/api/metrics/history?type=disk&duration=1h"
curl "http://localhost:8080/api/metrics/history?type=network&duration=15m"
```

### 8. Multiple Connections Test

Откройте **5-10 вкладок** браузера с дашбордом:

- ✅ Все вкладки должны обновляться синхронно
- ✅ Нет задержек или зависаний
- ✅ Статус "Connected" во всех вкладках
- ✅ В логах: "Client registered total_clients=5" (или больше)

### 9. Reconnection Test

1. Остановите сервер (Ctrl+C)
2. В браузере статус должен измениться на "● Disconnected" (красный)
3. Запустите сервер снова: `make run`
4. В течение 1-30 секунд статус должен вернуться в "● Connected" (зеленый)
5. Метрики снова обновляются

### 10. Load Test

Нагрузите систему:

```bash
# macOS
yes > /dev/null &
PID=$!

# Подождите 10-20 секунд
# CPU usage должен вырасти до 90-100%

# Остановите нагрузку
kill $PID
```

В дашборде должны увидеть:
- ✅ Рост CPU usage
- ✅ Карточка CPU становится warning (желтая) при > 75%
- ✅ Карточка CPU становится critical (красная) при > 90%
- ✅ График CPU показывает пик

### 11. Charts Verification

Проверьте графики:

- ✅ CPU History chart отображает данные за последний час
- ✅ Memory History chart отображает данные за последний час
- ✅ Графики обновляются в реальном времени (новые точки добавляются)
- ✅ Максимум 60 точек на графике (старые удаляются)

### 12. Error Handling Test

```bash
# Остановите PostgreSQL
brew services stop postgresql@14

# Перезапустите приложение
make run

# Должна быть ошибка:
# [ERROR] Failed to ping database ...

# Запустите PostgreSQL обратно
brew services start postgresql@14
```

### 13. Graceful Shutdown Test

```bash
# Запустите приложение
make run

# Нажмите Ctrl+C

# Ожидаемый вывод:
# [INFO] Shutdown signal received, starting graceful shutdown...
# [INFO] Metrics collector stopped
# [INFO] Server stopped gracefully
```

### 14. Performance Test

```bash
# Запустите приложение
make run

# Проверьте использование памяти
ps aux | grep monitoring-dashboard

# Должно быть ~50-100 MB

# Проверьте CPU usage самого приложения
top -pid $(pgrep monitoring-dashboard)

# Должно быть ~1-2% в idle
```

## Unit Tests (Future)

```bash
# Создайте тесты для:
# - Domain entities
# - Domain services
# - Use cases (с моками)
# - Repository (с test container)

go test ./internal/domain/...
go test ./internal/application/...
```

## Integration Tests (Future)

```bash
# Тесты с реальной БД:
go test -tags=integration ./internal/infrastructure/...

# Тесты WebSocket:
go test -tags=integration ./internal/interfaces/...
```

## Expected Results Summary

После успешного тестирования:

✅ Приложение запускается без ошибок
✅ Метрики собираются каждые 2 секунды
✅ WebSocket соединение работает стабильно
✅ Данные сохраняются в PostgreSQL
✅ Dashboard обновляется в реальном времени
✅ Графики отображают историю
✅ API endpoints возвращают корректные данные
✅ Множественные подключения работают
✅ Reconnection работает автоматически
✅ Graceful shutdown работает корректно

## Troubleshooting

### Проблема: Metrics not collecting

**Решение:**
```bash
# Проверьте что collector запущен
# В логах должно быть:
# [INFO] Metrics collector started

# Проверьте БД
psql -U postgres -d monitoring -c "SELECT COUNT(*) FROM metrics;"
```

### Проблема: WebSocket disconnects

**Решение:**
```bash
# Проверьте логи на ошибки
# Проверьте firewall
# Попробуйте другой браузер
```

### Проблема: High memory usage

**Решение:**
```bash
# Нормально: 50-100 MB
# Проверьте количество подключенных клиентов
# Проверьте количество записей в БД
```

## Success Criteria

Проект считается успешно протестированным если:

1. ✅ Все компоненты запускаются без ошибок
2. ✅ Real-time обновления работают стабильно
3. ✅ Данные корректно сохраняются и читаются из БД
4. ✅ WebSocket соединения стабильны
5. ✅ Приложение корректно завершается
6. ✅ Performance в пределах нормы
7. ✅ Множественные клиенты поддерживаются

Good luck! 🚀
