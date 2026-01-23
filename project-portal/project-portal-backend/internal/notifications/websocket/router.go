package websocket

import (
	"context"
	"sync"

	wspkg "carbon-scribe/project-portal/project-portal-backend/pkg/websocket"
)

// Router routes WebSocket messages to appropriate handlers
type Router struct {
	manager     *Manager
	handlers    map[wspkg.MessageType]MessageHandler
	middlewares []Middleware
	mu          sync.RWMutex
}

// MessageHandler handles a specific type of message
type MessageHandler func(ctx context.Context, connectionID string, message *wspkg.Message) error

// Middleware is a function that wraps message handling
type Middleware func(next MessageHandler) MessageHandler

// NewRouter creates a new message router
func NewRouter(manager *Manager) *Router {
	r := &Router{
		manager:  manager,
		handlers: make(map[wspkg.MessageType]MessageHandler),
	}

	// Register default handlers
	r.RegisterHandler(wspkg.MessageTypePing, r.handlePing)
	r.RegisterHandler(wspkg.MessageTypeSubscribe, r.handleSubscribe)
	r.RegisterHandler(wspkg.MessageTypeUnsubscribe, r.handleUnsubscribe)

	return r
}

// RegisterHandler registers a handler for a message type
func (r *Router) RegisterHandler(msgType wspkg.MessageType, handler MessageHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[msgType] = handler
}

// Use adds middleware to the router
func (r *Router) Use(mw Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewares = append(r.middlewares, mw)
}

// Route routes a message to the appropriate handler
func (r *Router) Route(ctx context.Context, connectionID string, message *wspkg.Message) error {
	r.mu.RLock()
	handler, ok := r.handlers[message.Type]
	middlewares := r.middlewares
	r.mu.RUnlock()

	if !ok {
		return nil // Unknown message type, ignore
	}

	// Apply middlewares in reverse order
	finalHandler := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		finalHandler = middlewares[i](finalHandler)
	}

	return finalHandler(ctx, connectionID, message)
}

// Default handlers

func (r *Router) handlePing(ctx context.Context, connectionID string, message *wspkg.Message) error {
	pong := wspkg.NewPongMessage()
	data, err := pong.ToJSON()
	if err != nil {
		return err
	}
	return r.manager.apiGateway.PostToConnection(ctx, connectionID, data)
}

func (r *Router) handleSubscribe(ctx context.Context, connectionID string, message *wspkg.Message) error {
	return r.manager.HandleMessage(ctx, connectionID, message)
}

func (r *Router) handleUnsubscribe(ctx context.Context, connectionID string, message *wspkg.Message) error {
	return r.manager.HandleMessage(ctx, connectionID, message)
}

// SendToRoom sends a message to all connections in a room/channel
func (r *Router) SendToRoom(ctx context.Context, room string, message *wspkg.Message) error {
	return r.manager.SendToChannel(ctx, room, message)
}

// SendToUser sends a message to a specific user's connections
func (r *Router) SendToUser(ctx context.Context, userID string, message *wspkg.Message) error {
	return r.manager.SendToUser(ctx, userID, message)
}

// Broadcast sends a message to all connected clients
func (r *Router) Broadcast(ctx context.Context, message *wspkg.Message) error {
	return r.manager.Broadcast(ctx, message)
}

// LoggingMiddleware logs all messages
func LoggingMiddleware(logger func(string, ...interface{})) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, connectionID string, message *wspkg.Message) error {
			logger("Handling message: type=%s, id=%s, connection=%s", message.Type, message.ID, connectionID)
			return next(ctx, connectionID, message)
		}
	}
}

// RecoveryMiddleware recovers from panics
func RecoveryMiddleware(logger func(string, ...interface{})) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, connectionID string, message *wspkg.Message) (err error) {
			defer func() {
				if r := recover(); r != nil {
					logger("Panic recovered: %v", r)
					err = nil // Don't propagate panic errors
				}
			}()
			return next(ctx, connectionID, message)
		}
	}
}

// RateLimitMiddleware limits message rate per connection
func RateLimitMiddleware(maxPerSecond int) Middleware {
	// Simple implementation - in production, use a proper rate limiter
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, connectionID string, message *wspkg.Message) error {
			// TODO: Implement proper rate limiting
			return next(ctx, connectionID, message)
		}
	}
}
