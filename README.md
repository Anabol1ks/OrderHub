OrderHub — микросервисная платформа заказов с сагами и наблюдаемостью

Кратко
- Цель: показать практики enterprise-уровня на реальном примере: распределённые транзакции (Saga), событийная интеграция через Kafka, идемпотентность/аутбокс, API Gateway, наблюдаемость (tracing/metrics/logs) и real-time обновления статусов по WebSocket.
- Язык/стек: Go, gRPC/REST, Kafka, Redis, PostgreSQL (план), OpenTelemetry (план), Docker Compose для локальной развёртки.

Архитектура (микросервисы)
- api-gateway — внешний REST API и внутренний gRPC, rate-limit и auth-proxy.
- auth-service — аутентификация и выдача JWT (access/refresh), хранение/ротация JWK, управление блокировками токенов.
- order-service — CRUD заказов, оркестрация Саги создания/отмены заказа. (план)
- payment-service — резерв/чарж платежей с идемпотентностью. (план)
- inventory-service — резерв и возврат товарных позиций. (план)
- notification-service — потребитель Kafka, отправка email/push. Сейчас используется для событий auth. 
- status-ws — WebSocket-шина для стриминга статусов и прогресса заказа в реальном времени. (план)
- audit-service — журнал действий, CDC/аутбокс для надёжной доставки событий. (план)

Статус и прогресс
- Сервисы
	- [x] auth-service — базовые сценарии аутентификации, JWT (access/refresh), JWK, репозитории, миграции. (≈ 80%)
	- [x] notification-service — consumer Kafka + email-шаблоны для событий auth. (≈ 60%)
	- [x] api-gateway — прокси и эндпоинты для auth, Swagger. (≈ 50%)
	- [ ] order-service — CRUD и оркестрация Саги (создание/отмена), интеграция с payment/inventory. (0%)
	- [ ] payment-service — резерв/чарж с идемпотентностью, учёт транзакций, DLQ. (0%)
	- [ ] inventory-service — резерв/возврат запасов, согласование с Saga. (0%)
	- [ ] status-ws — WebSocket-стриминг статусов по orderId/пользователю. (0%)
	- [ ] audit-service — аутбокс/CDC, аудит, восстановление после сбоев. (0%)

- Сквозные направления
	- [ ] Наблюдаемость (OpenTelemetry tracing, Prometheus metrics, структурные логи, trace-id корелляция). (0%)
	- [ ] Надёжность (идемпотентность хендлеров/consumer’ов, ретраи, токсичные сообщения, DLQ). (0%)
	- [ ] Безопасность (ротация JWK, списки блокировок токенов, политики в gateway). (0%)
	- [x] Инфраструктура для событийной шины (Kafka via docker-compose). (100%)

Навигация по коду
- orderhub-api-gateway — API шлюз, Swagger в docs/, middleware и router внутри internal/.
- orderhub-auth-service — доменная логика аутентификации, репозитории, токены, gRPC-транспорт.
- orderhub-notification-service — Kafka consumer и отправка email (templates/ для писем).

Примечания
- Проект находится в активной разработке. Публичные контракты могут изменяться до стабилизации сервисов order/payment/inventory.
- После добавления БД (PostgreSQL) и метрик/трассировки README будет дополнен разделами по конфигурации и observability.
