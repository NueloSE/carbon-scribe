package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

// EventBridgeClient wraps the AWS EventBridge client
type EventBridgeClient struct {
	client      *eventbridge.Client
	eventBusArn string
	source      string
}

// EventBridgeConfig holds EventBridge client configuration
type EventBridgeConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	EventBusArn     string
	Source          string
}

// Event represents an event to be published
type Event struct {
	DetailType string                 `json:"detail_type"`
	Detail     map[string]interface{} `json:"detail"`
	Resources  []string               `json:"resources,omitempty"`
	Time       time.Time              `json:"time"`
}

// NewEventBridgeClient creates a new EventBridge client
func NewEventBridgeClient(ctx context.Context, cfg EventBridgeConfig) (*EventBridgeClient, error) {
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(cfg.Region))

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := eventbridge.NewFromConfig(awsCfg)
	
	source := cfg.Source
	if source == "" {
		source = "carbonscribe.notifications"
	}

	return &EventBridgeClient{
		client:      client,
		eventBusArn: cfg.EventBusArn,
		source:      source,
	}, nil
}

// PutEvent publishes a single event to EventBridge
func (c *EventBridgeClient) PutEvent(ctx context.Context, event Event) error {
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}

	detailJSON, err := json.Marshal(event.Detail)
	if err != nil {
		return fmt.Errorf("failed to marshal event detail: %w", err)
	}

	entry := types.PutEventsRequestEntry{
		Source:       aws.String(c.source),
		DetailType:   aws.String(event.DetailType),
		Detail:       aws.String(string(detailJSON)),
		Time:         aws.Time(event.Time),
		Resources:    event.Resources,
	}

	if c.eventBusArn != "" {
		entry.EventBusName = aws.String(c.eventBusArn)
	}

	_, err = c.client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{entry},
	})

	return err
}

// PutEvents publishes multiple events to EventBridge
func (c *EventBridgeClient) PutEvents(ctx context.Context, events []Event) error {
	var entries []types.PutEventsRequestEntry

	for _, event := range events {
		if event.Time.IsZero() {
			event.Time = time.Now().UTC()
		}

		detailJSON, err := json.Marshal(event.Detail)
		if err != nil {
			return fmt.Errorf("failed to marshal event detail: %w", err)
		}

		entry := types.PutEventsRequestEntry{
			Source:       aws.String(c.source),
			DetailType:   aws.String(event.DetailType),
			Detail:       aws.String(string(detailJSON)),
			Time:         aws.Time(event.Time),
			Resources:    event.Resources,
		}

		if c.eventBusArn != "" {
			entry.EventBusName = aws.String(c.eventBusArn)
		}

		entries = append(entries, entry)
	}

	// EventBridge allows max 10 entries per request
	batchSize := 10
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}

		_, err := c.client.PutEvents(ctx, &eventbridge.PutEventsInput{
			Entries: entries[i:end],
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// CreateRule creates an EventBridge rule
func (c *EventBridgeClient) CreateRule(ctx context.Context, name, description, scheduleExpression, eventPattern string) (string, error) {
	input := &eventbridge.PutRuleInput{
		Name:        aws.String(name),
		Description: aws.String(description),
		State:       types.RuleStateEnabled,
	}

	if scheduleExpression != "" {
		input.ScheduleExpression = aws.String(scheduleExpression)
	}
	if eventPattern != "" {
		input.EventPattern = aws.String(eventPattern)
	}
	if c.eventBusArn != "" {
		input.EventBusName = aws.String(c.eventBusArn)
	}

	result, err := c.client.PutRule(ctx, input)
	if err != nil {
		return "", err
	}

	return *result.RuleArn, nil
}

// DeleteRule deletes an EventBridge rule
func (c *EventBridgeClient) DeleteRule(ctx context.Context, name string) error {
	input := &eventbridge.DeleteRuleInput{
		Name: aws.String(name),
	}

	if c.eventBusArn != "" {
		input.EventBusName = aws.String(c.eventBusArn)
	}

	_, err := c.client.DeleteRule(ctx, input)
	return err
}

// EnableRule enables an EventBridge rule
func (c *EventBridgeClient) EnableRule(ctx context.Context, name string) error {
	input := &eventbridge.EnableRuleInput{
		Name: aws.String(name),
	}

	if c.eventBusArn != "" {
		input.EventBusName = aws.String(c.eventBusArn)
	}

	_, err := c.client.EnableRule(ctx, input)
	return err
}

// DisableRule disables an EventBridge rule
func (c *EventBridgeClient) DisableRule(ctx context.Context, name string) error {
	input := &eventbridge.DisableRuleInput{
		Name: aws.String(name),
	}

	if c.eventBusArn != "" {
		input.EventBusName = aws.String(c.eventBusArn)
	}

	_, err := c.client.DisableRule(ctx, input)
	return err
}

// PutTargets adds targets to an EventBridge rule
func (c *EventBridgeClient) PutTargets(ctx context.Context, ruleName string, targets []RuleTarget) error {
	var ebTargets []types.Target
	for _, t := range targets {
		target := types.Target{
			Id:    aws.String(t.ID),
			Arn:   aws.String(t.Arn),
			Input: aws.String(t.Input),
		}
		ebTargets = append(ebTargets, target)
	}

	input := &eventbridge.PutTargetsInput{
		Rule:    aws.String(ruleName),
		Targets: ebTargets,
	}

	if c.eventBusArn != "" {
		input.EventBusName = aws.String(c.eventBusArn)
	}

	_, err := c.client.PutTargets(ctx, input)
	return err
}

// RuleTarget represents an EventBridge rule target
type RuleTarget struct {
	ID    string
	Arn   string
	Input string
}

// Notification event types
const (
	EventTypeAlertTriggered     = "alert.triggered"
	EventTypeNotificationSent   = "notification.sent"
	EventTypeNotificationFailed = "notification.failed"
	EventTypeRuleEvaluated      = "rule.evaluated"
	EventTypePreferenceUpdated  = "preference.updated"
)

// NewAlertTriggeredEvent creates an alert triggered event
func NewAlertTriggeredEvent(ruleID, projectID string, details map[string]interface{}) Event {
	return Event{
		DetailType: EventTypeAlertTriggered,
		Detail: map[string]interface{}{
			"rule_id":    ruleID,
			"project_id": projectID,
			"details":    details,
		},
		Time: time.Now().UTC(),
	}
}

// NewNotificationSentEvent creates a notification sent event
func NewNotificationSentEvent(notificationID, userID, channel string) Event {
	return Event{
		DetailType: EventTypeNotificationSent,
		Detail: map[string]interface{}{
			"notification_id": notificationID,
			"user_id":         userID,
			"channel":         channel,
		},
		Time: time.Now().UTC(),
	}
}

// NewNotificationFailedEvent creates a notification failed event
func NewNotificationFailedEvent(notificationID, userID, channel, errorMessage string) Event {
	return Event{
		DetailType: EventTypeNotificationFailed,
		Detail: map[string]interface{}{
			"notification_id": notificationID,
			"user_id":         userID,
			"channel":         channel,
			"error":           errorMessage,
		},
		Time: time.Now().UTC(),
	}
}
