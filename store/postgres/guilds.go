package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// settingColumns are the columns UpdateGuild writes, in settingValues
// order.
const settingColumns = `prefix, post_limit, tags, reactions, crosspost, skip_first_tweet, quotes,
	nsfw_quotes, disabled_providers, repost_mode, repost_ttl, art_channels`

const guildColumns = `id, ` + settingColumns + `, created_at, updated_at`

type guildStore struct {
	pool *pgxpool.Pool
}

func (g *guildStore) Guild(ctx context.Context, id string) (*store.Guild, error) {
	if id == "" {
		return store.UserGuild(), nil
	}

	row := g.pool.QueryRow(ctx, `SELECT `+guildColumns+` FROM guilds WHERE id = $1`, id)

	guild := &store.Guild{}
	if err := scanGuild(row, guild); err != nil {
		return nil, err
	}

	return guild, nil
}

func (g *guildStore) CreateGuild(ctx context.Context, id string) (*store.Guild, error) {
	guild := store.DefaultGuild(id)
	args := append(append([]any{guild.ID}, settingValues(guild)...), guild.CreatedAt, guild.UpdatedAt)

	_, err := g.pool.Exec(ctx, `INSERT INTO guilds (`+guildColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO NOTHING`, args...)
	if err != nil {
		return nil, err
	}

	return g.Guild(ctx, id)
}

func (g *guildStore) UpdateGuild(ctx context.Context, guild *store.Guild) (*store.Guild, error) {
	guild.UpdatedAt = time.Now().UTC()
	args := append(append([]any{guild.ID}, settingValues(guild)...), guild.UpdatedAt)

	row := g.pool.QueryRow(ctx, `UPDATE guilds SET (`+settingColumns+`, updated_at)
		= ($2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		WHERE id = $1
		RETURNING `+guildColumns, args...)

	updated := &store.Guild{}
	if err := scanGuild(row, updated); err != nil {
		return nil, err
	}

	return updated, nil
}

func (g *guildStore) AddArtChannels(ctx context.Context, guildID string, channels []string) (*store.Guild, error) {
	row := g.pool.QueryRow(ctx, `UPDATE guilds SET art_channels = ARRAY(
			SELECT ch FROM unnest(art_channels || $2::text[]) WITH ORDINALITY AS t(ch, ord)
			WHERE ch IS NOT NULL GROUP BY ch ORDER BY min(ord)
		), updated_at = now()
		WHERE id = $1
		RETURNING `+guildColumns,
		guildID, channels,
	)

	guild := &store.Guild{}
	if err := scanGuild(row, guild); err != nil {
		return nil, err
	}

	return guild, nil
}

func (g *guildStore) DeleteArtChannels(ctx context.Context, guildID string, channels []string) (*store.Guild, error) {
	// A nil slice encodes as NULL, and <> ALL(NULL) would drop every channel.
	if channels == nil {
		channels = make([]string, 0)
	}

	row := g.pool.QueryRow(ctx, `UPDATE guilds SET art_channels = ARRAY(
			SELECT ch FROM unnest(art_channels) WITH ORDINALITY AS t(ch, ord)
			WHERE ch <> ALL($2::text[]) ORDER BY ord
		), updated_at = now()
		WHERE id = $1
		RETURNING `+guildColumns,
		guildID, channels,
	)

	guild := &store.Guild{}
	if err := scanGuild(row, guild); err != nil {
		return nil, err
	}

	return guild, nil
}

type guildRow interface {
	Scan(dest ...any) error
}

// settingValues lists a guild's settings in settingColumns order.
func settingValues(g *store.Guild) []any {
	disabled := g.DisabledProviders
	if disabled == nil {
		disabled = make([]string, 0)
	}

	channels := g.ArtChannels
	if channels == nil {
		channels = make([]string, 0)
	}

	p := g.Posting

	return []any{
		g.Prefix, p.Limit, p.Tags, p.Reactions, p.Crosspost, p.SkipFirstTweet, p.Quotes,
		p.NSFWQuotes, disabled, string(g.Repost.Mode), g.Repost.TTL, channels,
	}
}

func scanGuild(row guildRow, guild *store.Guild) error {
	var (
		mode string
		p    = &guild.Posting
	)

	if err := row.Scan(&guild.ID, &guild.Prefix, &p.Limit, &p.Tags, &p.Reactions, &p.Crosspost,
		&p.SkipFirstTweet, &p.Quotes, &p.NSFWQuotes, &guild.DisabledProviders, &mode, &guild.Repost.TTL,
		&guild.ArtChannels, &guild.CreatedAt, &guild.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.ErrGuildNotFound
		}

		return fmt.Errorf("failed to scan guild: %w", err)
	}

	guild.Repost.Mode = store.RepostMode(mode)

	if guild.DisabledProviders == nil {
		guild.DisabledProviders = make([]string, 0)
	}

	if guild.ArtChannels == nil {
		guild.ArtChannels = make([]string, 0)
	}

	return nil
}
