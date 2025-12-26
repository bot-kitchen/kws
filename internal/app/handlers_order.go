package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ak/kws/internal/domain/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ==================== Order handlers ====================

// CreateOrderBatchRequest creates multiple orders (one per recipe item)
type CreateOrderBatchRequest struct {
	TenantID            string             `json:"tenant_id" binding:"required"`
	RegionID            string             `json:"region_id" binding:"required"`
	SiteID              string             `json:"site_id" binding:"required"`
	OrderReference      string             `json:"order_reference" binding:"required"`
	CustomerName        string             `json:"customer_name"`
	Items               []OrderItemRequest `json:"items" binding:"required,min=1"`
	ExecutionTime       *time.Time         `json:"execution_time"`
	Priority            int                `json:"priority"`
	SpecialInstructions string             `json:"special_instructions"`
	Notes               string             `json:"notes"`
	Metadata            map[string]any     `json:"metadata"`
}

type OrderItemRequest struct {
	RecipeID      string                `json:"recipe_id" binding:"required"`
	Quantity      int                   `json:"quantity" binding:"required,min=1"` // Creates N separate orders
	PotPercentage int                   `json:"pot_percentage"`
	Modifications []ModificationRequest `json:"modifications"`
	Notes         string                `json:"notes"`
	Options       map[string]any        `json:"options"`
}

type ModificationRequest struct {
	Type       string `json:"type" binding:"required"`
	Ingredient string `json:"ingredient"`
	Notes      string `json:"notes"`
}

type UpdateOrderRequest struct {
	CustomerName        string         `json:"customer_name"`
	ExecutionTime       *time.Time     `json:"execution_time"`
	Priority            int            `json:"priority"`
	SpecialInstructions string         `json:"special_instructions"`
	Notes               string         `json:"notes"`
	Metadata            map[string]any `json:"metadata"`
}

func (a *Application) listOrders(c *gin.Context) {
	tenantIDStr := c.Query("tenant_id")
	if tenantIDStr == "" {
		errorResponse(c, http.StatusBadRequest, "MISSING_PARAM", "tenant_id is required")
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid tenant_id format")
		return
	}

	// Optional filters
	var siteID *primitive.ObjectID
	if siteIDStr := c.Query("site_id"); siteIDStr != "" {
		id, err := primitive.ObjectIDFromHex(siteIDStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid site_id format")
			return
		}
		siteID = &id
	}

	status := c.Query("status")
	page, limit := getPagination(c)

	orders, total, err := a.repos.Order.ListByTenant(c.Request.Context(), tenantID, siteID, status, page, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to list orders")
		return
	}

	paginatedResponse(c, orders, page, limit, total)
}

// createOrder creates one or more orders from the request
// Each item in the request creates separate orders (with quantity creating N orders)
func (a *Application) createOrder(c *gin.Context) {
	var req CreateOrderBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(req.TenantID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid tenant_id format")
		return
	}

	regionID, err := primitive.ObjectIDFromHex(req.RegionID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid region_id format")
		return
	}

	siteID, err := primitive.ObjectIDFromHex(req.SiteID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid site_id format")
		return
	}

	// Default execution time to now if not provided
	executionTime := time.Now()
	if req.ExecutionTime != nil {
		executionTime = *req.ExecutionTime
	}

	priority := req.Priority
	if priority == 0 {
		priority = 5
	}

	// Generate a group ID to link all orders from this batch
	groupID := primitive.NewObjectID().Hex()

	var orders []*models.Order
	orderNum := 0

	// Create one order per recipe item (with quantity creating N orders)
	for _, item := range req.Items {
		recipeID, err := primitive.ObjectIDFromHex(item.RecipeID)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid recipe_id format")
			return
		}

		// Get recipe name for denormalization
		recipe, err := a.repos.Recipe.GetByID(c.Request.Context(), recipeID)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get recipe")
			return
		}
		if recipe == nil {
			errorResponse(c, http.StatusBadRequest, "NOT_FOUND", fmt.Sprintf("Recipe %s not found", item.RecipeID))
			return
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
				TenantID:            tenantID,
				RegionID:            regionID,
				SiteID:              siteID,
				OrderReference:      orderRef,
				OrderGroupID:        groupID,
				CustomerName:        req.CustomerName,
				RecipeID:            recipeID,
				RecipeName:          recipe.Name,
				PotPercentage:       potPct,
				Modifications:       modifications,
				Status:              models.OrderStatusPending,
				ExecutionTime:       executionTime,
				Priority:            priority,
				SpecialInstructions: req.SpecialInstructions,
				Notes:               item.Notes,
				Metadata:            req.Metadata,
				Source:              models.OrderSourceAPI,
				KOSSyncStatus:       models.KOSSyncStatusPending,
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			}

			if err := a.repos.Order.Create(c.Request.Context(), order); err != nil {
				errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to create order")
				return
			}

			orders = append(orders, order)
		}
	}

	// Return all created orders
	if len(orders) == 1 {
		createdResponse(c, orders[0])
	} else {
		createdResponse(c, gin.H{
			"order_group_id": groupID,
			"orders":         orders,
			"count":          len(orders),
		})
	}
}

func (a *Application) getOrder(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	order, err := a.repos.Order.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get order")
		return
	}
	if order == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Order not found")
		return
	}

	successResponse(c, order)
}

func (a *Application) updateOrder(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	order, err := a.repos.Order.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get order")
		return
	}
	if order == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Order not found")
		return
	}

	// Can only update pending orders
	if order.Status != models.OrderStatusPending {
		errorResponse(c, http.StatusConflict, "ORDER_NOT_PENDING", "Can only update pending orders")
		return
	}

	var req UpdateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	// Update fields
	if req.CustomerName != "" {
		order.CustomerName = req.CustomerName
	}
	if req.ExecutionTime != nil {
		order.ExecutionTime = *req.ExecutionTime
	}
	if req.Priority > 0 {
		order.Priority = req.Priority
	}
	if req.SpecialInstructions != "" {
		order.SpecialInstructions = req.SpecialInstructions
	}
	if req.Notes != "" {
		order.Notes = req.Notes
	}
	if req.Metadata != nil {
		order.Metadata = req.Metadata
	}

	order.UpdatedAt = time.Now()

	if err := a.repos.Order.Update(c.Request.Context(), order); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update order")
		return
	}

	successResponse(c, order)
}

func (a *Application) cancelOrder(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	order, err := a.repos.Order.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get order")
		return
	}
	if order == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Order not found")
		return
	}

	// Can only cancel pending or in_progress orders
	if order.Status != models.OrderStatusPending && order.Status != models.OrderStatusInProgress {
		errorResponse(c, http.StatusConflict, "CANNOT_CANCEL", "Can only cancel pending or in-progress orders")
		return
	}

	order.Status = models.OrderStatusCancelled
	now := time.Now()
	order.CompletedAt = &now
	order.UpdatedAt = now

	if err := a.repos.Order.Update(c.Request.Context(), order); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to cancel order")
		return
	}

	successResponse(c, order)
}
