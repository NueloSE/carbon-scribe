package geospatial

import (
	"fmt"
	"time"

	"carbon-scribe/project-portal/project-portal-backend/pkg/postgis"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Repository handles geospatial data access
type Repository struct {
	db      *gorm.DB
	spatial *postgis.SpatialFunctions
}

// NewRepository creates a new geospatial repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db:      db,
		spatial: postgis.NewSpatialFunctions(db),
	}
}

// ============================================================================
// Project Geometry Operations
// ============================================================================

// CreateProjectGeometry creates a new project geometry record
func (r *Repository) CreateProjectGeometry(geometry *ProjectGeometry) error {
	return r.db.Create(geometry).Error
}

// GetProjectGeometry retrieves a project geometry by project ID
func (r *Repository) GetProjectGeometry(projectID uuid.UUID) (*ProjectGeometry, error) {
	var geometry ProjectGeometry
	err := r.db.Where("project_id = ?", projectID).First(&geometry).Error
	if err != nil {
		return nil, err
	}
	return &geometry, nil
}

// UpdateProjectGeometry updates an existing project geometry
func (r *Repository) UpdateProjectGeometry(geometry *ProjectGeometry) error {
	return r.db.Save(geometry).Error
}

// DeleteProjectGeometry deletes a project geometry
func (r *Repository) DeleteProjectGeometry(projectID uuid.UUID) error {
	return r.db.Where("project_id = ?", projectID).Delete(&ProjectGeometry{}).Error
}

// GetProjectGeometryByID retrieves a project geometry by its ID
func (r *Repository) GetProjectGeometryByID(id uuid.UUID) (*ProjectGeometry, error) {
	var geometry ProjectGeometry
	err := r.db.Where("id = ?", id).First(&geometry).Error
	if err != nil {
		return nil, err
	}
	return &geometry, nil
}

// ListProjectGeometries retrieves all project geometries with pagination
func (r *Repository) ListProjectGeometries(limit, offset int) ([]ProjectGeometry, error) {
	var geometries []ProjectGeometry
	err := r.db.Limit(limit).Offset(offset).Find(&geometries).Error
	return geometries, err
}

// GetProjectGeometryVersionHistory retrieves version history for a project
func (r *Repository) GetProjectGeometryVersionHistory(projectID uuid.UUID) ([]ProjectGeometry, error) {
	var geometries []ProjectGeometry
	err := r.db.Where("project_id = ?", projectID).
		Order("version DESC").
		Find(&geometries).Error
	return geometries, err
}

// ============================================================================
// Spatial Queries
// ============================================================================

// FindProjectsWithinRadius finds projects within a radius of a point
func (r *Repository) FindProjectsWithinRadius(lat, lng, radiusMeters float64, limit int) ([]ProjectGeometry, error) {
	var geometries []ProjectGeometry

	query := `
		SELECT *
		FROM project_geometries
		WHERE ST_DWithin(
			centroid,
			ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
			$3
		)
		ORDER BY ST_Distance(
			centroid,
			ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		)
		LIMIT $4
	`

	err := r.db.Raw(query, lng, lat, radiusMeters, limit).Scan(&geometries).Error
	return geometries, err
}

// FindProjectsWithinBounds finds projects within a bounding box
func (r *Repository) FindProjectsWithinBounds(minLat, maxLat, minLng, maxLng float64, limit int) ([]ProjectGeometry, error) {
	var geometries []ProjectGeometry

	query := `
		SELECT *
		FROM project_geometries
		WHERE ST_Within(
			centroid,
			ST_MakeEnvelope($1, $2, $3, $4, 4326)::geography
		)
		LIMIT $5
	`

	err := r.db.Raw(query, minLng, minLat, maxLng, maxLat, limit).Scan(&geometries).Error
	return geometries, err
}

// FindOverlappingProjects finds projects that overlap with a given geometry
func (r *Repository) FindOverlappingProjects(geometry string, excludeProjectID uuid.UUID) ([]ProjectIntersection, error) {
	var results []ProjectIntersection

	query := `
		SELECT
			project_id,
			ST_Area(ST_Intersection(geometry, $1::geography)) / 10000 as intersection_area_hectares,
			(ST_Area(ST_Intersection(geometry, $1::geography)) / ST_Area(geometry)) * 100 as overlap_percentage
		FROM project_geometries
		WHERE project_id != $2
		AND ST_Intersects(geometry, $1::geography)
		AND ST_Area(ST_Intersection(geometry, $1::geography)) > 0
	`

	err := r.db.Raw(query, geometry, excludeProjectID).Scan(&results).Error
	return results, err
}

// CalculateDistance calculates the distance between two projects
func (r *Repository) CalculateDistance(projectID1, projectID2 uuid.UUID) (decimal.Decimal, error) {
	var distance float64

	query := `
		SELECT ST_Distance(
			(SELECT centroid FROM project_geometries WHERE project_id = $1),
			(SELECT centroid FROM project_geometries WHERE project_id = $2)
		)
	`

	err := r.db.Raw(query, projectID1, projectID2).Scan(&distance).Error
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromFloat(distance), nil
}

// GetProjectsByRegion retrieves projects aggregated by administrative region
func (r *Repository) GetProjectsByRegion(adminLevel int) (map[string]int, error) {
	type Result struct {
		RegionName   string
		ProjectCount int
	}

	var results []Result

	query := `
		SELECT
			ab.name as region_name,
			COUNT(pg.id) as project_count
		FROM administrative_boundaries ab
		LEFT JOIN project_geometries pg ON ST_Within(pg.centroid, ab.geometry)
		WHERE ab.admin_level = $1
		GROUP BY ab.name
		ORDER BY project_count DESC
	`

	err := r.db.Raw(query, adminLevel).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	regionMap := make(map[string]int)
	for _, result := range results {
		regionMap[result.RegionName] = result.ProjectCount
	}

	return regionMap, nil
}

// ============================================================================
// Administrative Boundaries
// ============================================================================

// CreateAdministrativeBoundary creates a new administrative boundary
func (r *Repository) CreateAdministrativeBoundary(boundary *AdministrativeBoundary) error {
	return r.db.Create(boundary).Error
}

// GetAdministrativeBoundariesByLevel retrieves boundaries by admin level
func (r *Repository) GetAdministrativeBoundariesByLevel(level int) ([]AdministrativeBoundary, error) {
	var boundaries []AdministrativeBoundary
	err := r.db.Where("admin_level = ?", level).Find(&boundaries).Error
	return boundaries, err
}

// GetAdministrativeBoundaryByName retrieves a boundary by name
func (r *Repository) GetAdministrativeBoundaryByName(name string, level int) (*AdministrativeBoundary, error) {
	var boundary AdministrativeBoundary
	err := r.db.Where("name = ? AND admin_level = ?", name, level).First(&boundary).Error
	if err != nil {
		return nil, err
	}
	return &boundary, nil
}

// ============================================================================
// Geofence Operations
// ============================================================================

// CreateGeofence creates a new geofence
func (r *Repository) CreateGeofence(geofence *Geofence) error {
	return r.db.Create(geofence).Error
}

// GetGeofence retrieves a geofence by ID
func (r *Repository) GetGeofence(id uuid.UUID) (*Geofence, error) {
	var geofence Geofence
	err := r.db.Where("id = ?", id).First(&geofence).Error
	if err != nil {
		return nil, err
	}
	return &geofence, nil
}

// ListGeofences retrieves all active geofences
func (r *Repository) ListGeofences(isActive *bool, geofenceType *string) ([]Geofence, error) {
	query := r.db.Model(&Geofence{})

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	if geofenceType != nil {
		query = query.Where("geofence_type = ?", *geofenceType)
	}

	var geofences []Geofence
	err := query.Order("priority DESC").Find(&geofences).Error
	return geofences, err
}

// UpdateGeofence updates an existing geofence
func (r *Repository) UpdateGeofence(geofence *Geofence) error {
	return r.db.Save(geofence).Error
}

// DeleteGeofence deletes a geofence
func (r *Repository) DeleteGeofence(id uuid.UUID) error {
	return r.db.Delete(&Geofence{}, id).Error
}

// CheckProjectAgainstGeofences checks if a project intersects with any geofences
func (r *Repository) CheckProjectAgainstGeofences(projectID uuid.UUID) ([]GeofenceCheck, error) {
	type GeofenceCheck struct {
		GeofenceID     uuid.UUID
		GeofenceName   string
		GeofenceType   string
		Intersects     bool
		DistanceMeters float64
		EventType      string
	}

	var results []GeofenceCheck

	query := `
		SELECT
			g.id as geofence_id,
			g.name as geofence_name,
			g.geofence_type,
			ST_Intersects(pg.geometry, g.geometry) as intersects,
			ST_Distance(pg.centroid, g.geometry) as distance_meters,
			CASE
				WHEN ST_Intersects(pg.geometry, g.geometry) THEN 'enter'
				WHEN ST_Distance(pg.centroid, g.geometry) <= (g.alert_rules->>'proximity_meters')::float THEN 'proximity'
				ELSE 'none'
			END as event_type
		FROM project_geometries pg
		CROSS JOIN geofences g
		WHERE pg.project_id = $1
		AND g.is_active = true
		AND (
			ST_Intersects(pg.geometry, g.geometry)
			OR ST_Distance(pg.centroid, g.geometry) <= (g.alert_rules->>'proximity_meters')::float
		)
		ORDER BY g.priority DESC
	`

	err := r.db.Raw(query, projectID).Scan(&results).Error
	return results, err
}

// ============================================================================
// Geofence Events
// ============================================================================

// CreateGeofenceEvent creates a new geofence event
func (r *Repository) CreateGeofenceEvent(event *GeofenceEvent) error {
	return r.db.Create(event).Error
}

// GetGeofenceEvents retrieves geofence events with filters
func (r *Repository) GetGeofenceEvents(geofenceID, projectID *uuid.UUID, eventType *string, limit int) ([]GeofenceEvent, error) {
	query := r.db.Model(&GeofenceEvent{})

	if geofenceID != nil {
		query = query.Where("geofence_id = ?", *geofenceID)
	}

	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	if eventType != nil {
		query = query.Where("event_type = ?", *eventType)
	}

	var events []GeofenceEvent
	err := query.Order("created_at DESC").Limit(limit).Find(&events).Error
	return events, err
}

// ============================================================================
// Map Tile Cache Operations
// ============================================================================

// GetCachedTile retrieves a cached map tile
func (r *Repository) GetCachedTile(tileKey string) (*MapTileCache, error) {
	var tile MapTileCache
	err := r.db.Where("tile_key = ? AND expires_at > ?", tileKey, time.Now()).First(&tile).Error
	if err != nil {
		return nil, err
	}

	// Update access stats
	r.db.Model(&tile).Updates(map[string]interface{}{
		"accessed_count":   gorm.Expr("accessed_count + ?", 1),
		"last_accessed_at": time.Now(),
	})

	return &tile, nil
}

// SaveTileCache saves a map tile to cache
func (r *Repository) SaveTileCache(tile *MapTileCache) error {
	return r.db.Create(tile).Error
}

// DeleteExpiredTiles deletes expired tiles from cache
func (r *Repository) DeleteExpiredTiles() (int64, error) {
	result := r.db.Where("expires_at < ?", time.Now()).Delete(&MapTileCache{})
	return result.RowsAffected, result.Error
}

// GetCacheSizeBytes retrieves the total size of cached tiles in bytes
func (r *Repository) GetCacheSizeBytes() (int64, error) {
	var totalSize int64
	query := "SELECT COALESCE(SUM(LENGTH(tile_data)), 0) FROM map_tile_cache WHERE expires_at > $1"
	err := r.db.Raw(query, time.Now()).Scan(&totalSize).Error
	return totalSize, err
}

// DeleteLeastAccessedTiles deletes least accessed tiles to free up space
func (r *Repository) DeleteLeastAccessedTiles(limit int) (int64, error) {
	subquery := r.db.Model(&MapTileCache{}).
		Select("id").
		Order("accessed_count ASC, last_accessed_at ASC").
		Limit(limit)

	result := r.db.Where("id IN (?)", subquery).Delete(&MapTileCache{})
	return result.RowsAffected, result.Error
}
