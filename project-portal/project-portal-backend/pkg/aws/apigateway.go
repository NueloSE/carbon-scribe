package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
)

// APIGatewayClient wraps the AWS API Gateway Management API client
type APIGatewayClient struct {
	client *apigatewaymanagementapi.Client
}

// APIGatewayConfig holds API Gateway client configuration
type APIGatewayConfig struct {
	Region          string
	Endpoint        string // WebSocket API endpoint (e.g., https://{api-id}.execute-api.{region}.amazonaws.com/{stage})
	AccessKeyID     string
	SecretAccessKey string
}

// NewAPIGatewayClient creates a new API Gateway Management API client
func NewAPIGatewayClient(ctx context.Context, cfg APIGatewayConfig) (*APIGatewayClient, error) {
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

	client := apigatewaymanagementapi.NewFromConfig(awsCfg, func(o *apigatewaymanagementapi.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	return &APIGatewayClient{client: client}, nil
}

// PostToConnection sends a message to a specific WebSocket connection
func (c *APIGatewayClient) PostToConnection(ctx context.Context, connectionID string, data []byte) error {
	_, err := c.client.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         data,
	})
	return err
}

// PostJSONToConnection sends a JSON message to a specific WebSocket connection
func (c *APIGatewayClient) PostJSONToConnection(ctx context.Context, connectionID string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return c.PostToConnection(ctx, connectionID, jsonData)
}

// DeleteConnection forcefully closes a WebSocket connection
func (c *APIGatewayClient) DeleteConnection(ctx context.Context, connectionID string) error {
	_, err := c.client.DeleteConnection(ctx, &apigatewaymanagementapi.DeleteConnectionInput{
		ConnectionId: aws.String(connectionID),
	})
	return err
}

// GetConnection retrieves information about a WebSocket connection
func (c *APIGatewayClient) GetConnection(ctx context.Context, connectionID string) (*ConnectionInfo, error) {
	result, err := c.client.GetConnection(ctx, &apigatewaymanagementapi.GetConnectionInput{
		ConnectionId: aws.String(connectionID),
	})
	if err != nil {
		return nil, err
	}

	return &ConnectionInfo{
		ConnectedAt:  result.ConnectedAt,
		LastActiveAt: result.LastActiveAt,
		Identity: &ConnectionIdentity{
			SourceIP:  aws.ToString(result.Identity.SourceIp),
			UserAgent: aws.ToString(result.Identity.UserAgent),
		},
	}, nil
}

// ConnectionInfo represents information about a WebSocket connection
type ConnectionInfo struct {
	ConnectedAt  *interface{}
	LastActiveAt *interface{}
	Identity     *ConnectionIdentity
}

// ConnectionIdentity represents the identity information of a connection
type ConnectionIdentity struct {
	SourceIP  string
	UserAgent string
}

// BroadcastToConnections sends a message to multiple WebSocket connections
// It returns a map of connection IDs to errors (nil if successful)
func (c *APIGatewayClient) BroadcastToConnections(ctx context.Context, connectionIDs []string, data []byte) map[string]error {
	results := make(map[string]error)
	for _, connID := range connectionIDs {
		err := c.PostToConnection(ctx, connID, data)
		results[connID] = err
	}
	return results
}

// BroadcastJSONToConnections sends a JSON message to multiple WebSocket connections
func (c *APIGatewayClient) BroadcastJSONToConnections(ctx context.Context, connectionIDs []string, data interface{}) (map[string]error, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return c.BroadcastToConnections(ctx, connectionIDs, jsonData), nil
}
