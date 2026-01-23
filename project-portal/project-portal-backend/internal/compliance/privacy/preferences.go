package privacy

import (
	"context"

	"carbon-scribe/project-portal/project-portal-backend/internal/compliance"
	"github.com/google/uuid"
)

// PreferenceManager manages user privacy preferences
type PreferenceManager interface {
	GetPreferences(ctx context.Context, userID uuid.UUID) (*compliance.PrivacyPreference, error)
	UpdatePreferences(ctx context.Context, prefs *compliance.PrivacyPreference) error
}

type preferenceManager struct {
	repo compliance.Repository
}

// NewPreferenceManager creates a new preference manager
func NewPreferenceManager(repo compliance.Repository) PreferenceManager {
	return &preferenceManager{repo: repo}
}

func (m *preferenceManager) GetPreferences(ctx context.Context, userID uuid.UUID) (*compliance.PrivacyPreference, error) {
	prefs, err := m.repo.GetPrivacyPreferences(ctx, userID)
	if err != nil {
		// If not found, return default preferences
		// In a real app, we might want to create them on the fly or return a specific error
		return &compliance.PrivacyPreference{
			UserID:               userID,
			SystemNotifications:  true,
			AnalyticsTracking:    true,
			DataRetentionConsent: true,
			Jurisdiction:         "GDPR",
		}, nil
	}
	return prefs, nil
}

func (m *preferenceManager) UpdatePreferences(ctx context.Context, prefs *compliance.PrivacyPreference) error {
	// Versioning logic could go here
	prefs.Version++
	return m.repo.UpsertPrivacyPreferences(ctx, prefs)
}
