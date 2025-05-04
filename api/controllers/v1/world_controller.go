// package v1 contains version 1 API controllers
package v1

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/medinapdr/world-gen/models"
	"github.com/medinapdr/world-gen/services"
)

// WorldController manages requests related to worlds for API v1
type WorldController struct {
	worldService *services.WorldService
}

// NewWorldController creates a new instance of the controller
func NewWorldController(worldService *services.WorldService) *WorldController {
	return &WorldController{
		worldService: worldService,
	}
}

// RegisterRoutes registers the controller routes in Echo
func (c *WorldController) RegisterRoutes(g *echo.Group) {
	// Add a welcome/info endpoint at the API root
	g.GET("", c.Welcome)

	g.GET("/world", c.GenerateWorld)
	g.GET("/world/:id", c.GetWorldByID)
	g.GET("/worlds", c.SearchWorlds)
	g.GET("/history", c.GetHistory)
}

// @Tags API
// @Summary API v1 welcome page
// @Description Provides information about the API v1 endpoints
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /v1 [get]
func (c *WorldController) Welcome(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"api":     "World Generator API",
		"version": "v1",
		"endpoints": []map[string]string{
			{"path": "/v1/world", "method": "GET", "description": "Generate a new random world"},
			{"path": "/v1/world/{id}", "method": "GET", "description": "Get world by ID"},
			{"path": "/v1/worlds", "method": "GET", "description": "Search for worlds with filters"},
			{"path": "/v1/history", "method": "GET", "description": "Get recently generated worlds history"},
		},
		"documentation": "/swagger/index.html",
	})
}

// @Tags World
// @Summary Generates a new world
// @Description Creates a world with random characteristics based on the chosen theme
// @Produce json
// @Param theme query string false "World theme" Enums(fantasy,sci-fi,post-apocalyptic) default(fantasy)
// @Success 200 {object} models.World
// @Failure 429 {object} map[string]string
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /v1/world [get]
func (c *WorldController) GenerateWorld(ctx echo.Context) error {
	theme := ctx.QueryParam("theme")

	world, err := c.worldService.GenerateWorld(ctx.Request().Context(), theme)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to generate world",
		})
	}

	return ctx.JSON(http.StatusOK, world)
}

// @Tags World
// @Summary Gets a specific world by ID
// @Description Retrieves a world from the database by its ID
// @Produce json
// @Param id path int true "World ID"
// @Success 200 {object} models.World
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /v1/world/{id} [get]
func (c *WorldController) GetWorldByID(ctx echo.Context) error {
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid world ID",
		})
	}

	world, err := c.worldService.GetWorldByID(ctx.Request().Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "world not found" {
			status = http.StatusNotFound
		}
		return ctx.JSON(status, map[string]string{
			"error": err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, world)
}

// @Tags World
// @Summary Search for worlds
// @Description Search for worlds based on various criteria
// @Produce json
// @Param query query string false "Search query (name/description)"
// @Param theme query string false "Filter by theme"
// @Param climate query string false "Filter by climate"
// @Param limit query int false "Limit results" default(10)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} models.PaginatedWorldsResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /v1/worlds [get]
func (c *WorldController) SearchWorlds(ctx echo.Context) error {
	query := ctx.QueryParam("query")
	theme := ctx.QueryParam("theme")
	climate := ctx.QueryParam("climate")

	limit := parseLimitParam(ctx.QueryParam("limit"))
	offset := parseOffsetParam(ctx.QueryParam("offset"))

	worlds, total, err := c.worldService.SearchWorlds(
		ctx.Request().Context(),
		query,
		theme,
		climate,
		limit,
		offset,
	)

	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to search worlds",
		})
	}

	response := models.PaginatedWorldsResponse{
		Data:   worlds,
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}

	return ctx.JSON(http.StatusOK, response)
}

// @Tags World
// @Summary Gets world history
// @Description Retrieves the latest generated worlds (stored in Redis)
// @Produce json
// @Success 200 {array} models.World
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /v1/history [get]
func (c *WorldController) GetHistory(ctx echo.Context) error {
	worlds, err := c.worldService.GetWorldHistory(ctx.Request().Context())
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to retrieve history",
		})
	}

	return ctx.JSON(http.StatusOK, worlds)
}

// Helper functions

// parseID converts ID parameter string to int
func parseID(idParam string) (int, error) {
	return strconv.Atoi(idParam)
}

// parseLimitParam parses and validates the limit parameter
func parseLimitParam(limitStr string) int {
	const defaultLimit = 10
	const maxLimit = 100

	if limitStr == "" {
		return defaultLimit
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return defaultLimit
	}

	if limit > maxLimit {
		return maxLimit // Cap at maximum for safety
	}

	return limit
}

// parseOffsetParam parses and validates the offset parameter
func parseOffsetParam(offsetStr string) int {
	if offsetStr == "" {
		return 0
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		return 0
	}

	return offset
}
