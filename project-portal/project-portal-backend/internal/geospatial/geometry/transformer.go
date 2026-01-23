package geometry

import (
	"fmt"

	"carbon-scribe/project-portal/project-portal-backend/pkg/postgis"

	"gorm.io/gorm"
)

// Transformer handles coordinate system transformations
type Transformer struct {
	spatial *postgis.SpatialFunctions
}

// NewTransformer creates a new coordinate transformer
func NewTransformer(db *gorm.DB) *Transformer {
	return &Transformer{
		spatial: postgis.NewSpatialFunctions(db),
	}
}

// Common SRID (Spatial Reference ID) constants
const (
	SRID_WGS84        = 4326  // WGS84 - GPS coordinates (latitude/longitude)
	SRID_WEB_MERCATOR = 3857  // Web Mercator - Web maps (Google, OSM, etc.)
	SRID_UTM_NORTH    = 32600 // UTM Zones (add zone number, e.g., 32601 for Zone 1N)
	SRID_UTM_SOUTH    = 32700 // UTM Zones (add zone number, e.g., 32701 for Zone 1S)
)

// TransformToWebMercator transforms WGS84 to Web Mercator
func (t *Transformer) TransformToWebMercator(wkt string) (string, error) {
	return t.spatial.TransformSRID(wkt, SRID_WGS84, SRID_WEB_MERCATOR)
}

// TransformToWGS84 transforms any SRID to WGS84
func (t *Transformer) TransformToWGS84(wkt string, fromSRID int) (string, error) {
	return t.spatial.TransformSRID(wkt, fromSRID, SRID_WGS84)
}

// TransformSRID transforms geometry between any two coordinate systems
func (t *Transformer) TransformSRID(wkt string, fromSRID, toSRID int) (string, error) {
	if fromSRID == toSRID {
		return wkt, nil // No transformation needed
	}

	return t.spatial.TransformSRID(wkt, fromSRID, toSRID)
}

// NormalizeToWGS84 ensures geometry is in WGS84 (used for storage)
func (t *Transformer) NormalizeToWGS84(wkt string, srid int) (string, error) {
	if srid == 0 || srid == SRID_WGS84 {
		// Already WGS84 or unspecified (assume WGS84)
		return wkt, nil
	}

	return t.TransformToWGS84(wkt, srid)
}

// GetUTMZone calculates the appropriate UTM zone for a longitude
func (t *Transformer) GetUTMZone(longitude float64, isNorthern bool) int {
	// UTM zone calculation: ((longitude + 180) / 6) + 1
	zone := int((longitude+180)/6) + 1
	if zone < 1 {
		zone = 1
	}
	if zone > 60 {
		zone = 60
	}

	if isNorthern {
		return SRID_UTM_NORTH + zone
	}
	return SRID_UTM_SOUTH + zone
}

// TransformToUTM transforms geometry to the appropriate UTM zone
func (t *Transformer) TransformToUTM(wkt string, centerLongitude float64, isNorthern bool) (string, int, error) {
	utmSRID := t.GetUTMZone(centerLongitude, isNorthern)
	transformedWKT, err := t.spatial.TransformSRID(wkt, SRID_WGS84, utmSRID)
	if err != nil {
		return "", 0, err
	}

	return transformedWKT, utmSRID, nil
}

// CoordinatePoint represents a geographic coordinate
type CoordinatePoint struct {
	Latitude  float64
	Longitude float64
}

// TransformPoint transforms a single point between coordinate systems
func (t *Transformer) TransformPoint(lat, lng float64, fromSRID, toSRID int) (*CoordinatePoint, error) {
	wkt := fmt.Sprintf("POINT(%f %f)", lng, lat)
	transformedWKT, err := t.spatial.TransformSRID(wkt, fromSRID, toSRID)
	if err != nil {
		return nil, err
	}

	// Parse result
	var newLng, newLat float64
	_, err = fmt.Sscanf(transformedWKT, "POINT(%f %f)", &newLng, &newLat)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformed point: %w", err)
	}

	return &CoordinatePoint{
		Latitude:  newLat,
		Longitude: newLng,
	}, nil
}

// ValidateSRID checks if an SRID is valid and supported
func (t *Transformer) ValidateSRID(srid int) bool {
	supportedSRIDs := map[int]bool{
		SRID_WGS84:        true,
		SRID_WEB_MERCATOR: true,
	}

	// UTM zones (EPSG:32601-32660 for Northern, EPSG:32701-32760 for Southern)
	if (srid >= 32601 && srid <= 32660) || (srid >= 32701 && srid <= 32760) {
		return true
	}

	return supportedSRIDs[srid]
}

// GetSRIDName returns the name/description of an SRID
func (t *Transformer) GetSRIDName(srid int) string {
	switch srid {
	case SRID_WGS84:
		return "WGS84 (GPS Coordinates)"
	case SRID_WEB_MERCATOR:
		return "Web Mercator (Web Maps)"
	default:
		if srid >= 32601 && srid <= 32660 {
			zone := srid - SRID_UTM_NORTH
			return fmt.Sprintf("UTM Zone %dN", zone)
		}
		if srid >= 32701 && srid <= 32760 {
			zone := srid - SRID_UTM_SOUTH
			return fmt.Sprintf("UTM Zone %dS", zone)
		}
		return fmt.Sprintf("EPSG:%d", srid)
	}
}
