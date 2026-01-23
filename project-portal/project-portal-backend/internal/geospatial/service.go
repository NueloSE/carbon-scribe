package geospatial

import (
	"context"
	"fmt"
	"time"

	"carbon-scribe/project-portal/project-portal-backend/internal/geospatial/geometry"
	"carbon-scribe/project-portal/project-portal-backend/internal/geospatial/queries"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service provides business logic for geospatial operations
type Service struct {
	repo             *Repository
	processor        *geometry.Processor
	calculator       *geometry.Calculator
	transformer      *geometry.Transformer
	proximityQuery   *queries.ProximityQuery
	intersectionQuery *queries.IntersectionQuery
	logger           *zap.Logger
	db               *gorm.DB
}

// NewService creates a new geospatial service
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{
		repo:             NewRepository(db),
		processor:        geometry.NewProcessor(db),
		calculator:       geometry.NewCalculator(db),
		transformer:      geometry.NewTransformer(db),
		proximityQuery:   queries.NewProximityQuery(db),
		intersectionQuery: queries.NewIntersectionQuery(db),
		logger:           logger,
		db:               db,
	}
}

// ============================================================================
// Project Geometry Operations
// ============================================================================

// UploadProjectGeometry uploads and validates a project geometry
func (s *Service) UploadProjectGeometry(ctx context.Context, req *UploadGeometryRequest) (*GeometryResponse, error) {
	s.logger.Info("uploading project geometry",
		zap.String("project_id", req.ProjectID.String()),
		zap.String("source_type", req.SourceType))

	// 1. Process and validate GeoJSON
	maxVertices := 10000 // From requirements
	processed, err := s.processor.ProcessGeoJSON(req.GeoJSON, maxVertices, req.SimplificationTolerance)
	if err != nil {
		s.logger.Error("failed to process GeoJSON", zap.Error(err))
		return nil, fmt.Errorf("geometry processing failed: %w", err)
	}

	if !processed.IsValid {
		return nil, fmt.Errorf("invalid geometry: %v", processed.ValidationErrors)
	}

	// 2. Check for overlaps with existing projects
	overlaps, err := s.intersectionQuery.CheckOverlapWithGeometry(processed.WKT, &req.ProjectID)
	if err != nil {
		s.logger.Error("failed to check overlaps", zap.Error(err))
		return nil, fmt.Errorf("overlap check failed: %w", err)
	}

	if len(overlaps) > 0 {
		s.logger.Warn("geometry overlaps with existing projects",
			zap.Int("overlap_count", len(overlaps)))
		// Note: We log but don't block - business decision
	}

	// 3. Check if project geometry already exists (update vs create)
	existing, err := s.repo.GetProjectGeometry(req.ProjectID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check existing geometry: %w", err)
	}

	var newGeometry *ProjectGeometry

	if existing != nil {
		// Update existing - create new version
		s.logger.Info("updating existing geometry", zap.Int("current_version", existing.Version))

		newGeometry = &ProjectGeometry{
			ProjectID:               req.ProjectID,
			Geometry:                processed.WKT,
			Centroid:                processed.WKTCentroid,
			BoundingBox:             &processed.WKTBoundingBox,
			AreaHectares:            processed.AreaHectares,
			PerimeterMeters:         &processed.PerimeterMeters,
			IsValid:                 processed.IsValid,
			ValidationErrors:        processed.ValidationErrors,
			SimplificationTolerance: req.SimplificationTolerance,
			SourceType:              req.SourceType,
			SourceFile:              req.SourceFile,
			Version:                 existing.Version + 1,
			PreviousVersionID:       &existing.ID,
			CreatedAt:               time.Now(),
			UpdatedAt:               time.Now(),
		}

		if err := s.repo.CreateProjectGeometry(newGeometry); err != nil {
			return nil, fmt.Errorf("failed to create geometry version: %w", err)
		}
	} else {
		// Create new
		newGeometry = &ProjectGeometry{
			ProjectID:               req.ProjectID,
			Geometry:                processed.WKT,
			Centroid:                processed.WKTCentroid,
			BoundingBox:             &processed.WKTBoundingBox,
			AreaHectares:            processed.AreaHectares,
			PerimeterMeters:         &processed.PerimeterMeters,
			IsValid:                 processed.IsValid,
			ValidationErrors:        processed.ValidationErrors,
			SimplificationTolerance: req.SimplificationTolerance,
			SourceType:              req.SourceType,
			SourceFile:              req.SourceFile,
			Version:                 1,
			CreatedAt:               time.Now(),
			UpdatedAt:               time.Now(),
		}

		if err := s.repo.CreateProjectGeometry(newGeometry); err != nil {
			return nil, fmt.Errorf("failed to create geometry: %w", err)
		}
	}

	s.logger.Info("geometry uploaded successfully",
		zap.String("geometry_id", newGeometry.ID.String()),
		zap.Int("version", newGeometry.Version))

	// 4. Return response
	return &GeometryResponse{
		ID:               newGeometry.ID,
		ProjectID:        newGeometry.ProjectID,
		GeoJSON:          req.GeoJSON, // Return original GeoJSON
		AreaHectares:     newGeometry.AreaHectares,
		PerimeterMeters:  newGeometry.PerimeterMeters,
		IsValid:          newGeometry.IsValid,
		ValidationErrors: newGeometry.ValidationErrors,
		Version:          newGeometry.Version,
		CreatedAt:        newGeometry.CreatedAt,
		UpdatedAt:        newGeometry.UpdatedAt,
	}, nil
}

// GetProjectGeometry retrieves a project's geometry
func (s *Service) GetProjectGeometry(ctx context.Context, projectID uuid.UUID) (*GeometryResponse, error) {
	geom, err := s.repo.GetProjectGeometry(projectID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("geometry not found for project %s", projectID.String())
		}
		return nil, err
	}

	// TODO: Convert WKT back to GeoJSON for response
	return &GeometryResponse{
		ID:               geom.ID,
		ProjectID:        geom.ProjectID,
		AreaHectares:     geom.AreaHectares,
		PerimeterMeters:  geom.PerimeterMeters,
		IsValid:          geom.IsValid,
		ValidationErrors: geom.ValidationErrors,
		Version:          geom.Version,
		CreatedAt:        geom.CreatedAt,
		UpdatedAt:        geom.UpdatedAt,
	}, nil
}

// ============================================================================
// Spatial Queries
// ============================================================================

// FindNearbyProjects finds projects within a radius
func (s *Service) FindNearbyProjects(ctx context.Context, query *NearbyProjectsQuery) ([]ProjectGeometry, error) {
	s.logger.Info("finding nearby projects",
		zap.Float64("lat", query.Latitude),
		zap.Float64("lng", query.Longitude),
		zap.Float64("radius", query.Radius))

	limit := query.Limit
	if limit == 0 {
		limit = 50 // Default limit
	}

	geometries, err := s.repo.FindProjectsWithinRadius(
		query.Latitude,
		query.Longitude,
		query.Radius,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find nearby projects: %w", err)
	}

	s.logger.Info("found nearby projects", zap.Int("count", len(geometries)))
	return geometries, nil
}

// FindProjectsWithinArea finds projects within a bounding box
func (s *Service) FindProjectsWithinArea(ctx context.Context, query *WithinAreaQuery) ([]ProjectGeometry, error) {
	s.logger.Info("finding projects within area",
		zap.Float64("min_lat", query.MinLat),
		zap.Float64("max_lat", query.MaxLat),
		zap.Float64("min_lng", query.MinLng),
		zap.Float64("max_lng", query.MaxLng))

	limit := query.Limit
	if limit == 0 {
		limit = 100 // Default limit
	}

	geometries, err := s.repo.FindProjectsWithinBounds(
		query.MinLat,
		query.MaxLat,
		query.MinLng,
		query.MaxLng,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find projects within area: %w", err)
	}

	s.logger.Info("found projects within area", zap.Int("count", len(geometries)))
	return geometries, nil
}

// AnalyzeIntersections analyzes overlaps for a project
func (s *Service) AnalyzeIntersections(ctx context.Context, req *IntersectionAnalysisRequest) (*IntersectionResult, error) {
	s.logger.Info("analyzing intersections", zap.String("project_id", req.ProjectID.String()))

	overlaps, err := s.intersectionQuery.FindOverlappingProjects(req.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to find overlaps: %w", err)
	}

	// Convert to response format
	intersections := make([]ProjectIntersection, len(overlaps))
	for i, overlap := range overlaps {
		intersections[i] = ProjectIntersection{
			ProjectID:         overlap.ProjectID,
			IntersectionArea:  overlap.IntersectionAreaHa,
			OverlapPercentage: overlap.OverlapPercentage,
		}
	}

	result := &IntersectionResult{
		ProjectID:            req.ProjectID,
		IntersectingProjects: intersections,
		TotalIntersections:   len(intersections),
	}

	s.logger.Info("intersection analysis complete",
		zap.Int("total_intersections", result.TotalIntersections))

	return result, nil
}

// ============================================================================
// Geofence Operations
// ============================================================================

// CreateGeofence creates a new geofence
func (s *Service) CreateGeofence(ctx context.Context, req *CreateGeofenceRequest) (*Geofence, error) {
	s.logger.Info("creating geofence", zap.String("name", req.Name))

	// 1. Process and validate GeoJSON
	processed, err := s.processor.ProcessGeoJSON(req.GeoJSON, 10000, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid geofence geometry: %w", err)
	}

	// 2. Create geofence
	priority := 1
	if req.Priority != nil {
		priority = *req.Priority
	}

	geofence := &Geofence{
		Name:         req.Name,
		Description:  req.Description,
		Geometry:     processed.WKT,
		GeofenceType: req.GeofenceType,
		AlertRules:   req.AlertRules,
		IsActive:     true,
		Priority:     priority,
		Metadata:     req.Metadata,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.CreateGeofence(geofence); err != nil {
		return nil, fmt.Errorf("failed to create geofence: %w", err)
	}

	s.logger.Info("geofence created", zap.String("geofence_id", geofence.ID.String()))
	return geofence, nil
}

// CheckProjectAgainstGeofences checks if a project intersects with any geofences
func (s *Service) CheckProjectAgainstGeofences(ctx context.Context, projectID uuid.UUID) ([]GeofenceCheck, error) {
	s.logger.Info("checking project against geofences", zap.String("project_id", projectID.String()))

	checks, err := s.repo.CheckProjectAgainstGeofences(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to check geofences: %w", err)
	}

	// Log events for geofences that trigger alerts
	for _, check := range checks {
		if check.EventType != "none" {
			event := &GeofenceEvent{
				GeofenceID:     check.GeofenceID,
				ProjectID:      projectID,
				EventType:      check.EventType,
				AlertGenerated: true,
				CreatedAt:      time.Now(),
			}

			if err := s.repo.CreateGeofenceEvent(event); err != nil {
				s.logger.Error("failed to create geofence event", zap.Error(err))
			}
		}
	}

	s.logger.Info("geofence check complete", zap.Int("triggered_count", len(checks)))
	return checks, nil
}

// ============================================================================
// Administrative Boundaries
// ============================================================================

// GetAdministrativeBoundaries retrieves boundaries by level
func (s *Service) GetAdministrativeBoundaries(ctx context.Context, level int) ([]AdministrativeBoundary, error) {
	return s.repo.GetAdministrativeBoundariesByLevel(level)
}

// GetProjectAdministrativeLocation determines which boundaries a project is in
func (s *Service) GetProjectAdministrativeLocation(ctx context.Context, projectID uuid.UUID) ([]queries.AdministrativeBoundaryResult, error) {
	return s.intersectionQuery.GetProjectAdministrativeBoundaries(projectID, nil)
}

// ============================================================================
// Statistics & Analytics
// ============================================================================

// GetProjectsByRegion aggregates projects by administrative region
func (s *Service) GetProjectsByRegion(ctx context.Context, adminLevel int) (map[string]int, error) {
	return s.repo.GetProjectsByRegion(adminLevel)
}

// GetOverlapSummary returns overlap statistics for a project
func (s *Service) GetOverlapSummary(ctx context.Context, projectID uuid.UUID) (*queries.OverlapSummary, error) {
	return s.intersectionQuery.GetOverlapSummary(projectID)
}

// ============================================================================
// Utility Methods
// ============================================================================

// ValidateGeoJSON validates a GeoJSON string without saving
func (s *Service) ValidateGeoJSON(ctx context.Context, geoJSONStr string) (bool, []string, error) {
	processed, err := s.processor.ProcessGeoJSON(geoJSONStr, 10000, nil)
	if err != nil {
		return false, nil, err
	}

	return processed.IsValid, processed.ValidationErrors, nil
}

// DeleteProjectGeometry deletes a project's geometry
func (s *Service) DeleteProjectGeometry(ctx context.Context, projectID uuid.UUID) error {
	s.logger.Info("deleting project geometry", zap.String("project_id", projectID.String()))
	return s.repo.DeleteProjectGeometry(projectID)
}
