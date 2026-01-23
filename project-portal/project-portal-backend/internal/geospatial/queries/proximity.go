package queries

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ProximityQuery handles proximity-based spatial queries
type ProximityQuery struct {
	db *gorm.DB
}

// NewProximityQuery creates a new proximity query handler
func NewProximityQuery(db *gorm.DB) *ProximityQuery {
	return &ProximityQuery{db: db}
}

// ProjectResult represents a project with distance information
type ProjectResult struct {
	ProjectID       uuid.UUID       `json:"project_id"`
	DistanceMeters  decimal.Decimal `json:"distance_meters"`
	DistanceKm      decimal.Decimal `json:"distance_km"`
	AreaHectares    decimal.Decimal `json:"area_hectares"`
	CentroidLat     float64         `json:"centroid_lat"`
	CentroidLng     float64         `json:"centroid_lng"`
}

// FindNearbyProjects finds projects within a radius of a point
func (pq *ProximityQuery) FindNearbyProjects(lat, lng, radiusMeters float64, limit int) ([]ProjectResult, error) {
	var results []ProjectResult

	query := `
		SELECT
			project_id,
			ST_Distance(
				centroid,
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
			) as distance_meters,
			ST_Distance(
				centroid,
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
			) / 1000 as distance_km,
			area_hectares,
			ST_Y(centroid::geometry) as centroid_lat,
			ST_X(centroid::geometry) as centroid_lng
		FROM project_geometries
		WHERE ST_DWithin(
			centroid,
			ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
			$3
		)
		ORDER BY ST_Distance(
			centroid,
			ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		) ASC
		LIMIT $4
	`

	err := pq.db.Raw(query, lng, lat, radiusMeters, limit).Scan(&results).Error
	return results, err
}

// FindKNearestProjects finds the K nearest projects to a point
func (pq *ProximityQuery) FindKNearestProjects(lat, lng float64, k int) ([]ProjectResult, error) {
	var results []ProjectResult

	query := `
		SELECT
			project_id,
			ST_Distance(
				centroid,
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
			) as distance_meters,
			ST_Distance(
				centroid,
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
			) / 1000 as distance_km,
			area_hectares,
			ST_Y(centroid::geometry) as centroid_lat,
			ST_X(centroid::geometry) as centroid_lng
		FROM project_geometries
		ORDER BY centroid <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		LIMIT $3
	`

	err := pq.db.Raw(query, lng, lat, k).Scan(&results).Error
	return results, err
}

// FindProjectsWithinBounds finds projects within a bounding box
func (pq *ProximityQuery) FindProjectsWithinBounds(minLat, maxLat, minLng, maxLng float64, limit int) ([]ProjectResult, error) {
	var results []ProjectResult

	query := `
		SELECT
			project_id,
			0 as distance_meters,
			0 as distance_km,
			area_hectares,
			ST_Y(centroid::geometry) as centroid_lat,
			ST_X(centroid::geometry) as centroid_lng
		FROM project_geometries
		WHERE ST_Within(
			centroid,
			ST_MakeEnvelope($1, $2, $3, $4, 4326)::geography
		)
		ORDER BY area_hectares DESC
		LIMIT $5
	`

	err := pq.db.Raw(query, minLng, minLat, maxLng, maxLat, limit).Scan(&results).Error
	return results, err
}

// FindProjectsAlongPath finds projects along a path (line) within a buffer distance
func (pq *ProximityQuery) FindProjectsAlongPath(pathPoints [][]float64, bufferMeters float64, limit int) ([]ProjectResult, error) {
	if len(pathPoints) < 2 {
		return nil, fmt.Errorf("path must have at least 2 points")
	}

	// Build LineString WKT
	lineWKT := "LINESTRING("
	for i, point := range pathPoints {
		if i > 0 {
			lineWKT += ", "
		}
		lineWKT += fmt.Sprintf("%f %f", point[1], point[0]) // lng, lat
	}
	lineWKT += ")"

	var results []ProjectResult

	query := `
		SELECT
			project_id,
			ST_Distance(
				centroid,
				ST_GeomFromText($1, 4326)::geography
			) as distance_meters,
			ST_Distance(
				centroid,
				ST_GeomFromText($1, 4326)::geography
			) / 1000 as distance_km,
			area_hectares,
			ST_Y(centroid::geometry) as centroid_lat,
			ST_X(centroid::geometry) as centroid_lng
		FROM project_geometries
		WHERE ST_DWithin(
			centroid,
			ST_GeomFromText($1, 4326)::geography,
			$2
		)
		ORDER BY ST_Distance(
			centroid,
			ST_GeomFromText($1, 4326)::geography
		) ASC
		LIMIT $3
	`

	err := pq.db.Raw(query, lineWKT, bufferMeters, limit).Scan(&results).Error
	return results, err
}

// FindProjectsInCircle finds projects within a circular area
func (pq *ProximityQuery) FindProjectsInCircle(centerLat, centerLng, radiusMeters float64) ([]ProjectResult, error) {
	return pq.FindNearbyProjects(centerLat, centerLng, radiusMeters, 1000) // High limit for circle
}

// CalculateProjectDistance calculates distance between two projects
func (pq *ProximityQuery) CalculateProjectDistance(projectID1, projectID2 uuid.UUID) (decimal.Decimal, error) {
	var distance float64

	query := `
		SELECT ST_Distance(
			(SELECT centroid FROM project_geometries WHERE project_id = $1),
			(SELECT centroid FROM project_geometries WHERE project_id = $2)
		)
	`

	err := pq.db.Raw(query, projectID1, projectID2).Scan(&distance).Error
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromFloat(distance), nil
}

// ProximityStats contains proximity statistics
type ProximityStats struct {
	AverageDistance decimal.Decimal `json:"average_distance_meters"`
	MinDistance     decimal.Decimal `json:"min_distance_meters"`
	MaxDistance     decimal.Decimal `json:"max_distance_meters"`
	TotalProjects   int             `json:"total_projects"`
}

// GetProximityStatistics calculates proximity statistics for projects near a point
func (pq *ProximityQuery) GetProximityStatistics(lat, lng, radiusMeters float64) (*ProximityStats, error) {
	var stats ProximityStats

	query := `
		SELECT
			AVG(ST_Distance(centroid, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)) as average_distance,
			MIN(ST_Distance(centroid, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)) as min_distance,
			MAX(ST_Distance(centroid, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)) as max_distance,
			COUNT(*) as total_projects
		FROM project_geometries
		WHERE ST_DWithin(
			centroid,
			ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
			$3
		)
	`

	err := pq.db.Raw(query, lng, lat, radiusMeters).Scan(&stats).Error
	return &stats, err
}
