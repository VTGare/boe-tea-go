-- Artworks used to be looked up by their exact URL, so the same post was
-- saved again whenever its URL changed: twitter.com became x.com, authors
-- changed their handles, Pixiv URLs gained www. source_key identifies the
-- post itself. The expressions below build the same keys as
-- artworks.SourceKey does from each provider's Match; keep them in sync.
-- Artworks from providers Boe Tea no longer supports keep a NULL key.

ALTER TABLE artworks ADD COLUMN source_key TEXT;

UPDATE artworks SET source_key = COALESCE(
	'twitter:' || (regexp_match(url, '^https?://(?:mobile\.)?(?:(?:fix(?:up|v))?x|(?:[fv]x)?twitter)\.com/[^/?#]+/(?:status/)?(\d+)(?:[/?#]|$)'))[1],
	'pixiv:' || (regexp_match(url, 'https?://(?:www\.)?pixiv\.net/(?:en/)?(?:artworks/|member_illust\.php\?)(?:mode=medium&)?(?:illust_id=)?([0-9]+)', 'i'))[1],
	'deviantart:' || (regexp_match(url, 'https?://(?:www\.)?deviantart\.com/\w.+/art/([\w\-]+)', 'i'))[1],
	'bluesky:' || array_to_string(regexp_match(url, 'https://(?:www\.)?bsky\.app/profile/(\w.+)/post/([\w\-]+)', 'i'), ':')
);

-- The oldest copy of each artwork stays; the others merge into it.
CREATE TEMP TABLE artwork_merges ON COMMIT DROP AS
SELECT old_id, new_id FROM (
	SELECT id AS old_id, min(id) OVER (PARTITION BY source_key) AS new_id
	FROM artworks
	WHERE source_key IS NOT NULL
) copies
WHERE old_id <> new_id;

-- Users who bookmarked several copies keep one bookmark: the one they
-- already have on the oldest copy, or else their earliest.
INSERT INTO bookmarks (user_id, artwork_id, nsfw, created_at)
SELECT DISTINCT ON (b.user_id, m.new_id) b.user_id, m.new_id, b.nsfw, b.created_at
FROM bookmarks b
JOIN artwork_merges m ON m.old_id = b.artwork_id
ORDER BY b.user_id, m.new_id, b.created_at
ON CONFLICT (user_id, artwork_id) DO NOTHING;

DELETE FROM bookmarks b USING artwork_merges m WHERE b.artwork_id = m.old_id;

UPDATE artworks a
SET favourites = counts.n, updated_at = now()
FROM (
	SELECT kept.new_id, count(b.user_id) AS n
	FROM (SELECT DISTINCT new_id FROM artwork_merges) kept
	LEFT JOIN bookmarks b ON b.artwork_id = kept.new_id
	GROUP BY kept.new_id
) counts
WHERE a.id = counts.new_id;

DELETE FROM artworks a USING artwork_merges m WHERE a.id = m.old_id;

CREATE UNIQUE INDEX artworks_source_key_idx ON artworks (source_key);
