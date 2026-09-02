package commands

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Source Suite")
}

var _ = Describe("pixivArtworkURL", func() {
	DescribeTable("rewrites pximg.net image URLs to canonical Pixiv artwork URLs",
		func(input, expected string, expectedOK bool) {
			gotURL, gotOK := pixivArtworkURL(input)
			Expect(gotURL).To(Equal(expected))
			Expect(gotOK).To(Equal(expectedOK))
		},
		Entry("pximg original URL",
			"https://i.pximg.net/img-original/img/2025/08/09/00/51/34/133648578_p0.jpg",
			"https://pixiv.net/artworks/133648578", true),
		Entry("pximg original URL without extension (as SauceNAO returns it)",
			"https://i.pximg.net/img-original/img/2025/08/09/00/51/34/133648578",
			"https://pixiv.net/artworks/133648578", true),
		Entry("pximg original URL without page suffix",
			"https://i.pximg.net/img-original/img/2024/01/02/03/04/05/987654321.jpg",
			"https://pixiv.net/artworks/987654321", true),
		Entry("pximg original URL with query string",
			"https://i.pximg.net/img-original/img/2025/08/09/00/51/34/133648578_p0.jpg?token=abc",
			"https://pixiv.net/artworks/133648578", true),
		Entry("pximg http (not https)",
			"http://i.pximg.net/img-original/img/2025/08/09/00/51/34/133648578_p0.jpg",
			"https://pixiv.net/artworks/133648578", true),
		Entry("already canonical pixiv artwork URL passes through",
			"https://pixiv.net/artworks/133648578",
			"https://pixiv.net/artworks/133648578", false),
		Entry("canonical english URL passes through",
			"https://www.pixiv.net/en/artworks/133648578",
			"https://www.pixiv.net/en/artworks/133648578", false),
		Entry("non-pixiv URL unchanged",
			"https://example.com/animegirl.png",
			"https://example.com/animegirl.png", false),
		Entry("empty string",
			"",
			"", false),
	)
})
