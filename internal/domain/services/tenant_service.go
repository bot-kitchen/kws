package services

import (
	"context"
	"fmt"

	"github.com/ak/kws/internal/domain/models"
	"github.com/ak/kws/internal/domain/repositories"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TenantService handles tenant business logic
type TenantService interface {
	Create(ctx context.Context, req CreateTenantRequest) (*models.Tenant, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Tenant, error)
	GetByCode(ctx context.Context, code string) (*models.Tenant, error)
	Update(ctx context.Context, id primitive.ObjectID, req UpdateTenantRequest) (*models.Tenant, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, filter repositories.TenantFilter) ([]*models.Tenant, int64, error)
	Suspend(ctx context.Context, id primitive.ObjectID) error
	Activate(ctx context.Context, id primitive.ObjectID) error
}

type CreateTenantRequest struct {
	Code         string `json:"code" binding:"required"`
	Name         string `json:"name" binding:"required"`
	Plan         string `json:"plan" binding:"required"`
	ContactEmail string `json:"contact_email" binding:"required,email"`
	ContactPhone string `json:"contact_phone"`
}

type UpdateTenantRequest struct {
	Name         string `json:"name"`
	Plan         string `json:"plan"`
	ContactEmail string `json:"contact_email"`
	ContactPhone string `json:"contact_phone"`
}

type tenantService struct {
	tenantRepo  repositories.TenantRepository
	keycloakSvc KeycloakService
}

// NewTenantService creates a new tenant service
func NewTenantService(tenantRepo repositories.TenantRepository, keycloakSvc KeycloakService) TenantService {
	return &tenantService{
		tenantRepo:  tenantRepo,
		keycloakSvc: keycloakSvc,
	}
}

func (s *tenantService) Create(ctx context.Context, req CreateTenantRequest) (*models.Tenant, error) {
	// Check for duplicate code
	existing, err := s.tenantRepo.GetByCode(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing tenant: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("tenant with code '%s' already exists", req.Code)
	}

	// Generate Keycloak realm name
	realmName := fmt.Sprintf("tenant-%s", req.Code)

	// Create Keycloak realm if service is available
	if s.keycloakSvc != nil {
		if err := s.keycloakSvc.CreateRealm(ctx, realmName, req.Name); err != nil {
			return nil, fmt.Errorf("failed to create Keycloak realm: %w", err)
		}
	}

	tenant := &models.Tenant{
		Code:              req.Code,
		Name:              req.Name,
		Status:            models.TenantStatusTrial,
		Plan:              req.Plan,
		KeycloakRealmName: realmName,
		ContactEmail:      req.ContactEmail,
		ContactPhone:      req.ContactPhone,
		Settings: &models.TenantSettings{
			DefaultTimezone:   "UTC",
			DefaultCurrency:   "USD",
			RecipeSyncEnabled: true,
			OrderSyncEnabled:  true,
		},
	}

	if err := s.tenantRepo.Create(ctx, tenant); err != nil {
		// Rollback Keycloak realm on failure
		if s.keycloakSvc != nil {
			_ = s.keycloakSvc.DeleteRealm(ctx, realmName)
		}
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	return tenant, nil
}

func (s *tenantService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Tenant, error) {
	return s.tenantRepo.GetByID(ctx, id)
}

func (s *tenantService) GetByCode(ctx context.Context, code string) (*models.Tenant, error) {
	return s.tenantRepo.GetByCode(ctx, code)
}

func (s *tenantService) Update(ctx context.Context, id primitive.ObjectID, req UpdateTenantRequest) (*models.Tenant, error) {
	tenant, err := s.tenantRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, fmt.Errorf("tenant not found")
	}

	if req.Name != "" {
		tenant.Name = req.Name
	}
	if req.Plan != "" {
		tenant.Plan = req.Plan
	}
	if req.ContactEmail != "" {
		tenant.ContactEmail = req.ContactEmail
	}
	if req.ContactPhone != "" {
		tenant.ContactPhone = req.ContactPhone
	}

	if err := s.tenantRepo.Update(ctx, tenant); err != nil {
		return nil, err
	}

	return tenant, nil
}

func (s *tenantService) List(ctx context.Context, filter repositories.TenantFilter) ([]*models.Tenant, int64, error) {
	return s.tenantRepo.List(ctx, filter)
}

func (s *tenantService) Delete(ctx context.Context, id primitive.ObjectID) error {
	tenant, err := s.tenantRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if tenant == nil {
		return fmt.Errorf("tenant not found")
	}

	// Delete Keycloak realm first
	if s.keycloakSvc != nil && tenant.KeycloakRealmName != "" {
		if err := s.keycloakSvc.DeleteRealm(ctx, tenant.KeycloakRealmName); err != nil {
			// Log but don't fail - realm might already be deleted
			// TODO: Add proper logging
		}
	}

	// Delete tenant from MongoDB
	return s.tenantRepo.Delete(ctx, id)
}

func (s *tenantService) Suspend(ctx context.Context, id primitive.ObjectID) error {
	tenant, err := s.tenantRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if tenant == nil {
		return fmt.Errorf("tenant not found")
	}

	tenant.Status = models.TenantStatusSuspended

	// Disable Keycloak realm
	if s.keycloakSvc != nil {
		if err := s.keycloakSvc.DisableRealm(ctx, tenant.KeycloakRealmName); err != nil {
			return fmt.Errorf("failed to disable Keycloak realm: %w", err)
		}
	}

	return s.tenantRepo.Update(ctx, tenant)
}

func (s *tenantService) Activate(ctx context.Context, id primitive.ObjectID) error {
	tenant, err := s.tenantRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if tenant == nil {
		return fmt.Errorf("tenant not found")
	}

	tenant.Status = models.TenantStatusActive

	// Enable Keycloak realm
	if s.keycloakSvc != nil {
		if err := s.keycloakSvc.EnableRealm(ctx, tenant.KeycloakRealmName); err != nil {
			return fmt.Errorf("failed to enable Keycloak realm: %w", err)
		}
	}

	return s.tenantRepo.Update(ctx, tenant)
}
