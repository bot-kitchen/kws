package repositories

import (
	"context"

	"github.com/ak/kws/internal/domain/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TenantRepository defines operations for tenant data access
type TenantRepository interface {
	Create(ctx context.Context, tenant *models.Tenant) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Tenant, error)
	GetByCode(ctx context.Context, code string) (*models.Tenant, error)
	Update(ctx context.Context, tenant *models.Tenant) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, filter TenantFilter) ([]*models.Tenant, int64, error)
}

type TenantFilter struct {
	Status string
	Page   int
	Limit  int
}

// RegionRepository defines operations for region data access
type RegionRepository interface {
	Create(ctx context.Context, region *models.Region) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Region, error)
	Update(ctx context.Context, region *models.Region) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	ListByTenant(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]*models.Region, int64, error)
}

// SiteRepository defines operations for site data access
type SiteRepository interface {
	Create(ctx context.Context, site *models.Site) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Site, error)
	Update(ctx context.Context, site *models.Site) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	ListByTenant(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]*models.Site, int64, error)
	ListByRegion(ctx context.Context, regionID primitive.ObjectID, page, limit int) ([]*models.Site, int64, error)
}

// KitchenRepository defines operations for kitchen data access
type KitchenRepository interface {
	Create(ctx context.Context, kitchen *models.Kitchen) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Kitchen, error)
	Update(ctx context.Context, kitchen *models.Kitchen) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	ListBySite(ctx context.Context, siteID primitive.ObjectID) ([]*models.Kitchen, error)
}

// KOSInstanceRepository defines operations for KOS instance data access
type KOSInstanceRepository interface {
	Create(ctx context.Context, kos *models.KOSInstance) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.KOSInstance, error)
	GetBySiteID(ctx context.Context, siteID primitive.ObjectID) (*models.KOSInstance, error)
	GetByCertificateSerial(ctx context.Context, serial string) (*models.KOSInstance, error)
	Update(ctx context.Context, kos *models.KOSInstance) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	ListByTenant(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]*models.KOSInstance, int64, error)
	RecordHeartbeat(ctx context.Context, heartbeat *models.KOSHeartbeat) error
}

// IngredientRepository defines operations for ingredient data access
type IngredientRepository interface {
	Create(ctx context.Context, ingredient *models.Ingredient) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Ingredient, error)
	Update(ctx context.Context, ingredient *models.Ingredient) error
	Delete(ctx context.Context, id primitive.ObjectID) error     // Soft delete (set is_active=false)
	HardDelete(ctx context.Context, id primitive.ObjectID) error // Permanent delete
	ListByTenant(ctx context.Context, tenantID primitive.ObjectID, activeOnly bool, page, limit int) ([]*models.Ingredient, int64, error)
	GetByIDs(ctx context.Context, ids []primitive.ObjectID) ([]*models.Ingredient, error)
	CountRecipesUsingIngredient(ctx context.Context, ingredientID primitive.ObjectID) (int64, error)
}

// RecipeRepository defines operations for recipe data access
type RecipeRepository interface {
	Create(ctx context.Context, recipe *models.Recipe) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Recipe, error)
	Update(ctx context.Context, recipe *models.Recipe) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	ListByTenant(ctx context.Context, tenantID primitive.ObjectID, status string, page, limit int) ([]*models.Recipe, int64, error)
	GetPublishedForSite(ctx context.Context, siteID primitive.ObjectID) ([]*models.Recipe, error)
}

type RecipeFilter struct {
	Status   string
	Category string
	Page     int
	Limit    int
}

// OrderRepository defines operations for order data access
type OrderRepository interface {
	Create(ctx context.Context, order *models.Order) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetByReference(ctx context.Context, tenantID primitive.ObjectID, reference string) (*models.Order, error)
	GetByGroupID(ctx context.Context, tenantID primitive.ObjectID, groupID string) ([]*models.Order, error)
	GetByKOSOrderID(ctx context.Context, kosOrderID string) (*models.Order, error)
	Update(ctx context.Context, order *models.Order) error
	ListByTenant(ctx context.Context, tenantID primitive.ObjectID, siteID *primitive.ObjectID, status string, page, limit int) ([]*models.Order, int64, error)
	GetPendingForSite(ctx context.Context, siteID primitive.ObjectID) ([]*models.Order, error)
}

type OrderFilter struct {
	Status   string
	SiteID   primitive.ObjectID
	RegionID primitive.ObjectID
	Page     int
	Limit    int
}

// AuditLogRepository defines operations for audit log data access
type AuditLogRepository interface {
	Create(ctx context.Context, log *AuditLog) error
	ListByTenant(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]*AuditLog, int64, error)
	ListByResource(ctx context.Context, resourceType, resourceID string, page, limit int) ([]*AuditLog, int64, error)
}

type AuditLog struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	TenantID     primitive.ObjectID `bson:"tenant_id"`
	UserID       string             `bson:"user_id"`
	Action       string             `bson:"action"`
	ResourceType string             `bson:"resource_type"`
	ResourceID   string             `bson:"resource_id"`
	OldValue     interface{}        `bson:"old_value,omitempty"`
	NewValue     interface{}        `bson:"new_value,omitempty"`
	IPAddress    string             `bson:"ip_address,omitempty"`
	CreatedAt    primitive.DateTime `bson:"created_at"`
}
