package postgis

import (
	"fmt"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// SpatialFunctions provides PostGIS spatial function wrappers
type SpatialFunctions struct {
	db *gorm.DB
}

// NewSpatialFunctions creates a new SpatialFunctions instance
func NewSpatialFunctions(db *gorm.DB) *SpatialFunctions {
	return &SpatialFunctions{db: db}
}

// CalculateArea calculates the area of a geography in square meters
func (sf *SpatialFunctions) CalculateArea(geography string) (decimal.Decimal, error) {
	var area float64
	query := fmt.Sprintf("SELECT ST_Area('%s'::geography)", geography)
	result := sf.db.Raw(query).Scan(&area)
	if result.Error != nil {
		return decimal.Zero, result.Error
	}
	return decimal.NewFromFloat(area), nil
}

// CalculateAreaHectares calculates area in hectares
func (sf *SpatialFunctions) CalculateAreaHectares(geography string) (decimal.Decimal, error) {
	area, err := sf.CalculateArea(geography)
	if err != nil {
		return decimal.Zero, err
	}
	// Convert square meters to hectares (1 hectare = 10,000 square meters)
	hectares := area.Div(decimal.NewFromInt(10000))
	return hectares, nil
}

// CalculatePerimeter calculates the perimeter of a geography in meters
func (sf *SpatialFunctions) CalculatePerimeter(geography string) (decimal.Decimal, error) {
	var perimeter float64
	query := fmt.Sprintf("SELECT ST_Perimeter('%s'::geography)", geography)
	result := sf.db.Raw(query).Scan(&perimeter)
	if result.Error != nil {
		return decimal.Zero, result.Error
	}
	return decimal.NewFromFloat(perimeter), nil
}

// CalculateCentroid calculates the centroid of a geometry
func (sf *SpatialFunctions) CalculateCentroid(geometry string) (string, error) {
	var centroid string
	query := fmt.Sprintf("SELECT ST_AsText(ST_Centroid('%s'::geometry))", geometry)
	result := sf.db.Raw(query).Scan(&centroid)
	if result.Error != nil {
		return "", result.Error
	}
	return centroid, nil
}

// CalculateDistance calculates the distance between two geographies in meters
func (sf *SpatialFunctions) CalculateDistance(geo1, geo2 string) (decimal.Decimal, error) {
	var distance float64
	query := fmt.Sprintf("SELECT ST_Distance('%s'::geography, '%s'::geography)", geo1, geo2)
	result := sf.db.Raw(query).Scan(&distance)
	if result.Error != nil {
		return decimal.Zero, result.Error
	}
	return decimal.NewFromFloat(distance), nil
}

// CheckIntersection checks if two geographies intersect
func (sf *SpatialFunctions) CheckIntersection(geo1, geo2 string) (bool, error) {
	var intersects bool
	query := fmt.Sprintf("SELECT ST_Intersects('%s'::geography, '%s'::geography)", geo1, geo2)
	result := sf.db.Raw(query).Scan(&intersects)
	if result.Error != nil {
		return false, result.Error
	}
	return intersects, nil
}

// CalculateIntersectionArea calculates the area of intersection between two geographies
func (sf *SpatialFunctions) CalculateIntersectionArea(geo1, geo2 string) (decimal.Decimal, error) {
	var area float64
	query := fmt.Sprintf(
		"SELECT ST_Area(ST_Intersection('%s'::geography, '%s'::geography))",
		geo1, geo2,
	)
	result := sf.db.Raw(query).Scan(&area)
	if result.Error != nil {
		return decimal.Zero, result.Error
	}
	return decimal.NewFromFloat(area), nil
}

// SimplifyGeometry simplifies a geometry using Douglas-Peucker algorithm
func (sf *SpatialFunctions) SimplifyGeometry(geometry string, tolerance float64) (string, error) {
	var simplified string
	query := fmt.Sprintf(
		"SELECT ST_AsText(ST_Simplify('%s'::geometry, %f))",
		geometry, tolerance,
	)
	result := sf.db.Raw(query).Scan(&simplified)
	if result.Error != nil {
		return "", result.Error
	}
	return simplified, nil
}

// IsValidGeometry checks if a geometry is valid
func (sf *SpatialFunctions) IsValidGeometry(geometry string) (bool, error) {
	var isValid bool
	query := fmt.Sprintf("SELECT ST_IsValid('%s'::geometry)", geometry)
	result := sf.db.Raw(query).Scan(&isValid)
	if result.Error != nil {
		return false, result.Error
	}
	return isValid, nil
}

// GetValidationReason returns the reason why a geometry is invalid
func (sf *SpatialFunctions) GetValidationReason(geometry string) (string, error) {
	var reason string
	query := fmt.Sprintf("SELECT ST_IsValidReason('%s'::geometry)", geometry)
	result := sf.db.Raw(query).Scan(&reason)
	if result.Error != nil {
		return "", result.Error
	}
	return reason, nil
}

// MakeValidGeometry attempts to make an invalid geometry valid
func (sf *SpatialFunctions) MakeValidGeometry(geometry string) (string, error) {
	var valid string
	query := fmt.Sprintf("SELECT ST_AsText(ST_MakeValid('%s'::geometry))", geometry)
	result := sf.db.Raw(query).Scan(&valid)
	if result.Error != nil {
		return "", result.Error
	}
	return valid, nil
}

// CreateBoundingBox creates a bounding box for a geometry
func (sf *SpatialFunctions) CreateBoundingBox(geometry string) (string, error) {
	var bbox string
	query := fmt.Sprintf("SELECT ST_AsText(ST_Envelope('%s'::geometry))", geometry)
	result := sf.db.Raw(query).Scan(&bbox)
	if result.Error != nil {
		return "", result.Error
	}
	return bbox, nil
}

// TransformSRID transforms geometry from one SRID to another
func (sf *SpatialFunctions) TransformSRID(geometry string, fromSRID, toSRID int) (string, error) {
	var transformed string
	query := fmt.Sprintf(
		"SELECT ST_AsText(ST_Transform(ST_SetSRID('%s'::geometry, %d), %d))",
		geometry, fromSRID, toSRID,
	)
	result := sf.db.Raw(query).Scan(&transformed)
	if result.Error != nil {
		return "", result.Error
	}
	return transformed, nil
}

// CreateBuffer creates a buffer around a geography
func (sf *SpatialFunctions) CreateBuffer(geography string, radiusMeters float64) (string, error) {
	var buffer string
	query := fmt.Sprintf(
		"SELECT ST_AsText(ST_Buffer('%s'::geography, %f)::geometry)",
		geography, radiusMeters,
	)
	result := sf.db.Raw(query).Scan(&buffer)
	if result.Error != nil {
		return "", result.Error
	}
	return buffer, nil
}

// FindProjectsWithinRadius finds projects within a radius of a point
func (sf *SpatialFunctions) FindProjectsWithinRadius(lat, lng, radiusMeters float64, limit int) ([]string, error) {
	var projectIDs []string
	query := fmt.Sprintf(`
		SELECT project_id::text
		FROM project_geometries
		WHERE ST_DWithin(
			centroid,
			ST_SetSRID(ST_MakePoint(%f, %f), 4326)::geography,
			%f
		)
		ORDER BY ST_Distance(
			centroid,
			ST_SetSRID(ST_MakePoint(%f, %f), 4326)::geography
		)
		LIMIT %d
	`, lng, lat, radiusMeters, lng, lat, limit)

	result := sf.db.Raw(query).Scan(&projectIDs)
	if result.Error != nil {
		return nil, result.Error
	}
	return projectIDs, nil
}

// FindProjectsWithinBounds finds projects within a bounding box
func (sf *SpatialFunctions) FindProjectsWithinBounds(minLat, maxLat, minLng, maxLng float64, limit int) ([]string, error) {
	var projectIDs []string
	query := fmt.Sprintf(`
		SELECT project_id::text
		FROM project_geometries
		WHERE ST_Within(
			centroid,
			ST_MakeEnvelope(%f, %f, %f, %f, 4326)::geography
		)
		LIMIT %d
	`, minLng, minLat, maxLng, maxLat, limit)

	result := sf.db.Raw(query).Scan(&projectIDs)
	if result.Error != nil {
		return nil, result.Error
	}
	return projectIDs, nil
}

// CheckOverlappingProjects checks if a project geometry overlaps with existing projects
func (sf *SpatialFunctions) CheckOverlappingProjects(geometry string, excludeProjectID string) ([]OverlapResult, error) {
	var results []OverlapResult
	query := fmt.Sprintf(`
		SELECT
			project_id::text,
			ST_Area(ST_Intersection(geometry, '%s'::geography)) / 10000 as overlap_area_hectares,
			(ST_Area(ST_Intersection(geometry, '%s'::geography)) / ST_Area(geometry)) * 100 as overlap_percentage
		FROM project_geometries
		WHERE project_id::text != '%s'
		AND ST_Intersects(geometry, '%s'::geography)
	`, geometry, geometry, excludeProjectID, geometry)

	result := sf.db.Raw(query).Scan(&results)
	if result.Error != nil {
		return nil, result.Error
	}
	return results, nil
}

// OverlapResult represents an overlap detection result
type OverlapResult struct {
	ProjectID          string  `json:"project_id"`
	OverlapAreaHectares float64 `json:"overlap_area_hectares"`
	OverlapPercentage  float64 `json:"overlap_percentage"`
}
