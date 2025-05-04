package main

import (
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/medinapdr/world-gen/config"
	"github.com/medinapdr/world-gen/controllers"
	customMiddleware "github.com/medinapdr/world-gen/middlewares"
	"github.com/medinapdr/world-gen/services"

	_ "github.com/medinapdr/world-gen/docs"
)

// @title World Generator API
// @version 1.0
// @description API for generating fantasy worlds
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
// @BasePath /
// @schemes http https

func apiVersions(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"versions": []map[string]interface{}{
			{
				"version":  "v1",
				"status":   "stable",
				"docs":     "/swagger/index.html",
				"released": "2025-05-04",
			},
		},
		"current_version": "v1",
	})
}

func main() {
	// Initialize random seed
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// Set up configuration
	dbConfig := config.NewDatabaseConfig()
	appConfig := config.NewAppConfig()

	// Connect to database services
	setupDatabaseConnections(dbConfig)
	defer dbConfig.Close()

	// Initialize services
	worldService := services.NewWorldService(dbConfig, appConfig)

	// Create router
	apiRouter := controllers.NewAPIRouter(worldService)

	// Set up and start Echo server
	e := setupEchoServer(dbConfig, appConfig, apiRouter)
	e.Logger.Fatal(e.Start(":8080"))
}

func setupDatabaseConnections(dbConfig *config.DatabaseConfig) {
	if err := dbConfig.ConnectPostgres(); err != nil {
		log.Printf("Warning: Failed to connect to PostgreSQL: %v", err)
	}

	if err := dbConfig.ConnectRedis(); err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v", err)
	}
}

func setupEchoServer(dbConfig *config.DatabaseConfig, appConfig *config.AppConfig, apiRouter *controllers.APIRouter) *echo.Echo {
	e := echo.New()

	// Set up middleware
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	if dbConfig.RedisClient != nil {
		rateLimiter := customMiddleware.NewRateLimiter(dbConfig.RedisClient, appConfig)
		e.Use(rateLimiter.Middleware())
	}

	// Set up routes
	e.GET("/", redirectToV1)
	e.GET("/health", healthCheck)
	e.GET("/swagger/*", echoSwagger.WrapHandler)
	e.GET("/api", apiVersions)

	// Register API routes
	apiRouter.RegisterRoutes(e)

	return e
}

func redirectToV1(c echo.Context) error {
	return c.Redirect(http.StatusMovedPermanently, "/v1")
}

func healthCheck(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}
