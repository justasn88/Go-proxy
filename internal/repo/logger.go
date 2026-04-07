package repo

import (
	"context"
	"database/sql"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type TrafficLog struct {
	Username  string
	BytesUsed int64
}

type AsyncLogger struct {
	db    *sql.DB
	redis *redis.Client
}

func NewAsyncLogger(db *sql.DB, redisClient *redis.Client) *AsyncLogger {
	return &AsyncLogger{
		db:    db,
		redis: redisClient,
	}
}

func (l *AsyncLogger) Start() {
	log.Println("Async log worker started")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ctx := context.Background()

	for range ticker.C {
		err := l.redis.Rename(ctx, "pending_db_logs", "processing_db_logs").Err()
		if err != nil {
			if err.Error() == "ERR no such key" {
				continue
			}
			log.Printf("Worker redis error: %v", err)
			continue
		}
		logs, err := l.redis.HGetAll(ctx, "processing_db_logs").Result()
		if err != nil || len(logs) == 0 {
			continue
		}

		for username, bytesStr := range logs {
			bytes, _ := strconv.ParseInt(bytesStr, 10, 64)

			_, err := l.db.Exec(
				"INSERT INTO traffic_logs (username, bytes_used) VALUES ($1, $2)",
				username, bytes,
			)
			if err != nil {
				log.Printf("error when writing logs for user %s: %v", username, err)
			}
		}
		l.redis.Del(ctx, "processing_db_logs")
		log.Printf("Successfuly saved %d users data from Redis to PostgreSQL", len(logs))
	}

}
