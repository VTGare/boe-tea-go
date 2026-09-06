package bluesky

import (
	"github.com/VTGare/boe-tea-go/artworks"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Render", func() {
	It("returns escaped text and linked tags", func() {
		a := &Artwork{
			AuthorHandle:      "user",
			AuthorDisplayName: "Name",
			Text:              "hello *world*",
			Tags:              []string{"tag1"},
			Images:            []string{"https://example.com/1.png"},
			Likes:             2,
		}

		rendered, err := a.Render()

		Expect(err).NotTo(HaveOccurred())
		Expect(rendered.Title).To(Equal("Name (user)"))
		Expect(rendered.Description).To(Equal(artworks.EscapeMarkdown("hello *world*")))
		Expect(rendered.TagLinkTemplate).To(Equal("[%v](https://bsky.app/hashtag/%v)"))
		Expect(rendered.Fields).To(HaveLen(1))
	})
})
