package geospatial

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Handler handles HTTP requests for geospatial operations
type Handler struct {
	service *Service
	logger  *zap.Logger
}

// NewHandler creates a new geospatial handler
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers all geospatial routes
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	geospatial := router.Group("/geospatial")
	{
		// Project Geometry Endpoints
		geospatial.POST("/projects/:id/geometry", h.UploadProjectGeometry)
		geospatial.GET("/projects/:id/geometry", h.GetProjectGeometry)
		geospatial.GET("/projects/:id/boundary", h.GetProjectBoundary)

		// Spatial Query Endpoints
		geospatial.GET("/projects/nearby", h.FindNearbyProjects)
		geospatial.GET("/projects/within", h.FindProjectsWithinArea)
		geospatial.POST("/analysis/intersect", h.AnalyzeIntersections)

		// Map Endpoints
		geospatial.GET("/maps/static", h.GenerateStaticMap)
		geospatial.GET("/maps/tile/:z/:x/:y", h.GetMapTile)

		// Geofence Endpoints
		geospatial.POST("/geofences", h.CreateGeofence)
		geospatial.GET("/geofences/project/:id", h.CheckProjectGeofences)

		// Administrative Boundary Endpoints
		geospatial.GET("/boundaries/:level", h.GetAdministrativeBoundaries)
	}
}

// ============================================================================
// Endpoint 1: POST /api/v1/geospatial/projects/:id/geometry
// ============================================================================

// UploadProjectGeometry uploads a project boundary
// @Summary Upload project geometry
// @Description Upload and validate GeoJSON geometry for a project
// @Tags Geospatial
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body UploadGeometryRequest true "Geometry data"
// @Success 201 {object} GeometryResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/geospatial/projects/{id}/geometry [post]
func (h *Handler) UploadProjectGeometry(c *gin.Context) {
	// Parse project ID
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		h.logger.Error("invalid project ID", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}

	// Parse request body
	var req UploadGeometryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("invalid request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure project ID matches
	req.ProjectID = projectID

	// Upload geometry
	response, err := h.service.UploadProjectGeometry(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to upload geometry", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// ============================================================================
// Endpoint 2: GET /api/v1/geospatial/projects/:id/geometry
// ============================================================================

// GetProjectGeometry retrieves a project's geometry
// @Summary Get project geometry
// @Description Retrieve geometry data for a project
// @Tags Geospatial
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} GeometryResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/geospatial/projects/{id}/geometry [get]
func (h *Handler) GetProjectGeometry(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}

	response, err := h.service.GetProjectGeometry(c.Request.Context(), projectID)
	if err != nil {
		h.logger.Error("failed to get geometry", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ============================================================================
// Endpoint 3: GET /api/v1/geospatial/projects/:id/boundary
// ============================================================================

// GetProjectBoundary retrieves a project's boundary in various formats
// @Summary Get project boundary
// @Description Get project boundary in GeoJSON, WKT, or KML format
// @Tags Geospatial
// @Produce json
// @Param id path string true "Project ID"
// @Param format query string false "Format (geojson, wkt, kml)" default(geojson)
// @Success 200 {object} GeometryResponse
// @Router /api/v1/geospatial/projects/{id}/boundary [get]
func (h *Handler) GetProjectBoundary(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}

	format := c.DefaultQuery("format", "geojson")

	response, err := h.service.GetProjectGeometry(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// TODO: Convert to requested format
	_ = format // Use format to convert geometry

	c.JSON(http.StatusOK, response)
}

// ============================================================================
// Endpoint 4: GET /api/v1/geospatial/projects/nearby
// ============================================================================

// FindNearbyProjects finds projects within a radius
// @Summary Find nearby projects
// @Description Find projects within specified radius of a point
// @Tags Geospatial
// @Produce json
// @Param lat query number true "Latitude"
// @Param lng query number true "Longitude"
// @Param radius query number true "Radius in meters"
// @Param limit query int false "Result limit" default(50)
// @Success 200 {array} ProjectGeometry
// @Router /api/v1/geospatial/projects/nearby [get]
func (h *Handler) FindNearbyProjects(c *gin.Context) {
	var query NearbyProjectsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		h.logger.Error("invalid query parameters", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	projects, err := h.service.FindNearbyProjects(c.Request.Context(), &query)
	if err != nil {
		h.logger.Error("failed to find nearby projects", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count":    len(projects),
		"projects": projects,
		"query":    query,
	})
}

// ============================================================================
// Endpoint 5: GET /api/v1/geospatial/projects/within
// ============================================================================

// FindProjectsWithinArea finds projects within a bounding box
// @Summary Find projects within area
// @Description Find projects within a bounding box
// @Tags Geospatial
// @Produce json
// @Param min_lat query number true "Minimum latitude"
// @Param max_lat query number true "Maximum latitude"
// @Param min_lng query number true "Minimum longitude"
// @Param max_lng query number true "Maximum longitude"
// @Param limit query int false "Result limit" default(100)
// @Success 200 {array} ProjectGeometry
// @Router /api/v1/geospatial/projects/within [get]
func (h *Handler) FindProjectsWithinArea(c *gin.Context) {
	var query WithinAreaQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		h.logger.Error("invalid query parameters", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	projects, err := h.service.FindProjectsWithinArea(c.Request.Context(), &query)
	if err != nil {
		h.logger.Error("failed to find projects within area", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count":    len(projects),
		"projects": projects,
		"query":    query,
	})
}

// ============================================================================
// Endpoint 6: POST /api/v1/geospatial/analysis/intersect
// ============================================================================

// AnalyzeIntersections analyzes spatial intersections for a project
// @Summary Analyze spatial intersections
// @Description Detect overlapping project boundaries
// @Tags Geospatial
// @Accept json
// @Produce json
// @Param request body IntersectionAnalysisRequest true "Analysis request"
// @Success 200 {object} IntersectionResult
// @Router /api/v1/geospatial/analysis/intersect [post]
func (h *Handler) AnalyzeIntersections(c *gin.Context) {
	var req IntersectionAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.service.AnalyzeIntersections(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to analyze intersections", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ============================================================================
// Endpoint 7: GET /api/v1/geospatial/maps/static
// ============================================================================

// GenerateStaticMap generates a static map image
// @Summary Generate static map
// @Description Generate static map with project overlays
// @Tags Maps
// @Produce json
// @Param center_lat query number true "Center latitude"
// @Param center_lng query number true "Center longitude"
// @Param zoom query int true "Zoom level (1-20)"
// @Param width query int false "Image width" default(800)
// @Param height query int false "Image height" default(600)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/geospatial/maps/static [get]
func (h *Handler) GenerateStaticMap(c *gin.Context) {
	var req StaticMapRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement map generation using Mapbox/Google Maps
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "Static map generation not yet implemented",
		"request": req,
	})
}

// ============================================================================
// Endpoint 8: GET /api/v1/geospatial/maps/tile/:z/:x/:y
// ============================================================================

// GetMapTile retrieves a map tile
// @Summary Get map tile
// @Description Get vector or raster map tile
// @Tags Maps
// @Produce application/octet-stream
// @Param z path int true "Zoom level"
// @Param x path int true "X coordinate"
// @Param y path int true "Y coordinate"
// @Success 200 {file} binary
// @Router /api/v1/geospatial/maps/tile/{z}/{x}/{y} [get]
func (h *Handler) GetMapTile(c *gin.Context) {
	z := c.Param("z")
	x := c.Param("x")
	y := c.Param("y")

	// TODO: Implement tile serving with caching
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Map tile serving not yet implemented",
		"tile":  map[string]string{"z": z, "x": x, "y": y},
	})
}

// ============================================================================
// Endpoint 9: POST /api/v1/geospatial/geofences
// ============================================================================

// CreateGeofence creates a new geofence
// @Summary Create geofence
// @Description Create geographic alert zone
// @Tags Geofencing
// @Accept json
// @Produce json
// @Param request body CreateGeofenceRequest true "Geofence data"
// @Success 201 {object} Geofence
// @Router /api/v1/geospatial/geofences [post]
func (h *Handler) CreateGeofence(c *gin.Context) {
	var req CreateGeofenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	geofence, err := h.service.CreateGeofence(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to create geofence", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, geofence)
}

// ============================================================================
// Endpoint 10: GET /api/v1/geospatial/geofences/project/:id
// ============================================================================

// CheckProjectGeofences checks if a project intersects with any geofences
// @Summary Check project geofences
// @Description Check if project intersects with alert zones
// @Tags Geofencing
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {array} GeofenceCheck
// @Router /api/v1/geospatial/geofences/project/{id} [get]
func (h *Handler) CheckProjectGeofences(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project ID"})
		return
	}

	checks, err := h.service.CheckProjectAgainstGeofences(c.Request.Context(), projectID)
	if err != nil {
		h.logger.Error("failed to check geofences", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"project_id": projectID,
		"checks":     checks,
		"count":      len(checks),
	})
}

// ============================================================================
// Endpoint 11: GET /api/v1/geospatial/boundaries/:level
// ============================================================================

// GetAdministrativeBoundaries retrieves administrative boundaries by level
// @Summary Get administrative boundaries
// @Description Get country/state/county boundaries by admin level
// @Tags Administrative
// @Produce json
// @Param level path int true "Admin level (0=country, 1=state, 2=county)"
// @Success 200 {array} AdministrativeBoundary
// @Router /api/v1/geospatial/boundaries/{level} [get]
func (h *Handler) GetAdministrativeBoundaries(c *gin.Context) {
	levelStr := c.Param("level")
	var level int
	if _, err := fmt.Sscanf(levelStr, "%d", &level); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid admin level"})
		return
	}

	boundaries, err := h.service.GetAdministrativeBoundaries(c.Request.Context(), level)
	if err != nil {
		h.logger.Error("failed to get boundaries", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"admin_level": level,
		"count":       len(boundaries),
		"boundaries":  boundaries,
	})
}

// ============================================================================
// Helper Types
// ============================================================================

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}
