package flags

// func TestFromArgs(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		args    []string
// 		flags   []FlagType
// 		want    map[FlagType]any
// 		wantErr bool
// 	}{
// 		{
// 			name:  "limit 200",
// 			args:  []string{"limit:200"},
// 			flags: []FlagType{FlagTypeLimit},
// 			want: map[FlagType]any{
// 				FlagTypeLimit: int64(200),
// 			},
// 		},
// 		{
// 			name:  "limit 200 and during week",
// 			args:  []string{"limit:200", "during:week"},
// 			flags: []FlagType{FlagTypeLimit, FlagTypeDuring},
// 			want: map[FlagType]any{
// 				FlagTypeLimit:  int64(200),
// 				FlagTypeDuring: 7 * 24 * time.Hour,
// 			},
// 		},
// 		{
// 			name:    "limit fail",
// 			args:    []string{"limit:fsfs"},
// 			flags:   []FlagType{FlagTypeLimit},
// 			wantErr: true,
// 		},
// 		{
// 			name:  "order ascending",
// 			args:  []string{"order:asc"},
// 			flags: []FlagType{FlagTypeOrder},
// 			want: map[FlagType]any{
// 				FlagTypeOrder: store.Ascending,
// 			},
// 		},
// 		{
// 			name:  "order descending",
// 			args:  []string{"order:desc"},
// 			flags: []FlagType{FlagTypeOrder},
// 			want: map[FlagType]any{
// 				FlagTypeOrder: store.Descending,
// 			},
// 		},
// 		{
// 			name:  "sort time",
// 			args:  []string{"sort:time"},
// 			flags: []FlagType{FlagTypeSort},
// 			want: map[FlagType]any{
// 				FlagTypeSort: store.ByTime,
// 			},
// 		},
// 		{
// 			name:  "sort favs",
// 			args:  []string{"sort:favs"},
// 			flags: []FlagType{FlagTypeSort},
// 			want: map[FlagType]any{
// 				FlagTypeSort: store.ByFavourites,
// 			},
// 		},
// 	}
// }
