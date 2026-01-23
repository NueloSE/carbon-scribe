-- CarbonScribe Geospatial Service Database Migration
-- Migration: 011_geospatial_tables.sql
-- Description: Creates PostGIS-enabled tables for geospatial data management
-- Author: CarbonScribe Team
-- Date: 2025-01-21

-- Enable PostGIS extension
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS postgis_topology;

-- ============================================================================
-- Table: project_geometries
-- Description: Stores project boundary geometries with spatial indexing
-- ============================================================================
CREATE TABLE IF NOT EXISTS project_geometries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,

    -- Geometry storage (WGS84 - SRID 4326)
    geometry GEOGRAPHY(POLYGON, 4326) NOT NULL,
    centroid GEOGRAPHY(POINT, 4326) NOT NULL,
    bounding_box GEOGRAPHY(POLYGON, 4326),

    -- Derived properties
    area_hectares DECIMAL(12, 4) NOT NULL,
    perimeter_meters DECIMAL(12, 4),

    -- Validation metadata
    is_valid BOOLEAN DEFAULT TRUE,
    validation_errors TEXT[],
    simplification_tolerance DECIMAL(10, 6), -- Tolerance used for simplification

    -- Source information
    source_type VARCHAR(50) DEFAULT 'manual', -- 'manual', 'import', 'satellite'
    source_file VARCHAR(500),
    accuracy_score DECIMAL(3, 2),

    -- Versioning
    version INTEGER DEFAULT 1,
    previous_version_id UUID REFERENCES project_geometries(id),

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Spatial indexes for performance
CREATE INDEX idx_project_geometries_geometry ON project_geometries USING GIST (geometry);
CREATE INDEX idx_project_geometries_centroid ON project_geometries USING GIST (centroid);
CREATE INDEX idx_project_geometries_bounding_box ON project_geometries USING GIST (bounding_box);
CREATE INDEX idx_project_geometries_project_id ON project_geometries (project_id);
CREATE INDEX idx_project_geometries_created_at ON project_geometries (created_at);

-- ============================================================================
-- Table: administrative_boundaries
-- Description: Reference data for country/state/county boundaries
-- ============================================================================
CREATE TABLE IF NOT EXISTS administrative_boundaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    admin_level INTEGER NOT NULL, -- 0: country, 1: state, 2: county, etc.
    country_code CHAR(2),

    -- Geometry
    geometry GEOGRAPHY(MULTIPOLYGON, 4326) NOT NULL,
    centroid GEOGRAPHY(POINT, 4326),

    -- Metadata
    source VARCHAR(100) DEFAULT 'natural_earth',
    source_version VARCHAR(50),

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_admin_boundaries_geometry ON administrative_boundaries USING GIST (geometry);
CREATE INDEX idx_admin_boundaries_centroid ON administrative_boundaries USING GIST (centroid);
CREATE INDEX idx_admin_boundaries_admin_level ON administrative_boundaries (admin_level);
CREATE INDEX idx_admin_boundaries_country_code ON administrative_boundaries (country_code);

-- ============================================================================
-- Table: geofences
-- Description: Geographic zones for location-based alerts
-- ============================================================================
CREATE TABLE IF NOT EXISTS geofences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- Geometry and type
    geometry GEOGRAPHY(POLYGON, 4326) NOT NULL,
    geofence_type VARCHAR(50) NOT NULL, -- 'protected_area', 'restricted', 'sensitive', 'administrative'

    -- Alert configuration
    alert_rules JSONB NOT NULL DEFAULT '{
        "on_enter": true,
        "on_exit": false,
        "on_proximity": true,
        "proximity_meters": 1000
    }',

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    priority INTEGER DEFAULT 1, -- Higher number = higher priority

    -- Metadata
    metadata JSONB DEFAULT '{}',

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_geofences_geometry ON geofences USING GIST (geometry);
CREATE INDEX idx_geofences_geofence_type ON geofences (geofence_type);
CREATE INDEX idx_geofences_is_active ON geofences (is_active);
CREATE INDEX idx_geofences_priority ON geofences (priority);

-- ============================================================================
-- Table: geofence_events
-- Description: Log of geofence crossing events
-- ============================================================================
CREATE TABLE IF NOT EXISTS geofence_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    geofence_id UUID NOT NULL REFERENCES geofences(id),
    project_id UUID NOT NULL REFERENCES projects(id),

    -- Event details
    event_type VARCHAR(50) NOT NULL, -- 'enter', 'exit', 'proximity'
    distance_meters DECIMAL(10, 2), -- Distance to geofence if proximity event

    -- Location at time of event
    location GEOGRAPHY(POINT, 4326),

    -- Alert status
    alert_generated BOOLEAN DEFAULT FALSE,
    alert_id UUID REFERENCES alerts(id),

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_geofence_events_geofence_id ON geofence_events (geofence_id);
CREATE INDEX idx_geofence_events_project_id ON geofence_events (project_id);
CREATE INDEX idx_geofence_events_event_type ON geofence_events (event_type);
CREATE INDEX idx_geofence_events_created_at ON geofence_events (created_at);

-- ============================================================================
-- Table: map_tile_cache
-- Description: Cache for map tiles to improve performance
-- ============================================================================
CREATE TABLE IF NOT EXISTS map_tile_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tile_key VARCHAR(500) UNIQUE NOT NULL, -- Composite key: z/x/y/style/timestamp
    tile_data BYTEA NOT NULL, -- PNG or vector tile data
    content_type VARCHAR(50) NOT NULL, -- 'image/png', 'application/vnd.mapbox-vector-tile'

    -- Metadata
    map_style VARCHAR(100),
    zoom_level INTEGER,
    x_coordinate INTEGER,
    y_coordinate INTEGER,

    -- Cache management
    accessed_count INTEGER DEFAULT 0,
    last_accessed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_map_tile_cache_key ON map_tile_cache (tile_key);
CREATE INDEX idx_map_tile_cache_expiry ON map_tile_cache (expires_at);
CREATE INDEX idx_map_tile_cache_zoom_coords ON map_tile_cache (zoom_level, x_coordinate, y_coordinate);

-- ============================================================================
-- Triggers for updated_at timestamps
-- ============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_project_geometries_updated_at BEFORE UPDATE ON project_geometries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_geofences_updated_at BEFORE UPDATE ON geofences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Comments for documentation
-- ============================================================================
COMMENT ON TABLE project_geometries IS 'Stores project boundary geometries with version history and spatial indexing';
COMMENT ON TABLE administrative_boundaries IS 'Reference data for country/state/county boundaries from Natural Earth';
COMMENT ON TABLE geofences IS 'Geographic zones for location-based alerts and monitoring';
COMMENT ON TABLE geofence_events IS 'Log of all geofence crossing events for analytics';
COMMENT ON TABLE map_tile_cache IS 'Performance cache for map tiles with TTL management';

COMMENT ON COLUMN project_geometries.geometry IS 'Project boundary polygon in WGS84 (SRID 4326)';
COMMENT ON COLUMN project_geometries.centroid IS 'Geographic center point of the project';
COMMENT ON COLUMN project_geometries.area_hectares IS 'Calculated area in hectares using geodesic calculations';
COMMENT ON COLUMN project_geometries.version IS 'Version number for audit trail';

COMMENT ON COLUMN geofences.alert_rules IS 'JSON configuration for alert triggers (on_enter, on_exit, on_proximity)';
COMMENT ON COLUMN geofences.priority IS 'Alert priority level (higher number = higher priority)';

-- ============================================================================
-- Sample data notes
-- ============================================================================
-- Note: This migration creates tables only.
-- Administrative boundaries data should be loaded separately from Natural Earth datasets.
-- Geofences should be created through the API endpoints.
