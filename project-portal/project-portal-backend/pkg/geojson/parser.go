package geojson

import (
	"encoding/json"
	"fmt"

	"github.com/paulmach/go.geojson"
)

// Parser handles GeoJSON parsing and conversion
type Parser struct{}

// NewParser creates a new GeoJSON parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseGeoJSON parses a GeoJSON string into a Feature
func (p *Parser) ParseGeoJSON(geoJSONStr string) (*geojson.Feature, error) {
	feature, err := geojson.UnmarshalFeature([]byte(geoJSONStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse GeoJSON: %w", err)
	}
	return feature, nil
}

// ParseGeoJSONGeometry parses a GeoJSON string and extracts only the geometry
func (p *Parser) ParseGeoJSONGeometry(geoJSONStr string) (*geojson.Geometry, error) {
	// Try parsing as Feature first
	feature, err := geojson.UnmarshalFeature([]byte(geoJSONStr))
	if err == nil && feature.Geometry != nil {
		return feature.Geometry, nil
	}

	// Try parsing as Geometry directly
	geometry, err := geojson.UnmarshalGeometry([]byte(geoJSONStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse GeoJSON geometry: %w", err)
	}

	return geometry, nil
}

// GeometryToWKT converts a GeoJSON geometry to WKT (Well-Known Text) format
func (p *Parser) GeometryToWKT(geometry *geojson.Geometry) (string, error) {
	switch geometry.Type {
	case geojson.GeometryPoint:
		return p.pointToWKT(geometry), nil
	case geojson.GeometryLineString:
		return p.lineStringToWKT(geometry), nil
	case geojson.GeometryPolygon:
		return p.polygonToWKT(geometry), nil
	case geojson.GeometryMultiPoint:
		return p.multiPointToWKT(geometry), nil
	case geojson.GeometryMultiLineString:
		return p.multiLineStringToWKT(geometry), nil
	case geojson.GeometryMultiPolygon:
		return p.multiPolygonToWKT(geometry), nil
	default:
		return "", fmt.Errorf("unsupported geometry type: %s", geometry.Type)
	}
}

// pointToWKT converts a Point geometry to WKT
func (p *Parser) pointToWKT(geometry *geojson.Geometry) string {
	coords := geometry.Point
	return fmt.Sprintf("POINT(%f %f)", coords[0], coords[1])
}

// lineStringToWKT converts a LineString geometry to WKT
func (p *Parser) lineStringToWKT(geometry *geojson.Geometry) string {
	coords := geometry.LineString
	wkt := "LINESTRING("
	for i, coord := range coords {
		if i > 0 {
			wkt += ", "
		}
		wkt += fmt.Sprintf("%f %f", coord[0], coord[1])
	}
	wkt += ")"
	return wkt
}

// polygonToWKT converts a Polygon geometry to WKT
func (p *Parser) polygonToWKT(geometry *geojson.Geometry) string {
	polygon := geometry.Polygon
	wkt := "POLYGON("
	for i, ring := range polygon {
		if i > 0 {
			wkt += ", "
		}
		wkt += "("
		for j, coord := range ring {
			if j > 0 {
				wkt += ", "
			}
			wkt += fmt.Sprintf("%f %f", coord[0], coord[1])
		}
		wkt += ")"
	}
	wkt += ")"
	return wkt
}

// multiPointToWKT converts a MultiPoint geometry to WKT
func (p *Parser) multiPointToWKT(geometry *geojson.Geometry) string {
	coords := geometry.MultiPoint
	wkt := "MULTIPOINT("
	for i, coord := range coords {
		if i > 0 {
			wkt += ", "
		}
		wkt += fmt.Sprintf("(%f %f)", coord[0], coord[1])
	}
	wkt += ")"
	return wkt
}

// multiLineStringToWKT converts a MultiLineString geometry to WKT
func (p *Parser) multiLineStringToWKT(geometry *geojson.Geometry) string {
	lines := geometry.MultiLineString
	wkt := "MULTILINESTRING("
	for i, line := range lines {
		if i > 0 {
			wkt += ", "
		}
		wkt += "("
		for j, coord := range line {
			if j > 0 {
				wkt += ", "
			}
			wkt += fmt.Sprintf("%f %f", coord[0], coord[1])
		}
		wkt += ")"
	}
	wkt += ")"
	return wkt
}

// multiPolygonToWKT converts a MultiPolygon geometry to WKT
func (p *Parser) multiPolygonToWKT(geometry *geojson.Geometry) string {
	polygons := geometry.MultiPolygon
	wkt := "MULTIPOLYGON("
	for i, polygon := range polygons {
		if i > 0 {
			wkt += ", "
		}
		wkt += "("
		for j, ring := range polygon {
			if j > 0 {
				wkt += ", "
			}
			wkt += "("
			for k, coord := range ring {
				if k > 0 {
					wkt += ", "
				}
				wkt += fmt.Sprintf("%f %f", coord[0], coord[1])
			}
			wkt += ")"
		}
		wkt += ")"
	}
	wkt += ")"
	return wkt
}

// WKTToGeoJSON converts WKT to GeoJSON geometry
func (p *Parser) WKTToGeoJSON(wkt string) (string, error) {
	// Note: This is a placeholder. In a real implementation, you'd use a WKT parser library
	// For now, we return an error as WKT parsing is complex
	return "", fmt.Errorf("WKT to GeoJSON conversion not yet implemented")
}

// ValidateGeoJSONStructure performs basic structural validation
func (p *Parser) ValidateGeoJSONStructure(geoJSONStr string) error {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(geoJSONStr), &raw); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Check for required GeoJSON fields
	typeField, ok := raw["type"]
	if !ok {
		return fmt.Errorf("missing 'type' field")
	}

	typeStr, ok := typeField.(string)
	if !ok {
		return fmt.Errorf("'type' field must be a string")
	}

	// Validate based on type
	switch typeStr {
	case "Feature":
		if _, ok := raw["geometry"]; !ok {
			return fmt.Errorf("Feature must have 'geometry' field")
		}
	case "FeatureCollection":
		if _, ok := raw["features"]; !ok {
			return fmt.Errorf("FeatureCollection must have 'features' field")
		}
	case "Point", "LineString", "Polygon", "MultiPoint", "MultiLineString", "MultiPolygon", "GeometryCollection":
		if _, ok := raw["coordinates"]; !ok {
			return fmt.Errorf("Geometry must have 'coordinates' field")
		}
	default:
		return fmt.Errorf("invalid GeoJSON type: %s", typeStr)
	}

	return nil
}

// GetGeometryType returns the type of geometry in the GeoJSON
func (p *Parser) GetGeometryType(geoJSONStr string) (string, error) {
	geometry, err := p.ParseGeoJSONGeometry(geoJSONStr)
	if err != nil {
		return "", err
	}
	return geometry.Type, nil
}

// GetCoordinateCount counts the number of coordinates in a geometry
func (p *Parser) GetCoordinateCount(geometry *geojson.Geometry) int {
	switch geometry.Type {
	case geojson.GeometryPoint:
		return 1
	case geojson.GeometryLineString:
		return len(geometry.LineString)
	case geojson.GeometryPolygon:
		count := 0
		for _, ring := range geometry.Polygon {
			count += len(ring)
		}
		return count
	case geojson.GeometryMultiPoint:
		return len(geometry.MultiPoint)
	case geojson.GeometryMultiLineString:
		count := 0
		for _, line := range geometry.MultiLineString {
			count += len(line)
		}
		return count
	case geojson.GeometryMultiPolygon:
		count := 0
		for _, polygon := range geometry.MultiPolygon {
			for _, ring := range polygon {
				count += len(ring)
			}
		}
		return count
	default:
		return 0
	}
}

// ExtractBounds extracts the bounding box from a geometry
func (p *Parser) ExtractBounds(geometry *geojson.Geometry) (minLng, minLat, maxLng, maxLat float64, err error) {
	var coords [][]float64

	switch geometry.Type {
	case geojson.GeometryPoint:
		return geometry.Point[0], geometry.Point[1], geometry.Point[0], geometry.Point[1], nil
	case geojson.GeometryLineString:
		coords = geometry.LineString
	case geojson.GeometryPolygon:
		if len(geometry.Polygon) > 0 {
			coords = geometry.Polygon[0] // Use exterior ring
		}
	default:
		return 0, 0, 0, 0, fmt.Errorf("unsupported geometry type for bounds extraction: %s", geometry.Type)
	}

	if len(coords) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("no coordinates found")
	}

	minLng, minLat = coords[0][0], coords[0][1]
	maxLng, maxLat = coords[0][0], coords[0][1]

	for _, coord := range coords {
		if coord[0] < minLng {
			minLng = coord[0]
		}
		if coord[0] > maxLng {
			maxLng = coord[0]
		}
		if coord[1] < minLat {
			minLat = coord[1]
		}
		if coord[1] > maxLat {
			maxLat = coord[1]
		}
	}

	return minLng, minLat, maxLng, maxLat, nil
}
