package rules

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"carbon-scribe/project-portal/project-portal-backend/internal/notifications"
)

// Engine evaluates notification rules
type Engine struct {
	repo      RuleRepository
	evaluator *Evaluator
	mu        sync.RWMutex
}

// RuleRepository defines the interface for rule storage
type RuleRepository interface {
	CreateRule(ctx context.Context, rule *notifications.NotificationRule) error
	GetRule(ctx context.Context, projectID, ruleID string) (*notifications.NotificationRule, error)
	ListRulesByProject(ctx context.Context, projectID string) ([]notifications.NotificationRule, error)
	DeleteRule(ctx context.Context, projectID, ruleID string) error
}

// NewEngine creates a new rule engine
func NewEngine(repo RuleRepository) *Engine {
	return &Engine{
		repo:      repo,
		evaluator: NewEvaluator(),
	}
}

// CreateRule creates a new notification rule
func (e *Engine) CreateRule(ctx context.Context, req *notifications.CreateRuleRequest) (*notifications.NotificationRule, error) {
	ruleID := uuid.New().String()

	rule := &notifications.NotificationRule{
		RuleID:      ruleID,
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		Description: req.Description,
		Conditions:  req.Conditions,
		Actions:     req.Actions,
		IsActive:    req.IsActive,
		Metadata:    req.Metadata,
	}

	if err := e.repo.CreateRule(ctx, rule); err != nil {
		return nil, err
	}

	return rule, nil
}

// GetRule retrieves a rule by ID
func (e *Engine) GetRule(ctx context.Context, projectID, ruleID string) (*notifications.NotificationRule, error) {
	return e.repo.GetRule(ctx, projectID, ruleID)
}

// ListRules lists all rules for a project
func (e *Engine) ListRules(ctx context.Context, projectID string) ([]notifications.NotificationRule, error) {
	return e.repo.ListRulesByProject(ctx, projectID)
}

// DeleteRule deletes a rule
func (e *Engine) DeleteRule(ctx context.Context, projectID, ruleID string) error {
	return e.repo.DeleteRule(ctx, projectID, ruleID)
}

// Evaluate evaluates a rule against data
func (e *Engine) Evaluate(ctx context.Context, rule *notifications.NotificationRule, data map[string]interface{}) (*EvaluationResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !rule.IsActive {
		return &EvaluationResult{
			Triggered: false,
			Message:   "Rule is inactive",
		}, nil
	}

	matchedConditions := make([]int, 0)
	allMatched := true

	for i, condition := range rule.Conditions {
		matched, err := e.evaluator.EvaluateCondition(condition, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate condition %d: %w", i, err)
		}
		if matched {
			matchedConditions = append(matchedConditions, i)
		} else {
			allMatched = false
		}
	}

	return &EvaluationResult{
		Triggered:         allMatched && len(matchedConditions) > 0,
		MatchedConditions: matchedConditions,
		Message:           e.buildResultMessage(allMatched, matchedConditions, len(rule.Conditions)),
	}, nil
}

// EvaluateAll evaluates all active rules for a project
func (e *Engine) EvaluateAll(ctx context.Context, projectID string, data map[string]interface{}) ([]TriggeredRule, error) {
	rules, err := e.repo.ListRulesByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var triggered []TriggeredRule
	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}

		result, err := e.Evaluate(ctx, &rule, data)
		if err != nil {
			continue
		}

		if result.Triggered {
			triggered = append(triggered, TriggeredRule{
				Rule:   rule,
				Result: result,
			})
		}
	}

	return triggered, nil
}

// TestRule tests a rule with sample data
func (e *Engine) TestRule(ctx context.Context, projectID, ruleID string, sampleData map[string]interface{}) (*notifications.TestRuleResponse, error) {
	rule, err := e.repo.GetRule(ctx, projectID, ruleID)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, fmt.Errorf("rule not found")
	}

	// Temporarily enable the rule for testing
	originalActive := rule.IsActive
	rule.IsActive = true
	defer func() { rule.IsActive = originalActive }()

	result, err := e.Evaluate(ctx, rule, sampleData)
	if err != nil {
		return nil, err
	}

	return &notifications.TestRuleResponse{
		Triggered:         result.Triggered,
		MatchedConditions: result.MatchedConditions,
		Message:           result.Message,
	}, nil
}

func (e *Engine) buildResultMessage(allMatched bool, matched []int, total int) string {
	if allMatched && len(matched) > 0 {
		return fmt.Sprintf("Rule triggered: all %d conditions matched", total)
	}
	return fmt.Sprintf("Rule not triggered: %d of %d conditions matched", len(matched), total)
}

// EvaluationResult represents the result of evaluating a rule
type EvaluationResult struct {
	Triggered         bool
	MatchedConditions []int
	Message           string
	EvaluatedAt       time.Time
}

// TriggeredRule represents a rule that was triggered
type TriggeredRule struct {
	Rule   notifications.NotificationRule
	Result *EvaluationResult
}
