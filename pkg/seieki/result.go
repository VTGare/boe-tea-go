package seieki

import "strconv"

//Result is a top-level raw SauceNAO API response
type Result struct {
	Header  *TopHeader `json:"header"`
	Results []*Sauce   `json:"results"`
}

func (r *Result) filter() {
	filter := make([]*Sauce, 0)
	for _, source := range r.Results {
		if len(source.Data.URLs) == 0 && source.Data.Source == "" {
			continue
		}
		if source.Data.Title == "" {
			source.Data.Title = "No title"
		}

		for ind, uri := range source.Data.URLs {
			source.Data.URLs[ind] = beautifyPixiv(uri)
		}
		source.Data.Source = beautifyPixiv(source.Data.Source)

		filter = append(filter, source)
	}
	r.Results = filter
}

//FilterLowSimilarity filters results below min
func (r *Result) FilterLowSimilarity(min float64) error {
	filtered := make([]*Sauce, 0)

	for _, v := range r.Results {
		similarity, err := strconv.ParseFloat(v.Header.Similarity, 64)
		if err != nil {
			return err
		}

		if similarity >= min {
			filtered = append(filtered, v)
		}
	}

	r.Results = filtered
	return nil
}
