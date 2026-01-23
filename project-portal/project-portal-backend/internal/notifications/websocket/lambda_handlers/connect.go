package lambda_handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"carbon-scribe/project-portal/project-portal-backend/internal/notifications"
	awspkg "carbon-scribe/project-portal/project-portal-backend/pkg/aws"
)

// ConnectHandler handles WebSocket $connect route
type ConnectHandler struct {
	db        *awspkg.DynamoDBClient
	tableName string
	jwtSecret string
}

// NewConnectHandler creates a new connect handler
func NewConnectHandler(db *awspkg.DynamoDBClient, tableName, jwtSecret string) *ConnectHandler {
	return &ConnectHandler{
		db:        db,
		tableName: tableName,
		jwtSecret: jwtSecret,
	}
}

// Handle handles the WebSocket connect event
func (h *ConnectHandler) Handle(ctx context.Context, request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := request.RequestContext.ConnectionID

	// Extract authentication from query params or headers
	token := request.QueryStringParameters["token"]
	if token == "" {
		token = request.Headers["Authorization"]
	}

	// For now, allow all connections (auth can be done post-connect)
	// In production, validate the token here

	// Extract user info from token if present
	userID := "anonymous"
	var projectIDs []string

	// TODO: Implement proper JWT validation
	// For now, check for a simple user_id query param for testing
	if uid := request.QueryStringParameters["user_id"]; uid != "" {
		userID = uid
	}

	// Create connection record
	now := time.Now().UTC().Format(time.RFC3339)
	conn := &notifications.WebSocketConnection{
		ConnectionID: connectionID,
		UserID:       userID,
		ProjectIDs:   projectIDs,
		Channels:     []string{},
		ConnectedAt:  now,
		LastActivity: now,
		UserAgent:    request.Headers["User-Agent"],
		IPAddress:    request.RequestContext.Identity.SourceIP,
		TTL:          time.Now().Add(24 * time.Hour).Unix(),
	}

	// Save to DynamoDB
	if err := h.saveConnection(ctx, conn); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Failed to save connection",
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "Connected",
	}, nil
}

func (h *ConnectHandler) saveConnection(ctx context.Context, conn *notifications.WebSocketConnection) error {
	item := map[string]interface{}{
		"PK":           "CONNECTION#" + conn.ConnectionID,
		"connectionId": conn.ConnectionID,
		"userId":       conn.UserID,
		"connectedAt":  conn.ConnectedAt,
		"lastActivity": conn.LastActivity,
		"userAgent":    conn.UserAgent,
		"ipAddress":    conn.IPAddress,
		"ttl":          conn.TTL,
	}

	if len(conn.ProjectIDs) > 0 {
		item["projectIds"] = conn.ProjectIDs
	}
	if len(conn.Channels) > 0 {
		item["channels"] = conn.Channels
	}

	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	_ = data // TODO: Use DynamoDB client to save
	return nil
}
