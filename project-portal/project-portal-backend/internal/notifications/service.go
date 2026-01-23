package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"carbon-scribe/project-portal/project-portal-backend/internal/config"
	"carbon-scribe/project-portal/project-portal-backend/internal/notifications/channels"
	"carbon-scribe/project-portal/project-portal-backend/internal/notifications/rules"
	"carbon-scribe/project-portal/project-portal-backend/internal/notifications/templates"
	wspkg "carbon-scribe/project-portal/project-portal-backend/pkg/websocket"
)

// Service handles notification business logic
type Service struct {
	repo            *Repository
	templateManager *templates.Manager
	ruleEngine      *rules.Engine
	emailChannel    *channels.EmailChannel
	smsChannel      *channels.SMSChannel
	wsChannel       *channels.WebSocketChannel
	config          *config.NotificationConfig
}

// ServiceConfig holds service configuration
type ServiceConfig struct {
	Repository      *Repository
	TemplateManager *templates.Manager
	RuleEngine      *rules.Engine
	EmailChannel    *channels.EmailChannel
	SMSChannel      *channels.SMSChannel
	WSChannel       *channels.WebSocketChannel
	Config          *config.NotificationConfig
}

// NewService creates a new notification service
func NewService(cfg ServiceConfig) *Service {
	return &Service{
		repo:            cfg.Repository,
		templateManager: cfg.TemplateManager,
		ruleEngine:      cfg.RuleEngine,
		emailChannel:    cfg.EmailChannel,
		smsChannel:      cfg.SMSChannel,
		wsChannel:       cfg.WSChannel,
		config:          cfg.Config,
	}
}

// SendNotification sends a notification to users across multiple channels
func (s *Service) SendNotification(ctx context.Context, req *SendNotificationRequest) (*SendNotificationResponse, error) {
	notificationID := uuid.New().String()
	now := time.Now().UTC()

	// Get and render template if specified
	var subject, body string
	if req.TemplateID != "" {
		preview, err := s.templateManager.Preview(ctx, req.TemplateID, "en", req.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to render template: %w", err)
		}
		subject = preview.Subject
		body = preview.Body
	} else {
		subject = req.Subject
		body = req.Body
	}

	var deliveries []DeliveryStatusResponse

	for _, userID := range req.UserIDs {
		// Check user preferences
		for _, channel := range req.Channels {
			prefs, _ := s.repo.GetPreference(ctx, userID, channel, req.Category)
			
			// Check if notifications are enabled for this channel/category
			if prefs != nil && !prefs.Enabled {
				continue
			}

			// Check quiet hours
			if s.isQuietHours(prefs) {
				continue
			}

			// Create delivery log
			log := &DeliveryLog{
				NotificationID: notificationID,
				UserID:         userID,
				Channel:        channel,
				TemplateID:     req.TemplateID,
				Subject:        subject,
				Status:         StatusPending,
				CreatedAt:      now.Format(time.RFC3339),
			}
			_ = s.repo.CreateDeliveryLog(ctx, log)

			// Send via appropriate channel
			result, err := s.sendViaChannel(ctx, channel, userID, subject, body)
			if err != nil {
				log.Status = StatusFailed
				log.ErrorMessage = err.Error()
			} else {
				log.Status = StatusSent
				log.ProviderMessageID = result.MessageID
			}

			if result != nil {
				result.UserID = userID
				deliveries = append(deliveries, *result)
			}
		}
	}

	return &SendNotificationResponse{
		NotificationID: notificationID,
		Status:         "sent",
		Deliveries:     deliveries,
	}, nil
}

func (s *Service) sendViaChannel(ctx context.Context, channel Channel, userID, subject, body string) (*DeliveryStatusResponse, error) {
	switch channel {
	case ChannelEmail:
		// In production, lookup user's email from user service
		email := userID + "@example.com"
		return s.emailChannel.Send(ctx, email, subject, body, body)
	case ChannelSMS:
		// In production, lookup user's phone from user service
		phone := "+1234567890"
		return s.smsChannel.Send(ctx, phone, body)
	case ChannelWebSocket:
		message := wspkg.NewNotificationMessage(uuid.New().String(), map[string]interface{}{
			"subject": subject,
			"body":    body,
		})
		return s.wsChannel.SendToUser(ctx, userID, message)
	default:
		return nil, fmt.Errorf("unsupported channel: %s", channel)
	}
}

func (s *Service) isQuietHours(prefs *UserPreference) bool {
	if prefs == nil || prefs.QuietHoursStart == "" || prefs.QuietHoursEnd == "" {
		return false
	}

	now := time.Now()
	currentTime := now.Format("15:04")

	start := prefs.QuietHoursStart
	end := prefs.QuietHoursEnd

	// Handle overnight quiet hours (e.g., 22:00 to 08:00)
	if start > end {
		return currentTime >= start || currentTime <= end
	}
	return currentTime >= start && currentTime <= end
}

// GetUserNotifications retrieves notifications for a user
func (s *Service) GetUserNotifications(ctx context.Context, userID string, req *NotificationsListRequest) (*NotificationsListResponse, error) {
	limit := int32(20)
	if req.Limit > 0 && req.Limit <= 100 {
		limit = int32(req.Limit)
	}

	logs, cursor, err := s.repo.GetUserDeliveryLogs(ctx, userID, limit, req.Cursor)
	if err != nil {
		return nil, err
	}

	return &NotificationsListResponse{
		Notifications: logs,
		NextCursor:    cursor,
		TotalCount:    len(logs),
	}, nil
}

// GetNotificationStatus retrieves the status of a specific notification
func (s *Service) GetNotificationStatus(ctx context.Context, notificationID string) ([]DeliveryLog, error) {
	return s.repo.GetDeliveryLogs(ctx, notificationID)
}

// GetUserPreferences retrieves user notification preferences
func (s *Service) GetUserPreferences(ctx context.Context, userID string) (*UserPreferencesResponse, error) {
	prefs, err := s.repo.GetUserPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &UserPreferencesResponse{
		UserID:      userID,
		Preferences: prefs,
		UpdatedAt:   time.Now(),
	}, nil
}

// UpdateUserPreferences updates user notification preferences
func (s *Service) UpdateUserPreferences(ctx context.Context, userID string, req *UpdatePreferencesRequest) error {
	for _, update := range req.Preferences {
		pref := &UserPreference{
			UserID:          userID,
			Channel:         update.Channel,
			Category:        update.Category,
			Enabled:         update.Enabled,
			QuietHoursStart: update.QuietHoursStart,
			QuietHoursEnd:   update.QuietHoursEnd,
		}
		if err := s.repo.SavePreference(ctx, pref); err != nil {
			return err
		}
	}
	return nil
}

// CreateTemplate creates a new notification template
func (s *Service) CreateTemplate(ctx context.Context, req *CreateTemplateRequest) (*NotificationTemplate, error) {
	return s.templateManager.Create(ctx, req)
}

// ListTemplates lists all templates
func (s *Service) ListTemplates(ctx context.Context, limit int32, cursor string) ([]NotificationTemplate, string, error) {
	return s.templateManager.List(ctx, limit, cursor)
}

// PreviewTemplate previews a rendered template
func (s *Service) PreviewTemplate(ctx context.Context, templateType, language string, variables map[string]interface{}) (*TemplatePreviewResponse, error) {
	return s.templateManager.Preview(ctx, templateType, language, variables)
}

// CreateRule creates a new alert rule
func (s *Service) CreateRule(ctx context.Context, req *CreateRuleRequest) (*NotificationRule, error) {
	return s.ruleEngine.CreateRule(ctx, req)
}

// ListRules lists all rules for a project
func (s *Service) ListRules(ctx context.Context, projectID string) ([]NotificationRule, error) {
	return s.ruleEngine.ListRules(ctx, projectID)
}

// GetRule retrieves a rule by ID
func (s *Service) GetRule(ctx context.Context, projectID, ruleID string) (*NotificationRule, error) {
	return s.ruleEngine.GetRule(ctx, projectID, ruleID)
}

// DeleteRule deletes a rule
func (s *Service) DeleteRule(ctx context.Context, projectID, ruleID string) error {
	return s.ruleEngine.DeleteRule(ctx, projectID, ruleID)
}

// TestRule tests a rule with sample data
func (s *Service) TestRule(ctx context.Context, projectID, ruleID string, sampleData map[string]interface{}) (*TestRuleResponse, error) {
	return s.ruleEngine.TestRule(ctx, projectID, ruleID, sampleData)
}

// EvaluateRules evaluates all rules for a project against data
func (s *Service) EvaluateRules(ctx context.Context, projectID string, data map[string]interface{}) ([]rules.TriggeredRule, error) {
	return s.ruleEngine.EvaluateAll(ctx, projectID, data)
}

// GetMetrics retrieves notification delivery metrics
func (s *Service) GetMetrics(ctx context.Context, userID string) (*NotificationMetricsResponse, error) {
	// TODO: Implement proper metrics aggregation from delivery logs
	return &NotificationMetricsResponse{
		TotalSent:      0,
		TotalDelivered: 0,
		TotalFailed:    0,
		DeliveryRate:   0.0,
		OpenRate:       0.0,
		ClickRate:      0.0,
		ByChannel:      make(map[Channel]ChannelMetrics),
		ByCategory:     make(map[NotificationCategory]CategoryMetrics),
	}, nil
}

// BroadcastToWebSocket broadcasts a message to WebSocket clients
func (s *Service) BroadcastToWebSocket(ctx context.Context, req *BroadcastRequest) error {
	message := wspkg.NewMessage(wspkg.MessageType(req.Type), req.Message)

	if len(req.UserIDs) > 0 {
		_, err := s.wsChannel.BroadcastToUsers(ctx, req.UserIDs, message)
		return err
	}

	if len(req.Channels) > 0 {
		for _, channel := range req.Channels {
			if _, err := s.wsChannel.SendToChannel(ctx, channel, message); err != nil {
				return err
			}
		}
		return nil
	}

	// Broadcast to all
	_, err := s.wsChannel.Broadcast(ctx, message)
	return err
}

// ProcessSESWebhook processes an SES delivery status webhook
func (s *Service) ProcessSESWebhook(ctx context.Context, notification *SESNotification) error {
	// Update delivery status based on notification type
	messageID := notification.Mail.MessageId

	var status DeliveryStatus
	switch notification.NotificationType {
	case "Delivery":
		status = StatusDelivered
	case "Bounce":
		status = StatusBounced
	case "Complaint":
		status = StatusComplaint
	default:
		return nil
	}

	// TODO: Look up notification by provider message ID and update status
	_ = messageID
	_ = status

	return nil
}

// ProcessSNSWebhook processes an SNS delivery status webhook
func (s *Service) ProcessSNSWebhook(ctx context.Context, payload *WebhookPayload) error {
	// Handle subscription confirmation
	if payload.Type == "SubscriptionConfirmation" {
		// TODO: Confirm subscription by visiting SubscribeURL
		return nil
	}

	// Handle notification
	if payload.Type == "Notification" {
		// Parse message and update delivery status
	}

	return nil
}

// RetryFailedDelivery retries a failed notification delivery
func (s *Service) RetryFailedDelivery(ctx context.Context, notificationID string) error {
	logs, err := s.repo.GetDeliveryLogs(ctx, notificationID)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		return fmt.Errorf("notification not found")
	}

	// Find the most recent failed delivery
	var failedLog *DeliveryLog
	for i := range logs {
		if logs[i].Status == StatusFailed {
			failedLog = &logs[i]
			break
		}
	}

	if failedLog == nil {
		return fmt.Errorf("no failed delivery to retry")
	}

	// Check retry count
	if failedLog.RetryCount >= s.config.MaxRetries {
		// Move to dead letter queue
		return fmt.Errorf("max retries exceeded")
	}

	// Calculate backoff delay
	delay := time.Duration(1<<failedLog.RetryCount) * s.config.RetryBaseDelay
	time.Sleep(delay)

	// Retry the delivery
	_, err = s.sendViaChannel(ctx, failedLog.Channel, failedLog.UserID, failedLog.Subject, "")
	return err
}
