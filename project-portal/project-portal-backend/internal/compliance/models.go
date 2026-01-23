package compliance

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// RetentionPolicy represents a data retention policy
type RetentionPolicy struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name               string         `gorm:"type:varchar(255);not null" json:"name"`
	Description        string         `gorm:"type:text" json:"description"`
	DataCategory       string         `gorm:"type:varchar(100);not null" json:"data_category"`
	Jurisdiction       string         `gorm:"type:varchar(50);default:'global'" json:"jurisdiction"`
	RetentionPeriodDays int           `gorm:"not null" json:"retention_period_days"`
	ArchivalPeriodDays  *int          `json:"archival_period_days"`
	ReviewPeriodDays    int           `gorm:"default:365" json:"review_period_days"`
	DeletionMethod      string        `gorm:"type:varchar(50);default:'soft_delete'" json:"deletion_method"`
	AnonymizationRules  datatypes.JSON `gorm:"type:jsonb" json:"anonymization_rules"`
	LegalHoldEnabled    bool          `gorm:"default:true" json:"legal_hold_enabled"`
	IsActive            bool          `gorm:"default:true" json:"is_active"`
	Version             int           `gorm:"default:1" json:"version"`
	CreatedAt           time.Time     `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt           time.Time     `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// PrivacyRequest represents a GDPR/privacy request (export, deletion, etc.)
type PrivacyRequest struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID              uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	RequestType         string         `gorm:"type:varchar(50);not null" json:"request_type"`
	RequestSubtype      string         `gorm:"type:varchar(50)" json:"request_subtype"`
	Status              string         `gorm:"type:varchar(50);default:'received'" json:"status"`
	SubmittedAt         time.Time      `gorm:"default:CURRENT_TIMESTAMP;not null" json:"submitted_at"`
	CompletedAt         *time.Time     `json:"completed_at"`
	EstimatedCompletion *time.Time     `json:"estimated_completion"`
	DataCategories      []string       `gorm:"type:text[]" json:"data_categories"`
	DateRangeStart      *time.Time     `json:"date_range_start"`
	DateRangeEnd        *time.Time     `json:"date_range_end"`
	VerificationMethod  string         `gorm:"type:varchar(50)" json:"verification_method"`
	VerifiedBy          *uuid.UUID     `gorm:"type:uuid" json:"verified_by"`
	VerifiedAt          *time.Time     `json:"verified_at"`
	ExportFileURL       string         `gorm:"type:text" json:"export_file_url"`
	ExportFileHash      string         `gorm:"type:varchar(64)" json:"export_file_hash"`
	DeletionSummary     datatypes.JSON `gorm:"type:jsonb" json:"deletion_summary"`
	ErrorMessage        string         `gorm:"type:text" json:"error_message"`
	LegalBasis          string         `gorm:"type:varchar(100)" json:"legal_basis"`
	CreatedAt           time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt           time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// PrivacyPreference represents user privacy settings
type PrivacyPreference struct {
	ID                      uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID                  uuid.UUID `gorm:"type:uuid;not null;unique" json:"user_id"`
	MarketingEmails         bool      `gorm:"default:false" json:"marketing_emails"`
	PromotionalEmails       bool      `gorm:"default:false" json:"promotional_emails"`
	SystemNotifications     bool      `gorm:"default:true" json:"system_notifications"`
	ThirdPartySharing       bool      `gorm:"default:false" json:"third_party_sharing"`
	AnalyticsTracking       bool      `gorm:"default:true" json:"analytics_tracking"`
	DataRetentionConsent    bool      `gorm:"default:true" json:"data_retention_consent"`
	ResearchParticipation   bool      `gorm:"default:false" json:"research_participation"`
	AutomatedDecisionMaking bool      `gorm:"default:false" json:"automated_decision_making"`
	Jurisdiction            string    `gorm:"type:varchar(50);default:'GDPR'" json:"jurisdiction"`
	Version                 int       `gorm:"default:1" json:"version"`
	PreviousVersionID       *uuid.UUID `gorm:"type:uuid" json:"previous_version_id"`
	CreatedAt               time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt               time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// ConsentRecord represents a granular consent action
type ConsentRecord struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	ConsentType    string    `gorm:"type:varchar(100);not null" json:"consent_type"`
	ConsentVersion string    `gorm:"type:varchar(50);not null" json:"consent_version"`
	ConsentGiven   bool      `gorm:"not null" json:"consent_given"`
	Context        string    `gorm:"type:text" json:"context"`
	Purpose        string    `gorm:"type:text" json:"purpose"`
	IPAddress      string    `gorm:"type:inet" json:"ip_address"`
	UserAgent      string    `gorm:"type:text" json:"user_agent"`
	Geolocation    string    `gorm:"type:varchar(100)" json:"geolocation"`
	ExpiresAt      *time.Time `json:"expires_at"`
	WithdrawnAt    *time.Time `json:"withdrawn_at"`
	CreatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}

// AuditLog represents an immutable audit log entry
type AuditLog struct {
	LogID            int64          `gorm:"primary_key;autoIncrement" json:"log_id"`
	EventTime        time.Time      `gorm:"primary_key;default:CURRENT_TIMESTAMP" json:"event_time"`
	EventType        string         `gorm:"type:varchar(100);not null" json:"event_type"`
	EventAction      string         `gorm:"type:varchar(50);not null" json:"event_action"`
	ActorID          *uuid.UUID     `gorm:"type:uuid" json:"actor_id"`
	ActorType        string         `gorm:"type:varchar(50)" json:"actor_type"`
	ActorIP          string         `gorm:"type:inet" json:"actor_ip"`
	TargetType       string         `gorm:"type:varchar(100)" json:"target_type"`
	TargetID         *uuid.UUID     `gorm:"type:uuid" json:"target_id"`
	TargetOwnerID    *uuid.UUID     `gorm:"type:uuid" json:"target_owner_id"`
	DataCategory     string         `gorm:"type:varchar(100)" json:"data_category"`
	SensitivityLevel string         `gorm:"type:varchar(50);default:'normal'" json:"sensitivity_level"`
	ServiceName      string         `gorm:"type:varchar(100);not null" json:"service_name"`
	Endpoint         string         `gorm:"type:varchar(500)" json:"endpoint"`
	HTTPMethod       string         `gorm:"type:varchar(10)" json:"http_method"`
	OldValues        datatypes.JSON `gorm:"type:jsonb" json:"old_values"`
	NewValues        datatypes.JSON `gorm:"type:jsonb" json:"new_values"`
	PermissionUsed   string         `gorm:"type:varchar(100)" json:"permission_used"`
	Signature        string         `gorm:"type:varchar(512)" json:"signature"`
	HashChain        string         `gorm:"type:varchar(64)" json:"hash_chain"`
	CreatedAt        time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}

// RetentionSchedule represents the schedule for data retention actions
type RetentionSchedule struct {
	ID                 uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PolicyID           uuid.UUID `gorm:"type:uuid;not null" json:"policy_id"`
	DataType           string    `gorm:"type:varchar(100);not null" json:"data_type"`
	NextReviewDate     time.Time `gorm:"type:date;not null" json:"next_review_date"`
	NextActionDate     *time.Time `gorm:"type:date" json:"next_action_date"`
	ActionType         string    `gorm:"type:varchar(50)" json:"action_type"`
	LastActionDate     *time.Time `gorm:"type:date" json:"last_action_date"`
	LastActionType     string    `gorm:"type:varchar(50)" json:"last_action_type"`
	LastActionResult   string    `gorm:"type:varchar(50)" json:"last_action_result"`
	RecordCountEstimate int64     `json:"record_count_estimate"`
	CreatedAt          time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt          time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}
