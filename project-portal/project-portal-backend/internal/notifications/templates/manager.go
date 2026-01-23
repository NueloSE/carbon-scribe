package templates

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"carbon-scribe/project-portal/project-portal-backend/internal/notifications"
)

// Manager handles template operations
type Manager struct {
	repo  TemplateRepository
	cache *TemplateCache
}

// TemplateRepository defines the interface for template storage
type TemplateRepository interface {
	CreateTemplate(ctx context.Context, template *notifications.NotificationTemplate) error
	GetTemplate(ctx context.Context, templateType, language, version string) (*notifications.NotificationTemplate, error)
	GetActiveTemplate(ctx context.Context, templateType, language string) (*notifications.NotificationTemplate, error)
	ListTemplates(ctx context.Context, limit int32, cursor string) ([]notifications.NotificationTemplate, string, error)
}

// NewManager creates a new template manager
func NewManager(repo TemplateRepository) *Manager {
	return &Manager{
		repo:  repo,
		cache: NewTemplateCache(5 * time.Minute),
	}
}

// Create creates a new template
func (m *Manager) Create(ctx context.Context, req *notifications.CreateTemplateRequest) (*notifications.NotificationTemplate, error) {
	versionID := uuid.New().String()

	template := &notifications.NotificationTemplate{
		PK:        fmt.Sprintf("TEMPLATE#%s#%s", req.Type, req.Language),
		SK:        fmt.Sprintf("VERSION#%s", versionID),
		Name:      req.Name,
		Subject:   req.Subject,
		Body:      req.Body,
		Variables: req.Variables,
		Metadata:  req.Metadata,
		IsActive:  true,
	}

	if err := m.repo.CreateTemplate(ctx, template); err != nil {
		return nil, err
	}

	// Invalidate cache
	m.cache.Invalidate(req.Type, req.Language)

	return template, nil
}

// Get retrieves a template by type, language, and version
func (m *Manager) Get(ctx context.Context, templateType, language, version string) (*notifications.NotificationTemplate, error) {
	return m.repo.GetTemplate(ctx, templateType, language, version)
}

// GetActive retrieves the active version of a template
func (m *Manager) GetActive(ctx context.Context, templateType, language string) (*notifications.NotificationTemplate, error) {
	// Check cache first
	if cached := m.cache.Get(templateType, language); cached != nil {
		return cached, nil
	}

	template, err := m.repo.GetActiveTemplate(ctx, templateType, language)
	if err != nil {
		return nil, err
	}

	if template != nil {
		m.cache.Set(templateType, language, template)
	}

	return template, nil
}

// List lists all templates with pagination
func (m *Manager) List(ctx context.Context, limit int32, cursor string) ([]notifications.NotificationTemplate, string, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return m.repo.ListTemplates(ctx, limit, cursor)
}

// Preview previews a template with sample data
func (m *Manager) Preview(ctx context.Context, templateType, language string, variables map[string]interface{}) (*notifications.TemplatePreviewResponse, error) {
	template, err := m.GetActive(ctx, templateType, language)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, fmt.Errorf("template not found: %s/%s", templateType, language)
	}

	renderer := NewRenderer()
	subject, err := renderer.Render(template.Subject, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render subject: %w", err)
	}

	body, err := renderer.Render(template.Body, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render body: %w", err)
	}

	return &notifications.TemplatePreviewResponse{
		Subject: subject,
		Body:    body,
	}, nil
}

// GetOrDefault gets a template or returns a default
func (m *Manager) GetOrDefault(ctx context.Context, templateType, language string) (*notifications.NotificationTemplate, error) {
	template, err := m.GetActive(ctx, templateType, language)
	if err != nil {
		return nil, err
	}

	// If not found, try English as fallback
	if template == nil && language != "en" {
		template, err = m.GetActive(ctx, templateType, "en")
		if err != nil {
			return nil, err
		}
	}

	// If still not found, return a default template
	if template == nil {
		template = &notifications.NotificationTemplate{
			Name:    "Default " + templateType,
			Subject: "{{subject}}",
			Body:    "{{body}}",
		}
	}

	return template, nil
}

// TemplateCache provides in-memory caching for templates
type TemplateCache struct {
	cache map[string]*cacheEntry
	ttl   time.Duration
}

type cacheEntry struct {
	template  *notifications.NotificationTemplate
	expiresAt time.Time
}

// NewTemplateCache creates a new template cache
func NewTemplateCache(ttl time.Duration) *TemplateCache {
	return &TemplateCache{
		cache: make(map[string]*cacheEntry),
		ttl:   ttl,
	}
}

func (c *TemplateCache) key(templateType, language string) string {
	return fmt.Sprintf("%s#%s", templateType, language)
}

// Get retrieves a template from cache
func (c *TemplateCache) Get(templateType, language string) *notifications.NotificationTemplate {
	entry, ok := c.cache[c.key(templateType, language)]
	if !ok {
		return nil
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.cache, c.key(templateType, language))
		return nil
	}
	return entry.template
}

// Set stores a template in cache
func (c *TemplateCache) Set(templateType, language string, template *notifications.NotificationTemplate) {
	c.cache[c.key(templateType, language)] = &cacheEntry{
		template:  template,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Invalidate removes a template from cache
func (c *TemplateCache) Invalidate(templateType, language string) {
	delete(c.cache, c.key(templateType, language))
}

// Clear clears the entire cache
func (c *TemplateCache) Clear() {
	c.cache = make(map[string]*cacheEntry)
}
