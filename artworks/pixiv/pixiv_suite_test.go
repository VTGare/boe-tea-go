package pixiv_test

import (
	"testing"

	"github.com/VTGare/boe-tea-go/artworks/pixiv"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPixiv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pixiv Suite")
}

var _ = DescribeTable(
	"Match Pixiv URL",
	func(url string, expectedID string, expectedResult bool) {
		provider := pixiv.New("test.com")

		id, ok := provider.Match(url)
		Expect(id).To(BeEquivalentTo(expectedID))
		Expect(ok).To(BeEquivalentTo(expectedResult))
	},
	Entry("Valid artwork", "https://pixiv.net/artworks/123456", "123456", true),
	Entry("Query params", "https://pixiv.net/artworks/123456?param=1", "123456", true),
	Entry("English URL", "https://pixiv.net/en/artworks/123456", "123456", true),
	Entry("Legacy URL", "https://pixiv.net/member_illust.php?illust_id=123456", "123456", true),
	Entry("Legacy URL with query params", "https://pixiv.net/member_illust.php?illust_id=123456?param=1", "123456", true),
	Entry("Not artwork pixiv URL", "https://pixiv.net/users/123456", "", false),
	Entry("ID with letters", "https://pixiv.net/artworks/qwerty", "", false),
	Entry("Different domain", "https://google.com/artworks/123456", "", false),
)
