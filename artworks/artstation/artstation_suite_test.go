package artstation_test

import (
	"testing"

	"github.com/VTGare/boe-tea-go/artworks/artstation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestArtstation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Artstation Suite")
}

var _ = DescribeTable(
	"Match Artstation URL",
	func(url string, expectedID string, expectedResult bool) {
		as := artstation.New()

		id, ok := as.Match(url)
		Expect(id).To(BeEquivalentTo(expectedID))
		Expect(ok).To(BeEquivalentTo(expectedResult))
	},
	Entry("Valid artstation artwork", "https://www.artstation.com/artwork/q98e9N", "q98e9N", true),
	Entry("Different domain", "https://www.somethingelse.com/artwork/q98e9N", "", false),
	Entry("Query params", "https://www.artstation.com/artwork/q98e9N?iw=234", "q98e9N", true),
)
