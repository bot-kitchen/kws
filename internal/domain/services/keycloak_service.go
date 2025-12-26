package services

import (
	"context"
	"fmt"

	"github.com/Nerzal/gocloak/v13"
	"github.com/ak/kws/internal/infrastructure/config"
)

// KeycloakService handles Keycloak IAM operations
type KeycloakService interface {
	// Realm management
	CreateRealm(ctx context.Context, realmName, displayName string) error
	DeleteRealm(ctx context.Context, realmName string) error
	EnableRealm(ctx context.Context, realmName string) error
	DisableRealm(ctx context.Context, realmName string) error

	// Client management (for KOS devices)
	CreateKOSClient(ctx context.Context, realmName, clientID, clientName string) (string, error)
	DeleteKOSClient(ctx context.Context, realmName, clientID string) error

	// User management
	CreateUser(ctx context.Context, realmName, username, email, password string, roles []string) (string, error)
	DeleteUser(ctx context.Context, realmName, userID string) error
	UpdateUserRoles(ctx context.Context, realmName, userID string, roles []string) error

	// Role management
	CreateRole(ctx context.Context, realmName, roleName, description string) error
	GetRealmRoles(ctx context.Context, realmName string) ([]string, error)
}

type keycloakService struct {
	client *gocloak.GoCloak
	config config.KeycloakConfig
	token  *gocloak.JWT
}

// NewKeycloakService creates a new Keycloak service
func NewKeycloakService(cfg config.KeycloakConfig) (KeycloakService, error) {
	client := gocloak.NewClient(cfg.URL)

	// Get admin token
	ctx := context.Background()
	token, err := client.LoginAdmin(ctx, cfg.AdminUsername, cfg.AdminPassword, cfg.AdminRealm)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Keycloak: %w", err)
	}

	return &keycloakService{
		client: client,
		config: cfg,
		token:  token,
	}, nil
}

func (s *keycloakService) refreshToken(ctx context.Context) error {
	token, err := s.client.LoginAdmin(ctx, s.config.AdminUsername, s.config.AdminPassword, s.config.AdminRealm)
	if err != nil {
		return fmt.Errorf("failed to refresh admin token: %w", err)
	}
	s.token = token
	return nil
}

func (s *keycloakService) CreateRealm(ctx context.Context, realmName, displayName string) error {
	if err := s.refreshToken(ctx); err != nil {
		return err
	}

	enabled := true
	realm := gocloak.RealmRepresentation{
		Realm:       &realmName,
		DisplayName: &displayName,
		Enabled:     &enabled,
	}

	_, err := s.client.CreateRealm(ctx, s.token.AccessToken, realm)
	if err != nil {
		return fmt.Errorf("failed to create realm: %w", err)
	}

	// Create default roles for the tenant realm
	defaultRoles := []struct {
		name        string
		description string
	}{
		{"admin", "Tenant administrator with full access"},
		{"manager", "Kitchen manager with operational access"},
		{"operator", "Kitchen operator with limited access"},
		{"viewer", "Read-only access to dashboards"},
	}

	for _, role := range defaultRoles {
		if err := s.CreateRole(ctx, realmName, role.name, role.description); err != nil {
			// Log but don't fail - roles might already exist
			continue
		}
	}

	return nil
}

func (s *keycloakService) DeleteRealm(ctx context.Context, realmName string) error {
	if err := s.refreshToken(ctx); err != nil {
		return err
	}

	if err := s.client.DeleteRealm(ctx, s.token.AccessToken, realmName); err != nil {
		return fmt.Errorf("failed to delete realm: %w", err)
	}

	return nil
}

func (s *keycloakService) EnableRealm(ctx context.Context, realmName string) error {
	if err := s.refreshToken(ctx); err != nil {
		return err
	}

	enabled := true
	realm := gocloak.RealmRepresentation{
		Realm:   &realmName,
		Enabled: &enabled,
	}

	if err := s.client.UpdateRealm(ctx, s.token.AccessToken, realm); err != nil {
		return fmt.Errorf("failed to enable realm: %w", err)
	}

	return nil
}

func (s *keycloakService) DisableRealm(ctx context.Context, realmName string) error {
	if err := s.refreshToken(ctx); err != nil {
		return err
	}

	enabled := false
	realm := gocloak.RealmRepresentation{
		Realm:   &realmName,
		Enabled: &enabled,
	}

	if err := s.client.UpdateRealm(ctx, s.token.AccessToken, realm); err != nil {
		return fmt.Errorf("failed to disable realm: %w", err)
	}

	return nil
}

func (s *keycloakService) CreateKOSClient(ctx context.Context, realmName, clientID, clientName string) (string, error) {
	if err := s.refreshToken(ctx); err != nil {
		return "", err
	}

	enabled := true
	publicClient := false
	serviceAccountsEnabled := true
	directAccessGrantsEnabled := true
	protocol := "openid-connect"

	client := gocloak.Client{
		ClientID:                  &clientID,
		Name:                      &clientName,
		Enabled:                   &enabled,
		PublicClient:              &publicClient,
		ServiceAccountsEnabled:    &serviceAccountsEnabled,
		DirectAccessGrantsEnabled: &directAccessGrantsEnabled,
		Protocol:                  &protocol,
	}

	id, err := s.client.CreateClient(ctx, s.token.AccessToken, realmName, client)
	if err != nil {
		return "", fmt.Errorf("failed to create KOS client: %w", err)
	}

	return id, nil
}

func (s *keycloakService) DeleteKOSClient(ctx context.Context, realmName, clientID string) error {
	if err := s.refreshToken(ctx); err != nil {
		return err
	}

	// Find client by clientID
	clients, err := s.client.GetClients(ctx, s.token.AccessToken, realmName, gocloak.GetClientsParams{
		ClientID: &clientID,
	})
	if err != nil {
		return fmt.Errorf("failed to find client: %w", err)
	}

	if len(clients) == 0 {
		return fmt.Errorf("client not found: %s", clientID)
	}

	if err := s.client.DeleteClient(ctx, s.token.AccessToken, realmName, *clients[0].ID); err != nil {
		return fmt.Errorf("failed to delete client: %w", err)
	}

	return nil
}

func (s *keycloakService) CreateUser(ctx context.Context, realmName, username, email, password string, roles []string) (string, error) {
	if err := s.refreshToken(ctx); err != nil {
		return "", err
	}

	enabled := true
	emailVerified := true
	user := gocloak.User{
		Username:      &username,
		Email:         &email,
		Enabled:       &enabled,
		EmailVerified: &emailVerified,
	}

	userID, err := s.client.CreateUser(ctx, s.token.AccessToken, realmName, user)
	if err != nil {
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	// Set password
	if err := s.client.SetPassword(ctx, s.token.AccessToken, userID, realmName, password, false); err != nil {
		return "", fmt.Errorf("failed to set password: %w", err)
	}

	// Assign roles
	if len(roles) > 0 {
		if err := s.UpdateUserRoles(ctx, realmName, userID, roles); err != nil {
			return "", err
		}
	}

	return userID, nil
}

func (s *keycloakService) DeleteUser(ctx context.Context, realmName, userID string) error {
	if err := s.refreshToken(ctx); err != nil {
		return err
	}

	if err := s.client.DeleteUser(ctx, s.token.AccessToken, realmName, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

func (s *keycloakService) UpdateUserRoles(ctx context.Context, realmName, userID string, roles []string) error {
	if err := s.refreshToken(ctx); err != nil {
		return err
	}

	// Get all realm roles
	allRoles, err := s.client.GetRealmRoles(ctx, s.token.AccessToken, realmName, gocloak.GetRoleParams{})
	if err != nil {
		return fmt.Errorf("failed to get realm roles: %w", err)
	}

	// Filter to requested roles
	var rolesToAdd []gocloak.Role
	for _, role := range allRoles {
		for _, requestedRole := range roles {
			if *role.Name == requestedRole {
				rolesToAdd = append(rolesToAdd, *role)
				break
			}
		}
	}

	if len(rolesToAdd) > 0 {
		if err := s.client.AddRealmRoleToUser(ctx, s.token.AccessToken, realmName, userID, rolesToAdd); err != nil {
			return fmt.Errorf("failed to assign roles: %w", err)
		}
	}

	return nil
}

func (s *keycloakService) CreateRole(ctx context.Context, realmName, roleName, description string) error {
	if err := s.refreshToken(ctx); err != nil {
		return err
	}

	role := gocloak.Role{
		Name:        &roleName,
		Description: &description,
	}

	_, err := s.client.CreateRealmRole(ctx, s.token.AccessToken, realmName, role)
	if err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}

	return nil
}

func (s *keycloakService) GetRealmRoles(ctx context.Context, realmName string) ([]string, error) {
	if err := s.refreshToken(ctx); err != nil {
		return nil, err
	}

	roles, err := s.client.GetRealmRoles(ctx, s.token.AccessToken, realmName, gocloak.GetRoleParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to get realm roles: %w", err)
	}

	var roleNames []string
	for _, role := range roles {
		if role.Name != nil {
			roleNames = append(roleNames, *role.Name)
		}
	}

	return roleNames, nil
}
