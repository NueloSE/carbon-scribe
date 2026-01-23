package geojson

import (
	"fmt"
	"math"

	"github.com/paulmach/go.geojson"
)

// Validator provides GeoJSON validation according to RFC 7946
type Validator struct {
	parser *Parser
}

// NewValidator creates a new GeoJSON validator
func NewValidator() *Validator {
	return &Validator{
		parser: NewParser(),
	}
}

// ValidationResult holds validation results
type ValidationResult struct {
	IsValid bool     `json:"is_valid"`
	Errors  []string `json:"errors,omitempty"`
}

// Validate performs comprehensive GeoJSON validation
func (v *Validator) Validate(geoJSONStr string, maxVertices int) *ValidationResult {
	result := &ValidationResult{
		IsValid: true,
		Errors:  []string{},
	}

	// 1. Validate JSON structure
	if err := v.parser.ValidateGeoJSONStructure(geoJSONStr); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid GeoJSON structure: %s", err.Error()))
		return result
	}

	// 2. Parse geometry
	geometry, err := v.parser.ParseGeoJSONGeometry(geoJSONStr)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to parse geometry: %s", err.Error()))
		return result
	}

	// 3. Validate coordinate system (WGS84)
	if errs := v.validateCoordinates(geometry); len(errs) > 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, errs...)
	}

	// 4. Validate geometry type
	if err := v.validateGeometryType(geometry); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, err.Error())
	}

	// 5. Check vertex count
	vertexCount := v.parser.GetCoordinateCount(geometry)
	if vertexCount > maxVertices {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Too many vertices: %d (max: %d)", vertexCount, maxVertices))
	}

	// 6. Validate polygon-specific rules
	if geometry.Type == geojson.GeometryPolygon || geometry.Type == geojson.GeometryMultiPolygon {
		if errs := v.validatePolygon(geometry); len(errs) > 0 {
			result.IsValid = false
			result.Errors = append(result.Errors, errs...)
		}
	}

	return result
}

// validateCoordinates validates that all coordinates are within WGS84 bounds
func (v *Validator) validateCoordinates(geometry *geojson.Geometry) []string {
	errors := []string{}

	var coords [][]float64

	switch geometry.Type {
	case geojson.GeometryPoint:
		coords = [][]float64{geometry.Point}
	case geojson.GeometryLineString:
		coords = geometry.LineString
	case geojson.GeometryPolygon:
		for _, ring := range geometry.Polygon {
			coords = append(coords, ring...)
		}
	case geojson.GeometryMultiPoint:
		coords = geometry.MultiPoint
	case geojson.GeometryMultiLineString:
		for _, line := range geometry.MultiLineString {
			coords = append(coords, line...)
		}
	case geojson.GeometryMultiPolygon:
		for _, polygon := range geometry.MultiPolygon {
			for _, ring := range polygon {
				coords = append(coords, ring...)
			}
		}
	}

	for i, coord := range coords {
		if len(coord) < 2 {
			errors = append(errors, fmt.Sprintf("Coordinate %d has less than 2 dimensions", i))
			continue
		}

		lng, lat := coord[0], coord[1]

		// Validate longitude (-180 to 180)
		if lng < -180 || lng > 180 {
			errors = append(errors, fmt.Sprintf("Invalid longitude at coordinate %d: %f (must be between -180 and 180)", i, lng))
		}

		// Validate latitude (-90 to 90)
		if lat < -90 || lat > 90 {
			errors = append(errors, fmt.Sprintf("Invalid latitude at coordinate %d: %f (must be between -90 and 90)", i, lat))
		}

		// Check for NaN or Inf
		if math.IsNaN(lng) || math.IsNaN(lat) || math.IsInf(lng, 0) || math.IsInf(lat, 0) {
			errors = append(errors, fmt.Sprintf("Invalid coordinate at %d: contains NaN or Inf", i))
		}
	}

	return errors
}

// validateGeometryType validates that the geometry type is supported
func (v *Validator) validateGeometryType(geometry *geojson.Geometry) error {
	supportedTypes := map[string]bool{
		geojson.GeometryPoint:           true,
		geojson.GeometryLineString:      true,
		geojson.GeometryPolygon:         true,
		geojson.GeometryMultiPoint:      true,
		geojson.GeometryMultiLineString: true,
		geojson.GeometryMultiPolygon:    true,
	}

	if !supportedTypes[geometry.Type] {
		return fmt.Errorf("unsupported geometry type: %s", geometry.Type)
	}

	return nil
}

// validatePolygon validates polygon-specific rules
func (v *Validator) validatePolygon(geometry *geojson.Geometry) []string {
	errors := []string{}

	var polygons [][][]float64

	if geometry.Type == geojson.GeometryPolygon {
		polygons = [][]geometry.Polygon
	} else if geometry.Type == geojson.GeometryMultiPolygon {
		polygons = geometry.MultiPolygon
	}

	for polyIndex, polygon := range polygons {
		if len(polygon) == 0 {
			errors = append(errors, fmt.Sprintf("Polygon %d has no rings", polyIndex))
			continue
		}

		for ringIndex, ring := range polygon {
			// 1. Ring must have at least 4 coordinates
			if len(ring) < 4 {
				errors = append(errors, fmt.Sprintf("Polygon %d, ring %d: must have at least 4 coordinates (got %d)", polyIndex, ringIndex, len(ring)))
				continue
			}

			// 2. Ring must be closed (first and last coordinates must be identical)
			first := ring[0]
			last := ring[len(ring)-1]
			if first[0] != last[0] || first[1] != last[1] {
				errors = append(errors, fmt.Sprintf("Polygon %d, ring %d: ring is not closed", polyIndex, ringIndex))
			}

			// 3. Check for duplicate consecutive coordinates (except closing coordinate)
			for i := 1; i < len(ring)-1; i++ {
				if ring[i][0] == ring[i-1][0] && ring[i][1] == ring[i-1][1] {
					errors = append(errors, fmt.Sprintf("Polygon %d, ring %d: duplicate consecutive coordinates at position %d", polyIndex, ringIndex, i))
				}
			}

			// 4. Exterior ring (first ring) should be counter-clockwise
			// Interior rings (holes) should be clockwise
			if ringIndex == 0 {
				if !v.isCounterClockwise(ring) {
					errors = append(errors, fmt.Sprintf("Polygon %d: exterior ring should be counter-clockwise", polyIndex))
				}
			} else {
				if v.isCounterClockwise(ring) {
					errors = append(errors, fmt.Sprintf("Polygon %d, ring %d: interior ring (hole) should be clockwise", polyIndex, ringIndex))
				}
			}
		}
	}

	return errors
}

// isCounterClockwise determines if a ring is oriented counter-clockwise
func (v *Validator) isCounterClockwise(ring [][]float64) bool {
	// Calculate signed area using the shoelace formula
	area := 0.0
	n := len(ring)

	for i := 0; i < n-1; i++ {
		area += (ring[i+1][0] - ring[i][0]) * (ring[i+1][1] + ring[i][1])
	}

	// Positive area means counter-clockwise
	return area > 0
}

// ValidateMinimumArea checks if polygon meets minimum area requirement
func (v *Validator) ValidateMinimumArea(geometry *geojson.Geometry, minAreaHectares float64) error {
	// Note: This is a simplified check using bounding box
	// Actual area calculation should be done in PostGIS
	if geometry.Type != geojson.GeometryPolygon && geometry.Type != geojson.GeometryMultiPolygon {
		return fmt.Errorf("minimum area validation only applies to polygons")
	}

	minLng, minLat, maxLng, maxLat, err := v.parser.ExtractBounds(geometry)
	if err != nil {
		return fmt.Errorf("failed to extract bounds: %w", err)
	}

	// Rough approximation: 1 degree ≈ 111 km
	widthKm := (maxLng - minLng) * 111.0
	heightKm := (maxLat - minLat) * 111.0
	approximateAreaKm2 := widthKm * heightKm
	approximateAreaHectares := approximateAreaKm2 * 100 // 1 km² = 100 hectares

	if approximateAreaHectares < minAreaHectares {
		return fmt.Errorf("polygon area too small: approximately %.2f hectares (minimum: %.2f hectares)", approximateAreaHectares, minAreaHectares)
	}

	return nil
}

// CheckSelfIntersection is a placeholder for self-intersection detection
// Note: Actual self-intersection detection is complex and should be done in PostGIS
func (v *Validator) CheckSelfIntersection(geometry *geojson.Geometry) error {
	// This would require a robust computational geometry library
	// For now, we rely on PostGIS's ST_IsValid function
	return nil
}
