package app

import (
	"net/http"
	"strings"

	"github.com/ak/kws/internal/domain/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ==================== Ingredient handlers ====================

type CreateIngredientRequest struct {
	TenantID       string         `json:"tenant_id" form:"tenant_id" binding:"required"`
	Name           string         `json:"name" form:"name" binding:"required"`
	MoistureType   string         `json:"moisture_type" form:"moisture_type" binding:"required,oneof=dry wet liquid"`
	ShelfLifeHours int            `json:"shelf_life_hours" form:"shelf_life_hours"`
	AllergenInfo   string         `json:"allergen_info" form:"allergen_info"`
	Parameters     map[string]any `json:"parameters"`
	IsActive       string         `json:"is_active" form:"is_active"`
	// Nutrition fields (per 100g)
	CaloriesPer100g float64 `json:"calories_per_100g" form:"calories_per_100g"`
	ProteinPer100g  float64 `json:"protein_per_100g" form:"protein_per_100g"`
	FatPer100g      float64 `json:"fat_per_100g" form:"fat_per_100g"`
	CarbsPer100g    float64 `json:"carbs_per_100g" form:"carbs_per_100g"`
	SugarPer100g    float64 `json:"sugar_per_100g" form:"sugar_per_100g"`
	FiberPer100g    float64 `json:"fiber_per_100g" form:"fiber_per_100g"`
	SodiumPer100g   float64 `json:"sodium_per_100g" form:"sodium_per_100g"`
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
	if err := c.ShouldBind(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(req.TenantID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid tenant_id format")
		return
	}

	// Parse allergen info from comma-separated string
	var allergens []string
	if req.AllergenInfo != "" {
		for _, a := range strings.Split(req.AllergenInfo, ",") {
			if trimmed := strings.TrimSpace(a); trimmed != "" {
				allergens = append(allergens, trimmed)
			}
		}
	}

	// Convert hours to minutes for storage
	shelfLifeMinutes := req.ShelfLifeHours * 60

	// Checkbox sends "on" when checked, empty when unchecked
	isActive := req.IsActive == "on" || req.IsActive == "true" || req.IsActive == "1"

	// Build nutrition info if any field is provided
	var nutrition *models.NutritionInfo
	if req.CaloriesPer100g > 0 || req.ProteinPer100g > 0 || req.FatPer100g > 0 ||
		req.CarbsPer100g > 0 || req.SugarPer100g > 0 || req.FiberPer100g > 0 ||
		req.SodiumPer100g > 0 {
		nutrition = &models.NutritionInfo{
			CaloriesPer100g: req.CaloriesPer100g,
			ProteinPer100g:  req.ProteinPer100g,
			FatPer100g:      req.FatPer100g,
			CarbsPer100g:    req.CarbsPer100g,
			SugarPer100g:    req.SugarPer100g,
			FiberPer100g:    req.FiberPer100g,
			SodiumPer100g:   req.SodiumPer100g,
		}
	}

	ingredient := &models.Ingredient{
		TenantID:         tenantID,
		Name:             req.Name,
		MoistureType:     models.MoistureType(req.MoistureType),
		ShelfLifeMinutes: shelfLifeMinutes,
		AllergenInfo:     allergens,
		Nutrition:        nutrition,
		Parameters:       req.Parameters,
		IsActive:         isActive,
	}

	if err := a.repos.Ingredient.Create(c.Request.Context(), ingredient); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to create ingredient: "+err.Error())
		return
	}

	// Check if this is an HTMX request - redirect to list page
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/ingredients")
		c.Status(http.StatusOK)
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
	if err := c.ShouldBind(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	// Parse allergen info from comma-separated string
	var allergens []string
	if req.AllergenInfo != "" {
		for _, a := range strings.Split(req.AllergenInfo, ",") {
			if trimmed := strings.TrimSpace(a); trimmed != "" {
				allergens = append(allergens, trimmed)
			}
		}
	}

	// Checkbox sends "on" when checked, empty when unchecked
	isActive := req.IsActive == "on" || req.IsActive == "true" || req.IsActive == "1"

	// Build nutrition info if any field is provided
	var nutrition *models.NutritionInfo
	if req.CaloriesPer100g > 0 || req.ProteinPer100g > 0 || req.FatPer100g > 0 ||
		req.CarbsPer100g > 0 || req.SugarPer100g > 0 || req.FiberPer100g > 0 ||
		req.SodiumPer100g > 0 {
		nutrition = &models.NutritionInfo{
			CaloriesPer100g: req.CaloriesPer100g,
			ProteinPer100g:  req.ProteinPer100g,
			FatPer100g:      req.FatPer100g,
			CarbsPer100g:    req.CarbsPer100g,
			SugarPer100g:    req.SugarPer100g,
			FiberPer100g:    req.FiberPer100g,
			SodiumPer100g:   req.SodiumPer100g,
		}
	}

	ingredient.Name = req.Name
	ingredient.MoistureType = models.MoistureType(req.MoistureType)
	ingredient.ShelfLifeMinutes = req.ShelfLifeHours * 60
	ingredient.AllergenInfo = allergens
	ingredient.IsActive = isActive
	ingredient.Nutrition = nutrition
	if req.Parameters != nil {
		ingredient.Parameters = req.Parameters
	}

	if err := a.repos.Ingredient.Update(c.Request.Context(), ingredient); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update ingredient")
		return
	}

	// Check if this is an HTMX request - redirect to list page
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/ingredients")
		c.Status(http.StatusOK)
		return
	}

	successResponse(c, ingredient)
}

func (a *Application) deleteIngredient(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()

	ingredient, err := a.repos.Ingredient.GetByID(ctx, id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get ingredient")
		return
	}
	if ingredient == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Ingredient not found")
		return
	}

	// Check if ingredient is used in any recipes
	recipeCount, err := a.repos.Ingredient.CountRecipesUsingIngredient(ctx, id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to check ingredient usage")
		return
	}

	if recipeCount > 0 {
		// Ingredient is in use - soft delete only
		if err := a.repos.Ingredient.Delete(ctx, id); err != nil {
			errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to deactivate ingredient")
			return
		}
		successResponse(c, gin.H{
			"deleted":     false,
			"deactivated": true,
			"message":     "Ingredient is used in recipes and has been deactivated instead of deleted",
		})
		return
	}

	// Ingredient not in use - hard delete
	if err := a.repos.Ingredient.HardDelete(ctx, id); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to delete ingredient")
		return
	}

	successResponse(c, gin.H{"deleted": true})
}

func (a *Application) toggleIngredientActive(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()

	ingredient, err := a.repos.Ingredient.GetByID(ctx, id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get ingredient")
		return
	}
	if ingredient == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Ingredient not found")
		return
	}

	// Toggle the active status
	ingredient.IsActive = !ingredient.IsActive

	if err := a.repos.Ingredient.Update(ctx, ingredient); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update ingredient")
		return
	}

	successResponse(c, gin.H{
		"id":        ingredient.ID.Hex(),
		"is_active": ingredient.IsActive,
	})
}
