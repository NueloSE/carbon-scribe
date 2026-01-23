package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

// SESClient wraps the AWS SES client
type SESClient struct {
	client    *ses.Client
	fromEmail string
}

// SESConfig holds SES client configuration
type SESConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	FromEmail       string
}

// EmailMessage represents an email to be sent
type EmailMessage struct {
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	HTMLBody    string
	TextBody    string
	ReplyTo     []string
	Headers     map[string]string
	Tags        map[string]string
	ConfigSet   string
}

// EmailResult represents the result of sending an email
type EmailResult struct {
	MessageID string
	Success   bool
	Error     error
}

// NewSESClient creates a new SES client
func NewSESClient(ctx context.Context, cfg SESConfig) (*SESClient, error) {
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

	client := ses.NewFromConfig(awsCfg)
	return &SESClient{
		client:    client,
		fromEmail: cfg.FromEmail,
	}, nil
}

// SendEmail sends an email using SES
func (c *SESClient) SendEmail(ctx context.Context, msg EmailMessage) (*EmailResult, error) {
	destination := &types.Destination{
		ToAddresses:  msg.To,
		CcAddresses:  msg.CC,
		BccAddresses: msg.BCC,
	}

	body := &types.Body{}
	if msg.HTMLBody != "" {
		body.Html = &types.Content{
			Charset: aws.String("UTF-8"),
			Data:    aws.String(msg.HTMLBody),
		}
	}
	if msg.TextBody != "" {
		body.Text = &types.Content{
			Charset: aws.String("UTF-8"),
			Data:    aws.String(msg.TextBody),
		}
	}

	message := &types.Message{
		Subject: &types.Content{
			Charset: aws.String("UTF-8"),
			Data:    aws.String(msg.Subject),
		},
		Body: body,
	}

	input := &ses.SendEmailInput{
		Source:      aws.String(c.fromEmail),
		Destination: destination,
		Message:     message,
	}

	if len(msg.ReplyTo) > 0 {
		input.ReplyToAddresses = msg.ReplyTo
	}

	if msg.ConfigSet != "" {
		input.ConfigurationSetName = aws.String(msg.ConfigSet)
	}

	// Add tags for tracking
	if len(msg.Tags) > 0 {
		var tags []types.MessageTag
		for k, v := range msg.Tags {
			tags = append(tags, types.MessageTag{
				Name:  aws.String(k),
				Value: aws.String(v),
			})
		}
		input.Tags = tags
	}

	result, err := c.client.SendEmail(ctx, input)
	if err != nil {
		return &EmailResult{
			Success: false,
			Error:   err,
		}, err
	}

	return &EmailResult{
		MessageID: *result.MessageId,
		Success:   true,
	}, nil
}

// SendRawEmail sends a raw email (for more control over headers)
func (c *SESClient) SendRawEmail(ctx context.Context, rawMessage []byte, destinations []string) (*EmailResult, error) {
	input := &ses.SendRawEmailInput{
		Source:       aws.String(c.fromEmail),
		Destinations: destinations,
		RawMessage: &types.RawMessage{
			Data: rawMessage,
		},
	}

	result, err := c.client.SendRawEmail(ctx, input)
	if err != nil {
		return &EmailResult{
			Success: false,
			Error:   err,
		}, err
	}

	return &EmailResult{
		MessageID: *result.MessageId,
		Success:   true,
	}, nil
}

// SendTemplatedEmail sends an email using an SES template
func (c *SESClient) SendTemplatedEmail(ctx context.Context, to []string, templateName string, templateData string) (*EmailResult, error) {
	input := &ses.SendTemplatedEmailInput{
		Source: aws.String(c.fromEmail),
		Destination: &types.Destination{
			ToAddresses: to,
		},
		Template:     aws.String(templateName),
		TemplateData: aws.String(templateData),
	}

	result, err := c.client.SendTemplatedEmail(ctx, input)
	if err != nil {
		return &EmailResult{
			Success: false,
			Error:   err,
		}, err
	}

	return &EmailResult{
		MessageID: *result.MessageId,
		Success:   true,
	}, nil
}

// SendBulkTemplatedEmail sends templated emails to multiple recipients
func (c *SESClient) SendBulkTemplatedEmail(ctx context.Context, template string, destinations []BulkEmailDestination) ([]EmailResult, error) {
	var bulkDests []types.BulkEmailDestination
	for _, dest := range destinations {
		bulkDests = append(bulkDests, types.BulkEmailDestination{
			Destination: &types.Destination{
				ToAddresses: dest.To,
			},
			ReplacementTemplateData: aws.String(dest.TemplateData),
		})
	}

	input := &ses.SendBulkTemplatedEmailInput{
		Source:       aws.String(c.fromEmail),
		Template:     aws.String(template),
		Destinations: bulkDests,
		DefaultTemplateData: aws.String("{}"),
	}

	result, err := c.client.SendBulkTemplatedEmail(ctx, input)
	if err != nil {
		return nil, err
	}

	var results []EmailResult
	for _, status := range result.Status {
		r := EmailResult{
			Success: status.Status != nil && *status.Status == "Success",
		}
		if status.MessageId != nil {
			r.MessageID = *status.MessageId
		}
		if status.Error != nil {
			r.Error = fmt.Errorf("%s", *status.Error)
		}
		results = append(results, r)
	}

	return results, nil
}

// BulkEmailDestination represents a destination for bulk email
type BulkEmailDestination struct {
	To           []string
	TemplateData string
}

// VerifyEmailIdentity verifies an email address for sending
func (c *SESClient) VerifyEmailIdentity(ctx context.Context, email string) error {
	_, err := c.client.VerifyEmailIdentity(ctx, &ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(email),
	})
	return err
}

// GetSendQuota gets the current sending quota
func (c *SESClient) GetSendQuota(ctx context.Context) (*ses.GetSendQuotaOutput, error) {
	return c.client.GetSendQuota(ctx, &ses.GetSendQuotaInput{})
}

// GetSendStatistics gets sending statistics
func (c *SESClient) GetSendStatistics(ctx context.Context) (*ses.GetSendStatisticsOutput, error) {
	return c.client.GetSendStatistics(ctx, &ses.GetSendStatisticsInput{})
}
