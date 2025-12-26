package app

import (
	"net/http"

	"github.com/ak/kws/internal/domain/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ==================== Ingredient handlers ====================

type CreateIngredientRequest struct {
	TenantID         string                `json:"tenant_id" binding:"required"`
	Name             string                `json:"name" binding:"required"`
	MoistureType     string                `json:"moisture_type" binding:"required,oneof=dry wet liquid"`
	ShelfLifeMinutes int                   `json:"shelf_life_minutes"`
	AllergenInfo     []string              `json:"allergen_info"`
	Nutrition        *models.NutritionInfo `json:"nutrition"`
	Parameters       map[string]any        `json:"parameters"`
}

func (a *Application) listIngredients(c *gin.Context) {
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

	activeOnly := c.Query("active_only") != "false"
	page, limit := getPagination(c)

	ingredients, total, err := a.repos.Ingredient.ListByTenant(c.Request.Context(), tenantID, activeOnly, page, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to list ingredients")
		return
	}

	paginatedResponse(c, ingredients, page, limit, total)
}

func (a *Application) createIngredient(c *gin.Context) {
	var req CreateIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(req.TenantID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid tenant_id format")
		return
	}

	ingredient := &models.Ingredient{
		TenantID:         tenantID,
		Name:             req.Name,
		MoistureType:     models.MoistureType(req.MoistureType),
		ShelfLifeMinutes: req.ShelfLifeMinutes,
		AllergenInfo:     req.AllergenInfo,
		Nutrition:        req.Nutrition,
		Parameters:       req.Parameters,
		IsActive:         true,
	}

	if err := a.repos.Ingredient.Create(c.Request.Context(), ingredient); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to create ingredient")
		return
	}

	createdResponse(c, ingredient)
}

func (a *Application) getIngredient(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	ingredient, err := a.repos.Ingredient.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get ingredient")
		return
	}
	if ingredient == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Ingredient not found")
		return
	}

	successResponse(c, ingredient)
}

func (a *Application) updateIngredient(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	ingredient, err := a.repos.Ingredient.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get ingredient")
		return
	}
	if ingredient == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Ingredient not found")
		return
	}

	var req CreateIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	ingredient.Name = req.Name
	ingredient.MoistureType = models.MoistureType(req.MoistureType)
	ingredient.ShelfLifeMinutes = req.ShelfLifeMinutes
	ingredient.AllergenInfo = req.AllergenInfo
	if req.Nutrition != nil {
		ingredient.Nutrition = req.Nutrition
	}
	if req.Parameters != nil {
		ingredient.Parameters = req.Parameters
	}

	if err := a.repos.Ingredient.Update(c.Request.Context(), ingredient); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update ingredient")
		return
	}

	successResponse(c, ingredient)
}

func (a *Application) deleteIngredient(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	ingredient, err := a.repos.Ingredient.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get ingredient")
		return
	}
	if ingredient == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Ingredient not found")
		return
	}

	if err := a.repos.Ingredient.Delete(c.Request.Context(), id); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to delete ingredient")
		return
	}

	successResponse(c, gin.H{"deleted": true})
}
