package commands

import (
	"github.com/VTGare/boe-tea-go/artworks"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("artworks.SourceKey", func() {
	key := func(url string) string {
		for _, p := range realProviders() {
			if k, ok := artworks.SourceKey(p, url); ok {
				return k
			}
		}

		return ""
	}

	DescribeTable("gives every URL form of a post the same key",
		func(want string, urls ...string) {
			for _, url := range urls {
				Expect(key(url)).To(Equal(want), url)
			}
		},
		Entry("twitter", "twitter:1846987139428634858",
			"https://twitter.com/artist/status/1846987139428634858",
			"https://x.com/artist/status/1846987139428634858",
			"https://x.com/renamed_artist/status/1846987139428634858",
			"https://x.com/Artist/status/1846987139428634858/photo/1",
		),
		Entry("pixiv", "pixiv:86341538",
			"https://pixiv.net/artworks/86341538",
			"https://www.pixiv.net/en/artworks/86341538",
		),
		Entry("bluesky", "bluesky:did:plc:abc:3kq",
			"https://bsky.app/profile/did:plc:abc/post/3kq",
		),
	)

	It("has no key for unsupported URLs", func() {
		Expect(key("https://www.artstation.com/artwork/wJqLBY")).To(BeEmpty())
		Expect(key("https://twitter.com/artist/status/undefined")).To(BeEmpty())
	})
})
