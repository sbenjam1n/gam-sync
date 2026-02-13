package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect creates a connection pool to PostgreSQL.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate runs the SQL migration files against the database.
func Migrate(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	sqlFile := filepath.Join(migrationsDir, "001_initial.sql")
	sql, err := os.ReadFile(sqlFile)
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}
	return nil
}
