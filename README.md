# Order Service

Учебный сервис на Go для работы с заказами.
Сервис получает заказы из Kafka, сохраняет их в PostgreSQL, кеширует в Redis и отдаёт через HTTP API и простой веб-интерфейс.

⸻

# Запуск проекта:

1. Клонировать репозиторий:
git clone https://github.com/EliseevaAV/order-service.git
cd order-service
2. Запустить инфраструктуру (Kafka + PostgreSQL):
docker-compose up -d
3. Запустить сервис:
go run main.go

⸻

# HTTP API

Получить заказ по ID:
GET http://localhost:8081/order/<order_uid>

Пример:
GET http://localhost:8081/order/b563feb7b2b84b6test

Ответ (JSON):
{“order_uid”: “b563feb7b2b84b6test”, “track_number”: “WBILMTESTTRACK”, “entry”: “WBIL”, “delivery”: {“name”: “Test Testov”, “phone”: “+9720000000”, “zip”: “2639809”, “city”: “Kiryat Mozkin”, “address”: “Ploshad Mira 15”, “region”: “Kraiot”, “email”: “test@gmail.com”}, “payment”: {“transaction”: “b563feb7b2b84b6test”, “request_id”: “”, “currency”: “USD”, “provider”: “wbpay”, “amount”: 1817, “payment_dt”: 1637907727, “bank”: “alpha”, “delivery_cost”: 1500, “goods_total”: 317, “custom_fee”: 0}, “items”: [{“chrt_id”: 9934930, “track_number”: “WBILMTESTTRACK”, “price”: 453, “rid”: “ab4219087a764ae0btest”, “name”: “Mascaras”, “sale”: 30, “size”: “0”, “total_price”: 317, “nm_id”: 2389212, “brand”: “Vivienne Sabo”, “status”: 202}], “locale”: “en”, “internal_signature”: “”, “customer_id”: “test”, “delivery_service”: “meest”, “shardkey”: “9”, “sm_id”: 99, “date_created”: “2021-11-26T06:22:19Z”, “oof_shard”: “1”}

⸻

# Веб-интерфейс

Открой в браузере:
http://localhost:8081

На странице можно ввести order_uid: b563feb7b2b84b6test и получить данные о заказе в JSON-формате.

⸻

# Стек технологий
- Go (net/http)
- Kafka (брокер сообщений)
- PostgreSQL (база данных)
- Redis (кеш в памяти)
- Docker Compose (инфраструктура)

## Видео-демо
Видео работы сервиса: https://disk.yandex.ru/i/al_LjIc4Zggk2Q
