package websocket

import (
	"context"
	"sync"
	"time"

	"carbon-scribe/project-portal/project-portal-backend/internal/notifications"
	awspkg "carbon-scribe/project-portal/project-portal-backend/pkg/aws"
	wspkg "carbon-scribe/project-portal/project-portal-backend/pkg/websocket"
)

// Manager manages WebSocket connections and message routing
type Manager struct {
	apiGateway *awspkg.APIGatewayClient
	repo       ConnectionRepository
	auth       *wspkg.Authenticator
	channels   *ChannelManager
	mu         sync.RWMutex
}

// ConnectionRepository defines the interface for connection storage
type ConnectionRepository interface {
	SaveConnection(ctx context.Context, conn *notifications.WebSocketConnection) error
	GetConnection(ctx context.Context, connectionID string) (*notifications.WebSocketConnection, error)
	DeleteConnection(ctx context.Context, connectionID string) error
	GetConnectionsByUser(ctx context.Context, userID string) ([]notifications.WebSocketConnection, error)
	GetConnectionsByChannel(ctx context.Context, channel string) ([]notifications.WebSocketConnection, error)
}

// ManagerConfig holds configuration for the WebSocket manager
type ManagerConfig struct {
	JWTSecret              string
	MaxConnectionsPerUser  int
	ConnectionTTL          time.Duration
	PingInterval           time.Duration
}

// DefaultManagerConfig returns default configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxConnectionsPerUser: 5,
		ConnectionTTL:         24 * time.Hour,
		PingInterval:          30 * time.Second,
	}
}

// NewManager creates a new WebSocket manager
func NewManager(apiGateway *awspkg.APIGatewayClient, repo ConnectionRepository, cfg ManagerConfig) *Manager {
	return &Manager{
		apiGateway: apiGateway,
		repo:       repo,
		auth:       wspkg.NewAuthenticator(wspkg.AuthConfig{JWTSecret: cfg.JWTSecret}),
		channels:   NewChannelManager(),
	}
}

// HandleConnect handles a new WebSocket connection
func (m *Manager) HandleConnect(ctx context.Context, connectionID string, queryParams map[string]string, headers map[string]string) error {
	// Authenticate the connection
	authResult := m.auth.AuthenticateQueryParam(ctx, queryParams)
	if !authResult.Authenticated {
		return authResult.Error
	}

	// Create connection record
	conn := &notifications.WebSocketConnection{
		ConnectionID: connectionID,
		UserID:       authResult.UserID,
		ProjectIDs:   authResult.ProjectIDs,
		Channels:     []string{},
		ConnectedAt:  time.Now().UTC().Format(time.RFC3339),
		LastActivity: time.Now().UTC().Format(time.RFC3339),
		UserAgent:    headers["User-Agent"],
		IPAddress:    headers["X-Forwarded-For"],
	}

	// Save connection
	if err := m.repo.SaveConnection(ctx, conn); err != nil {
		return err
	}

	// Auto-subscribe to user channel
	userChannel := wspkg.GetUserChannel(authResult.UserID)
	m.channels.Subscribe(connectionID, userChannel)

	// Auto-subscribe to project channels
	for _, projectID := range authResult.ProjectIDs {
		projectChannel := wspkg.GetProjectChannel(projectID)
		m.channels.Subscribe(connectionID, projectChannel)
	}

	return nil
}

// HandleDisconnect handles a WebSocket disconnection
func (m *Manager) HandleDisconnect(ctx context.Context, connectionID string) error {
	// Get connection to clean up channel subscriptions
	conn, err := m.repo.GetConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if conn != nil {
		// Unsubscribe from all channels
		for _, channel := range conn.Channels {
			m.channels.Unsubscribe(connectionID, channel)
		}
	}

	// Delete connection record
	return m.repo.DeleteConnection(ctx, connectionID)
}

// HandleMessage handles an incoming WebSocket message
func (m *Manager) HandleMessage(ctx context.Context, connectionID string, message *wspkg.Message) error {
	// Update last activity
	conn, err := m.repo.GetConnection(ctx, connectionID)
	if err != nil {
		return err
	}
	if conn == nil {
		return nil
	}

	conn.LastActivity = time.Now().UTC().Format(time.RFC3339)
	if err := m.repo.SaveConnection(ctx, conn); err != nil {
		return err
	}

	switch message.Type {
	case wspkg.MessageTypePing:
		return m.handlePing(ctx, connectionID)
	case wspkg.MessageTypeSubscribe:
		return m.handleSubscribe(ctx, connectionID, conn, message)
	case wspkg.MessageTypeUnsubscribe:
		return m.handleUnsubscribe(ctx, connectionID, conn, message)
	default:
		// Unknown message type, ignore
		return nil
	}
}

func (m *Manager) handlePing(ctx context.Context, connectionID string) error {
	pong := wspkg.NewPongMessage()
	data, err := pong.ToJSON()
	if err != nil {
		return err
	}
	return m.apiGateway.PostToConnection(ctx, connectionID, data)
}

func (m *Manager) handleSubscribe(ctx context.Context, connectionID string, conn *notifications.WebSocketConnection, message *wspkg.Message) error {
	channels, ok := message.Payload["channels"].([]interface{})
	if !ok {
		return nil
	}

	authResult := &wspkg.AuthResult{
		Authenticated: true,
		UserID:        conn.UserID,
		ProjectIDs:    conn.ProjectIDs,
	}

	for _, ch := range channels {
		channel, ok := ch.(string)
		if !ok {
			continue
		}

		// Check if user can subscribe to this channel
		if !authResult.CanSubscribeToChannel(channel) {
			continue
		}

		m.channels.Subscribe(connectionID, channel)

		// Update connection record
		found := false
		for _, c := range conn.Channels {
			if c == channel {
				found = true
				break
			}
		}
		if !found {
			conn.Channels = append(conn.Channels, channel)
		}
	}

	// Save updated channels
	return m.repo.SaveConnection(ctx, conn)
}

func (m *Manager) handleUnsubscribe(ctx context.Context, connectionID string, conn *notifications.WebSocketConnection, message *wspkg.Message) error {
	channels, ok := message.Payload["channels"].([]interface{})
	if !ok {
		return nil
	}

	for _, ch := range channels {
		channel, ok := ch.(string)
		if !ok {
			continue
		}

		m.channels.Unsubscribe(connectionID, channel)

		// Update connection record
		newChannels := make([]string, 0)
		for _, c := range conn.Channels {
			if c != channel {
				newChannels = append(newChannels, c)
			}
		}
		conn.Channels = newChannels
	}

	return m.repo.SaveConnection(ctx, conn)
}

// SendToUser sends a message to all connections of a user
func (m *Manager) SendToUser(ctx context.Context, userID string, message *wspkg.Message) error {
	conns, err := m.repo.GetConnectionsByUser(ctx, userID)
	if err != nil {
		return err
	}

	data, err := message.ToJSON()
	if err != nil {
		return err
	}

	for _, conn := range conns {
		if err := m.apiGateway.PostToConnection(ctx, conn.ConnectionID, data); err != nil {
			// Connection might be stale, clean it up
			_ = m.repo.DeleteConnection(ctx, conn.ConnectionID)
		}
	}

	return nil
}

// SendToChannel sends a message to all connections subscribed to a channel
func (m *Manager) SendToChannel(ctx context.Context, channel string, message *wspkg.Message) error {
	connectionIDs := m.channels.GetSubscribers(channel)

	data, err := message.ToJSON()
	if err != nil {
		return err
	}

	for _, connID := range connectionIDs {
		if err := m.apiGateway.PostToConnection(ctx, connID, data); err != nil {
			// Connection might be stale
			m.channels.Unsubscribe(connID, channel)
			_ = m.repo.DeleteConnection(ctx, connID)
		}
	}

	return nil
}

// Broadcast sends a message to all connections
func (m *Manager) Broadcast(ctx context.Context, message *wspkg.Message) error {
	return m.SendToChannel(ctx, wspkg.GetGlobalChannel(), message)
}

// GetConnectionCount returns the number of active connections
func (m *Manager) GetConnectionCount() int {
	return m.channels.GetTotalConnectionCount()
}

// ChannelManager manages channel subscriptions in memory
type ChannelManager struct {
	channels map[string]map[string]bool // channel -> connectionID -> true
	mu       sync.RWMutex
}

// NewChannelManager creates a new channel manager
func NewChannelManager() *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]map[string]bool),
	}
}

// Subscribe subscribes a connection to a channel
func (cm *ChannelManager) Subscribe(connectionID, channel string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.channels[channel] == nil {
		cm.channels[channel] = make(map[string]bool)
	}
	cm.channels[channel][connectionID] = true
}

// Unsubscribe unsubscribes a connection from a channel
func (cm *ChannelManager) Unsubscribe(connectionID, channel string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.channels[channel] != nil {
		delete(cm.channels[channel], connectionID)
		if len(cm.channels[channel]) == 0 {
			delete(cm.channels, channel)
		}
	}
}

// GetSubscribers returns all connection IDs subscribed to a channel
func (cm *ChannelManager) GetSubscribers(channel string) []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	subscribers := make([]string, 0)
	if cm.channels[channel] != nil {
		for connID := range cm.channels[channel] {
			subscribers = append(subscribers, connID)
		}
	}
	return subscribers
}

// GetTotalConnectionCount returns the total number of unique connections
func (cm *ChannelManager) GetTotalConnectionCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	unique := make(map[string]bool)
	for _, conns := range cm.channels {
		for connID := range conns {
			unique[connID] = true
		}
	}
	return len(unique)
}
