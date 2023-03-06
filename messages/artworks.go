package messages

import "fmt"

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
			"Limit `%v` is too high. Please provide an integer number equal or lower than 100.",
			limit,
		),
	)
}
