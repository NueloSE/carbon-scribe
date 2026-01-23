package queries

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// IntersectionQuery handles intersection and overlap queries
type IntersectionQuery struct {
	db *gorm.DB
}

// NewIntersectionQuery creates a new intersection query handler
func NewIntersectionQuery(db *gorm.DB) *IntersectionQuery {
	return &IntersectionQuery{db: db}
}

// IntersectionResult represents an intersection between two projects
type IntersectionResult struct {
	ProjectID              uuid.UUID       `json:"project_id"`
	IntersectionAreaM2     decimal.Decimal `json:"intersection_area_m2"`
	IntersectionAreaHa     decimal.Decimal `json:"intersection_area_hectares"`
	OverlapPercentage      float64         `json:"overlap_percentage"`
	OverlapPercentageOther float64         `json:"overlap_percentage_other"`
	IntersectionType       string          `json:"intersection_type"` // 'contains', 'within', 'overlaps'
}

// FindOverlappingProjects finds all projects that overlap with a given project
func (iq *IntersectionQuery) FindOverlappingProjects(projectID uuid.UUID) ([]IntersectionResult, error) {
	var results []IntersectionResult

	query := `
		SELECT
			pg2.project_id,
			ST_Area(ST_Intersection(pg1.geometry, pg2.geometry)) as intersection_area_m2,
			ST_Area(ST_Intersection(pg1.geometry, pg2.geometry)) / 10000 as intersection_area_ha,
			(ST_Area(ST_Intersection(pg1.geometry, pg2.geometry)) / ST_Area(pg1.geometry)) * 100 as overlap_percentage,
			(ST_Area(ST_Intersection(pg1.geometry, pg2.geometry)) / ST_Area(pg2.geometry)) * 100 as overlap_percentage_other,
			CASE
				WHEN ST_Contains(pg1.geometry, pg2.geometry) THEN 'contains'
				WHEN ST_Within(pg1.geometry, pg2.geometry) THEN 'within'
				ELSE 'overlaps'
			END as intersection_type
		FROM project_geometries pg1
		CROSS JOIN project_geometries pg2
		WHERE pg1.project_id = $1
		AND pg2.project_id != $1
		AND ST_Intersects(pg1.geometry, pg2.geometry)
		AND ST_Area(ST_Intersection(pg1.geometry, pg2.geometry)) > 0
		ORDER BY intersection_area_ha DESC
	`

	err := iq.db.Raw(query, projectID).Scan(&results).Error
	return results, err
}

// CheckOverlapWithGeometry checks if a geometry overlaps with existing projects
func (iq *IntersectionQuery) CheckOverlapWithGeometry(geometryWKT string, excludeProjectID *uuid.UUID) ([]IntersectionResult, error) {
	var results []IntersectionResult

	query := `
		SELECT
			project_id,
			ST_Area(ST_Intersection(geometry, ST_GeomFromText($1, 4326)::geography)) as intersection_area_m2,
			ST_Area(ST_Intersection(geometry, ST_GeomFromText($1, 4326)::geography)) / 10000 as intersection_area_ha,
			(ST_Area(ST_Intersection(geometry, ST_GeomFromText($1, 4326)::geography)) / ST_Area(geometry)) * 100 as overlap_percentage,
			(ST_Area(ST_Intersection(geometry, ST_GeomFromText($1, 4326)::geography)) / ST_Area(ST_GeomFromText($1, 4326)::geography)) * 100 as overlap_percentage_other,
			CASE
				WHEN ST_Contains(geometry, ST_GeomFromText($1, 4326)::geography) THEN 'contains'
				WHEN ST_Within(geometry, ST_GeomFromText($1, 4326)::geography) THEN 'within'
				ELSE 'overlaps'
			END as intersection_type
		FROM project_geometries
		WHERE ($2::uuid IS NULL OR project_id != $2)
		AND ST_Intersects(geometry, ST_GeomFromText($1, 4326)::geography)
		AND ST_Area(ST_Intersection(geometry, ST_GeomFromText($1, 4326)::geography)) > 0
		ORDER BY intersection_area_ha DESC
	`

	err := iq.db.Raw(query, geometryWKT, excludeProjectID).Scan(&results).Error
	return results, err
}

// FindProjectsContaining finds projects that completely contain a geometry
func (iq *IntersectionQuery) FindProjectsContaining(geometryWKT string) ([]uuid.UUID, error) {
	var projectIDs []uuid.UUID

	query := `
		SELECT project_id
		FROM project_geometries
		WHERE ST_Contains(geometry, ST_GeomFromText($1, 4326)::geography)
	`

	err := iq.db.Raw(query, geometryWKT).Scan(&projectIDs).Error
	return projectIDs, err
}

// FindProjectsWithin finds projects completely within a geometry
func (iq *IntersectionQuery) FindProjectsWithin(geometryWKT string) ([]uuid.UUID, error) {
	var projectIDs []uuid.UUID

	query := `
		SELECT project_id
		FROM project_geometries
		WHERE ST_Within(geometry, ST_GeomFromText($1, 4326)::geography)
	`

	err := iq.db.Raw(query, geometryWKT).Scan(&projectIDs).Error
	return projectIDs, err
}

// AdministrativeBoundaryResult represents a project's administrative location
type AdministrativeBoundaryResult struct {
	ProjectID        uuid.UUID `json:"project_id"`
	BoundaryID       uuid.UUID `json:"boundary_id"`
	BoundaryName     string    `json:"boundary_name"`
	AdminLevel       int       `json:"admin_level"`
	CountryCode      *string   `json:"country_code,omitempty"`
	ContainmentType  string    `json:"containment_type"` // 'fully_within', 'partially_within'
	OverlapPercentage float64  `json:"overlap_percentage"`
}

// FindProjectsByAdministrativeBoundary finds projects within an administrative boundary
func (iq *IntersectionQuery) FindProjectsByAdministrativeBoundary(boundaryID uuid.UUID) ([]AdministrativeBoundaryResult, error) {
	var results []AdministrativeBoundaryResult

	query := `
		SELECT
			pg.project_id,
			ab.id as boundary_id,
			ab.name as boundary_name,
			ab.admin_level,
			ab.country_code,
			CASE
				WHEN ST_Within(pg.geometry, ab.geometry) THEN 'fully_within'
				ELSE 'partially_within'
			END as containment_type,
			(ST_Area(ST_Intersection(pg.geometry, ab.geometry)) / ST_Area(pg.geometry)) * 100 as overlap_percentage
		FROM project_geometries pg
		CROSS JOIN administrative_boundaries ab
		WHERE ab.id = $1
		AND ST_Intersects(pg.geometry, ab.geometry)
		ORDER BY overlap_percentage DESC
	`

	err := iq.db.Raw(query, boundaryID).Scan(&results).Error
	return results, err
}

// GetProjectAdministrativeBoundaries returns all administrative boundaries a project intersects
func (iq *IntersectionQuery) GetProjectAdministrativeBoundaries(projectID uuid.UUID, adminLevel *int) ([]AdministrativeBoundaryResult, error) {
	var results []AdministrativeBoundaryResult

	query := `
		SELECT
			pg.project_id,
			ab.id as boundary_id,
			ab.name as boundary_name,
			ab.admin_level,
			ab.country_code,
			CASE
				WHEN ST_Within(pg.geometry, ab.geometry) THEN 'fully_within'
				ELSE 'partially_within'
			END as containment_type,
			(ST_Area(ST_Intersection(pg.geometry, ab.geometry)) / ST_Area(pg.geometry)) * 100 as overlap_percentage
		FROM project_geometries pg
		CROSS JOIN administrative_boundaries ab
		WHERE pg.project_id = $1
		AND ($2::int IS NULL OR ab.admin_level = $2)
		AND ST_Intersects(pg.geometry, ab.geometry)
		ORDER BY ab.admin_level ASC, overlap_percentage DESC
	`

	err := iq.db.Raw(query, projectID, adminLevel).Scan(&results).Error
	return results, err
}

// SpatialRelationship represents the relationship between two geometries
type SpatialRelationship struct {
	Intersects bool `json:"intersects"`
	Contains   bool `json:"contains"`
	Within     bool `json:"within"`
	Touches    bool `json:"touches"`
	Crosses    bool `json:"crosses"`
	Overlaps   bool `json:"overlaps"`
	Disjoint   bool `json:"disjoint"`
}

// AnalyzeSpatialRelationship analyzes all spatial relationships between two projects
func (iq *IntersectionQuery) AnalyzeSpatialRelationship(projectID1, projectID2 uuid.UUID) (*SpatialRelationship, error) {
	var result SpatialRelationship

	query := `
		SELECT
			ST_Intersects(pg1.geometry, pg2.geometry) as intersects,
			ST_Contains(pg1.geometry, pg2.geometry) as contains,
			ST_Within(pg1.geometry, pg2.geometry) as within,
			ST_Touches(pg1.geometry, pg2.geometry) as touches,
			ST_Crosses(pg1.geometry, pg2.geometry) as crosses,
			ST_Overlaps(pg1.geometry, pg2.geometry) as overlaps,
			ST_Disjoint(pg1.geometry, pg2.geometry) as disjoint
		FROM project_geometries pg1
		CROSS JOIN project_geometries pg2
		WHERE pg1.project_id = $1
		AND pg2.project_id = $2
	`

	err := iq.db.Raw(query, projectID1, projectID2).Scan(&result).Error
	return &result, err
}

// OverlapSummary provides summary statistics about overlaps
type OverlapSummary struct {
	TotalOverlaps       int             `json:"total_overlaps"`
	TotalOverlapAreaHa  decimal.Decimal `json:"total_overlap_area_hectares"`
	MaxOverlapAreaHa    decimal.Decimal `json:"max_overlap_area_hectares"`
	AvgOverlapPercentage float64        `json:"avg_overlap_percentage"`
}

// GetOverlapSummary returns summary statistics about a project's overlaps
func (iq *IntersectionQuery) GetOverlapSummary(projectID uuid.UUID) (*OverlapSummary, error) {
	var summary OverlapSummary

	query := `
		SELECT
			COUNT(*) as total_overlaps,
			COALESCE(SUM(ST_Area(ST_Intersection(pg1.geometry, pg2.geometry)) / 10000), 0) as total_overlap_area_ha,
			COALESCE(MAX(ST_Area(ST_Intersection(pg1.geometry, pg2.geometry)) / 10000), 0) as max_overlap_area_ha,
			COALESCE(AVG((ST_Area(ST_Intersection(pg1.geometry, pg2.geometry)) / ST_Area(pg1.geometry)) * 100), 0) as avg_overlap_percentage
		FROM project_geometries pg1
		CROSS JOIN project_geometries pg2
		WHERE pg1.project_id = $1
		AND pg2.project_id != $1
		AND ST_Intersects(pg1.geometry, pg2.geometry)
		AND ST_Area(ST_Intersection(pg1.geometry, pg2.geometry)) > 0
	`

	err := iq.db.Raw(query, projectID).Scan(&summary).Error
	return &summary, err
}
