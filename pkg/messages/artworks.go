package messages

import "fmt"

type ArtworkSearchWarning struct {
	Title       string
	Description string
}

func SearchWarning() *ArtworkSearchWarning {
	return &ArtworkSearchWarning{
		Title:       "⚠ Warning!",
		Description: "Boe Tea's artworks database __may contain not safe for work results__, **there's no good way to filter them.** Use controls below to skip this warning.",
	}
}

func ErrArtworkNotFound(arg string) error {
	return newUserError(
		fmt.Sprintf(
			"Artwork with the following ID or URL `%v` wasn't found.",
			arg,
		),
	)
}

func ErrSearchArtworksNoResults(query string) error {
	return newUserError(
		fmt.Sprintf(
			"No artworks were found using `%v` query.",
			query,
		),
	)
}

func ErrLimitTooHigh(limit int64) error {
	return newUserError(
		fmt.Sprintf(
			"Limit `%v` is too high. Please provide a number up to 100.",
			limit,
		),
	)
}
