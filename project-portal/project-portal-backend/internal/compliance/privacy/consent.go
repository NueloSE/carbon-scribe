package privacy

import (
	"context"
	"time"

	"carbon-scribe/project-portal/project-portal-backend/internal/compliance"
	"github.com/google/uuid"
)

// ConsentManager manages user consents
type ConsentManager interface {
	RecordConsent(ctx context.Context, record *compliance.ConsentRecord) error
	WithdrawConsent(ctx context.Context, userID uuid.UUID, consentType string) error
}

type consentManager struct {
	repo compliance.Repository
}

// NewConsentManager creates a new consent manager
func NewConsentManager(repo compliance.Repository) ConsentManager {
	return &consentManager{repo: repo}
}

func (m *consentManager) RecordConsent(ctx context.Context, record *compliance.ConsentRecord) error {
	record.ConsentGiven = true
	record.CreatedAt = time.Now()
	return m.repo.CreateConsentRecord(ctx, record)
}

func (m *consentManager) WithdrawConsent(ctx context.Context, userID uuid.UUID, consentType string) error {
	return m.repo.WithdrawConsent(ctx, userID, consentType)
}
