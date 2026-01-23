package lambda_handlers

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awspkg "carbon-scribe/project-portal/project-portal-backend/pkg/aws"
)

// DisconnectHandler handles WebSocket $disconnect route
type DisconnectHandler struct {
	db        *awspkg.DynamoDBClient
	tableName string
}

// NewDisconnectHandler creates a new disconnect handler
func NewDisconnectHandler(db *awspkg.DynamoDBClient, tableName string) *DisconnectHandler {
	return &DisconnectHandler{
		db:        db,
		tableName: tableName,
	}
}

// Handle handles the WebSocket disconnect event
func (h *DisconnectHandler) Handle(ctx context.Context, request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := request.RequestContext.ConnectionID

	// Delete connection from DynamoDB
	if err := h.deleteConnection(ctx, connectionID); err != nil {
		// Log but don't fail - connection cleanup is best effort
		// The TTL will eventually clean up stale connections
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       "Disconnected (with errors)",
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "Disconnected",
	}, nil
}

func (h *DisconnectHandler) deleteConnection(ctx context.Context, connectionID string) error {
	pk := "CONNECTION#" + connectionID

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
	}

	return h.db.DeleteItem(ctx, h.tableName, key)
}

// CleanupStaleConnections removes connections that haven't been active
// This can be run periodically by a scheduled Lambda
func (h *DisconnectHandler) CleanupStaleConnections(ctx context.Context, maxAgeMinutes int64) (int, error) {
	// In production, this would scan for connections older than maxAgeMinutes
	// and delete them. The TTL attribute should handle this automatically,
	// but this provides an additional cleanup mechanism.
	
	input := &dynamodb.ScanInput{
		TableName: aws.String(h.tableName),
		FilterExpression: aws.String("begins_with(PK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":prefix": &types.AttributeValueMemberS{Value: "CONNECTION#"},
		},
	}

	result, err := h.db.Scan(ctx, input)
	if err != nil {
		return 0, err
	}

	// For now, just return the count
	// In production, check lastActivity and delete if stale
	return len(result.Items), nil
}
