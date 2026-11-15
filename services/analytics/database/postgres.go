package database

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"analytics/internal/config"
)

func Connect(cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.DBUser, cfg.DBPass),
		Host:   net.JoinHostPort(cfg.DBHost, strconv.Itoa(cfg.DBPort)),
		Path:   "/" + cfg.DBName,
	}
	query := dsn.Query()
	query.Set("sslmode", cfg.DBSSLMode)
	dsn.RawQuery = query.Encode()
	pool, err := pgxpool.New(context.Background(), dsn.String())
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return pool, nil
}
