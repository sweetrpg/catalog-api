package main

import (
	"log"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"
	"github.com/sweetrpg/catalog-api/server"
	"github.com/sweetrpg/catalog-api/util"
)

func main() {
	logging.Init()

	godotenv.Load(".env")

	sentryDsn, found := os.LookupEnv("SENTRY_DSN")
	if found {
		sentry.Init(sentry.ClientOptions{
			Dsn:   sentryDsn,
			Debug: true,
			// Release: "my-project-name@1.0.0",
		})
		defer func() {
			log.Print("Flusing Sentry...")
			sentry.Flush(2 * time.Second)
		}()
	}

	r := gin.Default()
	r.LoadHTMLGlob("tmpl/*")

	database.SetupDatabase()
	defer database.TeardownDatabase()

	server.SetupHandlers(r)

	// http.HandleFunc("/view/", server.MakeHandler(server.ViewHandler))
	// r.GET("/view/:name", server.ViewHandler)
	// http.HandleFunc("/edit/", server.MakeHandler(server.EditHandler))
	// http.HandleFunc("/save/", server.MakeHandler(server.SaveHandler))
	// http.HandleFunc("/", server.MainHandler)
	// r.GET("/", server.MainHandler)

	// log.Fatal(http.ListenAndServe(":8080" /*os.Getenv("ADDR")*/, nil))

	r.Run(util.GetEnv("ADDR", ":8000"))
}
