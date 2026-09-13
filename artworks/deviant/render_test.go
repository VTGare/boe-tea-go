package deviant

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Render", func() {
	It("shows one page with linked tags", func() {
		a := &Artwork{
			Title:       "Title",
			Author:      &Author{Name: "Author"},
			ImageURL:    "https://example.com/img.png",
			Tags:        []string{"tag1", "tag2"},
			Views:       7,
			Favorites:   8,
			AIGenerated: true,
		}

		rendered, err := a.Render()

		Expect(err).NotTo(HaveOccurred())
		Expect(rendered.Title).To(Equal("Title by Author"))
		Expect(rendered.TagLinkTemplate).To(Equal("[%v](https://www.deviantart.com/tag/%v)"))
		Expect(rendered.Images).To(HaveLen(1))
		Expect(rendered.Fields).To(HaveLen(2))
		Expect(rendered.AIGenerated).To(BeTrue())
	})

	It("still sends without an author", func() {
		rendered, err := (&Artwork{Title: "Title"}).Render()

		Expect(err).NotTo(HaveOccurred())
		Expect(rendered.Title).To(Equal("Title by "))
	})
})
