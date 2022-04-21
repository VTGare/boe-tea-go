package nitter_test

import (
	"testing"

	"github.com/VTGare/boe-tea-go/artworks/twitter/nitter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNitter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nitter Suite")
}

var _ = DescribeTable(
	"Match Nitter URL",
	func(url string, expectedID string, expectedResult bool) {
		provider := nitter.New()

		id, ok := provider.Match(url)
		Expect(id).To(BeEquivalentTo(expectedID))
		Expect(ok).To(BeEquivalentTo(expectedResult))
	},
	Entry("Valid artwork", "https://twitter.com/watsonameliaEN/status/1371674594675937282", "1371674594675937282", true),
	Entry("Query params", "https://twitter.com/watsonameliaEN/status/1371674594675937282?param=1", "1371674594675937282", true),
	Entry("Mobile URL", "https://mobile.twitter.com/watsonameliaEN/status/1371674594675937282", "1371674594675937282", true),
	Entry("No username", "https://twitter.com/i/status/1371674594675937282", "1371674594675937282", true),
	Entry("iweb URL", "https://twitter.com/i/web/status/1371674594675937282", "1371674594675937282", true),
	Entry("With photo suffix", "https://twitter.com/i/web/status/1371674594675937282/photo/1", "1371674594675937282", true),
	Entry("Not artwork pixiv URL", "https://pixiv.net/users/123456", "", false),
	Entry("ID with letters", "https://twitter.com/i/web/status/1371674594675937282f", "", false),
	Entry("Different domain", "https://google.com/i/status/123456", "", false),
	Entry("Invalid URL", "efe", "", false),
)
