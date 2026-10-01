package postgres

import (
	"context"
	"embed"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrationLock is the Postgres advisory lock that stops two bot
// instances from migrating at the same time.
const migrationLock = 7_241_503

type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads migrations/NNNN_name.sql in version order.
func loadMigrations() ([]migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	migrations := make([]migration, 0, len(entries))
	for _, e := range entries {
		num, name, ok := strings.Cut(strings.TrimSuffix(e.Name(), ".sql"), "_")
		if !ok {
			return nil, fmt.Errorf("migration %q: want NNNN_name.sql", e.Name())
		}

		version, err := strconv.Atoi(num)
		if err != nil {
			return nil, fmt.Errorf("migration %q: bad version: %w", e.Name(), err)
		}

		sql, err := migrationFiles.ReadFile(path.Join("migrations", e.Name()))
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, migration{version: version, name: name, sql: string(sql)})
	}

	slices.SortFunc(migrations, func(a, b migration) int { return a.version - b.version })

	return migrations, nil
}

// migrate applies pending migrations, each in its own transaction.
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("failed to create schema_migrations: %w", err)
	}

	for _, m := range migrations {
		if err := apply(ctx, pool, m); err != nil {
			return fmt.Errorf("migration %04d_%s: %w", m.version, m.name, err)
		}
	}

	return nil
}

func apply(ctx context.Context, pool *pgxpool.Pool, m migration) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// Another instance may have applied it while we waited.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, migrationLock); err != nil {
			return err
		}

		var applied bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, m.version).Scan(&applied); err != nil {
			return err
		}

		if applied {
			return nil
		}

		if _, err := tx.Exec(ctx, m.sql); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.version, m.name)
		return err
	})
}
