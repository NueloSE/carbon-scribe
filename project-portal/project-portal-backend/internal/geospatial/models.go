package geospatial

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// ProjectGeometry represents a project's geographic boundary
type ProjectGeometry struct {
	ID                      uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID               uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex" json:"project_id"`
	Geometry                string          `gorm:"type:geography(POLYGON,4326);not null" json:"geometry"`
	Centroid                string          `gorm:"type:geography(POINT,4326);not null" json:"centroid"`
	BoundingBox             *string         `gorm:"type:geography(POLYGON,4326)" json:"bounding_box,omitempty"`
	AreaHectares            decimal.Decimal `gorm:"type:decimal(12,4);not null" json:"area_hectares"`
	PerimeterMeters         *decimal.Decimal `gorm:"type:decimal(12,4)" json:"perimeter_meters,omitempty"`
	IsValid                 bool            `gorm:"default:true" json:"is_valid"`
	ValidationErrors        pq.StringArray  `gorm:"type:text[]" json:"validation_errors,omitempty"`
	SimplificationTolerance *decimal.Decimal `gorm:"type:decimal(10,6)" json:"simplification_tolerance,omitempty"`
	SourceType              string          `gorm:"type:varchar(50);default:'manual'" json:"source_type"`
	SourceFile              *string         `gorm:"type:varchar(500)" json:"source_file,omitempty"`
	AccuracyScore           *decimal.Decimal `gorm:"type:decimal(3,2)" json:"accuracy_score,omitempty"`
	Version                 int             `gorm:"default:1" json:"version"`
	PreviousVersionID       *uuid.UUID      `gorm:"type:uuid" json:"previous_version_id,omitempty"`
	CreatedAt               time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt               time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// TableName specifies the table name for GORM
func (ProjectGeometry) TableName() string {
	return "project_geometries"
}

// AdministrativeBoundary represents country/state/county boundaries
type AdministrativeBoundary struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name          string     `gorm:"type:varchar(255);not null" json:"name"`
	AdminLevel    int        `gorm:"not null" json:"admin_level"`
	CountryCode   *string    `gorm:"type:char(2)" json:"country_code,omitempty"`
	Geometry      string     `gorm:"type:geography(MULTIPOLYGON,4326);not null" json:"geometry"`
	Centroid      *string    `gorm:"type:geography(POINT,4326)" json:"centroid,omitempty"`
	Source        string     `gorm:"type:varchar(100);default:'natural_earth'" json:"source"`
	SourceVersion *string    `gorm:"type:varchar(50)" json:"source_version,omitempty"`
	CreatedAt     time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}

// TableName specifies the table name for GORM
func (AdministrativeBoundary) TableName() string {
	return "administrative_boundaries"
}

// Geofence represents a geographic alert zone
type Geofence struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name         string     `gorm:"type:varchar(255);not null" json:"name"`
	Description  *string    `gorm:"type:text" json:"description,omitempty"`
	Geometry     string     `gorm:"type:geography(POLYGON,4326);not null" json:"geometry"`
	GeofenceType string     `gorm:"type:varchar(50);not null" json:"geofence_type"`
	AlertRules   AlertRules `gorm:"type:jsonb;not null" json:"alert_rules"`
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	Priority     int        `gorm:"default:1" json:"priority"`
	Metadata     Metadata   `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	CreatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// TableName specifies the table name for GORM
func (Geofence) TableName() string {
	return "geofences"
}

// AlertRules defines when alerts should be triggered
type AlertRules struct {
	OnEnter         bool `json:"on_enter"`
	OnExit          bool `json:"on_exit"`
	OnProximity     bool `json:"on_proximity"`
	ProximityMeters int  `json:"proximity_meters"`
}

// Metadata stores additional flexible data
type Metadata map[string]interface{}

// GeofenceEvent represents a geofence crossing event
type GeofenceEvent struct {
	ID             uuid.UUID        `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	GeofenceID     uuid.UUID        `gorm:"type:uuid;not null" json:"geofence_id"`
	ProjectID      uuid.UUID        `gorm:"type:uuid;not null" json:"project_id"`
	EventType      string           `gorm:"type:varchar(50);not null" json:"event_type"`
	DistanceMeters *decimal.Decimal `gorm:"type:decimal(10,2)" json:"distance_meters,omitempty"`
	Location       *string          `gorm:"type:geography(POINT,4326)" json:"location,omitempty"`
	AlertGenerated bool             `gorm:"default:false" json:"alert_generated"`
	AlertID        *uuid.UUID       `gorm:"type:uuid" json:"alert_id,omitempty"`
	CreatedAt      time.Time        `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`

	// Relations
	Geofence Geofence `gorm:"foreignKey:GeofenceID" json:"geofence,omitempty"`
}

// TableName specifies the table name for GORM
func (GeofenceEvent) TableName() string {
	return "geofence_events"
}

// MapTileCache stores cached map tiles
type MapTileCache struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TileKey         string     `gorm:"type:varchar(500);not null;uniqueIndex" json:"tile_key"`
	TileData        []byte     `gorm:"type:bytea;not null" json:"-"`
	ContentType     string     `gorm:"type:varchar(50);not null" json:"content_type"`
	MapStyle        *string    `gorm:"type:varchar(100)" json:"map_style,omitempty"`
	ZoomLevel       *int       `json:"zoom_level,omitempty"`
	XCoordinate     *int       `json:"x_coordinate,omitempty"`
	YCoordinate     *int       `json:"y_coordinate,omitempty"`
	AccessedCount   int        `gorm:"default:0" json:"accessed_count"`
	LastAccessedAt  *time.Time `json:"last_accessed_at,omitempty"`
	ExpiresAt       time.Time  `gorm:"not null" json:"expires_at"`
	CreatedAt       time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}

// TableName specifies the table name for GORM
func (MapTileCache) TableName() string {
	return "map_tile_cache"
}

// ============================================================================
// Request/Response DTOs
// ============================================================================

// UploadGeometryRequest represents a request to upload project geometry
type UploadGeometryRequest struct {
	ProjectID               uuid.UUID `json:"project_id" binding:"required"`
	GeoJSON                 string    `json:"geojson" binding:"required"`
	SourceType              string    `json:"source_type" binding:"required,oneof=manual import satellite"`
	SourceFile              *string   `json:"source_file,omitempty"`
	SimplificationTolerance *float64  `json:"simplification_tolerance,omitempty"`
}

// GeometryResponse represents a geometry response
type GeometryResponse struct {
	ID                uuid.UUID       `json:"id"`
	ProjectID         uuid.UUID       `json:"project_id"`
	GeoJSON           interface{}     `json:"geojson"`
	Centroid          interface{}     `json:"centroid"`
	AreaHectares      decimal.Decimal `json:"area_hectares"`
	PerimeterMeters   *decimal.Decimal `json:"perimeter_meters,omitempty"`
	IsValid           bool            `json:"is_valid"`
	ValidationErrors  []string        `json:"validation_errors,omitempty"`
	Version           int             `json:"version"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// NearbyProjectsQuery represents a query for nearby projects
type NearbyProjectsQuery struct {
	Latitude  float64 `form:"lat" binding:"required,latitude"`
	Longitude float64 `form:"lng" binding:"required,longitude"`
	Radius    float64 `form:"radius" binding:"required,min=1,max=100000"` // in meters
	Limit     int     `form:"limit" binding:"omitempty,min=1,max=100"`
}

// WithinAreaQuery represents a query for projects within an area
type WithinAreaQuery struct {
	MinLat float64 `form:"min_lat" binding:"required,latitude"`
	MaxLat float64 `form:"max_lat" binding:"required,latitude"`
	MinLng float64 `form:"min_lng" binding:"required,longitude"`
	MaxLng float64 `form:"max_lng" binding:"required,longitude"`
	Limit  int     `form:"limit" binding:"omitempty,min=1,max=100"`
}

// IntersectionAnalysisRequest represents a request to analyze spatial intersections
type IntersectionAnalysisRequest struct {
	ProjectID uuid.UUID `json:"project_id" binding:"required"`
}

// IntersectionResult represents the result of an intersection analysis
type IntersectionResult struct {
	ProjectID          uuid.UUID       `json:"project_id"`
	IntersectingProjects []ProjectIntersection `json:"intersecting_projects"`
	TotalIntersections int             `json:"total_intersections"`
}

// ProjectIntersection represents a single project intersection
type ProjectIntersection struct {
	ProjectID         uuid.UUID       `json:"project_id"`
	IntersectionArea  decimal.Decimal `json:"intersection_area_hectares"`
	OverlapPercentage float64         `json:"overlap_percentage"`
}

// CreateGeofenceRequest represents a request to create a geofence
type CreateGeofenceRequest struct {
	Name         string     `json:"name" binding:"required"`
	Description  *string    `json:"description,omitempty"`
	GeoJSON      string     `json:"geojson" binding:"required"`
	GeofenceType string     `json:"geofence_type" binding:"required,oneof=protected_area restricted sensitive administrative"`
	AlertRules   AlertRules `json:"alert_rules" binding:"required"`
	Priority     *int       `json:"priority,omitempty"`
	Metadata     Metadata   `json:"metadata,omitempty"`
}

// StaticMapRequest represents a request for a static map
type StaticMapRequest struct {
	CenterLat   float64  `form:"center_lat" binding:"required,latitude"`
	CenterLng   float64  `form:"center_lng" binding:"required,longitude"`
	ZoomLevel   int      `form:"zoom" binding:"required,min=1,max=20"`
	Width       int      `form:"width" binding:"omitempty,min=100,max=2000"`
	Height      int      `form:"height" binding:"omitempty,min=100,max=2000"`
	MapStyle    string   `form:"style" binding:"omitempty"`
	Markers     []Marker `form:"markers,omitempty"`
	Overlays    []string `form:"overlays,omitempty"` // GeoJSON strings
}

// Marker represents a map marker
type Marker struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
	Label     string  `json:"label,omitempty"`
	Color     string  `json:"color,omitempty"`
}
