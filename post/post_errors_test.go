package post

import (
	"errors"
	"net/http"

	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/bwmarrin/discordgo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func restError(status int) error {
	return &discordgo.RESTError{Response: &http.Response{StatusCode: status}}
}

var _ = Describe("Classify", func() {
	It("marks skips as no-perms", func() {
		Expect(classify(sender.ErrSkipped)).To(Equal(KindNoPerms))
	})

	It("marks forbidden responses as no-perms", func() {
		Expect(classify(restError(http.StatusForbidden))).To(Equal(KindNoPerms))
		Expect(classify(restError(http.StatusUnauthorized))).To(Equal(KindNoPerms))
	})

	It("marks everything else transient", func() {
		Expect(classify(restError(http.StatusInternalServerError))).To(Equal(KindTransient))
		Expect(classify(restError(http.StatusTooManyRequests))).To(Equal(KindTransient))
		Expect(classify(errors.New("boom"))).To(Equal(KindTransient))
	})

	It("wraps send failures with their kind and unwraps to the cause", func() {
		cause := restError(http.StatusForbidden)
		err := &Error{Kind: classify(cause), Cause: cause}

		Expect(err.Kind).To(Equal(KindNoPerms))

		var restErr *discordgo.RESTError
		Expect(errors.As(err, &restErr)).To(BeTrue())
	})
})
