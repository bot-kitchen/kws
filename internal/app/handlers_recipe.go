package app

import (
	"net/http"
	"time"

	"github.com/ak/kws/internal/app/middleware"
	"github.com/ak/kws/internal/domain/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ==================== Recipe handlers ====================

type CreateRecipeRequest struct {
	TenantID string `json:"tenant_id"` // Optional in request, validated against session
	Name     string `json:"name" binding:"required"`
	Description             string                    `json:"description"`
	Category                string                    `json:"category"`
	PrepTime                int                       `json:"prep_time"`
	CookTime                int                       `json:"cook_time"`
	EstimatedPrepTimeSec    int                       `json:"estimated_prep_time_sec"`    // KOS-compatible field (seconds)
	EstimatedCookingTimeSec int                       `json:"estimated_cooking_time_sec"` // KOS-compatible field (seconds)
	Servings                int                       `json:"servings"`
	Steps                   []RecipeStepRequest       `json:"steps"`
	Ingredients             []RecipeIngredientRequest `json:"ingredients"`
	Parameters              map[string]any            `json:"parameters"`
}

// RecipeStepRequest represents a recipe step in API requests
// Action must be one of: add_liquid, add_solid, agitate, heat, open_pot_lid, close_pot_lid
// Parameters structure depends on action type (see models.RecipeStep documentation)
type RecipeStepRequest struct {
	StepNumber     int            `json:"step_number" binding:"required,min=1"` // Sequential order (1,2,3...)
	Action         string         `json:"action" binding:"required"`            // L4 action type
	Parameters     map[string]any `json:"parameters,omitempty"`                 // Action-specific parameters
	DependsOnSteps []int          `json:"depends_on_steps,omitempty"`           // Parent step numbers
	Name           string         `json:"name,omitempty"`                       // Human-readable name (KWS-only)
	Description    string         `json:"description,omitempty"`                // Step description (KWS-only)
}

type RecipeIngredientRequest struct {
	IngredientID     string  `json:"ingredient_id" binding:"required"`
	QuantityRequired float64 `json:"quantity_required"` // Required quantity per serving
	Unit             string  `json:"unit"`              // grams, ml
	TimingStep       int     `json:"timing_step"`       // Recipe step when added
	IsCritical       bool    `json:"is_critical"`       // Recipe fails without this
	PrepNotes        string  `json:"prep_notes,omitempty"`
}

type UpdateRecipeRequest struct {
	Name                    string                    `json:"name"`
	Description             string                    `json:"description"`
	Category                string                    `json:"category"`
	PrepTime                int                       `json:"prep_time"`
	CookTime                int                       `json:"cook_time"`
	EstimatedPrepTimeSec    int                       `json:"estimated_prep_time_sec"`
	EstimatedCookingTimeSec int                       `json:"estimated_cooking_time_sec"`
	Servings                int                       `json:"servings"`
	Steps                   []RecipeStepRequest       `json:"steps"`
	Ingredients             []RecipeIngredientRequest `json:"ingredients"`
	Parameters              map[string]any            `json:"parameters"`
}

type PublishRecipeRequest struct {
	SiteIDs []string `json:"site_ids"` // Optional - if empty, publishes globally
}

func (a *Application) listRecipes(c *gin.Context) {
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

	status := c.Query("status")
	page, limit := getPagination(c)

	recipes, total, err := a.repos.Recipe.ListByTenant(c.Request.Context(), tenantID, status, page, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to list recipes")
		return
	}

	paginatedResponse(c, recipes, page, limit, total)
}

func (a *Application) createRecipe(c *gin.Context) {
	var req CreateRecipeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	// Get tenant_id from session context to validate against request
	sessionTenantID := middleware.GetEffectiveTenantID(c)
	if sessionTenantID == "" || sessionTenantID == "platform" {
		// Platform admins must select a tenant before creating recipes
		errorResponse(c, http.StatusUnauthorized, "NO_TENANT", "No tenant context - please select a tenant first")
		return
	}

	// Validate session tenant_id is a valid ObjectID
	tenantID, err := primitive.ObjectIDFromHex(sessionTenantID)
	if err != nil {
		errorResponse(c, http.StatusUnauthorized, "INVALID_TENANT", "Invalid tenant context - please select a valid tenant")
		return
	}

	// If request provides tenant_id, validate it matches session
	if req.TenantID != "" && req.TenantID != sessionTenantID {
		errorResponse(c, http.StatusForbidden, "TENANT_MISMATCH", "Request tenant_id does not match session context")
		return
	}

	// Convert steps
	steps := make([]models.RecipeStep, len(req.Steps))
	for i, s := range req.Steps {
		steps[i] = models.RecipeStep{
			StepNumber:     s.StepNumber,
			Action:         models.L4Action(s.Action),
			Parameters:     s.Parameters,
			DependsOnSteps: s.DependsOnSteps,
			Name:           s.Name,
			Description:    s.Description,
		}
	}

	// Convert ingredients
	ingredients := make([]models.RecipeIngredient, len(req.Ingredients))
	for i, ing := range req.Ingredients {
		ingID, err := primitive.ObjectIDFromHex(ing.IngredientID)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid ingredient_id format")
			return
		}
		ingredients[i] = models.RecipeIngredient{
			IngredientID:     ingID,
			QuantityRequired: ing.QuantityRequired,
			Unit:             ing.Unit,
			TimingStep:       ing.TimingStep,
			IsCritical:       ing.IsCritical,
			PrepNotes:        ing.PrepNotes,
		}
	}

	recipe := &models.Recipe{
		TenantID:                tenantID,
		Name:                    req.Name,
		Description:             req.Description,
		Category:                req.Category,
		PrepTime:                req.PrepTime,
		CookTime:                req.CookTime,
		EstimatedPrepTimeSec:    req.EstimatedPrepTimeSec,
		EstimatedCookingTimeSec: req.EstimatedCookingTimeSec,
		Servings:                req.Servings,
		Steps:                   steps,
		Ingredients:             ingredients,
		Parameters:              req.Parameters,
		Status:                  models.RecipeStatusDraft,
		Version:                 1,
	}

	if err := a.repos.Recipe.Create(c.Request.Context(), recipe); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to create recipe")
		return
	}

	createdResponse(c, recipe)
}

func (a *Application) getRecipe(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	recipe, err := a.repos.Recipe.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get recipe")
		return
	}
	if recipe == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Recipe not found")
		return
	}

	successResponse(c, recipe)
}

func (a *Application) updateRecipe(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	recipe, err := a.repos.Recipe.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get recipe")
		return
	}
	if recipe == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Recipe not found")
		return
	}

	var req UpdateRecipeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	// Update fields
	if req.Name != "" {
		recipe.Name = req.Name
	}
	if req.Description != "" {
		recipe.Description = req.Description
	}
	if req.Category != "" {
		recipe.Category = req.Category
	}
	if req.PrepTime > 0 {
		recipe.PrepTime = req.PrepTime
	}
	if req.CookTime > 0 {
		recipe.CookTime = req.CookTime
	}
	if req.EstimatedPrepTimeSec > 0 {
		recipe.EstimatedPrepTimeSec = req.EstimatedPrepTimeSec
	}
	if req.EstimatedCookingTimeSec > 0 {
		recipe.EstimatedCookingTimeSec = req.EstimatedCookingTimeSec
	}
	if req.Servings > 0 {
		recipe.Servings = req.Servings
	}
	if req.Parameters != nil {
		recipe.Parameters = req.Parameters
	}

	// Update steps if provided
	if req.Steps != nil {
		steps := make([]models.RecipeStep, len(req.Steps))
		for i, s := range req.Steps {
			steps[i] = models.RecipeStep{
				StepNumber:     s.StepNumber,
				Action:         models.L4Action(s.Action),
				Parameters:     s.Parameters,
				DependsOnSteps: s.DependsOnSteps,
				Name:           s.Name,
				Description:    s.Description,
			}
		}
		recipe.Steps = steps
	}

	// Update ingredients if provided
	if req.Ingredients != nil {
		ingredients := make([]models.RecipeIngredient, len(req.Ingredients))
		for i, ing := range req.Ingredients {
			ingID, err := primitive.ObjectIDFromHex(ing.IngredientID)
			if err != nil {
				errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid ingredient_id format")
				return
			}
			ingredients[i] = models.RecipeIngredient{
				IngredientID:     ingID,
				QuantityRequired: ing.QuantityRequired,
				Unit:             ing.Unit,
				TimingStep:       ing.TimingStep,
				IsCritical:       ing.IsCritical,
				PrepNotes:        ing.PrepNotes,
			}
		}
		recipe.Ingredients = ingredients
	}

	// Increment version
	recipe.Version++

	if err := a.repos.Recipe.Update(c.Request.Context(), recipe); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update recipe")
		return
	}

	successResponse(c, recipe)
}

func (a *Application) deleteRecipe(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	recipe, err := a.repos.Recipe.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get recipe")
		return
	}
	if recipe == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Recipe not found")
		return
	}

	// Check if recipe is published
	if recipe.Status == models.RecipeStatusPublished {
		errorResponse(c, http.StatusConflict, "RECIPE_PUBLISHED", "Cannot delete a published recipe. Unpublish it first.")
		return
	}

	if err := a.repos.Recipe.Delete(c.Request.Context(), id); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to delete recipe")
		return
	}

	successResponse(c, gin.H{"deleted": true})
}

func (a *Application) publishRecipe(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	recipe, err := a.repos.Recipe.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get recipe")
		return
	}
	if recipe == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Recipe not found")
		return
	}

	// Parse optional request body for site_ids
	var req PublishRecipeRequest
	// Ignore binding errors - site_ids is optional
	_ = c.ShouldBindJSON(&req)

	// Convert site IDs if provided
	if len(req.SiteIDs) > 0 {
		siteIDs := make([]primitive.ObjectID, len(req.SiteIDs))
		for i, siteIDStr := range req.SiteIDs {
			siteID, err := primitive.ObjectIDFromHex(siteIDStr)
			if err != nil {
				errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid site_id format")
				return
			}
			siteIDs[i] = siteID
		}

		// Merge with existing published sites
		existingSites := make(map[primitive.ObjectID]bool)
		for _, s := range recipe.PublishedToSites {
			existingSites[s] = true
		}
		for _, s := range siteIDs {
			existingSites[s] = true
		}

		// Convert back to slice
		allSites := make([]primitive.ObjectID, 0, len(existingSites))
		for s := range existingSites {
			allSites = append(allSites, s)
		}
		recipe.PublishedToSites = allSites
	}

	// Set status to published
	recipe.Status = models.RecipeStatusPublished
	now := time.Now()
	recipe.PublishedAt = &now

	if err := a.repos.Recipe.Update(c.Request.Context(), recipe); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to publish recipe")
		return
	}

	successResponse(c, recipe)
}

func (a *Application) unpublishRecipe(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	recipe, err := a.repos.Recipe.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get recipe")
		return
	}
	if recipe == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Recipe not found")
		return
	}

	recipe.Status = models.RecipeStatusDraft
	recipe.PublishedToSites = nil
	recipe.PublishedAt = nil

	if err := a.repos.Recipe.Update(c.Request.Context(), recipe); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to unpublish recipe")
		return
	}

	successResponse(c, recipe)
}
