package nozoki

import (
	"testing"
)

var (
	n        *Nozoki
	expected = map[string]NHBook{
		"177013": {
			ID:      177013,
			URL:     "https://nhentai.net/g/177013",
			Titles:  NHTitle{Japanese: "", English: "[ShindoLA] METAMORPHOSIS (Complete) [English]", Pretty: "METAMORPHOSIS"},
			Artists: []string{""},
			Tags:    []string{""},
			Cover:   "",
			Pages:   225,
		},
	}
)

func init() {
	n = NewNozoki()
}

func TestNozoki(t *testing.T) {
	for id, book := range expected {
		res, err := n.GetBook(id)
		if err != nil {
			t.Logf("GetBook(): %v", err)
			t.FailNow()
		}

		switch {
		case res.ID != book.ID:
			t.Errorf("ID mismatch. Expected %v, got %v", book.ID, res.ID)
		case res.Pages != book.Pages:
			t.Errorf("Pages mismatch. Expected %v, got %v", book.Pages, res.Pages)
		}
	}
}
