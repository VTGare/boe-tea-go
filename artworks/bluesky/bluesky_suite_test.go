package bluesky_test

import (
	"testing"

	"github.com/VTGare/boe-tea-go/artworks/bluesky"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBluesky(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bluesky Suite")
}

var _ = DescribeTable(
	"Match Bluesky URL",
	func(url string, expectedID string, expectedResult bool) {
		provider := bluesky.New()

		id, ok := provider.Match(url)
		Expect(id).To(BeEquivalentTo(expectedID))
		Expect(ok).To(BeEquivalentTo(expectedResult))
	},
	Entry("Valid artwork", "https://bsky.app/profile/profile.bsky.social/post/1234", "profile.bsky.social:1234", true),
	Entry("Query params", "https://bsky.app/profile/profile.bsky.social/post/1234?iw=234", "profile.bsky.social:1234", true),
	Entry("Invalid URL", "https://bsky.app/profile.bsky.social/post/1234", "", false),
	Entry("Different domain", "https://www.somethingelse.com/q98e9N", "", false),
)
