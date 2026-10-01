//go:build integration

package postgres

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/boe-tea-go/store/conformance"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testStore store.Store

func init() {
	conformance.Specs(func() store.Store { return testStore })
}

func dsn() string {
	if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
		return dsn
	}

	return "postgres://boetea:test@127.0.0.1:5433/boetea_test?sslmode=disable"
}

var _ = BeforeSuite(func() {
	var err error

	testStore, err = New(context.Background(), dsn())
	Expect(err).NotTo(HaveOccurred())
	Expect(testStore.Init(context.Background())).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(testStore.Close(context.Background())).To(Succeed())
})

var _ = BeforeEach(func() {
	ps, ok := testStore.(*postgresStore)
	Expect(ok).To(BeTrue())

	_, err := ps.pool.Exec(context.Background(),
		`TRUNCATE bookmarks, user_groups, users, artworks, guilds RESTART IDENTITY CASCADE`)
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Guilds", func() {
	ctx := context.Background()

	It("creates, reads, updates and changes art channels", func() {
		g, err := testStore.CreateGuild(ctx, "g-it")
		Expect(err).NotTo(HaveOccurred())
		Expect(g.ID).To(Equal("g-it"))

		afterAdd, err := testStore.AddArtChannels(ctx, g.ID, []string{"c1", "c2"})
		Expect(err).NotTo(HaveOccurred())
		Expect(afterAdd.ArtChannels).To(HaveLen(2))

		afterDup, err := testStore.AddArtChannels(ctx, g.ID, []string{"c2", "c3"})
		Expect(err).NotTo(HaveOccurred())
		Expect(afterDup.ArtChannels).To(HaveLen(3))

		afterDel, err := testStore.DeleteArtChannels(ctx, g.ID, []string{"c1"})
		Expect(err).NotTo(HaveOccurred())
		Expect(afterDel.ArtChannels).To(HaveLen(2))

		g.Prefix = "zz!"
		_, err = testStore.UpdateGuild(ctx, g)
		Expect(err).NotTo(HaveOccurred())
	})

	It("accepts an empty art channel add on a guild with no channels", func() {
		_, err := testStore.CreateGuild(ctx, "g-empty")
		Expect(err).NotTo(HaveOccurred())

		g, err := testStore.AddArtChannels(ctx, "g-empty", []string{})
		Expect(err).NotTo(HaveOccurred())
		Expect(g.ArtChannels).To(BeEmpty())
	})

	It("keeps art channels in insertion order", func() {
		_, err := testStore.CreateGuild(ctx, "g-order")
		Expect(err).NotTo(HaveOccurred())

		_, err = testStore.AddArtChannels(ctx, "g-order", []string{"c9", "c1", "c5"})
		Expect(err).NotTo(HaveOccurred())

		g, err := testStore.AddArtChannels(ctx, "g-order", []string{"c1", "c3"})
		Expect(err).NotTo(HaveOccurred())
		Expect(g.ArtChannels).To(Equal([]string{"c9", "c1", "c5", "c3"}))

		g, err = testStore.DeleteArtChannels(ctx, "g-order", []string{"c1"})
		Expect(err).NotTo(HaveOccurred())
		Expect(g.ArtChannels).To(Equal([]string{"c9", "c5", "c3"}))

		g, err = testStore.DeleteArtChannels(ctx, "g-order", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(g.ArtChannels).To(Equal([]string{"c9", "c5", "c3"}))
	})

	It("returns the DM guild for empty IDs and errors on missing guilds", func() {
		dm, err := testStore.Guild(ctx, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(dm.Posting.Limit).To(Equal(100))

		_, err = testStore.UpdateGuild(ctx, store.DefaultGuild("missing-it"))
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Users and crossposts", func() {
	ctx := context.Background()

	It("auto-creates users and rejects duplicates", func() {
		u, err := testStore.User(ctx, "u-it")
		Expect(err).NotTo(HaveOccurred())
		Expect(u.ID).To(Equal("u-it"))

		_, err = testStore.CreateUser(ctx, "u-dup")
		Expect(err).NotTo(HaveOccurred())
		_, err = testStore.CreateUser(ctx, "u-dup")
		Expect(err).To(HaveOccurred())
	})

	It("adds crosspost channels once, in order, and removes them", func() {
		_, err := testStore.User(ctx, "u-ch")
		Expect(err).NotTo(HaveOccurred())

		_, err = testStore.CreateCrosspostGroup(ctx, "u-ch", &store.Group{Name: "g", Parent: "p"})
		Expect(err).NotTo(HaveOccurred())

		for _, ch := range []string{"c2", "c1", "c2"} {
			_, err = testStore.AddCrosspostChannel(ctx, "u-ch", "g", ch)
			Expect(err).NotTo(HaveOccurred())
		}

		u, err := testStore.User(ctx, "u-ch")
		Expect(err).NotTo(HaveOccurred())
		Expect(u.Groups).To(HaveLen(1))
		Expect(u.Groups[0].Children).To(Equal([]string{"c2", "c1"}))

		u, err = testStore.DeleteCrosspostChannel(ctx, "u-ch", "g", "c2")
		Expect(err).NotTo(HaveOccurred())
		Expect(u.Groups[0].Children).To(Equal([]string{"c1"}))
	})

	It("manages crosspost groups from creation to deletion", func() {
		_, err := testStore.User(ctx, "u-it")
		Expect(err).NotTo(HaveOccurred())

		_, err = testStore.CreateCrosspostGroup(ctx, "u-it", &store.Group{Name: "g1", Parent: "p1"})
		Expect(err).NotTo(HaveOccurred())

		_, err = testStore.CreateCrosspostGroup(ctx, "u-it", &store.Group{Name: "g2", Parent: "p1"})
		Expect(err).NotTo(HaveOccurred())

		_, err = testStore.AddCrosspostChannel(ctx, "u-it", "g1", "c1")
		Expect(err).NotTo(HaveOccurred())

		loaded, err := testStore.User(ctx, "u-it")
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Groups).To(HaveLen(2))

		_, err = testStore.DeleteCrosspostChannel(ctx, "u-it", "g1", "c1")
		Expect(err).NotTo(HaveOccurred())
		_, err = testStore.EditCrosspostParent(ctx, "u-it", "g1", "p2")
		Expect(err).NotTo(HaveOccurred())
		_, err = testStore.RenameCrosspostGroup(ctx, "u-it", "g1", "g1r")
		Expect(err).NotTo(HaveOccurred())
		_, err = testStore.DeleteCrosspostGroup(ctx, "u-it", "g1r")
		Expect(err).NotTo(HaveOccurred())
		_, err = testStore.DeleteCrosspostGroup(ctx, "u-it", "g2")
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("Artworks", func() {
	ctx := context.Background()

	It("creates and looks up by id and url", func() {
		created, err := testStore.CreateArtwork(ctx, &store.Artwork{
			Title: "Hello", Author: "World",
			URL:    fmt.Sprintf("https://example.com/%d", time.Now().UnixNano()),
			Images: []string{"https://example.com/a.jpg"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(created.ID).NotTo(BeZero())

		_, err = testStore.Artwork(ctx, 0, "")
		Expect(err).To(MatchError(store.ErrArtworkNotFound))

		byID, err := testStore.Artwork(ctx, created.ID, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(byID.URL).To(Equal(created.URL))

		byURL, err := testStore.Artwork(ctx, 0, created.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(byURL.ID).To(Equal(created.ID))

		_, err = testStore.CreateArtwork(ctx, &store.Artwork{Title: "d", Author: "d", URL: created.URL})
		Expect(err).To(HaveOccurred())
	})

	It("pages through tied popularity without overlap or gaps", func() {
		ids := make(map[int]struct{})
		for i := range 7 {
			a, err := testStore.CreateArtwork(ctx, &store.Artwork{
				Title: "tie", Author: "tie",
				URL: fmt.Sprintf("https://example.com/tie%d", i),
			})
			Expect(err).NotTo(HaveOccurred())
			ids[a.ID] = struct{}{}
		}

		seen := make(map[int]struct{})
		for skip := int64(0); skip < 7; skip += 3 {
			page, err := testStore.SearchArtworks(ctx, store.ArtworkFilter{}, store.ArtworkSearchOptions{
				Sort: store.ByPopularity, Order: store.Descending, Limit: 3, Skip: skip,
			})
			Expect(err).NotTo(HaveOccurred())

			for _, a := range page {
				Expect(seen).NotTo(HaveKey(a.ID))
				seen[a.ID] = struct{}{}
			}
		}

		Expect(seen).To(Equal(ids))
	})

	It("searches literal case-insensitive substrings", func() {
		_, err := testStore.CreateArtwork(ctx, &store.Artwork{
			Title:     "Hello World",
			Author:    "Some Author",
			URL:       fmt.Sprintf("https://example.com/s%d", time.Now().UnixNano()),
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		})
		Expect(err).NotTo(HaveOccurred())

		results, err := testStore.SearchArtworks(ctx, store.ArtworkFilter{Query: "hello"})
		Expect(err).NotTo(HaveOccurred())
		Expect(results).NotTo(BeEmpty())

		// % must be literal, not a wildcard
		escaped, err := testStore.SearchArtworks(ctx, store.ArtworkFilter{Query: "%"})
		Expect(err).NotTo(HaveOccurred())
		Expect(escaped).To(BeEmpty())
	})
})

var _ = Describe("Bookmarks", func() {
	ctx := context.Background()

	It("adds, lists, counts and deletes bookmarks and keeps favourite counts in sync", func() {
		art, err := testStore.CreateArtwork(ctx, &store.Artwork{
			Title: "BM", Author: "BM",
			URL: fmt.Sprintf("https://example.com/b%d", time.Now().UnixNano()),
		})
		Expect(err).NotTo(HaveOccurred())

		added, err := testStore.AddBookmark(ctx, &store.Bookmark{UserID: "bm-it", ArtworkID: art.ID})
		Expect(err).NotTo(HaveOccurred())
		Expect(added).To(BeTrue())

		dup, err := testStore.AddBookmark(ctx, &store.Bookmark{UserID: "bm-it", ArtworkID: art.ID})
		Expect(err).NotTo(HaveOccurred())
		Expect(dup).To(BeFalse())

		count, err := testStore.CountBookmarks(ctx, "bm-it")
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(BeEquivalentTo(1))

		reloaded, err := testStore.Artwork(ctx, art.ID, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(reloaded.Favorites).To(Equal(1))

		deleted, err := testStore.DeleteBookmark(ctx, &store.Bookmark{UserID: "bm-it", ArtworkID: art.ID})
		Expect(err).NotTo(HaveOccurred())
		Expect(deleted).To(BeTrue())

		floor, err := testStore.Artwork(ctx, art.ID, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(floor.Favorites).To(Equal(0))
	})
})

var _ = Describe("Guild settings", func() {
	ctx := context.Background()

	It("round-trips grouped settings, providers and repost TTLs", func() {
		g, err := testStore.CreateGuild(ctx, "g-settings")
		Expect(err).NotTo(HaveOccurred())
		Expect(g.Posting.SkipFirstTweet).To(BeTrue())
		Expect(g.Repost).To(Equal(store.Repost{Mode: store.RepostNotify, TTL: 24 * time.Hour}))

		g.Posting.Limit = 25
		g.Posting.Quotes = false
		g.SetProvider("bluesky", false)
		g.Repost = store.Repost{Mode: store.RepostStrict, TTL: 72 * time.Hour}

		updated, err := testStore.UpdateGuild(ctx, g)
		Expect(err).NotTo(HaveOccurred())
		Expect(updated.Posting.Limit).To(Equal(25))
		Expect(updated.Posting.Quotes).To(BeFalse())
		Expect(updated.ProviderEnabled("bluesky")).To(BeFalse())
		Expect(updated.Repost).To(Equal(store.Repost{Mode: store.RepostStrict, TTL: 72 * time.Hour}))
	})

	It("rejects out-of-range settings at the database", func() {
		g, err := testStore.CreateGuild(ctx, "g-bounds")
		Expect(err).NotTo(HaveOccurred())

		g.Posting.Limit = 500
		_, err = testStore.UpdateGuild(ctx, g)
		Expect(err).To(MatchError(ContainSubstring("guilds_post_limit_check")))

		g.Posting.Limit = 10
		g.Repost.TTL = time.Second
		_, err = testStore.UpdateGuild(ctx, g)
		Expect(err).To(MatchError(ContainSubstring("guilds_repost_ttl_check")))
	})
})

// oldSchemaPool returns a pool on a scratch schema holding the database
// as the pre-migration Init created it: old schema, no schema_migrations.
func oldSchemaPool(ctx context.Context) *pgxpool.Pool {
	admin := testStore.(*postgresStore).pool
	_, err := admin.Exec(ctx, `DROP SCHEMA IF EXISTS mig_test CASCADE; CREATE SCHEMA mig_test`)
	Expect(err).NotTo(HaveOccurred())

	cfg, err := pgxpool.ParseConfig(dsn())
	Expect(err).NotTo(HaveOccurred())
	cfg.ConnConfig.RuntimeParams["search_path"] = "mig_test, public"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	Expect(err).NotTo(HaveOccurred())

	DeferCleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, `DROP SCHEMA IF EXISTS mig_test CASCADE`)
	})

	migrations, err := loadMigrations()
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, migrations[0].sql)
	Expect(err).NotTo(HaveOccurred())

	return pool
}

var _ = Describe("Migrating guild settings from the old schema", func() {
	ctx := context.Background()

	var pool *pgxpool.Pool

	BeforeEach(func() {
		pool = oldSchemaPool(ctx)

		_, err := pool.Exec(ctx, `INSERT INTO guilds (id, prefix, pixiv, twitter, deviant, bluesky, tags, flavour_text,
			crosspost, reactions, skip_first, "limit", repost, repost_expiration, art_channels, nsfw, created_at, updated_at)
		VALUES
			('g-default', 'bt!', true, true, true, true, true, true, true, false, false, 10, 'enabled', 86400000000000, '{}', true, now(), now()),
			('g-odd', 'uwu ', false, true, false, true, false, false, false, true, true, 8583838484, 'disabled', 30000000000, '{}', false, now(), now()),
			('g-strict', '!', true, true, true, true, true, true, true, false, true, 0, 'strict', 259200000000000, '{a,b}', true, now(), now())`)
		Expect(err).NotTo(HaveOccurred())

		Expect(migrate(ctx, pool)).To(Succeed())
	})

	guild := func(id string) *store.Guild {
		g, err := (&guildStore{pool: pool}).Guild(ctx, id)
		Expect(err).NotTo(HaveOccurred())
		return g
	}

	It("keeps default guilds equivalent", func() {
		g := guild("g-default")

		Expect(g.Prefix).To(Equal("bt!"))
		Expect(g.Posting).To(Equal(store.Posting{Limit: 10, Tags: true, Crosspost: true, Quotes: true, NSFWQuotes: true}))
		Expect(g.DisabledProviders).To(BeEmpty())
		Expect(g.Repost).To(Equal(store.Repost{Mode: store.RepostNotify, TTL: 24 * time.Hour}))
	})

	It("converts providers, renames, clamps limits and resets bad TTLs", func() {
		g := guild("g-odd")

		Expect(g.Prefix).To(Equal("uwu "))
		Expect(g.Posting).To(Equal(store.Posting{Limit: 100, Reactions: true, SkipFirstTweet: true}))
		Expect(g.DisabledProviders).To(ConsistOf("pixiv", "deviantart"))
		Expect(g.Repost).To(Equal(store.Repost{Mode: store.RepostOff, TTL: 24 * time.Hour}))
	})

	It("keeps strict mode, valid TTLs and art channels", func() {
		g := guild("g-strict")

		Expect(g.Posting.Limit).To(Equal(1))
		Expect(g.Repost).To(Equal(store.Repost{Mode: store.RepostStrict, TTL: 72 * time.Hour}))
		Expect(g.ArtChannels).To(Equal([]string{"a", "b"}))
	})

	It("records versions and is a no-op when run again", func() {
		Expect(migrate(ctx, pool)).To(Succeed())

		var versions []int
		rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
		Expect(err).NotTo(HaveOccurred())
		versions, err = pgx.CollectRows(rows, pgx.RowTo[int])
		Expect(err).NotTo(HaveOccurred())
		Expect(versions).To(Equal([]int{1, 2, 3}))
	})
})

var _ = Describe("Merging duplicate artworks by source key", func() {
	ctx := context.Background()

	var pool *pgxpool.Pool

	BeforeEach(func() {
		pool = oldSchemaPool(ctx)

		// Copies of one tweet under twitter.com, x.com and a renamed handle,
		// one Pixiv post with and without www, and rows no provider matches.
		_, err := pool.Exec(ctx, `INSERT INTO artworks (id, title, author, url, favourites, created_at, updated_at) VALUES
			(1, '', 'artist', 'https://twitter.com/artist/status/100', 2, now(), now()),
			(2, '', 'artist', 'https://x.com/artist/status/100', 2, now(), now()),
			(3, '', 'renamed', 'https://x.com/renamed/status/100', 1, now(), now()),
			(4, 'p', 'p', 'https://pixiv.net/artworks/200', 1, now(), now()),
			(5, 'p', 'p', 'https://www.pixiv.net/en/artworks/200', 0, now(), now()),
			(6, '', '', 'https://www.artstation.com/artwork/wJqLBY', 0, now(), now()),
			(7, '', 'a', 'https://twitter.com/a/status/undefined', 0, now(), now()),
			(8, '', 'solo', 'https://twitter.com/solo/status/300', 1, now(), now())`)
		Expect(err).NotTo(HaveOccurred())

		// u1 already has the oldest copy; u2's earliest bookmark is on a
		// later copy, so that one moves.
		_, err = pool.Exec(ctx, `INSERT INTO bookmarks (user_id, artwork_id, nsfw, created_at) VALUES
			('u1', 1, false, '2024-02-01'),
			('u1', 2, true, '2024-01-01'),
			('u2', 2, true, '2024-03-01'),
			('u2', 3, false, '2024-01-01'),
			('u3', 1, false, '2024-01-01'),
			('u4', 5, true, '2024-01-01'),
			('u5', 8, false, '2024-01-01')`)
		Expect(err).NotTo(HaveOccurred())

		Expect(migrate(ctx, pool)).To(Succeed())
	})

	It("keeps the oldest copy of each artwork with its source key", func() {
		rows, err := pool.Query(ctx, `SELECT id, COALESCE(source_key, '') FROM artworks ORDER BY id`)
		Expect(err).NotTo(HaveOccurred())

		type row struct {
			ID  int
			Key string
		}
		got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
		Expect(err).NotTo(HaveOccurred())

		Expect(got).To(Equal([]row{
			{1, "twitter:100"},
			{4, "pixiv:200"},
			{6, ""},
			{7, ""},
			{8, "twitter:300"},
		}))
	})

	It("moves bookmarks to the kept copy, one per user", func() {
		rows, err := pool.Query(ctx, `SELECT user_id, artwork_id, nsfw, created_at::date::text FROM bookmarks ORDER BY user_id`)
		Expect(err).NotTo(HaveOccurred())

		type row struct {
			User      string
			ArtworkID int
			NSFW      bool
			CreatedAt string
		}
		got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
		Expect(err).NotTo(HaveOccurred())

		Expect(got).To(Equal([]row{
			{"u1", 1, false, "2024-02-01"},
			{"u2", 1, false, "2024-01-01"},
			{"u3", 1, false, "2024-01-01"},
			{"u4", 4, true, "2024-01-01"},
			{"u5", 8, false, "2024-01-01"},
		}))
	})

	It("recounts favourites of merged artworks only", func() {
		rows, err := pool.Query(ctx, `SELECT id, favourites FROM artworks WHERE id IN (1, 4, 8) ORDER BY id`)
		Expect(err).NotTo(HaveOccurred())

		got, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) ([2]int, error) {
			var v [2]int
			err := r.Scan(&v[0], &v[1])
			return v, err
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(got).To(Equal([][2]int{{1, 3}, {4, 1}, {8, 1}}))
	})

	It("rejects a second artwork with the same source key", func() {
		_, err := pool.Exec(ctx, `INSERT INTO artworks (id, url, source_key, created_at, updated_at)
			VALUES (99, 'https://x.com/new/status/100', 'twitter:100', now(), now())`)

		Expect(err).To(MatchError(ContainSubstring("artworks_source_key_idx")))
	})
})
