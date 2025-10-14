package db

import (
	"context"
	"fmt"
	"time"

	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func GetDB(pgDsn string) (*bun.DB, error) {
	if pgDsn == "" {
		return nil, fmt.Errorf("pg_dsn is required")
	}

	sqlConnection := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithDSN(pgDsn),
	))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlConnection.PingContext(ctx); err != nil {
		_ = sqlConnection.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db := bun.NewDB(sqlConnection, pgdialect.New())

	return db, nil
}
