package saucenaoapi

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"strconv"

	"github.com/VTGare/boe-tea-go/services"
)

//SauceNaoResult is a top-level raw SauceNAO API response
type SauceNaoResult struct {
	Header  *TopHeader `json:"header"`
	Results []*Sauce   `json:"results"`
}

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

var (
	baseSauceNAO = "https://saucenao.com/search.php?db=8191&output_type=2&api_key="
)

func init() {
	apiKey := os.Getenv("SAUCENAO_API")
	if apiKey == "" {
		log.Fatalln("SAUCENAO_API env does not exist")
	}
	baseSauceNAO += apiKey
}

//SearchSauceByURL permorfs a SauceNAO API call and returns its results.
func SearchSauceByURL(image string) (*SauceNaoResult, error) {
	image = url.QueryEscape(image)
	uri := baseSauceNAO + "&url=" + image

	body, err := services.FasthttpGet(uri)
	if err != nil {
		return nil, err
	}

	var res SauceNaoResult
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (s *SauceNaoResult) FilterLowSimilarity(min float64) error {
	filtered := make([]*Sauce, 0)

	for _, v := range s.Results {
		similarity, err := strconv.ParseFloat(v.Header.Similarity, 64)
		if err != nil {
			return err
		}

		if similarity >= min {
			filtered = append(filtered, v)
		}
	}

	s.Results = filtered
	return nil
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
