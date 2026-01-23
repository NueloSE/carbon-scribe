package websocket

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Authenticator handles WebSocket connection authentication
type Authenticator struct {
	jwtSecret []byte
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret string
}

// ConnectionClaims represents the claims for a WebSocket connection
type ConnectionClaims struct {
	UserID      string   `json:"user_id"`
	ProjectIDs  []string `json:"project_ids"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

// AuthResult represents the result of authentication
type AuthResult struct {
	Authenticated bool
	UserID        string
	ProjectIDs    []string
	Permissions   []string
	Error         error
}

// NewAuthenticator creates a new WebSocket authenticator
func NewAuthenticator(cfg AuthConfig) *Authenticator {
	return &Authenticator{
		jwtSecret: []byte(cfg.JWTSecret),
	}
}

// AuthenticateToken authenticates a JWT token
func (a *Authenticator) AuthenticateToken(ctx context.Context, tokenString string) *AuthResult {
	if tokenString == "" {
		return &AuthResult{
			Authenticated: false,
			Error:         errors.New("empty token"),
		}
	}

	// Remove Bearer prefix if present
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	token, err := jwt.ParseWithClaims(tokenString, &ConnectionClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.jwtSecret, nil
	})

	if err != nil {
		return &AuthResult{
			Authenticated: false,
			Error:         err,
		}
	}

	claims, ok := token.Claims.(*ConnectionClaims)
	if !ok || !token.Valid {
		return &AuthResult{
			Authenticated: false,
			Error:         errors.New("invalid token claims"),
		}
	}

	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return &AuthResult{
			Authenticated: false,
			Error:         errors.New("token expired"),
		}
	}

	return &AuthResult{
		Authenticated: true,
		UserID:        claims.UserID,
		ProjectIDs:    claims.ProjectIDs,
		Permissions:   claims.Permissions,
	}
}

// AuthenticateQueryParam authenticates from query parameter
func (a *Authenticator) AuthenticateQueryParam(ctx context.Context, queryParams map[string]string) *AuthResult {
	token, ok := queryParams["token"]
	if !ok || token == "" {
		// Try Authorization header format
		if auth, exists := queryParams["Authorization"]; exists {
			token = auth
		}
	}

	return a.AuthenticateToken(ctx, token)
}

// GenerateToken generates a JWT token for WebSocket authentication
func (a *Authenticator) GenerateToken(userID string, projectIDs []string, permissions []string, expiry time.Duration) (string, error) {
	claims := ConnectionClaims{
		UserID:      userID,
		ProjectIDs:  projectIDs,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "carbonscribe",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}

// HasPermission checks if the auth result has a specific permission
func (r *AuthResult) HasPermission(permission string) bool {
	for _, p := range r.Permissions {
		if p == permission || p == "*" {
			return true
		}
	}
	return false
}

// HasProjectAccess checks if the auth result has access to a specific project
func (r *AuthResult) HasProjectAccess(projectID string) bool {
	for _, pid := range r.ProjectIDs {
		if pid == projectID || pid == "*" {
			return true
		}
	}
	return false
}

// CanSubscribeToChannel checks if the user can subscribe to a channel
func (r *AuthResult) CanSubscribeToChannel(channel string) bool {
	// Parse channel to determine type
	parts := strings.SplitN(channel, ":", 2)
	if len(parts) != 2 {
		// Global channel
		if channel == "global" {
			return r.Authenticated
		}
		if channel == "admin" {
			return r.HasPermission("admin")
		}
		return false
	}

	channelType := parts[0]
	channelID := parts[1]

	switch channelType {
	case "project":
		return r.HasProjectAccess(channelID)
	case "user":
		return channelID == r.UserID
	default:
		return false
	}
}
