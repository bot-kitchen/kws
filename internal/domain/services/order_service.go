package services

import (
	"context"
	"fmt"
	"time"

	"github.com/ak/kws/internal/domain/models"
	"github.com/ak/kws/internal/domain/repositories"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrderService handles order business logic
type OrderService interface {
	// CreateBatch creates multiple orders from a batch request (one order per recipe item)
	CreateBatch(ctx context.Context, req CreateOrderBatchRequest) ([]*models.Order, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetByReference(ctx context.Context, tenantID primitive.ObjectID, reference string) (*models.Order, error)
	GetByGroupID(ctx context.Context, tenantID primitive.ObjectID, groupID string) ([]*models.Order, error)
	Update(ctx context.Context, id primitive.ObjectID, req UpdateOrderRequest) (*models.Order, error)
	Cancel(ctx context.Context, id primitive.ObjectID, reason string) error
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.OrderStatus, kosOrderID string, errorMsg string) error
	List(ctx context.Context, tenantID primitive.ObjectID, filter OrderListFilter) ([]*models.Order, int64, error)
	GetPendingForSite(ctx context.Context, siteID primitive.ObjectID) ([]*models.Order, error)
	// CreateFromKOS creates an order that originated from KOS local UI
	CreateFromKOS(ctx context.Context, req CreateOrderFromKOSRequest) (*models.Order, error)
}

// CreateOrderBatchRequest is used to create multiple orders at once
// Each item becomes a separate order (with Quantity creating N orders)
type CreateOrderBatchRequest struct {
	TenantID            primitive.ObjectID `json:"tenant_id" binding:"required"`
	RegionID            primitive.ObjectID `json:"region_id" binding:"required"`
	SiteID              primitive.ObjectID `json:"site_id" binding:"required"`
	OrderReference      string             `json:"order_reference" binding:"required"` // Base reference
	CustomerName        string             `json:"customer_name"`
	Items               []OrderItemRequest `json:"items" binding:"required,min=1"`
	Priority            int                `json:"priority"`
	ExecutionTime       *time.Time         `json:"execution_time"`
	SpecialInstructions string             `json:"special_instructions"`
	Source              string             `json:"source"`
}

type OrderItemRequest struct {
	RecipeID      primitive.ObjectID    `json:"recipe_id" binding:"required"`
	Quantity      int                   `json:"quantity" binding:"required,min=1"` // Creates N separate orders
	PotPercentage int                   `json:"pot_percentage"`
	Modifications []ModificationRequest `json:"modifications"`
}

type ModificationRequest struct {
	Type       string `json:"type" binding:"required"`
	Ingredient string `json:"ingredient"`
	Notes      string `json:"notes"`
}

// CreateOrderFromKOSRequest is used when KOS reports a locally-created order
type CreateOrderFromKOSRequest struct {
	TenantID            primitive.ObjectID    `json:"tenant_id" binding:"required"`
	RegionID            primitive.ObjectID    `json:"region_id" binding:"required"`
	SiteID              primitive.ObjectID    `json:"site_id" binding:"required"`
	KitchenID           primitive.ObjectID    `json:"kitchen_id" binding:"required"`
	KOSOrderID          string                `json:"kos_order_id" binding:"required"`
	OrderReference      string                `json:"order_reference"`
	CustomerName        string                `json:"customer_name"`
	RecipeID            primitive.ObjectID    `json:"recipe_id" binding:"required"`
	RecipeName          string                `json:"recipe_name"`
	PotPercentage       int                   `json:"pot_percentage"`
	Modifications       []ModificationRequest `json:"modifications"`
	Priority            int                   `json:"priority"`
	ExecutionTime       *time.Time            `json:"execution_time"`
	SpecialInstructions string                `json:"special_instructions"`
	Status              string                `json:"status"`
	StartedAt           *time.Time            `json:"started_at"`
	CompletedAt         *time.Time            `json:"completed_at"`
}

type UpdateOrderRequest struct {
	Priority            *int       `json:"priority"`
	ExecutionTime       *time.Time `json:"execution_time"`
	SpecialInstructions string     `json:"special_instructions"`
}

type OrderListFilter struct {
	Status string
	SiteID *primitive.ObjectID
	Page   int
	Limit  int
}

type orderService struct {
	orderRepo  repositories.OrderRepository
	recipeRepo repositories.RecipeRepository
	siteRepo   repositories.SiteRepository
}

// NewOrderService creates a new order service
func NewOrderService(
	orderRepo repositories.OrderRepository,
	recipeRepo repositories.RecipeRepository,
	siteRepo repositories.SiteRepository,
) OrderService {
	return &orderService{
		orderRepo:  orderRepo,
		recipeRepo: recipeRepo,
		siteRepo:   siteRepo,
	}
}

func (s *orderService) CreateBatch(ctx context.Context, req CreateOrderBatchRequest) ([]*models.Order, error) {
	// Validate site exists and belongs to tenant
	if s.siteRepo != nil {
		site, err := s.siteRepo.GetByID(ctx, req.SiteID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate site: %w", err)
		}
		if site == nil {
			return nil, fmt.Errorf("site not found: %s", req.SiteID.Hex())
		}
		if site.TenantID != req.TenantID {
			return nil, fmt.Errorf("site does not belong to tenant")
		}
		if site.RegionID != req.RegionID {
			return nil, fmt.Errorf("site does not belong to specified region")
		}
	}

	priority := req.Priority
	if priority == 0 {
		priority = 5 // Default priority
	}

	source := req.Source
	if source == "" {
		source = "api"
	}

	// Set execution time with default to now if not provided
	execTime := time.Now()
	if req.ExecutionTime != nil {
		execTime = *req.ExecutionTime
	}

	// Generate a group ID to link all orders from this batch
	groupID := primitive.NewObjectID().Hex()

	var orders []*models.Order
	orderNum := 0

	// Create one order per recipe item (with quantity creating N orders)
	for _, item := range req.Items {
		// Validate recipe exists and is published
		var recipeName string
		if s.recipeRepo != nil {
			recipe, err := s.recipeRepo.GetByID(ctx, item.RecipeID)
			if err != nil {
				return nil, fmt.Errorf("failed to validate recipe: %w", err)
			}
			if recipe == nil {
				return nil, fmt.Errorf("recipe not found: %s", item.RecipeID.Hex())
			}
			if recipe.Status != models.RecipeStatusPublished {
				return nil, fmt.Errorf("recipe '%s' is not published", recipe.Name)
			}
			recipeName = recipe.Name
		}

		var modifications []models.Modification
		for _, mod := range item.Modifications {
			modifications = append(modifications, models.Modification{
				Type:       mod.Type,
				Ingredient: mod.Ingredient,
				Notes:      mod.Notes,
			})
		}

		potPct := item.PotPercentage
		if potPct == 0 {
			potPct = 100
		}

		// Create N orders based on quantity
		for q := 0; q < item.Quantity; q++ {
			orderNum++
			orderRef := req.OrderReference
			if len(req.Items) > 1 || item.Quantity > 1 {
				orderRef = fmt.Sprintf("%s-%d", req.OrderReference, orderNum)
			}

			order := &models.Order{
				TenantID:            req.TenantID,
				RegionID:            req.RegionID,
				SiteID:              req.SiteID,
				OrderReference:      orderRef,
				OrderGroupID:        groupID,
				CustomerName:        req.CustomerName,
				RecipeID:            item.RecipeID,
				RecipeName:          recipeName,
				PotPercentage:       potPct,
				Modifications:       modifications,
				Status:              models.OrderStatusPending,
				Priority:            priority,
				ExecutionTime:       execTime,
				SpecialInstructions: req.SpecialInstructions,
				Source:              models.OrderSource(source),
				KOSSyncStatus:       models.KOSSyncStatusPending,
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			}

			if err := s.orderRepo.Create(ctx, order); err != nil {
				return nil, fmt.Errorf("failed to create order: %w", err)
			}

			orders = append(orders, order)
		}
	}

	return orders, nil
}

func (s *orderService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	return s.orderRepo.GetByID(ctx, id)
}

func (s *orderService) GetByReference(ctx context.Context, tenantID primitive.ObjectID, reference string) (*models.Order, error) {
	return s.orderRepo.GetByReference(ctx, tenantID, reference)
}

func (s *orderService) Update(ctx context.Context, id primitive.ObjectID, req UpdateOrderRequest) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("order not found")
	}

	// Cannot update orders that are in progress or completed
	if order.Status == models.OrderStatusInProgress || order.Status == models.OrderStatusCompleted {
		return nil, fmt.Errorf("cannot update order in status: %s", order.Status)
	}

	if req.Priority != nil {
		order.Priority = *req.Priority
	}
	if req.ExecutionTime != nil {
		order.ExecutionTime = *req.ExecutionTime
	}
	if req.SpecialInstructions != "" {
		order.SpecialInstructions = req.SpecialInstructions
	}

	order.UpdatedAt = time.Now()

	if err := s.orderRepo.Update(ctx, order); err != nil {
		return nil, err
	}

	return order, nil
}

func (s *orderService) Cancel(ctx context.Context, id primitive.ObjectID, reason string) error {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if order == nil {
		return fmt.Errorf("order not found")
	}

	// Cannot cancel orders that are completed
	if order.Status == models.OrderStatusCompleted {
		return fmt.Errorf("cannot cancel completed order")
	}

	order.Status = models.OrderStatusCancelled
	order.ErrorMessage = reason
	order.UpdatedAt = time.Now()

	return s.orderRepo.Update(ctx, order)
}

func (s *orderService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.OrderStatus, kosOrderID string, errorMsg string) error {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if order == nil {
		return fmt.Errorf("order not found")
	}

	order.Status = status
	order.UpdatedAt = time.Now()

	if kosOrderID != "" {
		order.KOSOrderID = kosOrderID
	}

	if errorMsg != "" {
		order.ErrorMessage = errorMsg
	}

	now := time.Now()
	switch status {
	case models.OrderStatusAccepted, models.OrderStatusScheduled:
		// Mark as synced to KOS
		order.KOSSyncStatus = models.KOSSyncStatusSynced
		order.KOSSyncedAt = &now
	case models.OrderStatusInProgress:
		order.StartedAt = &now
	case models.OrderStatusCompleted:
		order.CompletedAt = &now
	case models.OrderStatusFailed, models.OrderStatusCancelled:
		// Already handled error message above
	}

	return s.orderRepo.Update(ctx, order)
}

func (s *orderService) List(ctx context.Context, tenantID primitive.ObjectID, filter OrderListFilter) ([]*models.Order, int64, error) {
	return s.orderRepo.ListByTenant(ctx, tenantID, filter.SiteID, filter.Status, filter.Page, filter.Limit)
}

func (s *orderService) GetPendingForSite(ctx context.Context, siteID primitive.ObjectID) ([]*models.Order, error) {
	return s.orderRepo.GetPendingForSite(ctx, siteID)
}

func (s *orderService) GetByGroupID(ctx context.Context, tenantID primitive.ObjectID, groupID string) ([]*models.Order, error) {
	return s.orderRepo.GetByGroupID(ctx, tenantID, groupID)
}

// CreateFromKOS creates an order that originated from KOS local UI
// This is used for billing and central management tracking
func (s *orderService) CreateFromKOS(ctx context.Context, req CreateOrderFromKOSRequest) (*models.Order, error) {
	// Check if order with this KOS ID already exists
	existing, err := s.orderRepo.GetByKOSOrderID(ctx, req.KOSOrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing order: %w", err)
	}
	if existing != nil {
		// Update existing order with new status
		existing.Status = models.OrderStatus(req.Status)
		existing.UpdatedAt = time.Now()
		if req.StartedAt != nil {
			existing.StartedAt = req.StartedAt
		}
		if req.CompletedAt != nil {
			existing.CompletedAt = req.CompletedAt
		}
		if err := s.orderRepo.Update(ctx, existing); err != nil {
			return nil, fmt.Errorf("failed to update order: %w", err)
		}
		return existing, nil
	}

	// Set defaults
	priority := req.Priority
	if priority == 0 {
		priority = 5
	}

	potPct := req.PotPercentage
	if potPct == 0 {
		potPct = 100
	}

	execTime := time.Now()
	if req.ExecutionTime != nil {
		execTime = *req.ExecutionTime
	}

	status := models.OrderStatus(req.Status)
	if status == "" {
		status = models.OrderStatusPending
	}

	var modifications []models.Modification
	for _, mod := range req.Modifications {
		modifications = append(modifications, models.Modification{
			Type:       mod.Type,
			Ingredient: mod.Ingredient,
			Notes:      mod.Notes,
		})
	}

	kitchenID := req.KitchenID
	order := &models.Order{
		TenantID:            req.TenantID,
		RegionID:            req.RegionID,
		SiteID:              req.SiteID,
		KitchenID:           &kitchenID,
		OrderReference:      req.OrderReference,
		CustomerName:        req.CustomerName,
		RecipeID:            req.RecipeID,
		RecipeName:          req.RecipeName,
		PotPercentage:       potPct,
		Modifications:       modifications,
		Status:              status,
		Priority:            priority,
		ExecutionTime:       execTime,
		SpecialInstructions: req.SpecialInstructions,
		Source:              models.OrderSourceKOSLocal,
		KOSSyncStatus:       models.KOSSyncStatusSynced, // Already synced since it came from KOS
		KOSOrderID:          req.KOSOrderID,
		StartedAt:           req.StartedAt,
		CompletedAt:         req.CompletedAt,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	now := time.Now()
	order.KOSSyncedAt = &now

	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	return order, nil
}
