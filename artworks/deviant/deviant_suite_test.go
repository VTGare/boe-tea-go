package deviant_test

import (
	"testing"

	"github.com/VTGare/boe-tea-go/artworks/deviant"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDeviant(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Deviant Suite")
}

var _ = DescribeTable(
	"Match Deviant URL",
	func(url string, expectedID string, expectedResult bool) {
		provider := deviant.New()

		id, ok := provider.Match(url)
		Expect(id).To(BeEquivalentTo(expectedID))
		Expect(ok).To(BeEquivalentTo(expectedResult))
	},
	Entry("Valid artwork", "https://www.deviantart.com/bengeigerart/art/vt-123", "vt-123", true),
	Entry("Query params", "https://www.deviantart.com/bengeigerart/art/vt-456?iw=234", "vt-456", true),
	Entry("Invalid URL", "https://www.deviantart.com/art/Arbor-Vitae-877183179", "", false),
	Entry("Different domain", "https://www.somethingelse.com/q98e9N", "", false),
)
