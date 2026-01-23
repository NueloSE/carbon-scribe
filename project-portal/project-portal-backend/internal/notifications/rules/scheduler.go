package rules

import (
	"context"
	"fmt"
	"sync"
	"time"

	"carbon-scribe/project-portal/project-portal-backend/internal/notifications"
)

// Scheduler handles rule scheduling and rate limiting
type Scheduler struct {
	engine        *Engine
	rateLimiter   *RateLimiter
	schedules     map[string]*Schedule
	mu            sync.RWMutex
	stopCh        chan struct{}
	onTrigger     TriggerCallback
}

// TriggerCallback is called when a rule is triggered
type TriggerCallback func(ctx context.Context, rule notifications.NotificationRule, result *EvaluationResult) error

// Schedule represents a rule schedule
type Schedule struct {
	RuleID       string
	ProjectID    string
	Interval     time.Duration
	LastRun      time.Time
	NextRun      time.Time
	IsActive     bool
}

// RateLimiter limits rule trigger frequency
type RateLimiter struct {
	limits   map[string]*RateLimit
	mu       sync.RWMutex
}

// RateLimit tracks rate limiting for a rule
type RateLimit struct {
	RuleID        string
	MaxTriggers   int
	WindowSeconds int
	Triggers      []time.Time
}

// NewScheduler creates a new rule scheduler
func NewScheduler(engine *Engine) *Scheduler {
	return &Scheduler{
		engine:      engine,
		rateLimiter: NewRateLimiter(),
		schedules:   make(map[string]*Schedule),
		stopCh:      make(chan struct{}),
	}
}

// SetTriggerCallback sets the callback for triggered rules
func (s *Scheduler) SetTriggerCallback(cb TriggerCallback) {
	s.onTrigger = cb
}

// ScheduleRule schedules a rule for periodic evaluation
func (s *Scheduler) ScheduleRule(ruleID, projectID string, interval time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	schedule := &Schedule{
		RuleID:    ruleID,
		ProjectID: projectID,
		Interval:  interval,
		NextRun:   time.Now().Add(interval),
		IsActive:  true,
	}

	s.schedules[ruleID] = schedule
	return nil
}

// UnscheduleRule removes a rule from the schedule
func (s *Scheduler) UnscheduleRule(ruleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.schedules, ruleID)
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.checkSchedules(ctx, now)
		}
	}
}

func (s *Scheduler) checkSchedules(ctx context.Context, now time.Time) {
	s.mu.RLock()
	schedulesToRun := make([]*Schedule, 0)
	for _, schedule := range s.schedules {
		if schedule.IsActive && now.After(schedule.NextRun) {
			schedulesToRun = append(schedulesToRun, schedule)
		}
	}
	s.mu.RUnlock()

	for _, schedule := range schedulesToRun {
		s.runSchedule(ctx, schedule)
	}
}

func (s *Scheduler) runSchedule(ctx context.Context, schedule *Schedule) {
	// Get the rule
	rule, err := s.engine.GetRule(ctx, schedule.ProjectID, schedule.RuleID)
	if err != nil || rule == nil {
		return
	}

	// Check rate limit
	if !s.rateLimiter.Allow(schedule.RuleID) {
		return
	}

	// Update schedule
	s.mu.Lock()
	schedule.LastRun = time.Now()
	schedule.NextRun = time.Now().Add(schedule.Interval)
	s.mu.Unlock()
}

// EvaluateAndTrigger evaluates a rule and triggers callback if matched
func (s *Scheduler) EvaluateAndTrigger(ctx context.Context, projectID string, data map[string]interface{}) error {
	triggered, err := s.engine.EvaluateAll(ctx, projectID, data)
	if err != nil {
		return err
	}

	if s.onTrigger == nil {
		return nil
	}

	for _, t := range triggered {
		// Check rate limit
		if !s.rateLimiter.Allow(t.Rule.RuleID) {
			continue
		}

		if err := s.onTrigger(ctx, t.Rule, t.Result); err != nil {
			// Log error but continue
			continue
		}
	}

	return nil
}

// GetSchedule returns the schedule for a rule
func (s *Scheduler) GetSchedule(ruleID string) *Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.schedules[ruleID]
}

// ListSchedules returns all schedules
func (s *Scheduler) ListSchedules() []*Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedules := make([]*Schedule, 0, len(s.schedules))
	for _, schedule := range s.schedules {
		schedules = append(schedules, schedule)
	}
	return schedules
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limits: make(map[string]*RateLimit),
	}
}

// SetLimit sets the rate limit for a rule
func (r *RateLimiter) SetLimit(ruleID string, maxTriggers, windowSeconds int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.limits[ruleID] = &RateLimit{
		RuleID:        ruleID,
		MaxTriggers:   maxTriggers,
		WindowSeconds: windowSeconds,
		Triggers:      make([]time.Time, 0),
	}
}

// Allow checks if a rule trigger is allowed
func (r *RateLimiter) Allow(ruleID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit, ok := r.limits[ruleID]
	if !ok {
		// No limit set, allow
		return true
	}

	now := time.Now()
	windowStart := now.Add(-time.Duration(limit.WindowSeconds) * time.Second)

	// Clean up old triggers
	validTriggers := make([]time.Time, 0)
	for _, t := range limit.Triggers {
		if t.After(windowStart) {
			validTriggers = append(validTriggers, t)
		}
	}
	limit.Triggers = validTriggers

	// Check if we've exceeded the limit
	if len(limit.Triggers) >= limit.MaxTriggers {
		return false
	}

	// Record this trigger
	limit.Triggers = append(limit.Triggers, now)
	return true
}

// Reset resets the rate limiter for a rule
func (r *RateLimiter) Reset(ruleID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if limit, ok := r.limits[ruleID]; ok {
		limit.Triggers = make([]time.Time, 0)
	}
}

// GetRemainingTriggers returns the number of remaining triggers allowed
func (r *RateLimiter) GetRemainingTriggers(ruleID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit, ok := r.limits[ruleID]
	if !ok {
		return -1 // No limit
	}

	now := time.Now()
	windowStart := now.Add(-time.Duration(limit.WindowSeconds) * time.Second)

	validCount := 0
	for _, t := range limit.Triggers {
		if t.After(windowStart) {
			validCount++
		}
	}

	remaining := limit.MaxTriggers - validCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

// SchedulerConfig holds scheduler configuration
type SchedulerConfig struct {
	DefaultInterval   time.Duration
	DefaultMaxTriggers int
	DefaultWindowSeconds int
}

// DefaultSchedulerConfig returns default scheduler configuration
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		DefaultInterval:      5 * time.Minute,
		DefaultMaxTriggers:   10,
		DefaultWindowSeconds: 3600, // 1 hour
	}
}

// ParseInterval parses a duration string like "5m", "1h", etc.
func ParseInterval(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid interval: %s", s)
	}
	if d < time.Minute {
		return 0, fmt.Errorf("interval must be at least 1 minute")
	}
	return d, nil
}
