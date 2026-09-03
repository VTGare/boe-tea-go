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
	for _, stmt := range schemaDDL() {
		if _, err := p.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("failed to init postgres schema: %w", err)
		}
	}

	return nil
}

func (p *postgresStore) Close(_ context.Context) error {
	p.pool.Close()

	return nil
}

func schemaDDL() []string {
	return []string{
		`CREATE EXTENSION IF NOT EXISTS pg_trgm`,
		`CREATE SEQUENCE IF NOT EXISTS artwork_id_seq`,
		`CREATE TABLE IF NOT EXISTS guilds (
			id TEXT PRIMARY KEY,
			prefix TEXT NOT NULL,
			pixiv BOOLEAN NOT NULL DEFAULT TRUE,
			twitter BOOLEAN NOT NULL DEFAULT TRUE,
			deviant BOOLEAN NOT NULL DEFAULT TRUE,
			bluesky BOOLEAN NOT NULL DEFAULT TRUE,
			tags BOOLEAN NOT NULL DEFAULT TRUE,
			flavour_text BOOLEAN NOT NULL DEFAULT TRUE,
			crosspost BOOLEAN NOT NULL DEFAULT TRUE,
			reactions BOOLEAN NOT NULL DEFAULT FALSE,
			skip_first BOOLEAN NOT NULL DEFAULT FALSE,
			"limit" BIGINT NOT NULL DEFAULT 10,
			repost TEXT NOT NULL DEFAULT 'enabled',
			repost_expiration BIGINT NOT NULL DEFAULT 86400000000000,
			art_channels TEXT[] NOT NULL DEFAULT '{}',
			nsfw BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			dm BOOLEAN NOT NULL DEFAULT TRUE,
			crosspost BOOLEAN NOT NULL DEFAULT TRUE,
			ignore BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS user_groups (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			parent TEXT NOT NULL DEFAULT '',
			is_pair BOOLEAN NOT NULL DEFAULT FALSE,
			children TEXT[] NOT NULL DEFAULT '{}',
			PRIMARY KEY (user_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS artworks (
			id INTEGER PRIMARY KEY DEFAULT nextval('artwork_id_seq'),
			title TEXT NOT NULL DEFAULT '',
			author TEXT NOT NULL DEFAULT '',
			url TEXT UNIQUE NOT NULL,
			images TEXT[] NOT NULL DEFAULT '{}',
			favourites INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS bookmarks (
			user_id TEXT NOT NULL,
			artwork_id INTEGER NOT NULL,
			nsfw BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (user_id, artwork_id)
		)`,
		`CREATE INDEX IF NOT EXISTS artworks_created_at_idx ON artworks (created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS artworks_favourites_idx ON artworks (favourites DESC)`,
		`CREATE INDEX IF NOT EXISTS artworks_title_trgm_idx ON artworks USING gin (title gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS artworks_author_trgm_idx ON artworks USING gin (author gin_trgm_ops)`,
		`CREATE INDEX IF NOT EXISTS bookmarks_user_created_idx ON bookmarks (user_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS bookmarks_artwork_idx ON bookmarks (artwork_id)`,
		`CREATE INDEX IF NOT EXISTS user_groups_user_idx ON user_groups (user_id)`,
	}
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
