package wiki

import (
	"os"
)

type Page struct {
	Title string
	Body  []byte
}

func getPagePath(title string) string {
	return "pages/" + title + ".txt"
}

func (p *Page) Save() error {
	filename := getPagePath(p.Title)
	return os.WriteFile(filename, p.Body, 0600)
}

func LoadPage(title string) (*Page, error) {
	filename := getPagePath(title)
	body, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}
