package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"github.com/sweetrpg/api-core/tracing"
	apiconstants "github.com/sweetrpg/api-core/constants"
	"github.com/sweetrpg/catalog-api/server"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/common/logging"
	"github.com/sweetrpg/common/util"
	"github.com/sweetrpg/db/database"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func main() {
	logging.Init()

	godotenv.Load(".env")

	sentryDsn, found := os.LookupEnv(apiconstants.SENTRY_DSN)
	if found {
		sentryDebug, _ := strconv.ParseBool(util.GetEnv(apiconstants.SENTRY_DEBUG, "false"))
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              sentryDsn,
			Debug:            sentryDebug,
			AttachStacktrace: true,
			EnableTracing:    true,
			TracesSampleRate: 1.0,
			TracesSampler: sentry.TracesSampler(func(ctx sentry.SamplingContext) float64 {
				if strings.Contains(ctx.Span.Name, "/status/") {
					return 0.0
				}
				return 1.0
			}),
			ProfilesSampleRate: 1.0,
			ServerName:         constants.ServiceName,
		})
		if err != nil {
			logging.Logger.Error("Error while trying to initialize Sentry", "error", err.Error())
		}
		defer func() {
			log.Print("Flushing Sentry...")
			sentry.Flush(2 * time.Second)
		}()
	}

	r := gin.Default()
	// r.LoadHTMLGlob("tmpl/*")

	tracing.SetupTracing(constants.ServiceName)
	defer tracing.TeardownTracing()
	r.Use(otelgin.Middleware(constants.ServiceName))

	// Setup Prometheus metrics
	m := ginmetrics.GetMonitor()
	m.SetMetricPath("/metrics")
	m.SetSlowTime(10)
	m.SetDuration([]float64{0.1, 0.3, 1.2, 5, 10})
	m.Use(r)

	var cache persistence.CacheStore
	redisHost, found := os.LookupEnv(apiconstants.REDIS_HOST)
	if found {
		redisPort := util.GetEnv(apiconstants.REDIS_PORT, "6379")
		// TODO: redisDb := util.GetEnv(apiconstants.REDIS_DB, "0")
		redisPass := os.Getenv(apiconstants.REDIS_PASS)
		cache = persistence.NewRedisCache(fmt.Sprintf("%s:%s", redisHost, redisPort), redisPass, time.Hour)
	} else {
		cache = persistence.NewInMemoryStore(time.Hour)
	}

	database.SetupDatabase()
	defer database.TeardownDatabase()

	server.SetupHandlers(r, cache)

	r.Run(util.GetEnv(apiconstants.BIND_ADDRESS, ":8000"))
}
