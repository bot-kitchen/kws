package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// SessionConfig holds configuration for session middleware
type SessionConfig struct {
	// Keycloak OIDC settings
	KeycloakURL  string
	Realm        string
	ClientID     string
	ClientSecret string
	RedirectURL  string // Callback URL for OIDC

	// Session settings
	CookieName     string
	CookieDomain   string
	CookieSecure   bool
	CookieHTTPOnly bool
	SessionTTL     time.Duration

	// Development mode - allows skipping auth
	DevMode bool
}

// SessionData holds the session information stored in cookies
type SessionData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	TenantID     string    `json:"tenant_id"`
	Roles        []string  `json:"roles"`
	// For platform admins - the currently selected tenant context
	SelectedTenantID string `json:"selected_tenant_id,omitempty"`
}

// UserInfo represents the current logged-in user
type UserInfo struct {
	ID               string
	Email            string
	Name             string
	TenantID         string
	Roles            []string
	SelectedTenantID string
	IsPlatformAdmin  bool
}

// OIDCClaims represents Keycloak JWT claims
type OIDCClaims struct {
	jwt.RegisteredClaims
	Email             string                 `json:"email"`
	EmailVerified     bool                   `json:"email_verified"`
	Name              string                 `json:"name"`
	PreferredUsername string                 `json:"preferred_username"`
	TenantID          string                 `json:"tenant_id"`
	RealmAccess       map[string]interface{} `json:"realm_access"`
	ResourceAccess    map[string]interface{} `json:"resource_access"`
}

// TokenResponse represents Keycloak token endpoint response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// sessionStore provides in-memory session storage (can be replaced with Redis later)
type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionData
}

var store = &sessionStore{
	sessions: make(map[string]*SessionData),
}

func (s *sessionStore) Get(id string) (*SessionData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

func (s *sessionStore) Set(id string, session *SessionData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = session
}

func (s *sessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// generateSessionID creates a secure random session ID
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// RequireSession creates middleware that requires a valid session
func RequireSession(config SessionConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Dev mode bypass
		if config.DevMode {
			devSessionID := "dev-session"
			// Get or create dev session
			session, ok := store.Get(devSessionID)
			if !ok {
				session = &SessionData{
					UserID:    "dev-user",
					Email:     "dev@kws.local",
					Name:      "Developer",
					TenantID:  "platform",
					Roles:     []string{"platform_admin"},
					ExpiresAt: time.Now().Add(24 * time.Hour),
				}
				store.Set(devSessionID, session)
			}
			// Set mock user for development
			c.Set("user", &UserInfo{
				ID:               "dev-user",
				Email:            "dev@kws.local",
				Name:             "Developer",
				TenantID:         "platform",
				Roles:            []string{"platform_admin"},
				IsPlatformAdmin:  true,
				SelectedTenantID: session.SelectedTenantID,
			})
			c.Set("session_id", devSessionID)
			c.Next()
			return
		}

		// Get session cookie
		sessionID, err := c.Cookie(config.CookieName)
		if err != nil || sessionID == "" {
			redirectToLogin(c, config)
			return
		}

		// Get session data
		session, ok := store.Get(sessionID)
		if !ok {
			redirectToLogin(c, config)
			return
		}

		// Check if session expired
		if time.Now().After(session.ExpiresAt) {
			// Try to refresh token
			if session.RefreshToken != "" {
				newSession, err := refreshAccessToken(c.Request.Context(), config, session.RefreshToken)
				if err == nil {
					newSession.SelectedTenantID = session.SelectedTenantID
					store.Set(sessionID, newSession)
					session = newSession
				} else {
					store.Delete(sessionID)
					redirectToLogin(c, config)
					return
				}
			} else {
				store.Delete(sessionID)
				redirectToLogin(c, config)
				return
			}
		}

		// Extract roles from token
		roles := session.Roles
		isPlatformAdmin := containsRole(roles, "platform_admin")

		// Set user info in context
		user := &UserInfo{
			ID:               session.UserID,
			Email:            session.Email,
			Name:             session.Name,
			TenantID:         session.TenantID,
			Roles:            roles,
			SelectedTenantID: session.SelectedTenantID,
			IsPlatformAdmin:  isPlatformAdmin,
		}
		c.Set("user", user)
		c.Set("session_id", sessionID)

		c.Next()
	}
}

// OptionalSession creates middleware that loads session if present but doesn't require it
func OptionalSession(config SessionConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Dev mode
		if config.DevMode {
			devSessionID := "dev-session"
			session, ok := store.Get(devSessionID)
			if !ok {
				session = &SessionData{
					UserID:    "dev-user",
					Email:     "dev@kws.local",
					Name:      "Developer",
					TenantID:  "platform",
					Roles:     []string{"platform_admin"},
					ExpiresAt: time.Now().Add(24 * time.Hour),
				}
				store.Set(devSessionID, session)
			}
			c.Set("user", &UserInfo{
				ID:               "dev-user",
				Email:            "dev@kws.local",
				Name:             "Developer",
				TenantID:         "platform",
				Roles:            []string{"platform_admin"},
				IsPlatformAdmin:  true,
				SelectedTenantID: session.SelectedTenantID,
			})
			c.Set("session_id", devSessionID)
			c.Next()
			return
		}

		sessionID, err := c.Cookie(config.CookieName)
		if err != nil || sessionID == "" {
			c.Next()
			return
		}

		session, ok := store.Get(sessionID)
		if !ok || time.Now().After(session.ExpiresAt) {
			c.Next()
			return
		}

		roles := session.Roles
		user := &UserInfo{
			ID:               session.UserID,
			Email:            session.Email,
			Name:             session.Name,
			TenantID:         session.TenantID,
			Roles:            roles,
			SelectedTenantID: session.SelectedTenantID,
			IsPlatformAdmin:  containsRole(roles, "platform_admin"),
		}
		c.Set("user", user)
		c.Set("session_id", sessionID)

		c.Next()
	}
}

// GetUser extracts user info from context
func GetUser(c *gin.Context) *UserInfo {
	if user, exists := c.Get("user"); exists {
		if u, ok := user.(*UserInfo); ok {
			return u
		}
	}
	return nil
}

// GetEffectiveTenantID returns the tenant ID to use for queries
// For platform admins, this is their selected tenant; for regular users, it's their tenant
func GetEffectiveTenantID(c *gin.Context) string {
	user := GetUser(c)
	if user == nil {
		return ""
	}
	if user.IsPlatformAdmin && user.SelectedTenantID != "" {
		return user.SelectedTenantID
	}
	return user.TenantID
}

// RequireTenantContext ensures a tenant is selected (for platform admins)
func RequireTenantContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUser(c)
		if user == nil {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		// Platform admins must have a selected tenant
		if user.IsPlatformAdmin && user.SelectedTenantID == "" {
			c.Redirect(http.StatusSeeOther, "/tenants")
			c.Abort()
			return
		}

		c.Next()
	}
}

// CreateSession creates a new session from OIDC tokens
func CreateSession(c *gin.Context, config SessionConfig, tokenResp *TokenResponse) error {
	// Parse ID token to get user info
	claims, err := parseIDToken(tokenResp.IDToken)
	if err != nil {
		return fmt.Errorf("failed to parse ID token: %w", err)
	}

	// Extract roles
	roles := extractRoles(claims)

	session := &SessionData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		UserID:       claims.Subject,
		Email:        claims.Email,
		Name:         claims.Name,
		TenantID:     claims.TenantID,
		Roles:        roles,
	}

	sessionID, err := generateSessionID()
	if err != nil {
		return fmt.Errorf("failed to generate session ID: %w", err)
	}

	store.Set(sessionID, session)

	// Set session cookie
	c.SetCookie(
		config.CookieName,
		sessionID,
		int(config.SessionTTL.Seconds()),
		"/",
		config.CookieDomain,
		config.CookieSecure,
		config.CookieHTTPOnly,
	)

	return nil
}

// DestroySession removes the current session
func DestroySession(c *gin.Context, config SessionConfig) {
	sessionID, err := c.Cookie(config.CookieName)
	if err == nil && sessionID != "" {
		store.Delete(sessionID)
	}

	// Clear cookie
	c.SetCookie(
		config.CookieName,
		"",
		-1,
		"/",
		config.CookieDomain,
		config.CookieSecure,
		config.CookieHTTPOnly,
	)
}

// SetSelectedTenant updates the selected tenant for platform admins
func SetSelectedTenant(c *gin.Context, tenantID string) error {
	sessionID, exists := c.Get("session_id")
	if !exists {
		return errors.New("no session found")
	}

	session, ok := store.Get(sessionID.(string))
	if !ok {
		return errors.New("session not found in store")
	}

	session.SelectedTenantID = tenantID
	store.Set(sessionID.(string), session)
	return nil
}

// GetAuthorizationURL returns the Keycloak authorization URL for OIDC login
func GetAuthorizationURL(config SessionConfig, state string) string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/auth?"+
		"client_id=%s&"+
		"redirect_uri=%s&"+
		"response_type=code&"+
		"scope=openid+email+profile&"+
		"state=%s",
		config.KeycloakURL,
		config.Realm,
		config.ClientID,
		config.RedirectURL,
		state,
	)
}

// ExchangeCodeForToken exchanges authorization code for tokens
func ExchangeCodeForToken(ctx context.Context, config SessionConfig, code string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		config.KeycloakURL, config.Realm)

	data := fmt.Sprintf("grant_type=authorization_code&"+
		"client_id=%s&"+
		"client_secret=%s&"+
		"code=%s&"+
		"redirect_uri=%s",
		config.ClientID,
		config.ClientSecret,
		code,
		config.RedirectURL,
	)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// GetLogoutURL returns the Keycloak logout URL
func GetLogoutURL(config SessionConfig, redirectURL string) string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/logout?"+
		"client_id=%s&"+
		"post_logout_redirect_uri=%s",
		config.KeycloakURL,
		config.Realm,
		config.ClientID,
		redirectURL,
	)
}

// refreshAccessToken refreshes the access token using refresh token
func refreshAccessToken(ctx context.Context, config SessionConfig, refreshToken string) (*SessionData, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		config.KeycloakURL, config.Realm)

	data := fmt.Sprintf("grant_type=refresh_token&"+
		"client_id=%s&"+
		"client_secret=%s&"+
		"refresh_token=%s",
		config.ClientID,
		config.ClientSecret,
		refreshToken,
	)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	claims, err := parseIDToken(tokenResp.IDToken)
	if err != nil {
		return nil, err
	}

	return &SessionData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		UserID:       claims.Subject,
		Email:        claims.Email,
		Name:         claims.Name,
		TenantID:     claims.TenantID,
		Roles:        extractRoles(claims),
	}, nil
}

// parseIDToken parses the ID token without verification (Keycloak already verified it)
func parseIDToken(idToken string) (*OIDCClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims OIDCClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	return &claims, nil
}

// extractRoles extracts roles from OIDC claims
func extractRoles(claims *OIDCClaims) []string {
	var roles []string

	// Extract from realm_access
	if claims.RealmAccess != nil {
		if realmRoles, ok := claims.RealmAccess["roles"].([]interface{}); ok {
			for _, r := range realmRoles {
				if role, ok := r.(string); ok {
					roles = append(roles, role)
				}
			}
		}
	}

	return roles
}

// containsRole checks if a role exists in the roles slice
func containsRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// redirectToLogin redirects to login page
func redirectToLogin(c *gin.Context, config SessionConfig) {
	// Store the original URL to redirect back after login
	returnURL := c.Request.URL.String()
	c.SetCookie("return_url", returnURL, 300, "/", config.CookieDomain, config.CookieSecure, true)
	c.Redirect(http.StatusSeeOther, "/login")
	c.Abort()
}

// GenerateState generates a random state for OIDC
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
