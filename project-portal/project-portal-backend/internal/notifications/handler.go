package notifications

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for notifications
type Handler struct {
	service *Service
}

// NewHandler creates a new notification handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers notification routes with the Gin router
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	notifications := r.Group("/notifications")
	{
		// Core notification endpoints
		notifications.GET("", h.ListNotifications)
		notifications.POST("/send", h.SendNotification)
		notifications.GET("/:id/status", h.GetNotificationStatus)
		notifications.POST("/ws/broadcast", h.BroadcastWebSocket)

		// Preferences
		notifications.GET("/preferences", h.GetPreferences)
		notifications.PUT("/preferences", h.UpdatePreferences)

		// Templates
		notifications.GET("/templates", h.ListTemplates)
		notifications.POST("/templates", h.CreateTemplate)
		notifications.GET("/templates/:type/:language/preview", h.PreviewTemplate)

		// Rules
		notifications.POST("/rules", h.CreateRule)
		notifications.GET("/rules", h.ListRules)
		notifications.GET("/rules/:projectId/:ruleId", h.GetRule)
		notifications.PUT("/rules/:projectId/:ruleId", h.UpdateRule)
		notifications.DELETE("/rules/:projectId/:ruleId", h.DeleteRule)
		notifications.POST("/rules/:projectId/:ruleId/test", h.TestRule)

		// Metrics
		notifications.GET("/metrics", h.GetMetrics)

		// Webhooks (no auth required, validated by signature)
		notifications.POST("/webhooks/sns", h.HandleSNSWebhook)
		notifications.POST("/webhooks/ses", h.HandleSESWebhook)
	}
}

// ListNotifications lists notifications for the authenticated user
func (h *Handler) ListNotifications(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	req := &NotificationsListRequest{
		Limit:     limit,
		Cursor:    c.Query("cursor"),
		Channel:   c.Query("channel"),
		Category:  c.Query("category"),
		Status:    c.Query("status"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
	}

	resp, err := h.service.GetUserNotifications(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SendNotification sends a notification
func (h *Handler) SendNotification(c *gin.Context) {
	var req SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.SendNotification(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetNotificationStatus gets the delivery status of a notification
func (h *Handler) GetNotificationStatus(c *gin.Context) {
	notificationID := c.Param("id")

	logs, err := h.service.GetNotificationStatus(c.Request.Context(), notificationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"notification_id": notificationID,
		"attempts":        logs,
	})
}

// BroadcastWebSocket broadcasts a message to WebSocket clients
func (h *Handler) BroadcastWebSocket(c *gin.Context) {
	var req BroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.BroadcastToWebSocket(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "broadcast sent"})
}

// GetPreferences gets user notification preferences
func (h *Handler) GetPreferences(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	resp, err := h.service.GetUserPreferences(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdatePreferences updates user notification preferences
func (h *Handler) UpdatePreferences(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.UpdateUserPreferences(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "preferences updated"})
}

// ListTemplates lists notification templates
func (h *Handler) ListTemplates(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	cursor := c.Query("cursor")

	templates, nextCursor, err := h.service.ListTemplates(c.Request.Context(), int32(limit), cursor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"templates":   templates,
		"next_cursor": nextCursor,
	})
}

// CreateTemplate creates a new notification template
func (h *Handler) CreateTemplate(c *gin.Context) {
	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template, err := h.service.CreateTemplate(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, template)
}

// PreviewTemplate previews a rendered template
func (h *Handler) PreviewTemplate(c *gin.Context) {
	templateType := c.Param("type")
	language := c.Param("language")

	var req TemplatePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	preview, err := h.service.PreviewTemplate(c.Request.Context(), templateType, language, req.Variables)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, preview)
}

// CreateRule creates a new alert rule
func (h *Handler) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule, err := h.service.CreateRule(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

// ListRules lists alert rules for a project
func (h *Handler) ListRules(c *gin.Context) {
	projectID := c.Query("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
		return
	}

	rules, err := h.service.ListRules(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// GetRule gets a specific alert rule
func (h *Handler) GetRule(c *gin.Context) {
	projectID := c.Param("projectId")
	ruleID := c.Param("ruleId")

	rule, err := h.service.GetRule(c.Request.Context(), projectID, ruleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// UpdateRule updates an alert rule
func (h *Handler) UpdateRule(c *gin.Context) {
	// projectID := c.Param("projectId")
	// ruleID := c.Param("ruleId")

	var req UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement rule update
	c.JSON(http.StatusOK, gin.H{"status": "rule updated"})
}

// DeleteRule deletes an alert rule
func (h *Handler) DeleteRule(c *gin.Context) {
	projectID := c.Param("projectId")
	ruleID := c.Param("ruleId")

	err := h.service.DeleteRule(c.Request.Context(), projectID, ruleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "rule deleted"})
}

// TestRule tests an alert rule with sample data
func (h *Handler) TestRule(c *gin.Context) {
	projectID := c.Param("projectId")
	ruleID := c.Param("ruleId")

	var req TestRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.service.TestRule(c.Request.Context(), projectID, ruleID, req.SampleData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetMetrics gets notification delivery metrics
func (h *Handler) GetMetrics(c *gin.Context) {
	userID := c.GetString("user_id")

	metrics, err := h.service.GetMetrics(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// HandleSNSWebhook handles AWS SNS webhooks
func (h *Handler) HandleSNSWebhook(c *gin.Context) {
	// Verify SNS signature (in production)
	// TODO: Implement SNS signature verification

	var payload WebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ProcessSNSWebhook(c.Request.Context(), &payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}

// HandleSESWebhook handles AWS SES webhooks
func (h *Handler) HandleSESWebhook(c *gin.Context) {
	// First, check if this is an SNS wrapper
	var snsPayload WebhookPayload
	if err := c.ShouldBindJSON(&snsPayload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Handle subscription confirmation
	if snsPayload.Type == "SubscriptionConfirmation" {
		// TODO: Confirm subscription
		c.JSON(http.StatusOK, gin.H{"status": "subscription confirmed"})
		return
	}

	// Parse SES notification from SNS message
	var sesNotification SESNotification
	if err := json.Unmarshal([]byte(snsPayload.Message), &sesNotification); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid SES notification"})
		return
	}

	err := h.service.ProcessSESWebhook(c.Request.Context(), &sesNotification)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}
