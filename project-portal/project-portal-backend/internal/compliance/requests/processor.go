package requests

import (
	"context"
	"errors"
	"time"

	"carbon-scribe/project-portal/project-portal-backend/internal/compliance"
	"github.com/google/uuid"
)

// RequestProcessor handles privacy requests
type RequestProcessor interface {
	ProcessRequest(ctx context.Context, req *compliance.PrivacyRequest) error
	GetRequest(ctx context.Context, id uuid.UUID) (*compliance.PrivacyRequest, error)
}

type requestProcessor struct {
	repo     compliance.Repository
	exporter Exporter
	deleter  Deleter
}

// NewRequestProcessor creates a new request processor
func NewRequestProcessor(repo compliance.Repository, exporter Exporter, deleter Deleter) RequestProcessor {
	return &requestProcessor{
		repo:     repo,
		exporter: exporter,
		deleter:  deleter,
	}
}

func (p *requestProcessor) ProcessRequest(ctx context.Context, req *compliance.PrivacyRequest) error {
	// 1. Validate request
	if req.UserID == uuid.Nil {
		return errors.New("user ID is required")
	}

	// 2. Create request record
	req.Status = "received"
	req.SubmittedAt = time.Now()
	if err := p.repo.CreatePrivacyRequest(ctx, req); err != nil {
		return err
	}

	// 3. Trigger async processing (in a real app, this would be a queue)
	// For now, we'll just simulate it or call it synchronously if simple
	go p.handleAsync(req.ID)

	return nil
}

func (p *requestProcessor) GetRequest(ctx context.Context, id uuid.UUID) (*compliance.PrivacyRequest, error) {
	return p.repo.GetRequest(ctx, id) // Assuming repo has this method, need to ensure interface match
}

func (p *requestProcessor) handleAsync(reqID uuid.UUID) {
	ctx := context.Background()
	req, err := p.repo.GetPrivacyRequest(ctx, reqID)
	if err != nil {
		return // Log error
	}

	req.Status = "processing"
	p.repo.UpdatePrivacyRequest(ctx, req)

	var processErr error
	if req.RequestType == "export" {
		processErr = p.exporter.ExportData(ctx, req)
	} else if req.RequestType == "deletion" {
		processErr = p.deleter.DeleteData(ctx, req)
	}

	if processErr != nil {
		req.Status = "failed"
		req.ErrorMessage = processErr.Error()
	} else {
		req.Status = "completed"
		now := time.Now()
		req.CompletedAt = &now
	}
	p.repo.UpdatePrivacyRequest(ctx, req)
}
