package channels

import (
	"context"
	"fmt"

	"carbon-scribe/project-portal/project-portal-backend/internal/notifications"
	awspkg "carbon-scribe/project-portal/project-portal-backend/pkg/aws"
)

// EmailChannel handles email notifications via AWS SES
type EmailChannel struct {
	client *awspkg.SESClient
}

// NewEmailChannel creates a new email channel
func NewEmailChannel(client *awspkg.SESClient) *EmailChannel {
	return &EmailChannel{client: client}
}

// Send sends an email notification
func (c *EmailChannel) Send(ctx context.Context, to string, subject, htmlBody, textBody string) (*notifications.DeliveryStatusResponse, error) {
	msg := awspkg.EmailMessage{
		To:       []string{to},
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	}

	result, err := c.client.SendEmail(ctx, msg)
	if err != nil {
		return &notifications.DeliveryStatusResponse{
			Channel: notifications.ChannelEmail,
			Status:  notifications.StatusFailed,
			Error:   err.Error(),
		}, err
	}

	return &notifications.DeliveryStatusResponse{
		Channel:   notifications.ChannelEmail,
		Status:    notifications.StatusSent,
		MessageID: result.MessageID,
	}, nil
}

// SendBulk sends emails to multiple recipients
func (c *EmailChannel) SendBulk(ctx context.Context, recipients []string, subject, htmlBody, textBody string) ([]notifications.DeliveryStatusResponse, error) {
	results := make([]notifications.DeliveryStatusResponse, 0, len(recipients))

	for _, to := range recipients {
		result, err := c.Send(ctx, to, subject, htmlBody, textBody)
		if err != nil {
			results = append(results, notifications.DeliveryStatusResponse{
				UserID:  to,
				Channel: notifications.ChannelEmail,
				Status:  notifications.StatusFailed,
				Error:   err.Error(),
			})
			continue
		}
		result.UserID = to
		results = append(results, *result)
	}

	return results, nil
}

// SendTemplated sends an email using a template stored in SES
func (c *EmailChannel) SendTemplated(ctx context.Context, to []string, templateName, templateData string) (*notifications.DeliveryStatusResponse, error) {
	result, err := c.client.SendTemplatedEmail(ctx, to, templateName, templateData)
	if err != nil {
		return &notifications.DeliveryStatusResponse{
			Channel: notifications.ChannelEmail,
			Status:  notifications.StatusFailed,
			Error:   err.Error(),
		}, err
	}

	return &notifications.DeliveryStatusResponse{
		Channel:   notifications.ChannelEmail,
		Status:    notifications.StatusSent,
		MessageID: result.MessageID,
	}, nil
}

// SendWithOptions sends an email with advanced options
func (c *EmailChannel) SendWithOptions(ctx context.Context, options EmailOptions) (*notifications.DeliveryStatusResponse, error) {
	msg := awspkg.EmailMessage{
		To:        options.To,
		CC:        options.CC,
		BCC:       options.BCC,
		Subject:   options.Subject,
		HTMLBody:  options.HTMLBody,
		TextBody:  options.TextBody,
		ReplyTo:   options.ReplyTo,
		ConfigSet: options.ConfigSet,
		Tags:      options.Tags,
	}

	result, err := c.client.SendEmail(ctx, msg)
	if err != nil {
		return &notifications.DeliveryStatusResponse{
			Channel: notifications.ChannelEmail,
			Status:  notifications.StatusFailed,
			Error:   err.Error(),
		}, err
	}

	return &notifications.DeliveryStatusResponse{
		Channel:   notifications.ChannelEmail,
		Status:    notifications.StatusSent,
		MessageID: result.MessageID,
	}, nil
}

// EmailOptions represents advanced email options
type EmailOptions struct {
	To        []string
	CC        []string
	BCC       []string
	Subject   string
	HTMLBody  string
	TextBody  string
	ReplyTo   []string
	ConfigSet string
	Tags      map[string]string
}

// Validate validates the email options
func (o EmailOptions) Validate() error {
	if len(o.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	if o.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if o.HTMLBody == "" && o.TextBody == "" {
		return fmt.Errorf("either HTML or text body is required")
	}
	return nil
}
