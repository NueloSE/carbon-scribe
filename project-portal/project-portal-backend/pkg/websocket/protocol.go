package websocket

import (
	"encoding/json"
	"time"
)

// MessageType represents the type of WebSocket message
type MessageType string

const (
	// Client to Server message types
	MessageTypeSubscribe   MessageType = "subscribe"
	MessageTypeUnsubscribe MessageType = "unsubscribe"
	MessageTypePing        MessageType = "ping"
	MessageTypeAuth        MessageType = "auth"

	// Server to Client message types
	MessageTypeNotification MessageType = "notification"
	MessageTypeAlert        MessageType = "alert"
	MessageTypePong         MessageType = "pong"
	MessageTypeError        MessageType = "error"
	MessageTypeAck          MessageType = "ack"
	MessageTypePresence     MessageType = "presence"
)

// Message represents a WebSocket message
type Message struct {
	Type      MessageType            `json:"type"`
	ID        string                 `json:"id,omitempty"`
	Channel   string                 `json:"channel,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewMessage creates a new WebSocket message
func NewMessage(msgType MessageType, payload map[string]interface{}) *Message {
	return &Message{
		Type:      msgType,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
}

// NewNotificationMessage creates a notification message
func NewNotificationMessage(notificationID string, data map[string]interface{}) *Message {
	return &Message{
		Type:      MessageTypeNotification,
		ID:        notificationID,
		Payload:   data,
		Timestamp: time.Now().UTC(),
	}
}

// NewAlertMessage creates an alert message
func NewAlertMessage(alertID string, data map[string]interface{}) *Message {
	return &Message{
		Type:      MessageTypeAlert,
		ID:        alertID,
		Payload:   data,
		Timestamp: time.Now().UTC(),
	}
}

// NewErrorMessage creates an error message
func NewErrorMessage(code string, message string) *Message {
	return &Message{
		Type: MessageTypeError,
		Payload: map[string]interface{}{
			"code":    code,
			"message": message,
		},
		Timestamp: time.Now().UTC(),
	}
}

// NewPongMessage creates a pong message
func NewPongMessage() *Message {
	return &Message{
		Type:      MessageTypePong,
		Timestamp: time.Now().UTC(),
	}
}

// NewAckMessage creates an acknowledgment message
func NewAckMessage(messageID string) *Message {
	return &Message{
		Type: MessageTypeAck,
		ID:   messageID,
		Payload: map[string]interface{}{
			"acknowledged": true,
		},
		Timestamp: time.Now().UTC(),
	}
}

// NewPresenceMessage creates a presence update message
func NewPresenceMessage(channel string, users []string, action string) *Message {
	return &Message{
		Type:    MessageTypePresence,
		Channel: channel,
		Payload: map[string]interface{}{
			"users":  users,
			"action": action,
		},
		Timestamp: time.Now().UTC(),
	}
}

// ToJSON serializes the message to JSON
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// ParseMessage parses a JSON message
func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// SubscribeRequest represents a channel subscription request
type SubscribeRequest struct {
	Channels []string `json:"channels"`
}

// UnsubscribeRequest represents a channel unsubscription request
type UnsubscribeRequest struct {
	Channels []string `json:"channels"`
}

// AuthRequest represents an authentication request
type AuthRequest struct {
	Token string `json:"token"`
}

// BroadcastRequest represents a broadcast request from admin
type BroadcastRequest struct {
	Channels []string               `json:"channels,omitempty"` // Empty means broadcast to all
	UserIDs  []string               `json:"user_ids,omitempty"` // Empty means broadcast to all
	Message  map[string]interface{} `json:"message"`
	Type     MessageType            `json:"type"`
}

// ChannelType represents the type of channel
type ChannelType string

const (
	ChannelTypeProject  ChannelType = "project"
	ChannelTypeUser     ChannelType = "user"
	ChannelTypeGlobal   ChannelType = "global"
	ChannelTypeAdmin    ChannelType = "admin"
)

// Channel represents a notification channel
type Channel struct {
	ID   string      `json:"id"`
	Type ChannelType `json:"type"`
	Name string      `json:"name"`
}

// GetProjectChannel returns the channel name for a project
func GetProjectChannel(projectID string) string {
	return "project:" + projectID
}

// GetUserChannel returns the channel name for a user
func GetUserChannel(userID string) string {
	return "user:" + userID
}

// GetGlobalChannel returns the global channel name
func GetGlobalChannel() string {
	return "global"
}

// GetAdminChannel returns the admin channel name
func GetAdminChannel() string {
	return "admin"
}
