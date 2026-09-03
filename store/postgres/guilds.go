package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type guildStore struct {
	pool *pgxpool.Pool
}

func (g *guildStore) Guild(ctx context.Context, id string) (*store.Guild, error) {
	if id == "" {
		return store.UserGuild(), nil
	}

	row := g.pool.QueryRow(ctx, `SELECT id, prefix, pixiv, twitter, deviant, bluesky, tags, flavour_text,
		crosspost, reactions, skip_first, "limit", repost, repost_expiration, art_channels, nsfw, created_at, updated_at
		FROM guilds WHERE id = $1`, id)

	guild := &store.Guild{}
	if err := scanGuild(row, guild); err != nil {
		return nil, err
	}

	return guild, nil
}

func (g *guildStore) CreateGuild(ctx context.Context, id string) (*store.Guild, error) {
	guild := store.DefaultGuild(id)

	_, err := g.pool.Exec(ctx, `INSERT INTO guilds (id, prefix, pixiv, twitter, deviant, bluesky, tags, flavour_text,
		crosspost, reactions, skip_first, "limit", repost, repost_expiration, art_channels, nsfw, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		guild.ID, guild.Prefix, guild.Pixiv, guild.Twitter, guild.Deviant, guild.Bluesky, guild.Tags, guild.FlavorText,
		guild.Crosspost, guild.Reactions, guild.SkipFirst, guild.Limit, string(guild.Repost), int64(guild.RepostExpiration),
		guild.ArtChannels, guild.NSFW, guild.CreatedAt, guild.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return guild, nil
}

func (g *guildStore) UpdateGuild(ctx context.Context, guild *store.Guild) (*store.Guild, error) {
	guild.UpdatedAt = time.Now().UTC()

	row := g.pool.QueryRow(ctx, `UPDATE guilds SET prefix=$2, pixiv=$3, twitter=$4, deviant=$5, bluesky=$6, tags=$7,
		flavour_text=$8, crosspost=$9, reactions=$10, skip_first=$11, "limit"=$12, repost=$13, repost_expiration=$14,
		art_channels=$15, nsfw=$16, updated_at=$17 WHERE id = $1
		RETURNING id, prefix, pixiv, twitter, deviant, bluesky, tags, flavour_text,
		crosspost, reactions, skip_first, "limit", repost, repost_expiration, art_channels, nsfw, created_at, updated_at`,
		guild.ID, guild.Prefix, guild.Pixiv, guild.Twitter, guild.Deviant, guild.Bluesky, guild.Tags, guild.FlavorText,
		guild.Crosspost, guild.Reactions, guild.SkipFirst, guild.Limit, string(guild.Repost), int64(guild.RepostExpiration),
		guild.ArtChannels, guild.NSFW, guild.UpdatedAt,
	)

	updated := &store.Guild{}
	if err := scanGuild(row, updated); err != nil {
		return nil, err
	}

	return updated, nil
}

func (g *guildStore) AddArtChannels(ctx context.Context, guildID string, channels []string) (*store.Guild, error) {
	row := g.pool.QueryRow(ctx, `UPDATE guilds SET art_channels = (
			SELECT array_agg(DISTINCT ch) FROM unnest(art_channels || $2) AS ch WHERE ch IS NOT NULL
		), updated_at = now()
		WHERE id = $1
		RETURNING id, prefix, pixiv, twitter, deviant, bluesky, tags, flavour_text,
		crosspost, reactions, skip_first, "limit", repost, repost_expiration, art_channels, nsfw, created_at, updated_at`,
		guildID, channels,
	)

	guild := &store.Guild{}
	if err := scanGuild(row, guild); err != nil {
		return nil, err
	}

	return guild, nil
}

func (g *guildStore) DeleteArtChannels(ctx context.Context, guildID string, channels []string) (*store.Guild, error) {
	row := g.pool.QueryRow(ctx, `UPDATE guilds SET art_channels = COALESCE((
			SELECT array_agg(ch) FROM (
				SELECT unnest(art_channels) AS ch EXCEPT SELECT unnest($2::text[])
			) s WHERE ch IS NOT NULL
		), '{}'), updated_at = now()
		WHERE id = $1
		RETURNING id, prefix, pixiv, twitter, deviant, bluesky, tags, flavour_text,
		crosspost, reactions, skip_first, "limit", repost, repost_expiration, art_channels, nsfw, created_at, updated_at`,
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

func scanGuild(row guildRow, guild *store.Guild) error {
	var repost string
	var repostExpiration int64

	if err := row.Scan(&guild.ID, &guild.Prefix, &guild.Pixiv, &guild.Twitter, &guild.Deviant, &guild.Bluesky,
		&guild.Tags, &guild.FlavorText, &guild.Crosspost, &guild.Reactions, &guild.SkipFirst, &guild.Limit,
		&repost, &repostExpiration, &guild.ArtChannels, &guild.NSFW, &guild.CreatedAt, &guild.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("guild not found")
		}

		return fmt.Errorf("failed to scan guild: %w", err)
	}

	guild.Repost = store.GuildRepost(repost)
	guild.RepostExpiration = time.Duration(repostExpiration)

	if guild.ArtChannels == nil {
		guild.ArtChannels = make([]string, 0)
	}

	return nil
}
