package postgres

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeRow implements artworkRow/guildRow by assigning canned values in order.
type fakeRow struct {
	vals []any
	err  error
}

func (f *fakeRow) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}

	if len(dest) != len(f.vals) {
		return fmt.Errorf("fakeRow: %d dests for %d vals", len(dest), len(f.vals))
	}

	for i, d := range dest {
		rv := reflect.ValueOf(d)
		Expect(rv.Kind()).To(Equal(reflect.Ptr))

		v := reflect.ValueOf(f.vals[i])
		if v.IsValid() && v.Type().AssignableTo(rv.Elem().Type()) {
			rv.Elem().Set(v)
		} else if !v.IsValid() {
			rv.Elem().Set(reflect.Zero(rv.Elem().Type()))
		} else {
			return fmt.Errorf("fakeRow: val %d (%v) not assignable to %v", i, v.Type(), rv.Elem().Type())
		}
	}

	return nil
}

var _ = Describe("escapeILIKE", func() {
	DescribeTable(
		"escapes LIKE wildcards",
		func(in, expected string) {
			Expect(escapeILIKE(in)).To(Equal(expected))
		},
		Entry("plain text", "hello world", "hello world"),
		Entry("percent", "100%", `100\%`),
		Entry("underscore", "a_b", `a\_b`),
		Entry("backslash", `a\b`, `a\\b`),
		Entry("all combined", `%_\`, `\%\_\\`),
		Entry("empty", "", ""),
	)
})

var _ = Describe("artworkWhere", func() {
	It("filters by IDs with highest precedence", func() {
		where, args := artworkWhere(store.ArtworkFilter{IDs: []int{1, 2}, URL: "u", Query: "q"})

		Expect(where).To(Equal("WHERE id = ANY($1)"))
		Expect(args).To(HaveLen(1))
	})

	It("filters by URL when no IDs", func() {
		where, args := artworkWhere(store.ArtworkFilter{URL: "https://x/y"})

		Expect(where).To(Equal("WHERE url = $1"))
		Expect(args).To(Equal([]any{"https://x/y"}))
	})

	It("searches title and author with a literal substring", func() {
		where, args := artworkWhere(store.ArtworkFilter{Query: "100%_x"})

		Expect(where).To(ContainSubstring("author ILIKE $1"))
		Expect(where).To(ContainSubstring("title ILIKE $1"))
		Expect(where).To(ContainSubstring("OR"))
		Expect(args).To(Equal([]any{`%100\%\_x%`}))
	})

	It("combines author, title and time with AND", func() {
		where, args := artworkWhere(store.ArtworkFilter{Author: "a", Title: "t", Time: time.Hour})

		Expect(where).To(MatchRegexp(`^WHERE .* AND .* AND .*$`))
		Expect(where).To(ContainSubstring("author ILIKE $1"))
		Expect(where).To(ContainSubstring("title ILIKE $2"))
		Expect(where).To(ContainSubstring("created_at >="))
		Expect(args).To(HaveLen(3))
	})

	It("returns no filter for an empty filter", func() {
		where, args := artworkWhere(store.ArtworkFilter{})

		Expect(where).To(BeEmpty())
		Expect(args).To(BeNil())
	})
})

var _ = Describe("isUniqueViolation", func() {
	It("detects unique violations", func() {
		Expect(isUniqueViolation(&pgconn.PgError{Code: "23505"})).To(BeTrue())
	})

	It("detects wrapped unique violations", func() {
		err := fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505"})

		Expect(isUniqueViolation(err)).To(BeTrue())
	})

	It("rejects other pg errors and plain errors", func() {
		Expect(isUniqueViolation(&pgconn.PgError{Code: "23503"})).To(BeFalse())
		Expect(isUniqueViolation(errors.New("boom"))).To(BeFalse())
		Expect(isUniqueViolation(nil)).To(BeFalse())
	})
})

var _ = Describe("scanArtwork", func() {
	now := time.Now().UTC()

	It("scans a full row", func() {
		a := &store.Artwork{}

		err := scanArtwork(&fakeRow{vals: []any{7, "t", "a", "u", []string{"i"}, 3, now, now}}, a)

		Expect(err).NotTo(HaveOccurred())
		Expect(a.ID).To(Equal(7))
		Expect(a.Images).To(Equal([]string{"i"}))
		Expect(a.Favorites).To(Equal(3))
	})

	It("maps no rows to ErrArtworkNotFound", func() {
		err := scanArtwork(&fakeRow{err: pgx.ErrNoRows}, &store.Artwork{})

		Expect(err).To(MatchError(store.ErrArtworkNotFound))
	})

	It("coerces nil images to empty", func() {
		a := &store.Artwork{}

		err := scanArtwork(&fakeRow{vals: []any{7, "t", "a", "u", []string(nil), 0, now, now}}, a)

		Expect(err).NotTo(HaveOccurred())
		Expect(a.Images).ToNot(BeNil())
		Expect(a.Images).To(BeEmpty())
	})
})

var _ = Describe("scanGuild", func() {
	now := time.Now().UTC()
	vals := []any{
		"g", "bt!", true, true, true, true, true, true, true, false, false, 10,
		"enabled", int64(86400000000000),
		[]string{"c"},
		true, now, now,
	}

	It("scans a full row with repost and duration", func() {
		g := &store.Guild{}

		Expect(scanGuild(&fakeRow{vals: vals}, g)).To(Succeed())
		Expect(g.ID).To(Equal("g"))
		Expect(g.Repost).To(Equal(store.GuildRepostEnabled))
		Expect(g.RepostExpiration).To(Equal(24 * time.Hour))
	})

	It("maps no rows to an error", func() {
		Expect(scanGuild(&fakeRow{err: pgx.ErrNoRows}, &store.Guild{})).To(HaveOccurred())
	})
})
