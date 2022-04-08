package flags

import (
	"testing"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/stretchr/testify/assert"
)

func TestFromArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		flags   []FlagType
		want    map[FlagType]interface{}
		wantErr bool
	}{
		{
			name:  "limit 200",
			args:  []string{"limit:200"},
			flags: []FlagType{FlagTypeLimit},
			want: map[FlagType]interface{}{
				FlagTypeLimit: int64(200),
			},
		},
		{
			name:  "limit 200 and during week",
			args:  []string{"limit:200", "during:week"},
			flags: []FlagType{FlagTypeLimit, FlagTypeDuring},
			want: map[FlagType]interface{}{
				FlagTypeLimit:  int64(200),
				FlagTypeDuring: 7 * 24 * time.Hour,
			},
		},
		{
			name:    "limit fail",
			args:    []string{"limit:fsfs"},
			flags:   []FlagType{FlagTypeLimit},
			wantErr: true,
		},
		{
			name:  "order ascending",
			args:  []string{"order:asc"},
			flags: []FlagType{FlagTypeOrder},
			want: map[FlagType]interface{}{
				FlagTypeOrder: store.Ascending,
			},
		},
		{
			name:  "order descending",
			args:  []string{"order:desc"},
			flags: []FlagType{FlagTypeOrder},
			want: map[FlagType]interface{}{
				FlagTypeOrder: store.Descending,
			},
		},
		{
			name:  "sort time",
			args:  []string{"sort:time"},
			flags: []FlagType{FlagTypeSort},
			want: map[FlagType]interface{}{
				FlagTypeSort: store.ByTime,
			},
		},
		{
			name:  "sort favs",
			args:  []string{"sort:favs"},
			flags: []FlagType{FlagTypeSort},
			want: map[FlagType]interface{}{
				FlagTypeSort: store.ByFavourites,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := FromArgs(tt.args, tt.flags...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Equal(t, tt.want, res)
			}
		})
	}
}
