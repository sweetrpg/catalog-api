package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"
	"github.com/sweetrpg/catalog-api/server"
	"github.com/sweetrpg/catalog-api/tracing"
	"github.com/sweetrpg/catalog-api/util"
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
			log.Print("Flushing Sentry...")
			sentry.Flush(2 * time.Second)
		}()
	}

	r := gin.Default()
	// r.LoadHTMLGlob("tmpl/*")

	tracing.SetupTracing()
	defer tracing.TeardownTracing()

	// get global Monitor object
	m := ginmetrics.GetMonitor()
	m.SetMetricPath("/metrics")
	m.SetSlowTime(10)
	m.SetDuration([]float64{0.1, 0.3, 1.2, 5, 10})
	m.Use(r)

	var cache persistence.CacheStore
	redisHost, found := os.LookupEnv(constants.REDIS_HOST)
	if found {
		redisPort := util.GetEnv(constants.REDIS_PORT, "6379")
		cache = persistence.NewRedisCache(fmt.Sprintf("%s:%s", redisHost, redisPort), constants.REDIS_PASS, time.Hour)
	} else {
		cache = persistence.NewInMemoryStore(time.Hour)
	}

	database.SetupDatabase()
	defer database.TeardownDatabase()

	server.SetupHandlers(r, cache)

	r.Run(util.GetEnv(constants.BIND_ADDRESS, ":8000"))
}
