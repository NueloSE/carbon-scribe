package compliance

import (
	"context"

	"github.com/google/uuid"
)

// Service is the main interface for the compliance module
type Service interface {
	// Retention
	CreateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error
	ListRetentionPolicies(ctx context.Context) ([]RetentionPolicy, error)
	
	// Requests
	CreatePrivacyRequest(ctx context.Context, req *PrivacyRequest) error
	GetPrivacyRequest(ctx context.Context, id uuid.UUID) (*PrivacyRequest, error)
	
	// Preferences
	GetPrivacyPreferences(ctx context.Context, userID uuid.UUID) (*PrivacyPreference, error)
	UpdatePrivacyPreferences(ctx context.Context, prefs *PrivacyPreference) error
	
	// Consent
	RecordConsent(ctx context.Context, record *ConsentRecord) error
	WithdrawConsent(ctx context.Context, userID uuid.UUID, consentType string) error
	
	// Audit
	LogAction(ctx context.Context, log *AuditLog) error
}

type service struct {
	repo              Repository
	policyManager     retention.PolicyManager
	requestProcessor  requests.RequestProcessor
	preferenceManager privacy.PreferenceManager
	consentManager    privacy.ConsentManager
	auditLogger       audit.Logger // To be implemented
}

// NewService creates a new compliance service
func NewService(repo Repository) Service {
	return &service{
		repo:              repo,
		policyManager:     retention.NewPolicyManager(repo),
		requestProcessor:  requests.NewRequestProcessor(repo, requests.NewExporter(), requests.NewDeleter()),
		preferenceManager: privacy.NewPreferenceManager(repo),
		consentManager:    privacy.NewConsentManager(repo),
		auditLogger:       audit.NewLogger(repo),
	}
}

// --- Retention ---

func (s *service) CreateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error {
	return s.policyManager.CreateRetentionPolicy(ctx, policy)
}

func (s *service) ListRetentionPolicies(ctx context.Context) ([]RetentionPolicy, error) {
	return s.policyManager.ListPolicies(ctx)
}

// --- Requests ---

func (s *service) CreatePrivacyRequest(ctx context.Context, req *PrivacyRequest) error {
	return s.requestProcessor.ProcessRequest(ctx, req)
}

func (s *service) GetPrivacyRequest(ctx context.Context, id uuid.UUID) (*PrivacyRequest, error) {
	return s.requestProcessor.GetRequest(ctx, id)
}

// --- Preferences ---

func (s *service) GetPrivacyPreferences(ctx context.Context, userID uuid.UUID) (*PrivacyPreference, error) {
	return s.preferenceManager.GetPreferences(ctx, userID)
}

func (s *service) UpdatePrivacyPreferences(ctx context.Context, prefs *PrivacyPreference) error {
	return s.preferenceManager.UpdatePreferences(ctx, prefs)
}

// --- Consent ---

func (s *service) RecordConsent(ctx context.Context, record *ConsentRecord) error {
	return s.consentManager.RecordConsent(ctx, record)
}

func (s *service) WithdrawConsent(ctx context.Context, userID uuid.UUID, consentType string) error {
	return s.consentManager.WithdrawConsent(ctx, userID, consentType)
}

// --- Audit ---

func (s *service) LogAction(ctx context.Context, log *AuditLog) error {
	return s.auditLogger.LogAction(ctx, log)
}
