package controllers

import (
	"github.com/labstack/echo/v4"
	v1 "github.com/medinapdr/world-gen/controllers/v1"
	"github.com/medinapdr/world-gen/services"
)

// APIRouter handles routing requests to the appropriate API version controllers
type APIRouter struct {
	v1WorldController *v1.WorldController
}

// NewAPIRouter creates a new API router
func NewAPIRouter(worldService *services.WorldService) *APIRouter {
	return &APIRouter{
		v1WorldController: v1.NewWorldController(worldService),
	}
}

// RegisterRoutes registers all API version routes in Echo
func (r *APIRouter) RegisterRoutes(e *echo.Echo) {
	v1Group := e.Group("/v1")
	r.v1WorldController.RegisterRoutes(v1Group)
}
