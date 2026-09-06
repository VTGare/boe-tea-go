package pixiv

import (
	"github.com/VTGare/boe-tea-go/artworks"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Render", func() {
	artwork := func() *Artwork {
		return &Artwork{
			Title:  "Title",
			Author: "Author",
			Tags:   []string{"tag1"},
			Images: []*Image{
				{Preview: "https://i.pximg.net/p1.png", Original: "https://i.pximg.net/o1.png"},
				{Preview: "https://i.pximg.net/p2.png", Original: "https://i.pximg.net/o2.png"},
			},
			Likes: 5,

			proxy: "https://proxy",
		}
	}

	It("returns paged data with linked tags", func() {
		rendered, err := artwork().Render()

		Expect(err).NotTo(HaveOccurred())
		Expect(rendered.Title).To(Equal("Title by Author"))
		Expect(rendered.Tags).To(Equal([]string{"tag1"}))
		Expect(rendered.TagLinkTemplate).To(Equal("[%v](https://pixiv.net/en/tags/%v/artworks)"))
		Expect(rendered.Images).To(HaveLen(2))
		Expect(rendered.Images[0].Preview).To(Equal("https://proxy/p1.png"))
		Expect(rendered.Images[0].Original).To(Equal("https://proxy/o1.png"))
	})

	It("fails imageless artwork as not found", func() {
		a := artwork()
		a.Images = nil

		_, err := a.Render()

		Expect(err).To(MatchError(artworks.ErrArtworkNotFound))
	})
})
