-- The schema as of 2026-10-01, squashed from migrations 0001-0003. Prod
-- already records version 1, so it never runs there; it only builds new
-- databases. Column order matches prod so schema dumps compare cleanly.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE SEQUENCE artwork_id_seq;

CREATE TABLE guilds (
	id TEXT PRIMARY KEY,
	prefix TEXT NOT NULL DEFAULT 'bt!' CONSTRAINT guilds_prefix_check CHECK (char_length(prefix) BETWEEN 1 AND 5),
	tags BOOLEAN NOT NULL DEFAULT TRUE,
	quotes BOOLEAN NOT NULL DEFAULT TRUE,
	crosspost BOOLEAN NOT NULL DEFAULT TRUE,
	reactions BOOLEAN NOT NULL DEFAULT FALSE,
	skip_first_tweet BOOLEAN NOT NULL DEFAULT TRUE,
	post_limit SMALLINT NOT NULL DEFAULT 10 CONSTRAINT guilds_post_limit_check CHECK (post_limit BETWEEN 1 AND 100),
	repost_mode TEXT NOT NULL DEFAULT 'notify' CONSTRAINT guilds_repost_mode_check CHECK (repost_mode IN ('off', 'notify', 'strict')),
	art_channels TEXT[] NOT NULL DEFAULT '{}',
	nsfw_quotes BOOLEAN NOT NULL DEFAULT TRUE,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	disabled_providers TEXT[] NOT NULL DEFAULT '{}',
	repost_ttl INTERVAL NOT NULL DEFAULT '1 day'
		CONSTRAINT guilds_repost_ttl_check CHECK (repost_ttl BETWEEN interval '1 minute' AND interval '7 days')
);

CREATE TABLE users (
	id TEXT PRIMARY KEY,
	dm BOOLEAN NOT NULL DEFAULT TRUE,
	crosspost BOOLEAN NOT NULL DEFAULT TRUE,
	ignore BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE user_groups (
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	parent TEXT NOT NULL DEFAULT '',
	is_pair BOOLEAN NOT NULL DEFAULT FALSE,
	children TEXT[] NOT NULL DEFAULT '{}',
	PRIMARY KEY (user_id, name)
);

-- source_key identifies the post on its provider (artworks.SourceKey), so
-- a post is found again after its URL changes. NULL for artworks from
-- providers Boe Tea no longer supports.
CREATE TABLE artworks (
	id INTEGER PRIMARY KEY DEFAULT nextval('artwork_id_seq'),
	title TEXT NOT NULL DEFAULT '',
	author TEXT NOT NULL DEFAULT '',
	url TEXT UNIQUE NOT NULL,
	images TEXT[] NOT NULL DEFAULT '{}',
	favourites INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	source_key TEXT
);

CREATE TABLE bookmarks (
	user_id TEXT NOT NULL,
	artwork_id INTEGER NOT NULL,
	nsfw BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (user_id, artwork_id)
);

CREATE UNIQUE INDEX artworks_source_key_idx ON artworks (source_key);
CREATE INDEX artworks_created_at_id_idx ON artworks (created_at DESC, id DESC);
CREATE INDEX artworks_favourites_id_idx ON artworks (favourites DESC, id DESC);
CREATE INDEX artworks_title_trgm_idx ON artworks USING gin (title gin_trgm_ops);
CREATE INDEX artworks_author_trgm_idx ON artworks USING gin (author gin_trgm_ops);
CREATE INDEX bookmarks_user_created_idx ON bookmarks (user_id, created_at);
