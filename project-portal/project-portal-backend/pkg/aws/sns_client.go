package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// SNSClient wraps the AWS SNS client
type SNSClient struct {
	client   *sns.Client
	senderID string
}

// SNSConfig holds SNS client configuration
type SNSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SMSSenderID     string
}

// SMSMessage represents an SMS message to be sent
type SMSMessage struct {
	PhoneNumber string
	Message     string
	MessageType SMSMessageType
	SenderID    string
}

// SMSMessageType represents the type of SMS message
type SMSMessageType string

const (
	SMSTypeTransactional SMSMessageType = "Transactional"
	SMSTypePromotional   SMSMessageType = "Promotional"
)

// SMSResult represents the result of sending an SMS
type SMSResult struct {
	MessageID string
	Success   bool
	Error     error
}

// NewSNSClient creates a new SNS client
func NewSNSClient(ctx context.Context, cfg SNSConfig) (*SNSClient, error) {
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

	client := sns.NewFromConfig(awsCfg)
	return &SNSClient{
		client:   client,
		senderID: cfg.SMSSenderID,
	}, nil
}

// SendSMS sends an SMS message
func (c *SNSClient) SendSMS(ctx context.Context, msg SMSMessage) (*SMSResult, error) {
	senderID := msg.SenderID
	if senderID == "" {
		senderID = c.senderID
	}

	messageType := string(msg.MessageType)
	if messageType == "" {
		messageType = string(SMSTypeTransactional)
	}

	input := &sns.PublishInput{
		PhoneNumber: aws.String(msg.PhoneNumber),
		Message:     aws.String(msg.Message),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"AWS.SNS.SMS.SMSType": {
				DataType:    aws.String("String"),
				StringValue: aws.String(messageType),
			},
		},
	}

	if senderID != "" {
		input.MessageAttributes["AWS.SNS.SMS.SenderID"] = types.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(senderID),
		}
	}

	result, err := c.client.Publish(ctx, input)
	if err != nil {
		return &SMSResult{
			Success: false,
			Error:   err,
		}, err
	}

	return &SMSResult{
		MessageID: *result.MessageId,
		Success:   true,
	}, nil
}

// PublishToTopic publishes a message to an SNS topic
func (c *SNSClient) PublishToTopic(ctx context.Context, topicARN string, message string, attributes map[string]string) (*SMSResult, error) {
	msgAttrs := make(map[string]types.MessageAttributeValue)
	for k, v := range attributes {
		msgAttrs[k] = types.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(v),
		}
	}

	input := &sns.PublishInput{
		TopicArn:          aws.String(topicARN),
		Message:           aws.String(message),
		MessageAttributes: msgAttrs,
	}

	result, err := c.client.Publish(ctx, input)
	if err != nil {
		return &SMSResult{
			Success: false,
			Error:   err,
		}, err
	}

	return &SMSResult{
		MessageID: *result.MessageId,
		Success:   true,
	}, nil
}

// CreateTopic creates a new SNS topic
func (c *SNSClient) CreateTopic(ctx context.Context, name string, attributes map[string]string) (string, error) {
	input := &sns.CreateTopicInput{
		Name:       aws.String(name),
		Attributes: attributes,
	}

	result, err := c.client.CreateTopic(ctx, input)
	if err != nil {
		return "", err
	}

	return *result.TopicArn, nil
}

// Subscribe subscribes to an SNS topic
func (c *SNSClient) Subscribe(ctx context.Context, topicARN, protocol, endpoint string) (string, error) {
	input := &sns.SubscribeInput{
		TopicArn: aws.String(topicARN),
		Protocol: aws.String(protocol),
		Endpoint: aws.String(endpoint),
	}

	result, err := c.client.Subscribe(ctx, input)
	if err != nil {
		return "", err
	}

	if result.SubscriptionArn != nil {
		return *result.SubscriptionArn, nil
	}
	return "", nil
}

// Unsubscribe unsubscribes from an SNS topic
func (c *SNSClient) Unsubscribe(ctx context.Context, subscriptionARN string) error {
	_, err := c.client.Unsubscribe(ctx, &sns.UnsubscribeInput{
		SubscriptionArn: aws.String(subscriptionARN),
	})
	return err
}

// SetSMSAttributes sets SMS sending attributes
func (c *SNSClient) SetSMSAttributes(ctx context.Context, attributes map[string]string) error {
	_, err := c.client.SetSMSAttributes(ctx, &sns.SetSMSAttributesInput{
		Attributes: attributes,
	})
	return err
}

// CheckIfPhoneNumberIsOptedOut checks if a phone number has opted out of SMS
func (c *SNSClient) CheckIfPhoneNumberIsOptedOut(ctx context.Context, phoneNumber string) (bool, error) {
	result, err := c.client.CheckIfPhoneNumberIsOptedOut(ctx, &sns.CheckIfPhoneNumberIsOptedOutInput{
		PhoneNumber: aws.String(phoneNumber),
	})
	if err != nil {
		return false, err
	}
	return result.IsOptedOut, nil
}
