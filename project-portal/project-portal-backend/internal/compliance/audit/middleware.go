package audit

import (
	"bytes"
	"io"
	"time"

	"carbon-scribe/project-portal/project-portal-backend/internal/compliance"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Middleware logs all accesses to the audit log
func Middleware(service compliance.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// Read body for logging (and restore it)
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		c.Next()

		// Log after request
		// Extract user ID from context if available
		var actorID *uuid.UUID
		if val, exists := c.Get("userID"); exists {
			if id, ok := val.(uuid.UUID); ok {
				actorID = &id
			}
		}

		log := &compliance.AuditLog{
			EventTime:   start,
			EventType:   "api_access",
			EventAction: c.Request.Method,
			ActorID:     actorID,
			ActorType:   "user", // Default, could be system
			ActorIP:     c.ClientIP(),
			ServiceName: "project-portal",
			Endpoint:    c.Request.URL.Path,
			HTTPMethod:  c.Request.Method,
			// OldValues: nil, // Would need more complex logic to capture state before
			NewValues:   datatypes.JSON(bodyBytes), // Log request body (be careful with sensitive data!)
		}

		// Run in background to not block response
		go func(l *compliance.AuditLog) {
			// Create a background context
			ctx := context.Background()
			_ = service.LogAction(ctx, l)
		}(log)
	}
}
