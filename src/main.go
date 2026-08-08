package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ariesmaulana/ars-kit/config"
	"github.com/ariesmaulana/ars-kit/database"
	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/ariesmaulana/ars-kit/src/app/workflow"
	appmw "github.com/ariesmaulana/ars-kit/src/middleware"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
)

// @title Monthly Expense API
// @version 1.0
// @description API for managing monthly expenses, todos, and user authentication
// @host localhost:8080
// @BasePath /
// @schemes http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	conf, err := config.InitConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	db, err := database.NewPostgresDB(conf)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	if len(os.Args) < 2 {
		usage("")
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		serve(conf, db)
	case "worker":
		worker(conf, db)
	default:
		usage(os.Args[1])
		os.Exit(2)
	}
}

// buildApp wires the foundation and app modules shared by the serve and worker
// commands: permission, the workflow engine (with its definitions registered
// and installed as the package default for workflow.Register), and the user
// service.
func buildApp(conf *config.Config, db *database.PostgresDB) (*workflow.Engine, user.Service) {
	// foundation modules — work "globally" or are deps for other modules
	permissionStorage := permission.NewStorage(db.Pool)
	permissionService := permission.NewService(permissionStorage)

	// Workflow engine — PostgreSQL-backed multi-step background jobs. Business
	// code enqueues jobs through the package-level workflow.Register; workers
	// poll workflow_job directly and never touch the HTTP layer.
	workflowEngine := workflow.NewEngine(workflow.NewStore(db.Pool), workflow.Config{
		Workers:      conf.WorkflowWorkers,
		PollInterval: time.Duration(conf.WorkflowPollIntervalSec) * time.Second,
		StaleTimeout: time.Duration(conf.WorkflowStaleTimeoutMin) * time.Minute,
		DrainTimeout: time.Duration(conf.WorkflowDrainTimeoutSec) * time.Second,
	})

	// app modules
	userService := user.NewService(user.NewStorage(db.Pool), permissionService)

	// Register workflow definitions that depend on app modules, then install
	// the engine for the package-level workflow.Register.
	workflowEngine.Register(workflow.DemoWorkflow(userService))
	workflow.SetDefault(workflowEngine)

	return workflowEngine, userService
}

// serve runs the HTTP server. Business services enqueue workflow jobs here;
// the worker command executes them.
func serve(conf *config.Config, db *database.PostgresDB) {
	// Initialize Echo
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Security headers
	e.Use(appmw.SecurityHeaders())

	// Limit request body size to 1MB to prevent resource exhaustion
	e.Use(middleware.BodyLimit("1M"))

	// Global rate limiting: 20 req/s per IP with burst of 40
	e.Use(middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      20,
				Burst:     40,
				ExpiresIn: 3 * time.Minute,
			},
		),
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
				"success": false,
				"message": "Too many requests, please try again later",
			})
		},
	}))

	// CORS configuration - security-focused
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{conf.CORSAllowOrigin},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check route
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "alive")
	})

	jwtConfig := user.JWTConfig{
		SecretKey:       conf.JWTSecret,
		ExpirationHours: 24,
		CookieName:      "auth_token",
		CookieDomain:    "",
		CookieSecure:    conf.AppEnv == "production", // Secure cookies in production (HTTPS only)
		CookieHTTPOnly:  true,
	}

	_, userService := buildApp(conf, db)

	jwtService := user.NewJWTService(jwtConfig)
	userHandler := user.NewHandler(userService, jwtService)

	// API v1 group — pass to each domain handler for clean versioning
	v1 := e.Group("/api/v1")
	userHandler.RegisterRoutes(v1)

	// Configure server with timeouts to prevent slow clients from consuming resources
	server := &http.Server{
		Addr:         ":" + conf.Port,
		ReadTimeout:  15 * time.Second, // Time to read request headers and body
		WriteTimeout: 15 * time.Second, // Time to write response
		IdleTimeout:  60 * time.Second, // Keep-alive timeout for idle connections
	}

	if conf.Port == "" {
		server.Addr = ":8080"
	}

	// Start server in a goroutine
	go func() {
		log.Info().Str("address", server.Addr).Msg("Starting server")
		if err := e.StartServer(server); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Wait for an interrupt or SIGTERM to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Graceful shutdown with 10 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited properly")
}

// worker runs the workflow engine workers. Jobs enqueued by the serve process
// (or any other process) are executed here.
func worker(conf *config.Config, db *database.PostgresDB) {
	engine, _ := buildApp(conf, db)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		engine.Run(ctx)
		close(done)
	}()

	log.Info().Int("workers", conf.WorkflowWorkers).Msg("Workflow engine started")

	// Wait for an interrupt or SIGTERM to shut down the engine
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down workflow engine...")

	// Workers stop acquiring new jobs, and the in-flight step is allowed to
	// finish (bounded by DrainTimeout). Anything still 'processing' is
	// reclaimed by the next deployment's stale logic.
	cancel()
	select {
	case <-done:
		log.Info().Msg("Workflow engine drained")
	case <-time.After(time.Duration(conf.WorkflowDrainTimeoutSec) * time.Second):
		log.Warn().Msg("Workflow engine drain timed out; leaving processing jobs to stale reclaim")
	}

	log.Info().Msg("Worker exited properly")
}

func usage(cmd string) {
	if cmd != "" {
		fmt.Fprintf(os.Stderr, "ars-kit: unknown command %q\n", cmd)
	}
	fmt.Fprintln(os.Stderr, "usage: ars-kit <serve|worker>")
	fmt.Fprintln(os.Stderr, "  serve   run the HTTP server (enqueues workflow jobs)")
	fmt.Fprintln(os.Stderr, "  worker  run the workflow engine workers")
}
