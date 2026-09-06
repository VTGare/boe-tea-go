package twitter

import (
	"net/http"
	"net/http/httptest"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("pickVariant", func() {
	sizes := map[string]int64{
		"/high.mp4": 17 * 1024 * 1024,
		"/mid.mp4":  5 * 1024 * 1024,
		"/low.mp4":  512 * 1024,
	}

	var (
		srv *httptest.Server
		fxt *fxTwitter
	)

	BeforeEach(func() {
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if size, ok := sizes[r.URL.Path]; ok {
				w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
			}

			w.WriteHeader(http.StatusOK)
		}))

		fxt = &fxTwitter{client: srv.Client()}
	})

	AfterEach(func() {
		srv.Close()
	})

	variants := func() []fxVideoVariant {
		return []fxVideoVariant{
			{Bitrate: 0, ContentType: "application/x-mpegURL", URL: srv.URL + "/list.m3u8"},
			{Bitrate: 256000, ContentType: "video/mp4", URL: srv.URL + "/low.mp4"},
			{Bitrate: 2176000, ContentType: "video/mp4", URL: srv.URL + "/mid.mp4"},
			{Bitrate: 10368000, ContentType: "video/mp4", URL: srv.URL + "/high.mp4"},
		}
	}

	It("picks the best variant under the limit", func() {
		Expect(fxt.pickVariant("fallback", variants())).To(Equal(srv.URL + "/mid.mp4"))
	})

	It("falls back to the smallest variant when nothing fits", func() {
		onlyHuge := []fxVideoVariant{
			{Bitrate: 10368000, ContentType: "video/mp4", URL: srv.URL + "/high.mp4"},
		}

		Expect(fxt.pickVariant("fallback", onlyHuge)).To(Equal(srv.URL + "/high.mp4"))
	})

	It("falls back to the default URL without mp4 variants", func() {
		playlist := []fxVideoVariant{
			{Bitrate: 0, ContentType: "application/x-mpegURL", URL: srv.URL + "/list.m3u8"},
		}

		Expect(fxt.pickVariant("fallback", playlist)).To(Equal("fallback"))
	})
})
