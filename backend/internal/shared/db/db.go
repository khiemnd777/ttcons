package db

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

type Options struct {
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
	ConnMaxIdleTime  time.Duration
	StatementTimeout time.Duration
	LockTimeout      time.Duration
}

func Connect(ctx context.Context, databaseURL string, options ...Options) (*sql.DB, error) {
	opts := Options{
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
	if len(options) > 0 {
		provided := options[0]
		if provided.MaxOpenConns > 0 {
			opts.MaxOpenConns = provided.MaxOpenConns
		}
		if provided.MaxIdleConns > 0 {
			opts.MaxIdleConns = provided.MaxIdleConns
		}
		if provided.ConnMaxLifetime > 0 {
			opts.ConnMaxLifetime = provided.ConnMaxLifetime
		}
		if provided.ConnMaxIdleTime > 0 {
			opts.ConnMaxIdleTime = provided.ConnMaxIdleTime
		}
		opts.StatementTimeout = provided.StatementTimeout
		opts.LockTimeout = provided.LockTimeout
	}

	pgxConfig, err := connectionConfig(databaseURL, opts)
	if err != nil {
		return nil, err
	}
	conn := stdlib.OpenDB(*pgxConfig)
	conn.SetMaxOpenConns(opts.MaxOpenConns)
	conn.SetMaxIdleConns(opts.MaxIdleConns)
	conn.SetConnMaxLifetime(opts.ConnMaxLifetime)
	conn.SetConnMaxIdleTime(opts.ConnMaxIdleTime)
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func connectionConfig(databaseURL string, opts Options) (*pgx.ConnConfig, error) {
	pgxConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	if opts.StatementTimeout > 0 {
		pgxConfig.RuntimeParams["statement_timeout"] = durationMilliseconds(opts.StatementTimeout)
	}
	if opts.LockTimeout > 0 {
		pgxConfig.RuntimeParams["lock_timeout"] = durationMilliseconds(opts.LockTimeout)
	}
	return pgxConfig, nil
}

func durationMilliseconds(value time.Duration) string {
	milliseconds := value.Milliseconds()
	if milliseconds < 1 {
		milliseconds = 1
	}
	return strconv.FormatInt(milliseconds, 10)
}
