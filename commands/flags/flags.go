package flags

import (
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/store"
)

type FlagType int

const (
	FlagTypeLimit FlagType = iota
	FlagTypeDuring
	FlagTypeSort
	FlagTypeOrder
	FlagTypeMode
)

func FromArgs(args []string, flags ...FlagType) (map[FlagType]any, error) {
	m := make(map[FlagType]any)

	for _, t := range flags {
		for _, arg := range args {
			switch t {
			case FlagTypeLimit:
				if strings.HasPrefix(arg, "limit:") {
					f := strings.TrimPrefix(arg, "limit:")

					limit, err := strconv.ParseInt(f, 10, 64)
					if err != nil {
						return nil, messages.ErrParseInt(f)
					}

					m[FlagTypeLimit] = limit
				}
			case FlagTypeDuring:
				if strings.HasPrefix(arg, "during:") {
					f := strings.TrimPrefix(arg, "during:")

					switch f {
					case "day":
						m[FlagTypeDuring] = 24 * time.Hour
					case "week":
						m[FlagTypeDuring] = 7 * (24 * time.Hour)
					case "month":
						m[FlagTypeDuring] = 31 * (24 * time.Hour)
					}
				}
			case FlagTypeOrder:
				if strings.HasPrefix(arg, "order:") {
					f := strings.TrimPrefix(arg, "order:")

					if f == "asc" || f == "ascending" {
						m[FlagTypeOrder] = store.Ascending
					}

					if f == "desc" || f == "descending" {
						m[FlagTypeOrder] = store.Descending
					}
				}
			case FlagTypeSort:
				if strings.HasPrefix(arg, "sort:") {
					f := strings.TrimPrefix(arg, "sort:")

					if f == "popularity" {
						m[FlagTypeSort] = store.ByPopularity
					}

					if f == "time" {
						m[FlagTypeSort] = store.ByTime
					}

				}
			case FlagTypeMode:
				if strings.HasPrefix(arg, "mode:") {
					f := strings.TrimPrefix(arg, "mode:")

					switch f {
					case "sfw":
						m[FlagTypeMode] = store.BookmarkFilterSafe
					case "nsfw":
						m[FlagTypeMode] = store.BookmarkFilterUnsafe
					case "all":
						m[FlagTypeMode] = store.BookmarkFilterAll
					}
				}
			}
		}
	}

	return m, nil
}
