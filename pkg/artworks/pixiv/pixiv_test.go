package pixiv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPixivMatch(t *testing.T) {
	tests := []struct {
		name     string
		p        Pixiv
		url      string
		want     string
		wantBool bool
	}{
		{
			name:     "Old format",
			p:        Pixiv{},
			url:      "http://www.pixiv.net/member_illust.php?mode=medium&illust_id=1",
			want:     "1",
			wantBool: true,
		},
		{
			name:     "New format",
			p:        Pixiv{},
			url:      "https://pixiv.net/artworks/1",
			want:     "1",
			wantBool: true,
		},
		{
			name:     "New format EN",
			p:        Pixiv{},
			url:      "https://pixiv.net/en/artworks/1",
			want:     "1",
			wantBool: true,
		},
		{
			name:     "Just host",
			p:        Pixiv{},
			url:      "https://pixiv.net/",
			want:     "",
			wantBool: false,
		},
		{
			name:     "No artworks new",
			p:        Pixiv{},
			url:      "https://pixiv.net/en/",
			want:     "",
			wantBool: false,
		},
		{
			name:     "No ID new",
			p:        Pixiv{},
			url:      "https://pixiv.net/en/artworks/",
			want:     "",
			wantBool: false,
		},
		{
			name:     "Incorrect ID new",
			p:        Pixiv{},
			url:      "https://pixiv.net/en/artworks/whatever",
			want:     "",
			wantBool: false,
		},
		{
			name:     "No ID old",
			p:        Pixiv{},
			url:      "https://pixiv.net/member_illust.php?illust_id=",
			want:     "",
			wantBool: false,
		},
		{
			name:     "Incorrect ID old",
			p:        Pixiv{},
			url:      "https://pixiv.net/member_illust.php?illust_id=lol",
			want:     "",
			wantBool: false,
		},
		{
			name:     "Not artworks",
			p:        Pixiv{},
			url:      "https://pixiv.net/users/1234",
			want:     "",
			wantBool: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := tt.p.Match(tt.url)
			assert.Equal(t, tt.want, id)
			assert.Equal(t, tt.wantBool, ok)
		})
	}
}
