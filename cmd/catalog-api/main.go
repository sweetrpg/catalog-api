package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/penglongli/gin-metrics/ginmetrics"
	actuator "github.com/sinhashubham95/go-actuator"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	apiconstants "github.com/sweetrpg/api-core/constants"
	"github.com/sweetrpg/api-core/tracing"
	"github.com/sweetrpg/api-core/vo"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/docs"
	"github.com/sweetrpg/catalog-api/server"
	"github.com/sweetrpg/common/logging"
	"github.com/sweetrpg/common/util"
	"github.com/sweetrpg/db/database"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"golang.org/x/time/rate"
)

// @title Catalog API service
// @version 1.0
// @description Swagger APIs
// @termsOfService https://pilgrimagesoftware.com/terms/
// @contact.name API Support
// @contact.url https://sweetrpg.com
// @contact.email admin@sweetrpg.com
// @license.name MIT
// @license.url https://mit-license.org/
func main() {
	_ = godotenv.Load(".env")
	log.Printf("ENV: %v", os.Environ())

	logging.Init()

	setupSentry()

	r := gin.Default()
	// r.LoadHTMLGlob("tmpl/*")

	setupTracing(r)

	// Setup Prometheus metrics
	setupMetrics(r)

	cache := setupCache()

	database.SetupDatabase()
	defer database.TeardownDatabase()

	// Actuator
	setupAcuator(r)

	// Swagger
	setupSwagger(r)

	// Add rate limiter
	r.Use(RateLimiter())

	server.SetupHandlers(r, cache)

	_ = r.Run(util.GetEnv(apiconstants.BIND_ADDRESS, ":8000"))
}

func setupSwagger(r *gin.Engine) {
	logging.Logger.Info("Setting up Swagger...")

	docs.SwaggerInfo.Version = os.Getenv(apiconstants.VERSION)
	docs.SwaggerInfo.Host = util.GetEnv(apiconstants.INGRESS_HOST, "localhost")
	docs.SwaggerInfo.BasePath = util.GetEnv(apiconstants.INGRESS_BASE_PATH, "/")
	docs.SwaggerInfo.Schemes = strings.Split(util.GetEnv(apiconstants.INGRESS_SCHEMES, "http"), ",")
	// swagger middleware to serve the API docs
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
}

func setupSentry() {
	logging.Logger.Info("Setting up Sentry...")

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
}

func setupAcuator(r *gin.Engine) {
	logging.Logger.Info("Setting up actuator...")

	actuatorHandler := actuator.GetActuatorHandler(&actuator.Config{
		Endpoints: []int{
			actuator.Env,
			actuator.Info,
			actuator.Metrics,
			actuator.Ping,
			// actuator.Shutdown,
			actuator.ThreadDump,
		},
		Env:     util.GetEnv(apiconstants.ENV, "dev"),
		Name:    constants.ServiceName,
		Port:    util.GetEnvInt(apiconstants.PORT, 0),
		Version: util.GetEnv(apiconstants.VERSION, "v0.0.0"),
	})
	ginActuatorHandler := func(ctx *gin.Context) {
		actuatorHandler(ctx.Writer, ctx.Request)
	}
	r.GET("/actuator/*endpoint", ginActuatorHandler)
}

func setupCache() persistence.CacheStore {
	logging.Logger.Info("Setting up query cache...")

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

	return cache
}

func setupTracing(r *gin.Engine) {
	logging.Logger.Info("Setting up tracing...")

	tracing.SetupTracing(constants.ServiceName)
	defer tracing.TeardownTracing()
	r.Use(otelgin.Middleware(constants.ServiceName))
}

func setupMetrics(r *gin.Engine) {
	logging.Logger.Info("Setting up metrics endpoint...")

	m := ginmetrics.GetMonitor()
	m.SetMetricPath("/metrics")
	m.SetSlowTime(10)
	m.SetDuration([]float64{0.1, 0.3, 1.2, 5, 10})
	m.Use(r)
}

func RateLimiter() gin.HandlerFunc {
	limiter := rate.NewLimiter(1, util.GetEnvInt(apiconstants.RATE_LIMIT, 10))

	return func(c *gin.Context) {
		if limiter.Allow() {
			c.Next()
		} else {
			logging.Logger.Warn(fmt.Sprintf("Rate limit exceeded for request: %v", c.Request))
			c.JSON(http.StatusTooManyRequests, vo.ErrorVO{
				Error:   apiconstants.ErrorRateLimited,
				Message: "Limit exceeded",
			})
		}
	}
}
