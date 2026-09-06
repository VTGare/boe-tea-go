package twitter

import (
	"github.com/VTGare/boe-tea-go/artworks"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Render", func() {
	artwork := func() *Artwork {
		return &Artwork{
			FullName: "Name",
			Username: "user",
			Content:  "hello *world*",
			Photos:   []string{"https://example.com/1.png", "https://example.com/2.png"},
			Likes:    3,
			Retweets: 4,
		}
	}

	It("returns photo data with escaped content", func() {
		rendered, err := artwork().Render()

		Expect(err).NotTo(HaveOccurred())
		Expect(rendered.Title).To(Equal("Name (user)"))
		Expect(rendered.Description).To(Equal(artworks.EscapeMarkdown("hello *world*")))
		Expect(rendered.Images).To(HaveLen(2))
		Expect(rendered.Images[0].Preview).To(Equal("https://example.com/1.png"))
		Expect(rendered.Fields).To(HaveLen(2))
		Expect(rendered.Files).To(BeEmpty())
	})

	It("omits zero stat fields", func() {
		a := artwork()
		a.Likes = 0
		a.Retweets = 0

		rendered, err := a.Render()

		Expect(err).NotTo(HaveOccurred())
		Expect(rendered.Fields).To(BeEmpty())
	})

	It("returns the missing-tweet notice unescaped", func() {
		rendered, err := (&Artwork{}).Render()

		Expect(err).NotTo(HaveOccurred())
		Expect(rendered.Title).To(Equal("❎ Tweet doesn't exist."))
		Expect(rendered.Description).To(ContainSubstring("doesn't exist."))
		Expect(rendered.Images).To(BeEmpty())
	})
})
