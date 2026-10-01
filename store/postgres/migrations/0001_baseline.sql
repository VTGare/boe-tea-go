-- Schema as it existed before versioned migrations. Every statement is
-- idempotent, so databases created by the old Init accept it as a no-op.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE SEQUENCE IF NOT EXISTS artwork_id_seq;

CREATE TABLE IF NOT EXISTS guilds (
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
);

CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	dm BOOLEAN NOT NULL DEFAULT TRUE,
	crosspost BOOLEAN NOT NULL DEFAULT TRUE,
	ignore BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS user_groups (
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	parent TEXT NOT NULL DEFAULT '',
	is_pair BOOLEAN NOT NULL DEFAULT FALSE,
	children TEXT[] NOT NULL DEFAULT '{}',
	PRIMARY KEY (user_id, name)
);

CREATE TABLE IF NOT EXISTS artworks (
	id INTEGER PRIMARY KEY DEFAULT nextval('artwork_id_seq'),
	title TEXT NOT NULL DEFAULT '',
	author TEXT NOT NULL DEFAULT '',
	url TEXT UNIQUE NOT NULL,
	images TEXT[] NOT NULL DEFAULT '{}',
	favourites INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS bookmarks (
	user_id TEXT NOT NULL,
	artwork_id INTEGER NOT NULL,
	nsfw BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (user_id, artwork_id)
);

CREATE INDEX IF NOT EXISTS artworks_created_at_id_idx ON artworks (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS artworks_favourites_id_idx ON artworks (favourites DESC, id DESC);
CREATE INDEX IF NOT EXISTS artworks_title_trgm_idx ON artworks USING gin (title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS artworks_author_trgm_idx ON artworks USING gin (author gin_trgm_ops);
CREATE INDEX IF NOT EXISTS bookmarks_user_created_idx ON bookmarks (user_id, created_at);

-- Superseded by the id-tiebroken sort indexes above.
DROP INDEX IF EXISTS artworks_created_at_idx;
DROP INDEX IF EXISTS artworks_favourites_idx;
-- Unused: no query filters bookmarks by artwork_id alone.
DROP INDEX IF EXISTS bookmarks_artwork_idx;
-- Redundant with the (user_id, name) primary key.
DROP INDEX IF EXISTS user_groups_user_idx;
