package server

import (
	"github.com/sweetrpg/catalog-api/wiki"
	"html/template"
	"net/http"
)

var templates = template.Must(template.ParseFiles("tmpl/view.html", "tmpl/edit.html"))

func RenderTemplate(w http.ResponseWriter, tmpl string, p *wiki.Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
