package nhentai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNHentaiAPI_FindHentai(t *testing.T) {
	api := &API{}

	tests := []struct {
		name string
		id   string
		want struct {
			Title   string
			ID      int
			MediaID string
		}
		wantErr bool
	}{
		{
			name: "177013",
			id:   "177013",
			want: struct {
				Title   string
				ID      int
				MediaID string
			}{
				Title:   "METAMORPHOSIS",
				ID:      177013,
				MediaID: "987560",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := api.FindHentai(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tt.want.ID, got.ID)
			assert.Equal(t, tt.want.Title, got.Titles.Pretty)
			assert.Equal(t, tt.want.MediaID, got.MediaID)
		})
	}
}
