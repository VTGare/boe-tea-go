package services

import "github.com/gocolly/colly/v2"

var (
	iqdbURL = "http://iqdb.org/?url="
)

type iqdbResult struct {
	bestMatch         *iqdbSauce
	additionalMatches []*iqdbSauce
}

type iqdbSauce struct {
	URL        string
	Similarity string
	Thumbnail  string
	Tags       string
}

func getIQDB(imageURL string) (*iqdbResult, error) {
	result := &iqdbResult{}
	c := colly.NewCollector()

	c.OnHTML("tbody", func(e *colly.HTMLElement) {
		e.ForEach("tr", func(i int, e *colly.HTMLElement) {
			bestmatch := false
			match := &iqdbSauce{}
			switch i {
			case 0:
				if str := e.ChildText("th"); str == "Your image" || str == "No relevant matches" {
					return
				} else if str == "Best match" {
					bestmatch = true
				}
			case 1:

			}

			if bestmatch {
				result.bestMatch = match
			} else {
				result.additionalMatches = append(result.additionalMatches, match)
			}
		})
	})

	err := c.Visit(iqdbURL + imageURL)
	if err != nil {
		return nil, err
	}

	c.Wait()
	return result, nil
}
