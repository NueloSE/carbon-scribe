package channels

import (
	"context"
	"fmt"

	"carbon-scribe/project-portal/project-portal-backend/internal/notifications"
	awspkg "carbon-scribe/project-portal/project-portal-backend/pkg/aws"
)

// SMSChannel handles SMS notifications via AWS SNS
type SMSChannel struct {
	client *awspkg.SNSClient
}

// NewSMSChannel creates a new SMS channel
func NewSMSChannel(client *awspkg.SNSClient) *SMSChannel {
	return &SMSChannel{client: client}
}

// Send sends an SMS notification
func (c *SMSChannel) Send(ctx context.Context, phoneNumber, message string) (*notifications.DeliveryStatusResponse, error) {
	msg := awspkg.SMSMessage{
		PhoneNumber: phoneNumber,
		Message:     message,
		MessageType: awspkg.SMSTypeTransactional,
	}

	result, err := c.client.SendSMS(ctx, msg)
	if err != nil {
		return &notifications.DeliveryStatusResponse{
			Channel: notifications.ChannelSMS,
			Status:  notifications.StatusFailed,
			Error:   err.Error(),
		}, err
	}

	return &notifications.DeliveryStatusResponse{
		Channel:   notifications.ChannelSMS,
		Status:    notifications.StatusSent,
		MessageID: result.MessageID,
	}, nil
}

// SendPromotional sends a promotional SMS (lower cost, may be filtered by carriers)
func (c *SMSChannel) SendPromotional(ctx context.Context, phoneNumber, message string) (*notifications.DeliveryStatusResponse, error) {
	msg := awspkg.SMSMessage{
		PhoneNumber: phoneNumber,
		Message:     message,
		MessageType: awspkg.SMSTypePromotional,
	}

	result, err := c.client.SendSMS(ctx, msg)
	if err != nil {
		return &notifications.DeliveryStatusResponse{
			Channel: notifications.ChannelSMS,
			Status:  notifications.StatusFailed,
			Error:   err.Error(),
		}, err
	}

	return &notifications.DeliveryStatusResponse{
		Channel:   notifications.ChannelSMS,
		Status:    notifications.StatusSent,
		MessageID: result.MessageID,
	}, nil
}

// SendBulk sends SMS messages to multiple phone numbers
func (c *SMSChannel) SendBulk(ctx context.Context, phoneNumbers []string, message string) ([]notifications.DeliveryStatusResponse, error) {
	results := make([]notifications.DeliveryStatusResponse, 0, len(phoneNumbers))

	for _, phone := range phoneNumbers {
		result, err := c.Send(ctx, phone, message)
		if err != nil {
			results = append(results, notifications.DeliveryStatusResponse{
				UserID:  phone,
				Channel: notifications.ChannelSMS,
				Status:  notifications.StatusFailed,
				Error:   err.Error(),
			})
			continue
		}
		result.UserID = phone
		results = append(results, *result)
	}

	return results, nil
}

// SendWithOptions sends an SMS with custom options
func (c *SMSChannel) SendWithOptions(ctx context.Context, options SMSOptions) (*notifications.DeliveryStatusResponse, error) {
	msgType := awspkg.SMSTypeTransactional
	if options.IsPromotional {
		msgType = awspkg.SMSTypePromotional
	}

	msg := awspkg.SMSMessage{
		PhoneNumber: options.PhoneNumber,
		Message:     options.Message,
		MessageType: msgType,
		SenderID:    options.SenderID,
	}

	result, err := c.client.SendSMS(ctx, msg)
	if err != nil {
		return &notifications.DeliveryStatusResponse{
			Channel: notifications.ChannelSMS,
			Status:  notifications.StatusFailed,
			Error:   err.Error(),
		}, err
	}

	return &notifications.DeliveryStatusResponse{
		Channel:   notifications.ChannelSMS,
		Status:    notifications.StatusSent,
		MessageID: result.MessageID,
	}, nil
}

// CheckOptOut checks if a phone number has opted out of SMS
func (c *SMSChannel) CheckOptOut(ctx context.Context, phoneNumber string) (bool, error) {
	return c.client.CheckIfPhoneNumberIsOptedOut(ctx, phoneNumber)
}

// SMSOptions represents SMS sending options
type SMSOptions struct {
	PhoneNumber   string
	Message       string
	SenderID      string
	IsPromotional bool
}

// Validate validates the SMS options
func (o SMSOptions) Validate() error {
	if o.PhoneNumber == "" {
		return fmt.Errorf("phone number is required")
	}
	if o.Message == "" {
		return fmt.Errorf("message is required")
	}
	// SMS messages should be under 160 characters for single segment
	// AWS SNS will split longer messages
	if len(o.Message) > 1600 {
		return fmt.Errorf("message too long (max 1600 characters)")
	}
	return nil
}
