-- Reshapes guild settings: typed, bounded columns and providers as a
-- list of disabled ones.

-- Posting limit: clamp bad values, then bound it.
ALTER TABLE guilds RENAME COLUMN "limit" TO post_limit;
UPDATE guilds SET post_limit = LEAST(GREATEST(post_limit, 1), 100)
WHERE post_limit NOT BETWEEN 1 AND 100;
ALTER TABLE guilds
	ALTER COLUMN post_limit TYPE SMALLINT,
	ALTER COLUMN post_limit SET DEFAULT 10,
	ADD CONSTRAINT guilds_post_limit_check CHECK (post_limit BETWEEN 1 AND 100);

-- Renames. Footer quotes were the only use of the guild's NSFW flag.
ALTER TABLE guilds RENAME COLUMN skip_first TO skip_first_tweet;
ALTER TABLE guilds RENAME COLUMN flavour_text TO quotes;
ALTER TABLE guilds RENAME COLUMN nsfw TO nsfw_quotes;
ALTER TABLE guilds ALTER COLUMN skip_first_tweet SET DEFAULT TRUE;

-- Providers: one boolean each -> disabled list.
ALTER TABLE guilds ADD COLUMN disabled_providers TEXT[] NOT NULL DEFAULT '{}';
UPDATE guilds SET disabled_providers = array_remove(ARRAY[
	CASE WHEN NOT pixiv THEN 'pixiv' END,
	CASE WHEN NOT twitter THEN 'twitter' END,
	CASE WHEN NOT deviant THEN 'deviantart' END,
	CASE WHEN NOT bluesky THEN 'bluesky' END
], NULL)
WHERE NOT (pixiv AND twitter AND deviant AND bluesky);
ALTER TABLE guilds
	DROP COLUMN pixiv,
	DROP COLUMN twitter,
	DROP COLUMN deviant,
	DROP COLUMN bluesky;

-- Repost mode: enabled -> notify, disabled -> off.
ALTER TABLE guilds RENAME COLUMN repost TO repost_mode;
UPDATE guilds SET repost_mode = CASE repost_mode
	WHEN 'strict' THEN 'strict'
	WHEN 'disabled' THEN 'off'
	ELSE 'notify'
END;
ALTER TABLE guilds
	ALTER COLUMN repost_mode SET DEFAULT 'notify',
	ADD CONSTRAINT guilds_repost_mode_check CHECK (repost_mode IN ('off', 'notify', 'strict'));

-- Repost expiration: nanoseconds -> interval; out-of-range values reset
-- to the default.
ALTER TABLE guilds ADD COLUMN repost_ttl INTERVAL NOT NULL DEFAULT '1 day';
UPDATE guilds SET repost_ttl = make_interval(secs => repost_expiration::double precision / 1e9)
WHERE repost_expiration BETWEEN 60000000000 AND 604800000000000;
ALTER TABLE guilds
	DROP COLUMN repost_expiration,
	ADD CONSTRAINT guilds_repost_ttl_check CHECK (repost_ttl BETWEEN interval '1 minute' AND interval '7 days');

ALTER TABLE guilds
	ALTER COLUMN prefix SET DEFAULT 'bt!',
	ADD CONSTRAINT guilds_prefix_check CHECK (char_length(prefix) BETWEEN 1 AND 5);
