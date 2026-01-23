package audit

import (
	"context"
	"time"

	"carbon-scribe/project-portal/project-portal-backend/internal/compliance"
)

// Logger handles audit logging
type Logger interface {
	LogAction(ctx context.Context, log *compliance.AuditLog) error
}

type logger struct {
	repo compliance.Repository
}

// NewLogger creates a new audit logger
func NewLogger(repo compliance.Repository) Logger {
	return &logger{repo: repo}
}

func (l *logger) LogAction(ctx context.Context, log *compliance.AuditLog) error {
	// 1. Set timestamp if missing
	if log.EventTime.IsZero() {
		log.EventTime = time.Now()
	}

	// 2. Generate cryptographic signature (Immutability)
	// In a real app, we would sign the log content with a private key
	// or create a hash chain linking to the previous log.
	log.Signature = "mock-signature-" + log.EventTime.String()
	log.HashChain = "mock-hash-chain"

	// 3. Persist
	return l.repo.CreateAuditLog(ctx, log)
}
