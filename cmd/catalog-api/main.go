package main

import (
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"
	"github.com/sweetrpg/catalog-api/server"
	"github.com/sweetrpg/catalog-api/util"
	"log"
	"os"
	"time"
)

func main() {
	logging.Init()

	godotenv.Load(".env")

	sentryDsn, found := os.LookupEnv(constants.SENTRY_DSN)
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
	// r.LoadHTMLGlob("tmpl/*")

	var cache persistence.CacheStore
	redisHost, found := os.LookupEnv(constants.REDIS_HOST)
	if found {
		redisPort := util.GetEnv(constants.REDIS_PORT, "6379")
		cache = persistence.NewRedisCache(fmt.Sprintf("%s:%s", redisHost, redisPort), "", time.Hour)
	} else {
		cache = persistence.NewInMemoryStore(time.Hour)
	}

	database.SetupDatabase()
	defer database.TeardownDatabase()

	server.SetupHandlers(r, cache)

	// http.HandleFunc("/view/", server.MakeHandler(server.ViewHandler))
	// r.GET("/view/:name", server.ViewHandler)
	// http.HandleFunc("/edit/", server.MakeHandler(server.EditHandler))
	// http.HandleFunc("/save/", server.MakeHandler(server.SaveHandler))
	// http.HandleFunc("/", server.MainHandler)
	// r.GET("/", server.MainHandler)

	// log.Fatal(http.ListenAndServe(":8080" /*os.Getenv("ADDR")*/, nil))

	r.Run(util.GetEnv(constants.BIND_ADDRESS, ":8000"))
}
