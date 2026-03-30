package repo

import (
	"awesomeProject11/internal/domain"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

type redisUser struct {
	client   *redis.Client
	username string
	logger   *AsyncLogger
}

type RedisRepo struct {
	client      *redis.Client
	credentials map[string]string
}

func NewRedisRepo(client *redis.Client, creds map[string]string) domain.Repository {
	return &RedisRepo{
		client:      client,
		credentials: creds,
	}
}

func (r *RedisRepo) GetCredentials() map[string]string {
	return r.credentials
}

func (r *RedisRepo) GetOrCreateUser(username string) domain.User {
	return &redisUser{
		client:   r.client,
		username: username,
	}
}

func (u *redisUser) AddData(n int64) {
	key := "user:" + u.username + ":data_used"
	err := u.client.IncrBy(ctx, key, n).Err()
	if err != nil {
		log.Printf("Failed to update user %s, data used in Redis db: %v", u.username, err)
	}

	err = u.client.HIncrBy(ctx, "pending_db_logs", u.username, n).Err()
	if err != nil {
		log.Printf("failed to update pending logs in Redis: %v", err)
	}
}

func (u *redisUser) IsOverDataLimit(limit int64) bool {
	key := "user:" + u.username + ":data_used"

	val, err := u.client.Get(ctx, key).Result()

	if err == redis.Nil {
		return false
	} else if err != nil {
		log.Printf("Failed to read data from redis user: %s : %v", u.username, err)
		return true
	}

	used, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		log.Printf("Failed to convert string to int64 in Redis: %v", err)
		return true
	}
	return limit <= used
}
func (u *redisUser) TryIncrementConnections(max int64) bool {

	key := "user:" + u.username + ":connections"

	val, err := u.client.IncrBy(ctx, key, 1).Result()

	if err != nil {
		log.Printf("Cannot increment connections: user: %s %v", u.username, err)
		return false
	}
	if val > max {
		u.client.Decr(ctx, key)
		return false
	}
	return true
}

func (u *redisUser) DecrementConnections() {
	key := "user:" + u.username + ":connections"

	conns, err := u.client.Decr(ctx, key).Result()

	if err != nil {
		log.Printf("Cannot decrease connections: %v", err)
	}
	if conns < 0 {
		log.Printf("Decreased below 0")
	}
}

func CreateRedisCache(db int) (*redis.Client, error) {
	redisAddr := os.Getenv("REDIS_ADDR")

	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "asd",
		DB:       db,
	})

	ctx := context.Background()
	err := client.Ping(ctx).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %v", redisAddr, err)
	}
	return client, nil
}
