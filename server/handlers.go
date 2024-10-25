package server

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/wiki"
)

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

// func MainHandler(w http.ResponseWriter, r *http.Request) {
// 	http.Redirect(w, r, "/view/FrontPage", http.StatusFound)
// }

func SetupHandlers(g *gin.Engine) {
	setupLicenseHandlers(g)
	setupStatusHandlers(g)
}

func MainHandler(c *gin.Context) {
	c.Redirect(http.StatusFound, "/view/FrontPage")
}

func MakeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

// func ViewHandler(w http.ResponseWriter, r *http.Request, title string) {
// 	p, err := wiki.LoadPage(title)
// 	if err != nil {
// 		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
// 		return
// 	}
// 	RenderTemplate(w, "view", p)
// }

func ViewHandler(c *gin.Context) {
	title := c.Param("name")
	p, err := wiki.LoadPage(title)
	if err != nil {
		c.Redirect(http.StatusFound, "/edit/"+title)
		return
	}
	c.HTML(http.StatusOK, "view.html", gin.H{
		"Title": p.Title,
		"Body":  p.Body,
	})
}

func EditHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := wiki.LoadPage(title)
	if err != nil {
		p = &wiki.Page{Title: title}
	}
	RenderTemplate(w, "edit", p)
}

func SaveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &wiki.Page{Title: title, Body: []byte(body)}
	err := p.Save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}
