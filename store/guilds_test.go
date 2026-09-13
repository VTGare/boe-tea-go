package store

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type guildStub struct {
	Store

	guild     *Guild
	guildErr  error
	created   *Guild
	createErr error
}

func (s *guildStub) Guild(_ context.Context, _ string) (*Guild, error) {
	return s.guild, s.guildErr
}

func (s *guildStub) CreateGuild(_ context.Context, _ string) (*Guild, error) {
	return s.created, s.createErr
}

var _ = Describe("GetOrCreateGuild", func() {
	ctx := context.Background()

	It("returns the existing guild without creating one", func() {
		guild := DefaultGuild("g")
		stub := &guildStub{guild: guild}

		got, created, err := GetOrCreateGuild(ctx, stub, "g")

		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeFalse())
		Expect(got).To(Equal(guild))
	})

	It("creates the guild when it does not exist", func() {
		created := DefaultGuild("g")
		stub := &guildStub{guildErr: ErrGuildNotFound, created: created}

		got, wasCreated, err := GetOrCreateGuild(ctx, stub, "g")

		Expect(err).NotTo(HaveOccurred())
		Expect(wasCreated).To(BeTrue())
		Expect(got).To(Equal(created))
	})

	It("returns lookup errors instead of creating", func() {
		stub := &guildStub{guildErr: errors.New("boom")}

		_, _, err := GetOrCreateGuild(ctx, stub, "g")

		Expect(err).To(HaveOccurred())
	})

	It("returns errors when creating fails", func() {
		stub := &guildStub{guildErr: ErrGuildNotFound, createErr: errors.New("boom")}

		_, _, err := GetOrCreateGuild(ctx, stub, "g")

		Expect(err).To(HaveOccurred())
	})
})
