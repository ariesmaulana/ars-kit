package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ariesmaulana/ars-kit/config"
	"github.com/ariesmaulana/ars-kit/database"
	"github.com/ariesmaulana/ars-kit/src/app/user"
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

	// Initialize Echo
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

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

	// Initialize user app with JWT

	jwtConfig := user.JWTConfig{
		SecretKey:       conf.JWTSecret,
		ExpirationHours: 24,
		CookieName:      "auth_token",
		CookieDomain:    "",
		CookieSecure:    false, // Set to true in production with HTTPS
		CookieHTTPOnly:  true,
	}

	userStorage := user.NewStorage(db.Pool)
	userService := user.NewService(userStorage)
	jwtService := user.NewJWTService(jwtConfig)
	userHandler := user.NewHandler(userService, jwtService)
	userHandler.RegisterRoutes(e)

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

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
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
