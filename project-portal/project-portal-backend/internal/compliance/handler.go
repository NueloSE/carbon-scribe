package compliance

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	complianceGroup := router.Group("/compliance")
	{
		// Retention Policies
		complianceGroup.POST("/retention/policies", h.CreateRetentionPolicy)
		complianceGroup.GET("/retention/policies", h.ListRetentionPolicies)
		
		// Requests
		complianceGroup.POST("/requests/export", h.RequestExport)
		complianceGroup.POST("/requests/delete", h.RequestDeletion)
		complianceGroup.GET("/requests/:id", h.GetRequestStatus)
		
		// Preferences
		complianceGroup.GET("/preferences", h.GetPreferences)
		complianceGroup.PUT("/preferences", h.UpdatePreferences)
		
		// Consent
		complianceGroup.POST("/consents", h.RecordConsent)
		complianceGroup.DELETE("/consents/:type", h.WithdrawConsent)
		
		// Audit
		// complianceGroup.GET("/audit/logs", h.QueryAuditLogs) // To be implemented
	}
}

// --- Retention Policies ---

func (h *Handler) CreateRetentionPolicy(c *gin.Context) {
	var policy RetentionPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CreateRetentionPolicy(c.Request.Context(), &policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

func (h *Handler) ListRetentionPolicies(c *gin.Context) {
	policies, err := h.service.ListRetentionPolicies(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policies)
}

// --- Requests ---

func (h *Handler) RequestExport(c *gin.Context) {
	var req PrivacyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Ensure user_id is set from context (auth middleware)
	// For now assuming it's passed or we extract it.
	// userID := c.MustGet("userID").(uuid.UUID)
	// req.UserID = userID
	req.RequestType = "export"

	if err := h.service.CreatePrivacyRequest(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, req)
}

func (h *Handler) RequestDeletion(c *gin.Context) {
	var req PrivacyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	req.RequestType = "deletion"

	if err := h.service.CreatePrivacyRequest(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, req)
}

func (h *Handler) GetRequestStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request ID"})
		return
	}

	req, err := h.service.GetPrivacyRequest(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, req)
}

// --- Preferences ---

func (h *Handler) GetPreferences(c *gin.Context) {
	// userID := c.MustGet("userID").(uuid.UUID)
	// Mocking user ID for now as we don't have auth middleware context here yet
	userID := uuid.Nil 
	
	prefs, err := h.service.GetPrivacyPreferences(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, prefs)
}

func (h *Handler) UpdatePreferences(c *gin.Context) {
	var prefs PrivacyPreference
	if err := c.ShouldBindJSON(&prefs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdatePrivacyPreferences(c.Request.Context(), &prefs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

// --- Consent ---

func (h *Handler) RecordConsent(c *gin.Context) {
	var record ConsentRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.RecordConsent(c.Request.Context(), &record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, record)
}

func (h *Handler) WithdrawConsent(c *gin.Context) {
	consentType := c.Param("type")
	// userID := c.MustGet("userID").(uuid.UUID)
	userID := uuid.Nil

	if err := h.service.WithdrawConsent(c.Request.Context(), userID, consentType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "withdrawn"})
}
