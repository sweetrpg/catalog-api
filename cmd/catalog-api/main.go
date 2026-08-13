package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	"github.com/grafana/pyroscope-go"
	"github.com/joho/godotenv"
	"github.com/penglongli/gin-metrics/ginmetrics"
	sloggin "github.com/samber/slog-gin"
	actuator "github.com/sinhashubham95/go-actuator"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	apiconstants "github.com/sweetrpg/api-core.go/constants"
	"github.com/sweetrpg/api-core.go/featureflags"
	"github.com/sweetrpg/api-core.go/tracing"
	"github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/docs"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-api/proposedchanges"
	"github.com/sweetrpg/catalog-api/ratelimit"
	"github.com/sweetrpg/catalog-api/readiness"
	"github.com/sweetrpg/catalog-api/server"
	"github.com/sweetrpg/catalog-api/vocabularies"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/common.go/util"
	"github.com/sweetrpg/mongodb.go/database"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	xrate "golang.org/x/time/rate"
)

// redisConnectTimeout bounds startup/readiness pings to Redis so a stalled connection fails
// fast instead of hanging past the caller's own timeout (same rationale as api-core.go's
// healthCheckTimeout for Mongo).
const redisConnectTimeout = 5 * time.Second

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

	logging.Init()

	setupSentry()

	ff := featureflags.New(constants.ServiceName)

	if stopProfiling := setupProfiling(ff); stopProfiling != nil {
		defer stopProfiling()
	}

	httpLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	r := gin.New()
	r.Use(sloggin.New(httpLogger))
	r.Use(gin.Recovery())
	// r.LoadHTMLGlob("tmpl/*")

	setupTracing(r)
	defer tracing.TeardownTracing()

	// CORS
	setupCORS(r)

	// Setup Prometheus metrics
	setupMetrics(r)

	redisPool := setupRedisPool()
	if redisPool != nil {
		defer func() { _ = redisPool.Close() }()
	}
	cache := setupCache(redisPool)
	ttls := cachettl.Load()

	database.SetupDatabase()
	defer database.TeardownDatabase()
	if err := proposedchanges.EnsureIndexes(context.Background()); err != nil {
		logging.Logger.Error("Error while ensuring proposed_changes indexes", "error", err.Error())
	}
	if err := vocabularies.EnsureIndexes(context.Background()); err != nil {
		logging.Logger.Error("Error while ensuring vocabularies indexes", "error", err.Error())
	}
	if err := data.EnsureVolumeVersioningIndexes(context.Background()); err != nil {
		logging.Logger.Error("Error while ensuring volume versioning indexes", "error", err.Error())
	}
	if err := vocabularies.Backfill(context.Background()); err != nil {
		logging.Logger.Error("Error while backfilling vocabularies", "error", err.Error())
	}

	// Actuator
	setupAcuator(r)

	// Swagger
	setupSwagger(r)

	// Add rate limiter
	r.Use(RateLimiter(redisPool))

	authzClient := authz.NewClient(util.GetEnv(constants.AUTH_API_URL, ""))
	assetsClient := assets.NewClient(util.GetEnv(constants.ASSETS_WEB_URL, ""))

	editSessionPool := setupEditSessionPool()
	if editSessionPool != nil {
		defer func() { _ = editSessionPool.Close() }()
	}
	editSessions := editsession.NewStore(editSessionPool)

	server.SetupHandlers(r, cache, ttls, authzClient, assetsClient, editSessions)

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

func setupCORS(r *gin.Engine) {
	logging.Logger.Info("Setting up CORS...")

	origins := util.GetEnv(constants.ALLOWED_ORIGINS, "")
	if origins == "" {
		logging.Logger.Warn("ALLOWED_ORIGINS not set, no cross-origin requests will be allowed")
		return
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Split(origins, ","),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
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
			// ProfilesSampleRate: 1.0,
			ServerName: constants.ServiceName,
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

// setupProfiling starts continuous profiling only when the profiling-enabled
// feature flag evaluates to true, regardless of whether
// PYROSCOPE_SERVER_ADDRESS happens to be set - the flag is the on/off
// control, PYROSCOPE_SERVER_ADDRESS is only the destination. See the
// pyroscope-profiling-flag spec's three scenarios.
func setupProfiling(ff *featureflags.Client) func() {
	logging.Logger.Info("Setting up continuous profiling...")

	if !ff.BoolFlag(context.Background(), constants.ProfilingEnabledFlag, false) {
		logging.Logger.Info("profiling-enabled flag is off, continuous profiling disabled")
		return nil
	}

	serverAddress, found := os.LookupEnv(constants.PYROSCOPE_SERVER_ADDRESS)
	if !found {
		logging.Logger.Warn("profiling-enabled flag is on but PYROSCOPE_SERVER_ADDRESS not set, continuous profiling disabled")
		return nil
	}

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: constants.ServiceName,
		ServerAddress:   serverAddress,
		TenantID:        util.GetEnv(constants.PYROSCOPE_TENANT_ID, ""),
		Tags: map[string]string{
			"env": util.GetEnv(apiconstants.ENV, "dev"),
		},
	})
	if err != nil {
		logging.Logger.Error("Error while trying to initialize continuous profiling", "error", err.Error())
		return nil
	}

	return func() {
		_ = profiler.Stop()
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

// setupRedisPool builds a shared redigo connection pool for both the rate limiter's counters
// and the cache's startup connectivity check, when REDIS_HOST is configured. Returns nil when
// no Redis is configured, so the service runs entirely without an external dependency.
func setupRedisPool() *redis.Pool {
	redisHost, found := os.LookupEnv(apiconstants.REDIS_HOST)
	if !found {
		return nil
	}

	redisPort := util.GetEnv(apiconstants.REDIS_PORT, "6379")
	redisPass := os.Getenv(apiconstants.REDIS_PASS)
	addr := fmt.Sprintf("%s:%s", redisHost, redisPort)

	return &redis.Pool{
		MaxIdle:     5,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr, redis.DialConnectTimeout(redisConnectTimeout))
			if err != nil {
				return nil, err
			}
			if redisPass != "" {
				if _, err := c.Do("AUTH", redisPass); err != nil {
					_ = c.Close()
					return nil, err
				}
			}
			return c, nil
		},
	}
}

// editSessionRedisDB is the Redis DB index reserved for edit sessions on catalog-api's own
// Redis instance (REDIS_HOST/_PORT/_PASS) - no separate host/port config needed, see
// docs/frontend-conventions.md's Redis registry in sweetrpg/platform.
const editSessionRedisDB = 2

// setupEditSessionPool builds a second redigo pool against the same Redis instance as
// setupRedisPool, but selecting editSessionRedisDB - a separate pool (not a shared one with a
// per-command SELECT) so cache/rate-limit connections can never accidentally read/write edit
// session keys or vice versa. Returns nil when no Redis is configured, matching
// setupRedisPool's behavior.
func setupEditSessionPool() *redis.Pool {
	redisHost, found := os.LookupEnv(apiconstants.REDIS_HOST)
	if !found {
		return nil
	}

	redisPort := util.GetEnv(apiconstants.REDIS_PORT, "6379")
	redisPass := os.Getenv(apiconstants.REDIS_PASS)
	addr := fmt.Sprintf("%s:%s", redisHost, redisPort)

	return &redis.Pool{
		MaxIdle:     5,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			dialOpts := []redis.DialOption{
				redis.DialConnectTimeout(redisConnectTimeout),
				redis.DialDatabase(editSessionRedisDB),
			}
			if redisPass != "" {
				// DialPassword, not a manual AUTH after Dial - DialDatabase's SELECT runs
				// during Dial() itself, before any command this func could issue afterward;
				// against a password-protected Redis that SELECT would fail pre-auth. Passing
				// the password as a DialOption lets redigo order AUTH before SELECT internally.
				dialOpts = append(dialOpts, redis.DialPassword(redisPass))
			}
			return redis.Dial("tcp", addr, dialOpts...)
		},
	}
}

// setupCache builds the response cache store and, when REDIS_HOST is configured, registers the
// Redis pool with readiness so /status/health live-pings it on every call (see
// readiness.CacheReady) instead of trusting a boot-time snapshot - a Redis outage that resolves
// on its own then lets readiness recover without a pod restart. Also does one startup ping
// purely to log early if Redis is unreachable at boot; that ping no longer determines
// readiness by itself.
func setupCache(redisPool *redis.Pool) persistence.CacheStore {
	logging.Logger.Info("Setting up query cache...")

	redisHost, found := os.LookupEnv(apiconstants.REDIS_HOST)
	if !found {
		readiness.SetCachePool(nil)
		return persistence.NewInMemoryStore(time.Hour)
	}

	redisPort := util.GetEnv(apiconstants.REDIS_PORT, "6379")
	redisPass := os.Getenv(apiconstants.REDIS_PASS)
	cache := persistence.NewRedisCache(fmt.Sprintf("%s:%s", redisHost, redisPort), redisPass, time.Hour)

	readiness.SetCachePool(redisPool)

	ctx, cancel := context.WithTimeout(context.Background(), redisConnectTimeout)
	defer cancel()
	if err := ratelimit.Ping(ctx, redisPool); err != nil {
		logging.Logger.Error("REDIS_HOST is configured but unreachable at startup; readiness will keep live-checking on each /status/health call",
			"redis_host", redisHost, "error", err.Error())
	}

	return cache
}

func setupTracing(r *gin.Engine) {
	logging.Logger.Info("Setting up tracing...")

	// Teardown is deferred by the caller (main), not here - deferring it in this function
	// would run it as soon as this function returns, shutting down the tracer provider
	// before the server ever serves a request, silently dropping every span.
	tracing.SetupTracing(constants.ServiceName)
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

// rateLimitTierFor groups routes into a looser "cheap" tier (shallow status endpoints) and a
// stricter "standard" tier (everything else), per design.md's decision to start with two tiers
// keyed by route cost rather than one entry per resource.
func rateLimitTierFor(path string) string {
	if strings.HasPrefix(path, "/status/") {
		return "cheap"
	}
	return "standard"
}

// rateLimitClientKey identifies the caller for per-client limiting: API key if the client sent
// one, otherwise client IP. This doesn't introduce a new auth mechanism - it keys off whatever
// identity already exists on the request (see design.md's open question on API-key issuance).
func rateLimitClientKey(c *gin.Context) string {
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		return "key:" + apiKey
	}
	return "ip:" + c.ClientIP()
}

// RateLimiter selects between the new Redis-backed per-client limiter and the legacy
// process-wide token bucket, gated by DISTRIBUTED_RATE_LIMIT_ENABLED so the new limiter can be
// validated in dev before the old one is removed (tasks 4.5-4.6).
func RateLimiter(redisPool *redis.Pool) gin.HandlerFunc {
	distributedEnabled, _ := strconv.ParseBool(util.GetEnv(constants.DISTRIBUTED_RATE_LIMIT_ENABLED, "false"))
	if distributedEnabled && redisPool != nil {
		return distributedRateLimiter(redisPool)
	}

	if distributedEnabled && redisPool == nil {
		logging.Logger.Warn("DISTRIBUTED_RATE_LIMIT_ENABLED is set but REDIS_HOST is not; falling back to the process-wide limiter")
	}

	return globalRateLimiter()
}

// distributedRateLimiter enforces per-client, per-tier limits backed by Redis, consistent
// across replicas. Fails closed (503) if the Redis backend is unreachable, rather than letting
// requests through unlimited during an outage.
func distributedRateLimiter(redisPool *redis.Pool) gin.HandlerFunc {
	limiter := ratelimit.New(redisPool, map[string]ratelimit.Tier{
		"cheap": {
			Limit:  util.GetEnvInt(constants.RATE_LIMIT_CHEAP, 120),
			Window: util.GetEnvInt(constants.RATE_LIMIT_CHEAP_WINDOW, 60),
		},
		"standard": {
			Limit:  util.GetEnvInt(constants.RATE_LIMIT_STANDARD, 30),
			Window: util.GetEnvInt(constants.RATE_LIMIT_STANDARD_WINDOW, 60),
		},
	})

	return func(c *gin.Context) {
		tier := rateLimitTierFor(c.Request.URL.Path)
		clientKey := rateLimitClientKey(c)

		allowed, err := limiter.Allow(c.Request.Context(), clientKey, tier)
		if err != nil {
			logging.Logger.Error("Rate-limit backend unreachable; rejecting request (fail closed)", "error", err.Error())
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, vo.ErrorVO{
				Error:   constants.ErrorRateLimitUnavailable,
				Message: "Rate limiting is temporarily unavailable",
			})
			return
		}
		if !allowed {
			logging.Logger.Warn("Rate limit exceeded", "client", clientKey, "tier", tier, "path", c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, vo.ErrorVO{
				Error:   apiconstants.ErrorRateLimited,
				Message: "Limit exceeded",
			})
			return
		}
		c.Next()
	}
}

// globalRateLimiter is the legacy process-wide token bucket: one shared limit across every
// client and route, effectively N times looser with N replicas since each pod holds its own
// bucket. Kept behind the DISTRIBUTED_RATE_LIMIT_ENABLED toggle until the new limiter is
// validated in dev (task 4.6 removes this once that happens).
func globalRateLimiter() gin.HandlerFunc {
	limiter := xrate.NewLimiter(1, util.GetEnvInt(apiconstants.RATE_LIMIT, 10))

	return func(c *gin.Context) {
		if limiter.Allow() {
			c.Next()
		} else {
			logging.Logger.Warn(fmt.Sprintf("Rate limit exceeded for request: %v", c.Request))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, vo.ErrorVO{
				Error:   apiconstants.ErrorRateLimited,
				Message: "Limit exceeded",
			})
		}
	}
}
