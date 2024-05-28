package twitter_test

import (
	"testing"

	"github.com/VTGare/boe-tea-go/artworks/twitter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTwitter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Twitter Suite")
}

var _ = DescribeTable(
	"Match Twitter URL",
	func(url string, expectedID string, expectedResult bool) {
		provider := twitter.New()

		id, ok := provider.Match(url)
		Expect(id).To(BeEquivalentTo(expectedID))
		Expect(ok).To(BeEquivalentTo(expectedResult))
	},
	Entry("Valid artwork", "https://twitter.com/watsonameliaEN/status/1371674594675937282", "1371674594675937282", true),
	Entry("Query params", "https://twitter.com/watsonameliaEN/status/1371674594675937282?param=1", "1371674594675937282", true),
	Entry("Mobile Twitter URL", "https://mobile.twitter.com/watsonameliaEN/status/1371674594675937282", "1371674594675937282", true),
	Entry("Mobile X URL", "https://mobile.x.com/watsonameliaEN/status/1371674594675937282", "1371674594675937282", true),
	Entry("No username", "https://twitter.com/i/status/1371674594675937282", "1371674594675937282", true),
	Entry("iweb URL", "https://twitter.com/i/web/status/1371674594675937282", "1371674594675937282", true),
	Entry("With photo suffix", "https://twitter.com/i/web/status/1371674594675937282/photo/1", "1371674594675937282", true),
	Entry("Not artwork pixiv URL", "https://pixiv.net/users/123456", "", false),
	Entry("ID with letters", "https://twitter.com/i/web/status/1371674594675937282f", "", false),
	Entry("Different domain", "https://google.com/i/status/123456", "", false),
	Entry("Invalid URL", "efe", "", false),
	Entry("fxtwitter link", "https://fxtwitter.com/i/status/1234", "1234", true),
	Entry("vxtwitter link", "https://vxtwitter.com/i/status/1234", "1234", true),
	Entry("X link", "https://x.com/i/status/1234", "1234", true),
	Entry("fixupx link", "https://fixupx.com/i/status/1234", "1234", true),
	Entry("fixvx link", "https://fixvx.com/i/status/1234", "1234", true),
)
