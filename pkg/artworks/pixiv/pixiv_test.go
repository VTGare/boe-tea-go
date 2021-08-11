package pixiv

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPixivMatch(t *testing.T) {
	client := &Pixiv{
		regex: regexp.MustCompile(
			`(?i)http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?(?:artworks\/|member_illust\.php\?)(?:mode=medium\&)?(?:illust_id=)?([0-9]+)`,
		),
	}

	tests := []struct {
		name     string
		url      string
		want     string
		wantBool bool
	}{
		{
			name:     "Old format",
			url:      "http://www.pixiv.net/member_illust.php?mode=medium&illust_id=1",
			want:     "1",
			wantBool: true,
		},
		{
			name:     "New format",
			url:      "https://pixiv.net/artworks/1",
			want:     "1",
			wantBool: true,
		},
		{
			name:     "New format EN",
			url:      "https://pixiv.net/en/artworks/1",
			want:     "1",
			wantBool: true,
		},
		{
			name:     "Just host",
			url:      "https://pixiv.net/",
			want:     "",
			wantBool: false,
		},
		{
			name:     "No artworks new",
			url:      "https://pixiv.net/en/",
			want:     "",
			wantBool: false,
		},
		{
			name:     "No ID new",
			url:      "https://pixiv.net/en/artworks/",
			want:     "",
			wantBool: false,
		},
		{
			name:     "Incorrect ID new",
			url:      "https://pixiv.net/en/artworks/whatever",
			want:     "",
			wantBool: false,
		},
		{
			name:     "No ID old",
			url:      "https://pixiv.net/member_illust.php?illust_id=",
			want:     "",
			wantBool: false,
		},
		{
			name:     "Incorrect ID old",
			url:      "https://pixiv.net/member_illust.php?illust_id=lol",
			want:     "",
			wantBool: false,
		},
		{
			name:     "Not artworks",
			url:      "https://pixiv.net/users/1234",
			want:     "",
			wantBool: false,
		},
		{
			name:     "Manga suffix",
			url:      "https://www.pixiv.net/en/artworks/91786642#manga",
			want:     "91786642",
			wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := client.Match(tt.url)
			assert.Equal(t, tt.want, id)
			assert.Equal(t, tt.wantBool, ok)
		})
	}
}
