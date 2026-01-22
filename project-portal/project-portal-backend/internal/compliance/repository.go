package compliance

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines the interface for compliance data access
type Repository interface {
	// Retention Policies
	CreateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error
	GetRetentionPolicy(ctx context.Context, id uuid.UUID) (*RetentionPolicy, error)
	ListRetentionPolicies(ctx context.Context) ([]RetentionPolicy, error)
	UpdateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error
	DeleteRetentionPolicy(ctx context.Context, id uuid.UUID) error

	// Privacy Requests
	CreatePrivacyRequest(ctx context.Context, request *PrivacyRequest) error
	GetPrivacyRequest(ctx context.Context, id uuid.UUID) (*PrivacyRequest, error)
	UpdatePrivacyRequest(ctx context.Context, request *PrivacyRequest) error
	ListPrivacyRequests(ctx context.Context, userID uuid.UUID) ([]PrivacyRequest, error)

	// Privacy Preferences
	GetPrivacyPreferences(ctx context.Context, userID uuid.UUID) (*PrivacyPreference, error)
	UpsertPrivacyPreferences(ctx context.Context, prefs *PrivacyPreference) error

	// Consent Records
	CreateConsentRecord(ctx context.Context, record *ConsentRecord) error
	GetLatestConsent(ctx context.Context, userID uuid.UUID, consentType string) (*ConsentRecord, error)
	WithdrawConsent(ctx context.Context, userID uuid.UUID, consentType string) error

	// Audit Logs
	CreateAuditLog(ctx context.Context, log *AuditLog) error
	QueryAuditLogs(ctx context.Context, filter AuditLogFilter) ([]AuditLog, error)
}

// AuditLogFilter defines filters for querying audit logs
type AuditLogFilter struct {
	StartTime     *time.Time
	EndTime       *time.Time
	ActorID       *uuid.UUID
	TargetID      *uuid.UUID
	EventType     string
	Limit         int
	Offset        int
}

type repository struct {
	db *gorm.DB
}

// NewRepository creates a new compliance repository
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// --- Retention Policies ---

func (r *repository) CreateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error {
	return r.db.WithContext(ctx).Create(policy).Error
}

func (r *repository) GetRetentionPolicy(ctx context.Context, id uuid.UUID) (*RetentionPolicy, error) {
	var policy RetentionPolicy
	if err := r.db.WithContext(ctx).First(&policy, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *repository) ListRetentionPolicies(ctx context.Context) ([]RetentionPolicy, error) {
	var policies []RetentionPolicy
	if err := r.db.WithContext(ctx).Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *repository) UpdateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

func (r *repository) DeleteRetentionPolicy(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&RetentionPolicy{}, "id = ?", id).Error
}

// --- Privacy Requests ---

func (r *repository) CreatePrivacyRequest(ctx context.Context, request *PrivacyRequest) error {
	return r.db.WithContext(ctx).Create(request).Error
}

func (r *repository) GetPrivacyRequest(ctx context.Context, id uuid.UUID) (*PrivacyRequest, error) {
	var req PrivacyRequest
	if err := r.db.WithContext(ctx).First(&req, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *repository) UpdatePrivacyRequest(ctx context.Context, request *PrivacyRequest) error {
	return r.db.WithContext(ctx).Save(request).Error
}

func (r *repository) ListPrivacyRequests(ctx context.Context, userID uuid.UUID) ([]PrivacyRequest, error) {
	var requests []PrivacyRequest
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&requests).Error; err != nil {
		return nil, err
	}
	return requests, nil
}

// --- Privacy Preferences ---

func (r *repository) GetPrivacyPreferences(ctx context.Context, userID uuid.UUID) (*PrivacyPreference, error) {
	var prefs PrivacyPreference
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&prefs).Error; err != nil {
		return nil, err
	}
	return &prefs, nil
}

func (r *repository) UpsertPrivacyPreferences(ctx context.Context, prefs *PrivacyPreference) error {
	// Check if exists to handle versioning logic if needed, but simple upsert for now
	// Using Save which performs upsert if ID is present, or create if not.
	// However, since we want to handle unique user_id, we might need a specific query.
	// For now, assuming the service layer handles fetching and setting ID if it exists.
	return r.db.WithContext(ctx).Save(prefs).Error
}

// --- Consent Records ---

func (r *repository) CreateConsentRecord(ctx context.Context, record *ConsentRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *repository) GetLatestConsent(ctx context.Context, userID uuid.UUID, consentType string) (*ConsentRecord, error) {
	var record ConsentRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND consent_type = ?", userID, consentType).
		Order("created_at DESC").
		First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *repository) WithdrawConsent(ctx context.Context, userID uuid.UUID, consentType string) error {
	// Find latest active consent and mark withdrawn
	// This is a simplified logic; real logic might need to insert a new record indicating withdrawal
	// or update the existing one. Based on schema, we have withdrawn_at.
	return r.db.WithContext(ctx).
		Model(&ConsentRecord{}).
		Where("user_id = ? AND consent_type = ? AND withdrawn_at IS NULL", userID, consentType).
		Update("withdrawn_at", time.Now()).Error
}

// --- Audit Logs ---

func (r *repository) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *repository) QueryAuditLogs(ctx context.Context, filter AuditLogFilter) ([]AuditLog, error) {
	query := r.db.WithContext(ctx).Model(&AuditLog{})

	if filter.StartTime != nil {
		query = query.Where("event_time >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("event_time <= ?", filter.EndTime)
	}
	if filter.ActorID != nil {
		query = query.Where("actor_id = ?", filter.ActorID)
	}
	if filter.TargetID != nil {
		query = query.Where("target_id = ?", filter.TargetID)
	}
	if filter.EventType != "" {
		query = query.Where("event_type = ?", filter.EventType)
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var logs []AuditLog
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}
