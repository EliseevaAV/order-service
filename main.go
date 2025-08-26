package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
)

// =====================
// Модель заказа
// =====================

type Order struct {
	OrderUID string          `json:"order_uid"`
	Track    string          `json:"track_number"`
	Delivery json.RawMessage `json:"delivery"`
	Payment  json.RawMessage `json:"payment"`
	Items    json.RawMessage `json:"items"`
}

// =====================
// In-memory кэш
// =====================

type memCacheStore struct {
	mu sync.RWMutex
	m  map[string]Order
}

var memCache = memCacheStore{m: make(map[string]Order)}

func cacheGet(id string) (Order, bool) {
	memCache.mu.RLock()
	defer memCache.mu.RUnlock()
	o, ok := memCache.m[id]
	return o, ok
}

func cacheSet(id string, o Order) {
	memCache.mu.Lock()
	memCache.m[id] = o
	memCache.mu.Unlock()
}

// =====================
// Основной main
// =====================

func main() {
	connStr := "postgres://testuser:testpass@localhost:5432/ordersdb?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Ошибка подключения к PostgreSQL:", err)
	}
	defer db.Close()

	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	loadCacheOnStartup(db, rdb, 10)
	go consumeKafka(db, rdb)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
	})

	http.HandleFunc("/order/", func(w http.ResponseWriter, r *http.Request) {
		uid := r.URL.Path[len("/order/"):]
		if uid == "" {
			http.Error(w, "order_uid не указан", http.StatusBadRequest)
			return
		}

		var order Order

		if o, ok := cacheGet(uid); ok {
			order = o
		} else if cached, err := rdb.Get(ctx, uid).Result(); err == nil {
			_ = json.Unmarshal([]byte(cached), &order)
			cacheSet(uid, order)
		} else {
			row := db.QueryRow("SELECT order_uid, track_number, delivery, payment, items FROM orders WHERE order_uid=$1", uid)
			if err := row.Scan(&order.OrderUID, &order.Track, &order.Delivery, &order.Payment, &order.Items); err != nil {
				http.Error(w, "Заказ не найден", http.StatusNotFound)
				return
			}
			cacheSet(uid, order)
			data, _ := json.Marshal(order)
			_ = rdb.Set(ctx, uid, data, time.Minute).Err()
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(order)
	})

	log.Println("Сервер запущен на http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

// =====================
// Kafka consumer
// =====================

func consumeKafka(db *sql.DB, rdb *redis.Client) {
	brokers := []string{"localhost:9092"}
	topic := "orders"

	cfg := sarama.NewConfig()
	cfg.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumer(brokers, cfg)
	if err != nil {
		log.Fatal("Ошибка подключения к Kafka:", err)
	}
	defer consumer.Close()

	pc, err := consumer.ConsumePartition(topic, 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatal("Ошибка при создании партиции Kafka:", err)
	}
	defer pc.Close()

	ctx := context.Background()

	for msg := range pc.Messages() {
		var order Order
		if err := json.Unmarshal(msg.Value, &order); err != nil {
			log.Printf("Ошибка парсинга JSON: %v", err)
			continue
		}

		_, err = db.Exec(`
            INSERT INTO orders (order_uid, track_number, delivery, payment, items)
            VALUES ($1, $2, $3, $4, $5)
            ON CONFLICT (order_uid) DO NOTHING`,
			order.OrderUID, order.Track, order.Delivery, order.Payment, order.Items,
		)
		if err != nil {
			log.Printf("Ошибка сохранения заказа %s в Postgres: %v", order.OrderUID, err)
			continue
		}

		cacheSet(order.OrderUID, order)
		data, _ := json.Marshal(order)
		_ = rdb.Set(ctx, order.OrderUID, data, time.Minute).Err()
	}
}

// =====================
// Прогрев кэша
// =====================

func loadCacheOnStartup(db *sql.DB, rdb *redis.Client, n int) {
	rows, err := db.Query(`
        SELECT order_uid, track_number, delivery, payment, items
        FROM orders
        ORDER BY order_uid DESC
        LIMIT $1
    `, n)
	if err != nil {
		log.Printf("Ошибка чтения заказов из Postgres: %v", err)
		return
	}
	defer rows.Close()

	ctx := context.Background()

	for rows.Next() {
		var order Order
		var delivery, payment, items []byte

		if err := rows.Scan(&order.OrderUID, &order.Track, &delivery, &payment, &items); err != nil {
			log.Printf("Ошибка сканирования: %v", err)
			continue
		}

		order.Delivery = json.RawMessage(delivery)
		order.Payment = json.RawMessage(payment)
		order.Items = json.RawMessage(items)

		cacheSet(order.OrderUID, order)
		data, _ := json.Marshal(order)
		_ = rdb.Set(ctx, order.OrderUID, data, 0).Err()
	}
}

// =====================
// Простая HTML-страница
// =====================

var indexHTML = `
<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8"/>
  <title>Order Lookup</title>
  <style>
    body { font-family: system-ui, Arial, sans-serif; margin: 40px; }
    .row { display:flex; gap:8px; align-items:center; }
    input { padding:8px; width: 340px; }
    button { padding:8px 12px; cursor:pointer; }
    pre { background:#f6f8fa; padding:16px; border-radius:8px; max-width: 820px; overflow:auto; }
    .hint { color:#666; font-size:14px; margin-top:6px; }
  </style>
</head>
<body>
  <h1>Поиск заказа</h1>
  <div class="row">
    <input id="orderId" placeholder="Введите order_uid, напр. b563feb7b2b84b6test"/>
    <button onclick="fetchOrder()">Найти</button>
  </div>
  <div class="hint">Запрос идёт в API: <code>/order/&lt;order_uid&gt;</code></div>
  <h2>Результат</h2>
  <pre id="result">Пусто...</pre>
  <script>
    async function fetchOrder() {
      const id = document.getElementById('orderId').value.trim();
      const out = document.getElementById('result');
      if (!id) { alert('Введите order_uid'); return; }
      out.textContent = 'Загрузка...';
      try {
        const res = await fetch('/order/' + encodeURIComponent(id));
        if (!res.ok) {
          out.textContent = 'Ошибка: заказ не найден (HTTP ' + res.status + ')';
          return;
        }
        const data = await res.json();
        out.textContent = JSON.stringify(data, null, 2);
      } catch (e) {
        out.textContent = 'Ошибка запроса: ' + e;
      }
    }
  </script>
</body>
</html>`
