package wiki

import (
	"fmt"
	"testing"
)

func TestWikiPageSaveAndLoad(t *testing.T) {
	text := "This is a sample Page."
	p1 := &Page{Title: "TestPage", Body: []byte(text)}
	p1.Save()
	p2, err := LoadPage("TestPage")
	if err != nil {
		t.Fail()
	}
	body := fmt.Sprintf(string(p2.Body))
	if body != text {
		t.Fail()
	}
}
