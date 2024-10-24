package wiki

import (
	"fmt"
	"sweetrpg.com/catalog-api/wiki"
	"testing"
)

func TestWikiPageSaveAndLoad(t *testing.T) {
	text := "This is a sample Page."
	p1 := &wiki.Page{Title: "TestPage", Body: []byte(text)}
	p1.save()
	p2, err := wiki.loadPage("TestPage")
	if err != nil {
		t.Fail()
	}
	body := fmt.Sprintf(string(p2.Body))
	if body != text {
		t.Fail()
	}
}
