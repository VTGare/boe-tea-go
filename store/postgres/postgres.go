package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresStore struct {
	pool *pgxpool.Pool

	*artworkStore
	*userStore
	*guildStore
	*bookmarkStore
}

func New(ctx context.Context, dsn string) (store.Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres dsn: %w", err)
	}

	cfg.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &postgresStore{
		pool:          pool,
		artworkStore:  &artworkStore{pool: pool},
		userStore:     &userStore{pool: pool},
		guildStore:    &guildStore{pool: pool},
		bookmarkStore: &bookmarkStore{pool: pool},
	}, nil
}

func (p *postgresStore) Init(ctx context.Context) error {
	if err := migrate(ctx, p.pool); err != nil {
		return fmt.Errorf("failed to migrate postgres schema: %w", err)
	}

	return nil
}

func (p *postgresStore) Close(_ context.Context) error {
	p.pool.Close()

	return nil
}

func isUniqueViolation(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == "23505"
	}

	return false
}

// escapeILIKE escapes %, _ and \ so user input is a literal substring for ILIKE.
func escapeILIKE(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)

	return s
}
