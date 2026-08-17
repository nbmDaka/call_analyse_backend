// Package database configures PostgreSQL access and runs embedded migrations.
package database

import (
	"context"
	"fmt"
	"time"

	"call_analyse_backend/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	connectTimeout = 10 * time.Second
	acquireTimeout = 5 * time.Second
)

// Open creates a PostgreSQL pool and verifies that a connection can be acquired.
func Open(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	poolConfig.ConnConfig.ConnectTimeout = connectTimeout

	connectCtx, cancelConnect := context.WithTimeout(ctx, connectTimeout)
	defer cancelConnect()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}

	acquireCtx, cancelAcquire := context.WithTimeout(ctx, acquireTimeout)
	defer cancelAcquire()

	connection, err := pool.Acquire(acquireCtx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("acquire database connection: %w", err)
	}
	connection.Release()

	return pool, nil
}
