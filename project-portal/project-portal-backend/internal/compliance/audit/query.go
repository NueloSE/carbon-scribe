package audit

import (
	"context"
	"time"

	"carbon-scribe/project-portal/project-portal-backend/internal/compliance"
	"github.com/google/uuid"
)

// QueryService handles audit log queries
type QueryService interface {
	QueryLogs(ctx context.Context, filter compliance.AuditLogFilter) ([]compliance.AuditLog, error)
}

type queryService struct {
	repo compliance.Repository
}

func NewQueryService(repo compliance.Repository) QueryService {
	return &queryService{repo: repo}
}

func (s *queryService) QueryLogs(ctx context.Context, filter compliance.AuditLogFilter) ([]compliance.AuditLog, error) {
	// Add default limits if not set
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	if filter.EndTime == nil {
		now := time.Now()
		filter.EndTime = &now
	}
	
	return s.repo.QueryAuditLogs(ctx, filter)
}
