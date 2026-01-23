package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBClient wraps the AWS DynamoDB client
type DynamoDBClient struct {
	client *dynamodb.Client
}

// DynamoDBConfig holds DynamoDB client configuration
type DynamoDBConfig struct {
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
}

// NewDynamoDBClient creates a new DynamoDB client
func NewDynamoDBClient(ctx context.Context, cfg DynamoDBConfig) (*DynamoDBClient, error) {
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

	var clientOpts []func(*dynamodb.Options)
	if cfg.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := dynamodb.NewFromConfig(awsCfg, clientOpts...)
	return &DynamoDBClient{client: client}, nil
}

// PutItem puts an item into a DynamoDB table
func (c *DynamoDBClient) PutItem(ctx context.Context, tableName string, item map[string]types.AttributeValue) error {
	_, err := c.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	return err
}

// GetItem retrieves an item from a DynamoDB table
func (c *DynamoDBClient) GetItem(ctx context.Context, tableName string, key map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	result, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	})
	if err != nil {
		return nil, err
	}
	return result.Item, nil
}

// DeleteItem deletes an item from a DynamoDB table
func (c *DynamoDBClient) DeleteItem(ctx context.Context, tableName string, key map[string]types.AttributeValue) error {
	_, err := c.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	})
	return err
}

// Query queries items from a DynamoDB table
func (c *DynamoDBClient) Query(ctx context.Context, input *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
	return c.client.Query(ctx, input)
}

// Scan scans items from a DynamoDB table
func (c *DynamoDBClient) Scan(ctx context.Context, input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
	return c.client.Scan(ctx, input)
}

// UpdateItem updates an item in a DynamoDB table
func (c *DynamoDBClient) UpdateItem(ctx context.Context, input *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	return c.client.UpdateItem(ctx, input)
}

// BatchWriteItem writes multiple items to DynamoDB tables
func (c *DynamoDBClient) BatchWriteItem(ctx context.Context, input *dynamodb.BatchWriteItemInput) (*dynamodb.BatchWriteItemOutput, error) {
	return c.client.BatchWriteItem(ctx, input)
}

// TransactWriteItems performs a transactional write
func (c *DynamoDBClient) TransactWriteItems(ctx context.Context, input *dynamodb.TransactWriteItemsInput) (*dynamodb.TransactWriteItemsOutput, error) {
	return c.client.TransactWriteItems(ctx, input)
}

// CreateTable creates a new DynamoDB table (for testing/local development)
func (c *DynamoDBClient) CreateTable(ctx context.Context, input *dynamodb.CreateTableInput) (*dynamodb.CreateTableOutput, error) {
	return c.client.CreateTable(ctx, input)
}

// DescribeTable describes a DynamoDB table
func (c *DynamoDBClient) DescribeTable(ctx context.Context, tableName string) (*dynamodb.DescribeTableOutput, error) {
	return c.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
}
