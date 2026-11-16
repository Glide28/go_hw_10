package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Glide28/go_hw_10/ledger/internal/repository/pg"
	redisrepo "github.com/Glide28/go_hw_10/ledger/internal/repository/redis"
	"github.com/Glide28/go_hw_10/ledger/internal/service"
)

func Open(ctx context.Context) (service.Service, func(), error) {
	db, err := sql.Open("pgx", databaseURL())
	if err != nil {
		return nil, nil, fmt.Errorf("open PostgreSQL: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	redisDB, _ := strconv.Atoi(envOrDefault("REDIS_DB", "0"))
	cache, err := redisrepo.New(ctx, envOrDefault("REDIS_ADDR", "localhost:6379"), os.Getenv("REDIS_PASSWORD"), redisDB)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("connect to Redis: %w", err)
	}
	log.Printf("connected to Redis at %s (db=%d)", envOrDefault("REDIS_ADDR", "localhost:6379"), redisDB)

	application := service.New(pg.NewBudgetRepository(db), pg.NewExpenseRepository(db), cache)
	closeFn := func() {
		_ = cache.Close()
		_ = db.Close()
	}
	return application, closeFn, nil
}

func databaseURL() string {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		envOrDefault("DB_USER", "postgres"), envOrDefault("DB_PASS", "postgres"),
		envOrDefault("DB_HOST", "localhost"), envOrDefault("DB_PORT", "5432"),
		envOrDefault("DB_NAME", "cashapp"))
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
