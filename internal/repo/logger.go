package repo

import (
	"database/sql"
	"log"
)

type TrafficLog struct {
	Username  string
	BytesUsed int64
}

type AsyncLogger struct {
	db    *sql.DB
	queue chan TrafficLog
}

func NewAsyncLogger(db *sql.DB, bufferSize int) *AsyncLogger {
	return &AsyncLogger{
		db:    db,
		queue: make(chan TrafficLog, 2000),
	}
}

func (l *AsyncLogger) start() {
	log.Println("Async log worker started")

	for entry := range l.queue {
		_, err := l.db.Exec(
			"INSERT INTO traffic_logs (username, bytes_used) values ($1, $2)",
			entry.Username,
			entry.BytesUsed)
		if err != nil {
			log.Printf("Worker error when inputing logs %s: %v", entry.Username, err)
		}
	}
}

func (l *AsyncLogger) Push(username string, bytes int64) {
	select {
	case l.queue <- TrafficLog{Username: username, BytesUsed: bytes}:

	default:
		log.Printf("Queue is full, lost %d bytes for user %s", bytes, username)
	}
}
