package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/erfajc97/lenapay/apps/api/migrations"
)

// Migrate applies every pending migration (embedded in the binary; River's tables are one of
// them). Used by cmd/migrate, the integration tests and the drift test.
func Migrate(ctx context.Context, url string) error {
	provider, closeFn, err := newProvider(ctx, url)
	if err != nil {
		return err
	}
	defer closeFn()
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("db: migrate up: %w", err)
	}
	return nil
}

// MigrateDownToZero rolls every migration back: the drift test proves each Down works.
func MigrateDownToZero(ctx context.Context, url string) error {
	provider, closeFn, err := newProvider(ctx, url)
	if err != nil {
		return err
	}
	defer closeFn()
	if _, err := provider.DownTo(ctx, 0); err != nil {
		return fmt.Errorf("db: migrate down: %w", err)
	}
	return nil
}

func newProvider(ctx context.Context, url string) (*goose.Provider, func(), error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, nil, fmt.Errorf("db: open: %w", err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	closeFn := func() {
		_ = sqlDB.Close() // closing a pool-backed sql.DB cannot fail in a way we can act on
		pool.Close()
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		closeFn()
		return nil, nil, fmt.Errorf("db: goose: %w", err)
	}
	return provider, closeFn, nil
}
