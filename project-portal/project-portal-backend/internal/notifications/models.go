package notifications

import (
	"time"
)

// Channel represents a notification channel type
type Channel string

const (
	ChannelEmail     Channel = "EMAIL"
	ChannelSMS       Channel = "SMS"
	ChannelWebSocket Channel = "WEBSOCKET"
	ChannelPush      Channel = "PUSH"
	ChannelInApp     Channel = "IN_APP"
)

// DeliveryStatus represents the status of a notification delivery
type DeliveryStatus string

const (
	StatusPending   DeliveryStatus = "PENDING"
	StatusSent      DeliveryStatus = "SENT"
	StatusDelivered DeliveryStatus = "DELIVERED"
	StatusFailed    DeliveryStatus = "FAILED"
	StatusBounced   DeliveryStatus = "BOUNCED"
	StatusComplaint DeliveryStatus = "COMPLAINT"
)

// NotificationCategory represents a category of notifications
type NotificationCategory string

const (
	CategoryMonitoringAlerts  NotificationCategory = "MONITORING_ALERTS"
	CategoryPaymentUpdates    NotificationCategory = "PAYMENT_UPDATES"
	CategoryProjectUpdates    NotificationCategory = "PROJECT_UPDATES"
	CategorySystemAnnouncements NotificationCategory = "SYSTEM_ANNOUNCEMENTS"
	CategoryVerificationStatus NotificationCategory = "VERIFICATION_STATUS"
)

// NotificationTemplate represents a notification template stored in DynamoDB
type NotificationTemplate struct {
	PK        string            `dynamodb:"PK"`        // TEMPLATE#{type}#{language}
	SK        string            `dynamodb:"SK"`        // VERSION#{version_id}
	Name      string            `dynamodb:"name"`
	Subject   string            `dynamodb:"subject"`
	Body      string            `dynamodb:"body"`
	Variables []string          `dynamodb:"variables"`
	Metadata  map[string]string `dynamodb:"metadata"`
	IsActive  bool              `dynamodb:"isActive"`
	CreatedAt string            `dynamodb:"createdAt"`
	UpdatedAt string            `dynamodb:"updatedAt"`
}

// NotificationRule represents an alert rule stored in DynamoDB
type NotificationRule struct {
	PK            string          `dynamodb:"PK"`            // RULE#{project_id}
	SK            string          `dynamodb:"SK"`            // RULE#{rule_id}
	RuleID        string          `dynamodb:"ruleId"`
	ProjectID     string          `dynamodb:"projectId"`
	Name          string          `dynamodb:"name"`
	Description   string          `dynamodb:"description"`
	Conditions    []RuleCondition `dynamodb:"conditions"`
	Actions       []RuleAction    `dynamodb:"actions"`
	IsActive      bool            `dynamodb:"isActive"`
	LastTriggered string          `dynamodb:"lastTriggered"`
	TriggerCount  int             `dynamodb:"triggerCount"`
	Metadata      map[string]string `dynamodb:"metadata"`
	CreatedAt     string          `dynamodb:"createdAt"`
	UpdatedAt     string          `dynamodb:"updatedAt"`
}

// RuleCondition represents a condition in an alert rule
type RuleCondition struct {
	Type      string      `json:"type"`      // threshold, rate_of_change, pattern
	Field     string      `json:"field"`
	Operator  string      `json:"operator"`  // gt, gte, lt, lte, eq, neq, contains
	Value     interface{} `json:"value"`
	Duration  string      `json:"duration,omitempty"`  // For rate-of-change conditions
	Pattern   string      `json:"pattern,omitempty"`   // For pattern-based conditions
}

// RuleAction represents an action to take when a rule triggers
type RuleAction struct {
	Type       string   `json:"type"`       // email, sms, websocket, in_app
	Recipients []string `json:"recipients"` // User IDs or email addresses
	TemplateID string   `json:"template_id"`
	Priority   string   `json:"priority"`   // high, medium, low
}

// UserPreference represents user notification preferences stored in DynamoDB
type UserPreference struct {
	PK              string   `dynamodb:"PK"`              // USER#{user_id}
	SK              string   `dynamodb:"SK"`              // PREF#{channel}#{category}
	UserID          string   `dynamodb:"userId"`
	Channel         Channel  `dynamodb:"channel"`
	Category        NotificationCategory `dynamodb:"category"`
	Enabled         bool     `dynamodb:"enabled"`
	QuietHoursStart string   `dynamodb:"quietHoursStart"` // HH:MM format
	QuietHoursEnd   string   `dynamodb:"quietHoursEnd"`   // HH:MM format
	Channels        []Channel `dynamodb:"channels"`
	UpdatedAt       string   `dynamodb:"updatedAt"`
}

// WebSocketConnection represents a WebSocket connection stored in DynamoDB
type WebSocketConnection struct {
	PK           string   `dynamodb:"PK"`           // CONNECTION#{connection_id}
	ConnectionID string   `dynamodb:"connectionId"`
	UserID       string   `dynamodb:"userId"`
	ProjectIDs   []string `dynamodb:"projectIds"`
	Channels     []string `dynamodb:"channels"`
	ConnectedAt  string   `dynamodb:"connectedAt"`
	LastActivity string   `dynamodb:"lastActivity"`
	UserAgent    string   `dynamodb:"userAgent"`
	IPAddress    string   `dynamodb:"ipAddress"`
	TTL          int64    `dynamodb:"ttl"`          // DynamoDB TTL for auto-cleanup
}

// DeliveryLog represents a notification delivery log stored in DynamoDB
type DeliveryLog struct {
	PK                string            `dynamodb:"PK"`                // NOTIFICATION#{notification_id}
	SK                string            `dynamodb:"SK"`                // ATTEMPT#{timestamp}
	NotificationID    string            `dynamodb:"notificationId"`
	UserID            string            `dynamodb:"userId"`
	Channel           Channel           `dynamodb:"channel"`
	TemplateID        string            `dynamodb:"templateId"`
	Subject           string            `dynamodb:"subject"`
	Status            DeliveryStatus    `dynamodb:"status"`
	ProviderMessageID string            `dynamodb:"providerMessageId"`
	ProviderResponse  map[string]string `dynamodb:"providerResponse"`
	RetryCount        int               `dynamodb:"retryCount"`
	FinalStatus       DeliveryStatus    `dynamodb:"finalStatus"`
	ErrorMessage      string            `dynamodb:"errorMessage"`
	CreatedAt         string            `dynamodb:"createdAt"`
	SentAt            string            `dynamodb:"sentAt"`
	DeliveredAt       string            `dynamodb:"deliveredAt"`
}

// ================================
// Request/Response DTOs
// ================================

// SendNotificationRequest represents a request to send a notification
type SendNotificationRequest struct {
	UserIDs    []string               `json:"user_ids" binding:"required"`
	Channels   []Channel              `json:"channels" binding:"required"`
	TemplateID string                 `json:"template_id"`
	Subject    string                 `json:"subject"`
	Body       string                 `json:"body"`
	Variables  map[string]interface{} `json:"variables"`
	Priority   string                 `json:"priority"`
	Category   NotificationCategory   `json:"category"`
}

// SendNotificationResponse represents the response after sending a notification
type SendNotificationResponse struct {
	NotificationID string                   `json:"notification_id"`
	Status         string                   `json:"status"`
	Deliveries     []DeliveryStatusResponse `json:"deliveries"`
}

// DeliveryStatusResponse represents the delivery status for a single channel
type DeliveryStatusResponse struct {
	UserID    string         `json:"user_id"`
	Channel   Channel        `json:"channel"`
	Status    DeliveryStatus `json:"status"`
	MessageID string         `json:"message_id,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// CreateTemplateRequest represents a request to create a notification template
type CreateTemplateRequest struct {
	Type      string            `json:"type" binding:"required"`
	Language  string            `json:"language" binding:"required"`
	Name      string            `json:"name" binding:"required"`
	Subject   string            `json:"subject" binding:"required"`
	Body      string            `json:"body" binding:"required"`
	Variables []string          `json:"variables"`
	Metadata  map[string]string `json:"metadata"`
}

// UpdateTemplateRequest represents a request to update a notification template
type UpdateTemplateRequest struct {
	Name      string            `json:"name"`
	Subject   string            `json:"subject"`
	Body      string            `json:"body"`
	Variables []string          `json:"variables"`
	Metadata  map[string]string `json:"metadata"`
	IsActive  *bool             `json:"is_active"`
}

// TemplatePreviewRequest represents a request to preview a template
type TemplatePreviewRequest struct {
	Variables map[string]interface{} `json:"variables"`
}

// TemplatePreviewResponse represents the preview of a rendered template
type TemplatePreviewResponse struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// CreateRuleRequest represents a request to create an alert rule
type CreateRuleRequest struct {
	ProjectID   string          `json:"project_id" binding:"required"`
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	Conditions  []RuleCondition `json:"conditions" binding:"required"`
	Actions     []RuleAction    `json:"actions" binding:"required"`
	IsActive    bool            `json:"is_active"`
	Metadata    map[string]string `json:"metadata"`
}

// UpdateRuleRequest represents a request to update an alert rule
type UpdateRuleRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Conditions  []RuleCondition `json:"conditions"`
	Actions     []RuleAction    `json:"actions"`
	IsActive    *bool           `json:"is_active"`
	Metadata    map[string]string `json:"metadata"`
}

// TestRuleRequest represents a request to test a rule with sample data
type TestRuleRequest struct {
	SampleData map[string]interface{} `json:"sample_data" binding:"required"`
}

// TestRuleResponse represents the result of testing a rule
type TestRuleResponse struct {
	Triggered  bool   `json:"triggered"`
	MatchedConditions []int `json:"matched_conditions"`
	Message    string `json:"message"`
}

// UpdatePreferencesRequest represents a request to update user preferences
type UpdatePreferencesRequest struct {
	Preferences []PreferenceUpdate `json:"preferences" binding:"required"`
}

// PreferenceUpdate represents an update to a single preference
type PreferenceUpdate struct {
	Channel         Channel              `json:"channel"`
	Category        NotificationCategory `json:"category"`
	Enabled         bool                 `json:"enabled"`
	QuietHoursStart string               `json:"quiet_hours_start"`
	QuietHoursEnd   string               `json:"quiet_hours_end"`
}

// UserPreferencesResponse represents the user's notification preferences
type UserPreferencesResponse struct {
	UserID      string               `json:"user_id"`
	Preferences []UserPreference     `json:"preferences"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

// NotificationsListRequest represents query parameters for listing notifications
type NotificationsListRequest struct {
	Limit      int    `form:"limit"`
	Cursor     string `form:"cursor"`
	Channel    string `form:"channel"`
	Category   string `form:"category"`
	Status     string `form:"status"`
	StartDate  string `form:"start_date"`
	EndDate    string `form:"end_date"`
}

// NotificationsListResponse represents a paginated list of notifications
type NotificationsListResponse struct {
	Notifications []DeliveryLog `json:"notifications"`
	NextCursor    string        `json:"next_cursor,omitempty"`
	TotalCount    int           `json:"total_count"`
}

// NotificationMetricsResponse represents delivery analytics
type NotificationMetricsResponse struct {
	TotalSent       int                      `json:"total_sent"`
	TotalDelivered  int                      `json:"total_delivered"`
	TotalFailed     int                      `json:"total_failed"`
	DeliveryRate    float64                  `json:"delivery_rate"`
	OpenRate        float64                  `json:"open_rate"`
	ClickRate       float64                  `json:"click_rate"`
	ByChannel       map[Channel]ChannelMetrics `json:"by_channel"`
	ByCategory      map[NotificationCategory]CategoryMetrics `json:"by_category"`
}

// ChannelMetrics represents metrics for a specific channel
type ChannelMetrics struct {
	Sent         int     `json:"sent"`
	Delivered    int     `json:"delivered"`
	Failed       int     `json:"failed"`
	DeliveryRate float64 `json:"delivery_rate"`
}

// CategoryMetrics represents metrics for a specific category
type CategoryMetrics struct {
	Sent      int     `json:"sent"`
	Delivered int     `json:"delivered"`
	OpenRate  float64 `json:"open_rate"`
}

// BroadcastRequest represents a request to broadcast to WebSocket clients
type BroadcastRequest struct {
	Channels []string               `json:"channels"`
	UserIDs  []string               `json:"user_ids"`
	Message  map[string]interface{} `json:"message" binding:"required"`
	Type     string                 `json:"type"`
}

// WebhookPayload represents an incoming webhook from AWS SNS/SES
type WebhookPayload struct {
	Type             string `json:"Type"`
	MessageId        string `json:"MessageId"`
	TopicArn         string `json:"TopicArn"`
	Subject          string `json:"Subject"`
	Message          string `json:"Message"`
	Timestamp        string `json:"Timestamp"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
	SubscribeURL     string `json:"SubscribeURL"`
	UnsubscribeURL   string `json:"UnsubscribeURL"`
}

// SESNotification represents an SES notification
type SESNotification struct {
	NotificationType string          `json:"notificationType"`
	Mail             SESMail         `json:"mail"`
	Bounce           *SESBounce      `json:"bounce,omitempty"`
	Complaint        *SESComplaint   `json:"complaint,omitempty"`
	Delivery         *SESDelivery    `json:"delivery,omitempty"`
}

// SESMail represents email metadata in an SES notification
type SESMail struct {
	Timestamp        string   `json:"timestamp"`
	MessageId        string   `json:"messageId"`
	Source           string   `json:"source"`
	SourceArn        string   `json:"sourceArn"`
	Destination      []string `json:"destination"`
}

// SESBounce represents bounce information
type SESBounce struct {
	BounceType        string `json:"bounceType"`
	BounceSubType     string `json:"bounceSubType"`
	Timestamp         string `json:"timestamp"`
}

// SESComplaint represents complaint information
type SESComplaint struct {
	ComplaintFeedbackType string `json:"complaintFeedbackType"`
	Timestamp             string `json:"timestamp"`
}

// SESDelivery represents delivery information
type SESDelivery struct {
	Timestamp            string   `json:"timestamp"`
	ProcessingTimeMillis int      `json:"processingTimeMillis"`
	Recipients           []string `json:"recipients"`
	SmtpResponse         string   `json:"smtpResponse"`
}
