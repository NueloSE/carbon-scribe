package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"carbon-scribe/project-portal/project-portal-backend/internal/notifications"
	awspkg "carbon-scribe/project-portal/project-portal-backend/pkg/aws"
	wspkg "carbon-scribe/project-portal/project-portal-backend/pkg/websocket"
)

// WebSocketChannel handles WebSocket notifications
type WebSocketChannel struct {
	apiGateway *awspkg.APIGatewayClient
	repo       ConnectionRepository
	mu         sync.RWMutex
}

// ConnectionRepository defines the interface for connection storage
type ConnectionRepository interface {
	GetConnection(ctx context.Context, connectionID string) (*notifications.WebSocketConnection, error)
	GetConnectionsByUser(ctx context.Context, userID string) ([]notifications.WebSocketConnection, error)
	GetConnectionsByChannel(ctx context.Context, channel string) ([]notifications.WebSocketConnection, error)
	SaveConnection(ctx context.Context, conn *notifications.WebSocketConnection) error
	DeleteConnection(ctx context.Context, connectionID string) error
}

// NewWebSocketChannel creates a new WebSocket channel
func NewWebSocketChannel(apiGateway *awspkg.APIGatewayClient, repo ConnectionRepository) *WebSocketChannel {
	return &WebSocketChannel{
		apiGateway: apiGateway,
		repo:       repo,
	}
}

// SendToUser sends a message to all connections of a user
func (c *WebSocketChannel) SendToUser(ctx context.Context, userID string, message *wspkg.Message) (*notifications.DeliveryStatusResponse, error) {
	conns, err := c.repo.GetConnectionsByUser(ctx, userID)
	if err != nil {
		return &notifications.DeliveryStatusResponse{
			UserID:  userID,
			Channel: notifications.ChannelWebSocket,
			Status:  notifications.StatusFailed,
			Error:   err.Error(),
		}, err
	}

	if len(conns) == 0 {
		return &notifications.DeliveryStatusResponse{
			UserID:  userID,
			Channel: notifications.ChannelWebSocket,
			Status:  notifications.StatusFailed,
			Error:   "no active connections",
		}, fmt.Errorf("no active connections for user")
	}

	data, err := message.ToJSON()
	if err != nil {
		return &notifications.DeliveryStatusResponse{
			UserID:  userID,
			Channel: notifications.ChannelWebSocket,
			Status:  notifications.StatusFailed,
			Error:   err.Error(),
		}, err
	}

	var sentCount int
	var lastErr error
	for _, conn := range conns {
		if err := c.apiGateway.PostToConnection(ctx, conn.ConnectionID, data); err != nil {
			lastErr = err
			// Connection might be stale, delete it
			_ = c.repo.DeleteConnection(ctx, conn.ConnectionID)
			continue
		}
		sentCount++
	}

	if sentCount == 0 {
		return &notifications.DeliveryStatusResponse{
			UserID:  userID,
			Channel: notifications.ChannelWebSocket,
			Status:  notifications.StatusFailed,
			Error:   lastErr.Error(),
		}, lastErr
	}

	return &notifications.DeliveryStatusResponse{
		UserID:  userID,
		Channel: notifications.ChannelWebSocket,
		Status:  notifications.StatusDelivered,
	}, nil
}

// SendToConnection sends a message to a specific connection
func (c *WebSocketChannel) SendToConnection(ctx context.Context, connectionID string, message *wspkg.Message) error {
	data, err := message.ToJSON()
	if err != nil {
		return err
	}

	err = c.apiGateway.PostToConnection(ctx, connectionID, data)
	if err != nil {
		// Connection might be stale, delete it
		_ = c.repo.DeleteConnection(ctx, connectionID)
		return err
	}

	return nil
}

// SendToChannel sends a message to all connections subscribed to a channel
func (c *WebSocketChannel) SendToChannel(ctx context.Context, channel string, message *wspkg.Message) ([]notifications.DeliveryStatusResponse, error) {
	conns, err := c.repo.GetConnectionsByChannel(ctx, channel)
	if err != nil {
		return nil, err
	}

	data, err := message.ToJSON()
	if err != nil {
		return nil, err
	}

	results := make([]notifications.DeliveryStatusResponse, 0, len(conns))
	for _, conn := range conns {
		err := c.apiGateway.PostToConnection(ctx, conn.ConnectionID, data)
		if err != nil {
			_ = c.repo.DeleteConnection(ctx, conn.ConnectionID)
			results = append(results, notifications.DeliveryStatusResponse{
				UserID:  conn.UserID,
				Channel: notifications.ChannelWebSocket,
				Status:  notifications.StatusFailed,
				Error:   err.Error(),
			})
			continue
		}
		results = append(results, notifications.DeliveryStatusResponse{
			UserID:  conn.UserID,
			Channel: notifications.ChannelWebSocket,
			Status:  notifications.StatusDelivered,
		})
	}

	return results, nil
}

// Broadcast sends a message to all active connections
func (c *WebSocketChannel) Broadcast(ctx context.Context, message *wspkg.Message) ([]notifications.DeliveryStatusResponse, error) {
	return c.SendToChannel(ctx, wspkg.GetGlobalChannel(), message)
}

// SendNotification sends a notification message to a user
func (c *WebSocketChannel) SendNotification(ctx context.Context, userID, notificationID string, payload map[string]interface{}) (*notifications.DeliveryStatusResponse, error) {
	message := wspkg.NewNotificationMessage(notificationID, payload)
	return c.SendToUser(ctx, userID, message)
}

// SendAlert sends an alert message to a user
func (c *WebSocketChannel) SendAlert(ctx context.Context, userID, alertID string, payload map[string]interface{}) (*notifications.DeliveryStatusResponse, error) {
	message := wspkg.NewAlertMessage(alertID, payload)
	return c.SendToUser(ctx, userID, message)
}

// BroadcastToUsers broadcasts a message to multiple users
func (c *WebSocketChannel) BroadcastToUsers(ctx context.Context, userIDs []string, message *wspkg.Message) ([]notifications.DeliveryStatusResponse, error) {
	results := make([]notifications.DeliveryStatusResponse, 0, len(userIDs))

	for _, userID := range userIDs {
		result, _ := c.SendToUser(ctx, userID, message)
		results = append(results, *result)
	}

	return results, nil
}

// SendRaw sends raw JSON data to a connection
func (c *WebSocketChannel) SendRaw(ctx context.Context, connectionID string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = c.apiGateway.PostToConnection(ctx, connectionID, jsonData)
	if err != nil {
		_ = c.repo.DeleteConnection(ctx, connectionID)
		return err
	}

	return nil
}

// DisconnectUser forcefully disconnects all connections for a user
func (c *WebSocketChannel) DisconnectUser(ctx context.Context, userID string) error {
	conns, err := c.repo.GetConnectionsByUser(ctx, userID)
	if err != nil {
		return err
	}

	for _, conn := range conns {
		_ = c.apiGateway.DeleteConnection(ctx, conn.ConnectionID)
		_ = c.repo.DeleteConnection(ctx, conn.ConnectionID)
	}

	return nil
}
