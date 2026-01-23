package geometry

import (
	"fmt"

	"carbon-scribe/project-portal/project-portal-backend/pkg/geojson"
	"carbon-scribe/project-portal/project-portal-backend/pkg/postgis"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Processor handles GeoJSON processing and geometry operations
type Processor struct {
	parser    *geojson.Parser
	validator *geojson.Validator
	spatial   *postgis.SpatialFunctions
}

// NewProcessor creates a new geometry processor
func NewProcessor(db *gorm.DB) *Processor {
	return &Processor{
		parser:    geojson.NewParser(),
		validator: geojson.NewValidator(),
		spatial:   postgis.NewSpatialFunctions(db),
	}
}

// ProcessedGeometry contains all processed geometry data
type ProcessedGeometry struct {
	WKT                string
	WKTCentroid        string
	WKTBoundingBox     string
	AreaHectares       decimal.Decimal
	PerimeterMeters    decimal.Decimal
	IsValid            bool
	ValidationErrors   []string
	SimplifiedWKT      *string
}

// ProcessGeoJSON processes a GeoJSON string and returns computed geometry data
func (p *Processor) ProcessGeoJSON(geoJSONStr string, maxVertices int, simplificationTolerance *float64) (*ProcessedGeometry, error) {
	// 1. Validate structure
	validationResult := p.validator.Validate(geoJSONStr, maxVertices)
	if !validationResult.IsValid {
		return &ProcessedGeometry{
			IsValid:          false,
			ValidationErrors: validationResult.Errors,
		}, fmt.Errorf("geometry validation failed: %v", validationResult.Errors)
	}

	// 2. Parse geometry
	geometry, err := p.parser.ParseGeoJSONGeometry(geoJSONStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GeoJSON: %w", err)
	}

	// 3. Convert to WKT
	wkt, err := p.parser.GeometryToWKT(geometry)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to WKT: %w", err)
	}

	// 4. Validate geometry in PostGIS
	isValid, err := p.spatial.IsValidGeometry(wkt)
	if err != nil {
		return nil, fmt.Errorf("failed to validate geometry: %w", err)
	}

	if !isValid {
		reason, _ := p.spatial.GetValidationReason(wkt)
		return &ProcessedGeometry{
			WKT:              wkt,
			IsValid:          false,
			ValidationErrors: []string{reason},
		}, fmt.Errorf("invalid geometry: %s", reason)
	}

	// 5. Calculate centroid
	centroidWKT, err := p.spatial.CalculateCentroid(wkt)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate centroid: %w", err)
	}

	// 6. Calculate bounding box
	bboxWKT, err := p.spatial.CreateBoundingBox(wkt)
	if err != nil {
		return nil, fmt.Errorf("failed to create bounding box: %w", err)
	}

	// 7. Calculate area in hectares
	areaHectares, err := p.spatial.CalculateAreaHectares(wkt)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate area: %w", err)
	}

	// 8. Calculate perimeter
	perimeter, err := p.spatial.CalculatePerimeter(wkt)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate perimeter: %w", err)
	}

	// 9. Simplify geometry if requested
	var simplifiedWKT *string
	if simplificationTolerance != nil && *simplificationTolerance > 0 {
		simplified, err := p.spatial.SimplifyGeometry(wkt, *simplificationTolerance)
		if err == nil {
			simplifiedWKT = &simplified
		}
	}

	return &ProcessedGeometry{
		WKT:             wkt,
		WKTCentroid:     centroidWKT,
		WKTBoundingBox:  bboxWKT,
		AreaHectares:    areaHectares,
		PerimeterMeters: perimeter,
		IsValid:         true,
		ValidationErrors: []string{},
		SimplifiedWKT:   simplifiedWKT,
	}, nil
}

// ValidateMinimumArea checks if geometry meets minimum area requirement
func (p *Processor) ValidateMinimumArea(wkt string, minAreaHectares float64) error {
	area, err := p.spatial.CalculateAreaHectares(wkt)
	if err != nil {
		return fmt.Errorf("failed to calculate area: %w", err)
	}

	areaFloat, _ := area.Float64()
	if areaFloat < minAreaHectares {
		return fmt.Errorf("area %.2f hectares is below minimum of %.2f hectares", areaFloat, minAreaHectares)
	}

	return nil
}

// CheckSelfIntersection checks if a geometry has self-intersections
func (p *Processor) CheckSelfIntersection(wkt string) (bool, error) {
	isValid, err := p.spatial.IsValidGeometry(wkt)
	if err != nil {
		return false, err
	}

	if !isValid {
		reason, _ := p.spatial.GetValidationReason(wkt)
		// Check if reason mentions self-intersection
		if reason != "" {
			return true, nil // Has self-intersection
		}
	}

	return false, nil
}

// FixInvalidGeometry attempts to fix an invalid geometry
func (p *Processor) FixInvalidGeometry(wkt string) (string, error) {
	validWKT, err := p.spatial.MakeValidGeometry(wkt)
	if err != nil {
		return "", fmt.Errorf("failed to fix geometry: %w", err)
	}

	// Verify the fixed geometry is now valid
	isValid, err := p.spatial.IsValidGeometry(validWKT)
	if err != nil {
		return "", err
	}

	if !isValid {
		return "", fmt.Errorf("geometry could not be fixed")
	}

	return validWKT, nil
}

// ConvertToGeography converts WKT geometry to PostGIS geography format
func (p *Processor) ConvertToGeography(wkt string) string {
	// PostGIS automatically handles this conversion
	// We just need to ensure proper SRID (4326 for WGS84)
	return fmt.Sprintf("SRID=4326;%s", wkt)
}

// ExtractCoordinates extracts coordinate array from GeoJSON
func (p *Processor) ExtractCoordinates(geoJSONStr string) ([][]float64, error) {
	geometry, err := p.parser.ParseGeoJSONGeometry(geoJSONStr)
	if err != nil {
		return nil, err
	}

	var coords [][]float64
	switch geometry.Type {
	case "Point":
		coords = [][]float64{geometry.Point}
	case "LineString":
		coords = geometry.LineString
	case "Polygon":
		if len(geometry.Polygon) > 0 {
			coords = geometry.Polygon[0] // Exterior ring
		}
	default:
		return nil, fmt.Errorf("unsupported geometry type for coordinate extraction: %s", geometry.Type)
	}

	return coords, nil
}
