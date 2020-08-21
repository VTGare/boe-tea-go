package seieki

//TopHeader is a top-level SauceNAO header
type TopHeader struct {
	ResultsReturned int `json:"results_returned"`
}

//Sauce is a wrap around raw SauceNAO source image result.
type Sauce struct {
	Header *SauceHeader `json:"header"`
	Data   *SauceData   `json:"data"`
}

//SauceHeader is a source image header
type SauceHeader struct {
	Similarity string `json:"similarity"`
	Thumbnail  string `json:"thumbnail"`
}

//SauceData is a raw SauceNAO API source image response.
type SauceData struct {
	URLs       []string    `json:"ext_urls"`
	Title      string      `json:"title"`
	MemberName string      `json:"member_name"`
	Creator    interface{} `json:"creator"`
	Author     string      `json:"author"`
	Source     string      `json:"source"`
}

func (s *Sauce) URL() string {
	if s.Data.Source != "" {
		return s.Data.Source
	}

	return s.Data.URLs[0]
}

func (s *Sauce) Author() string {
	if s.Data.MemberName != "" {
		return s.Data.MemberName
	} else if s.Data.Author != "" {
		return s.Data.Author
	} else if creator, ok := s.Data.Creator.(string); ok && creator != "" {
		return creator
	}

	return "-"
}

func (s *Sauce) Title() string {
	if s.Data.Title != "" {
		return s.Data.Title
	}

	return "No title"
}
