package retention

import (
	"context"
	"errors"

	"carbon-scribe/project-portal/project-portal-backend/internal/compliance"
	"github.com/google/uuid"
)

// PolicyManager manages retention policies
type PolicyManager interface {
	CreatePolicy(ctx context.Context, policy *compliance.RetentionPolicy) error
	GetPolicy(ctx context.Context, id uuid.UUID) (*compliance.RetentionPolicy, error)
	ListPolicies(ctx context.Context) ([]compliance.RetentionPolicy, error)
	UpdatePolicy(ctx context.Context, policy *compliance.RetentionPolicy) error
	DeletePolicy(ctx context.Context, id uuid.UUID) error
}

type policyManager struct {
	repo compliance.Repository
}

// NewPolicyManager creates a new policy manager
func NewPolicyManager(repo compliance.Repository) PolicyManager {
	return &policyManager{repo: repo}
}

func (m *policyManager) CreatePolicy(ctx context.Context, policy *compliance.RetentionPolicy) error {
	if policy.Name == "" {
		return errors.New("policy name is required")
	}
	if policy.DataCategory == "" {
		return errors.New("data category is required")
	}
	return m.repo.CreateRetentionPolicy(ctx, policy)
}

func (m *policyManager) GetPolicy(ctx context.Context, id uuid.UUID) (*compliance.RetentionPolicy, error) {
	return m.repo.GetRetentionPolicy(ctx, id)
}

func (m *policyManager) ListPolicies(ctx context.Context) ([]compliance.RetentionPolicy, error) {
	return m.repo.ListRetentionPolicies(ctx)
}

func (m *policyManager) UpdatePolicy(ctx context.Context, policy *compliance.RetentionPolicy) error {
	// In a real app, we might want to fetch the existing policy to check for version conflicts
	// or to ensure we're not modifying a deleted policy.
	policy.Version++
	return m.repo.UpdateRetentionPolicy(ctx, policy)
}

func (m *policyManager) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	return m.repo.DeleteRetentionPolicy(ctx, id)
}
