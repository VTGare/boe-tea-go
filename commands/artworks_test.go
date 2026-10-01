package commands

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseSkipIndices", func() {
	DescribeTable("accepts numbers and ranges separated by spaces or commas",
		func(raw string, want []int) {
			got, err := parseSkipIndices(raw)
			Expect(err).NotTo(HaveOccurred())

			keys := make([]int, 0, len(got))
			for k := range got {
				keys = append(keys, k)
			}

			Expect(keys).To(ConsistOf(want))
		},
		Entry("spaces", "1 3-5", []int{1, 3, 4, 5}),
		Entry("commas with spaces", "1, 3-5", []int{1, 3, 4, 5}),
		Entry("commas only", "1,3-5", []int{1, 3, 4, 5}),
		Entry("empty", "", []int{}),
	)

	It("rejects anything else", func() {
		_, err := parseSkipIndices("1, x")
		Expect(err).To(HaveOccurred())
	})
})
