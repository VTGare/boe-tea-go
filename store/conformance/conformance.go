// Package conformance holds the shared Store adapter-parity specs.
//
// Every backend runs the same miss-contract cases so
// the next adapter-shaped bug is found by the suite instead of in
// production. Host suites must isolate specs (truncate or fresh IDs):
// all cases below use the "conf-" ID prefix and assume an empty store.
//
// Wire it from an integration-tagged suite file:
//
//	var _ = conformance.Specs(func() store.Store { return testStore })
package conformance

import (
	"context"

	"github.com/VTGare/boe-tea-go/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Specs registers the adapter conformance cases against newStore. The
// func is evaluated per spec, so it may return a shared handle.
func Specs(newStore func() store.Store) {
	Describe("Store adapter conformance", func() {
		ctx := context.Background()

		It("reports ErrGuildNotFound for missing guilds", func() {
			_, err := newStore().Guild(ctx, "conf-missing")

			Expect(err).To(MatchError(store.ErrGuildNotFound))
		})

		It("returns the DM guild for empty IDs", func() {
			guild, err := newStore().Guild(ctx, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(guild.Limit).To(Equal(100))
		})

		It("creates guilds idempotently", func() {
			s := newStore()

			first, err := s.CreateGuild(ctx, "conf-g")

			Expect(err).NotTo(HaveOccurred())
			Expect(first.ID).To(Equal("conf-g"))

			second, err := s.CreateGuild(ctx, "conf-g")

			Expect(err).NotTo(HaveOccurred())
			Expect(second.ID).To(Equal("conf-g"))
		})

		It("reports ErrGuildNotFound updating missing guilds", func() {
			_, err := newStore().UpdateGuild(ctx, store.DefaultGuild("conf-missing"))

			Expect(err).To(MatchError(store.ErrGuildNotFound))
		})

		It("manages art channels idempotently and misses loudly", func() {
			s := newStore()

			_, err := s.CreateGuild(ctx, "conf-ch")
			Expect(err).NotTo(HaveOccurred())

			afterAdd, err := s.AddArtChannels(ctx, "conf-ch", []string{"c1", "c2"})

			Expect(err).NotTo(HaveOccurred())
			Expect(afterAdd.ArtChannels).To(HaveLen(2))

			afterDup, err := s.AddArtChannels(ctx, "conf-ch", []string{"c2"})

			Expect(err).NotTo(HaveOccurred())
			Expect(afterDup.ArtChannels).To(HaveLen(2))

			afterDel, err := s.DeleteArtChannels(ctx, "conf-ch", []string{"c3"})

			Expect(err).NotTo(HaveOccurred())
			Expect(afterDel.ArtChannels).To(HaveLen(2))

			_, err = s.AddArtChannels(ctx, "conf-missing", []string{"c1"})

			Expect(err).To(MatchError(store.ErrGuildNotFound))

			_, err = s.DeleteArtChannels(ctx, "conf-missing", []string{"c1"})

			Expect(err).To(MatchError(store.ErrGuildNotFound))
		})

		It("auto-creates users on read", func() {
			s := newStore()

			user, err := s.User(ctx, "conf-u")

			Expect(err).NotTo(HaveOccurred())
			Expect(user.ID).To(Equal("conf-u"))

			again, err := s.User(ctx, "conf-u")

			Expect(err).NotTo(HaveOccurred())
			Expect(again.ID).To(Equal("conf-u"))
		})

		It("reports ErrUserNotFound for crosspost mutations on missing users", func() {
			_, err := newStore().AddCrosspostChannel(ctx, "conf-nobody", "g", "c")

			Expect(err).To(MatchError(store.ErrUserNotFound))
		})

		It("reports ErrArtworkNotFound for missing artwork", func() {
			_, err := newStore().Artwork(ctx, -1, "https://conformance.invalid/missing")

			Expect(err).To(MatchError(store.ErrArtworkNotFound))
		})

		It("creates missing guilds exactly once via GetOrCreateGuild", func() {
			s := newStore()

			guild, created, err := store.GetOrCreateGuild(ctx, s, "conf-goc")

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeTrue())
			Expect(guild.ID).To(Equal("conf-goc"))

			_, created, err = store.GetOrCreateGuild(ctx, s, "conf-goc")

			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeFalse())
		})
	})
}
