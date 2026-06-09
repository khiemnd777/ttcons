package db

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Connect(ctx context.Context, databaseURL string) (*sql.DB, error) {
	conn, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(20)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(30 * time.Minute)
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}
