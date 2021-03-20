package nozoki

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/valyala/fasthttp"
)

//Nozoki is an NHentai API client
type Nozoki struct {
	baseURL      string
	cache        []*NHBook
	maxCacheSize int
}

func get(uri string) (*fasthttp.Response, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(uri)
	req.Header.SetMethod("GET")
	err := fasthttp.Do(req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

//NewNozoki creates a new Nozoki instance with default configuration
func NewNozoki() *Nozoki {
	return &Nozoki{"https://nhentai.net", make([]*NHBook, 0), 1024}
}

//GetBook returns a new NHBook struct or an error if status code was not 200
func (n *Nozoki) GetBook(id string) (*NHBook, error) {
	ID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid id, must be an integer")
	}

	if n.maxCacheSize != 0 {
		if found := n.find(ID); found != nil {
			return found, nil
		}
	}

	raw, err := n.getRawBook(ID)
	if err != nil {
		return nil, err
	}

	book, err := raw.toBook()
	if err != nil {
		return nil, err
	}

	if n.maxCacheSize != 0 {
		n.push(book)
	}

	return book, nil
}

func (n *Nozoki) getRawBook(id int) (*rawNHBook, error) {
	resp, err := get(fmt.Sprintf("%v/api/gallery/%v", n.baseURL, id))
	defer fasthttp.ReleaseResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("expected 200, got %v", resp.StatusCode())
	}

	var book rawNHBook
	err = json.Unmarshal(resp.Body(), &book)
	if err != nil {
		return nil, err
	}

	return &book, nil
}

func (n *Nozoki) find(id int) *NHBook {
	for _, book := range n.cache {
		if book.ID == id {
			return book
		}
	}

	return nil
}

func (n *Nozoki) push(book *NHBook) {
	n.cache = append(n.cache, book)
	if len(n.cache) > n.maxCacheSize {
		n.pop()
	}
}

func (n *Nozoki) pop() {
	if len(n.cache) > 0 {
		n.cache = n.cache[1:]
	}
}

//NHBook is a struct that represents an nhentai doujinshi
type NHBook struct {
	ID         int
	URL        string
	Titles     NHTitle
	Artists    []string
	Tags       []string
	Cover      string
	Pages      []string
	PageCount  int
	Favourites int
}
