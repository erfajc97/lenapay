// Package dbtest starts ONE real Postgres per test binary (testcontainers), migrates a template
// database with the real migrations, and gives each test its own database cloned from the
// template (fast, isolated, parallel-safe). FT6: data rules are proven on real Postgres.
//
// Use it from integration tests (`//go:build integration`):
//
//	var pg *dbtest.Postgres
//
//	func TestMain(m *testing.M) { os.Exit(dbtest.Main(context.Background(), m, &pg)) }
//
//	func TestX(t *testing.T) { pool := pg.NewDatabase(t) ... }
package dbtest

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"lenapay/apps/api/internal/platform/db"
)

const (
	image    = "postgres:17" // the same major as docker-compose.yml
	template = "lenapay_template"
)

// Postgres is the per-binary server with its migrated template.
type Postgres struct {
	ctr   *postgres.PostgresContainer
	admin *pgxpool.Pool
	dsn   func(database string) string
	n     atomic.Int64
}

// Start runs the container and migrates the template. Call it from TestMain.
func Start(ctx context.Context) (*Postgres, error) {
	ctr, err := postgres.Run(ctx, image,
		postgres.WithDatabase(template),
		postgres.WithUsername("lenapay"),
		postgres.WithPassword("lenapay"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("dbtest: start: %w", err)
	}
	base, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("dbtest: dsn: %w", err)
	}
	cfg, err := pgx.ParseConfig(base)
	if err != nil {
		return nil, fmt.Errorf("dbtest: parse dsn: %w", err)
	}
	dsn := func(database string) string {
		return fmt.Sprintf("postgres://lenapay:lenapay@%s:%d/%s?sslmode=disable", cfg.Host, cfg.Port, database)
	}
	if err := db.Migrate(ctx, base); err != nil { // the REAL migrations, River included
		return nil, err
	}
	admin, err := pgxpool.New(ctx, dsn("postgres"))
	if err != nil {
		return nil, fmt.Errorf("dbtest: admin pool: %w", err)
	}
	return &Postgres{ctr: ctr, admin: admin, dsn: dsn}, nil
}

// Main is the TestMain body: start, run the tests, stop. It returns the exit code.
func Main(ctx context.Context, m *testing.M, pg **Postgres) int {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	started, err := Start(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	*pg = started
	code := m.Run()
	if err := started.Stop(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return code
}

// NewDatabase clones the migrated template for one test and closes it at cleanup.
func (p *Postgres) NewDatabase(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx := t.Context()
	name := fmt.Sprintf("t_%d", p.n.Add(1))
	if _, err := p.admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE "+template); err != nil {
		t.Fatalf("dbtest: create database: %v", err)
	}
	pool, err := db.NewPool(ctx, p.dsn(name), 10)
	if err != nil {
		t.Fatalf("dbtest: open: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// EmptyDatabase is a database with NO migrations (the drift test migrates it itself).
func (p *Postgres) EmptyDatabase(t testing.TB) string {
	t.Helper()
	name := fmt.Sprintf("e_%d", p.n.Add(1))
	if _, err := p.admin.Exec(t.Context(), "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatalf("dbtest: create database: %v", err)
	}
	return p.dsn(name)
}

// Stop removes the container.
func (p *Postgres) Stop() error {
	p.admin.Close()
	return testcontainers.TerminateContainer(p.ctr)
}
