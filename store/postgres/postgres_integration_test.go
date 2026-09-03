//go:build integration

package postgres

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/VTGare/boe-tea-go/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testStore store.Store

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

	It("creates, reads, updates and mutates art channels per channel", func() {
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

	It("returns the DM guild for empty IDs and errors on missing guilds", func() {
		dm, err := testStore.Guild(ctx, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(dm.Limit).To(Equal(100))

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

	It("manages crosspost groups with unique-name semantics", func() {
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

	It("adds, dedups, counts, lists and deletes with favourites accounting", func() {
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
