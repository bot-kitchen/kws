package services

import (
	"context"
	"fmt"
	"time"

	"github.com/ak/kws/internal/domain/models"
	"github.com/ak/kws/internal/domain/repositories"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RecipeService handles recipe business logic
type RecipeService interface {
	Create(ctx context.Context, req CreateRecipeRequest) (*models.Recipe, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Recipe, error)
	Update(ctx context.Context, id primitive.ObjectID, req UpdateRecipeRequest) (*models.Recipe, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, tenantID primitive.ObjectID, filter RecipeListFilter) ([]*models.Recipe, int64, error)
	Publish(ctx context.Context, id primitive.ObjectID) error
	Unpublish(ctx context.Context, id primitive.ObjectID) error
	GetPublishedForSite(ctx context.Context, siteID primitive.ObjectID) ([]*models.Recipe, error)
	ValidateRecipe(ctx context.Context, recipe *models.Recipe) error
}

type CreateRecipeRequest struct {
	TenantID    primitive.ObjectID        `json:"tenant_id" binding:"required"`
	Name        string                    `json:"name" binding:"required"`
	Description string                    `json:"description"`
	Category    string                    `json:"category"`
	PrepTimeSec int                       `json:"prep_time_sec"`
	CookTimeSec int                       `json:"cook_time_sec"`
	Servings    int                       `json:"servings"`
	Allergens   []string                  `json:"allergens"`
	Ingredients []models.RecipeIngredient `json:"ingredients"`
	Steps       []models.RecipeStep       `json:"steps"`
}

type UpdateRecipeRequest struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Category    string                    `json:"category"`
	PrepTimeSec int                       `json:"prep_time_sec"`
	CookTimeSec int                       `json:"cook_time_sec"`
	Servings    int                       `json:"servings"`
	Allergens   []string                  `json:"allergens"`
	Ingredients []models.RecipeIngredient `json:"ingredients"`
	Steps       []models.RecipeStep       `json:"steps"`
}

type RecipeListFilter struct {
	Status   string
	Category string
	Page     int
	Limit    int
}

type recipeService struct {
	recipeRepo     repositories.RecipeRepository
	ingredientRepo repositories.IngredientRepository
}

// NewRecipeService creates a new recipe service
func NewRecipeService(recipeRepo repositories.RecipeRepository, ingredientRepo repositories.IngredientRepository) RecipeService {
	return &recipeService{
		recipeRepo:     recipeRepo,
		ingredientRepo: ingredientRepo,
	}
}

func (s *recipeService) Create(ctx context.Context, req CreateRecipeRequest) (*models.Recipe, error) {
	recipe := &models.Recipe{
		TenantID:                req.TenantID,
		Name:                    req.Name,
		Description:             req.Description,
		Category:                req.Category,
		EstimatedPrepTimeSec:    req.PrepTimeSec,
		EstimatedCookingTimeSec: req.CookTimeSec,
		Servings:                req.Servings,
		AllergenWarnings:        req.Allergens,
		Ingredients:             req.Ingredients,
		Steps:                   req.Steps,
		Status:                  models.RecipeStatusDraft,
		Version:                 1,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}

	// Validate the recipe
	if err := s.ValidateRecipe(ctx, recipe); err != nil {
		return nil, err
	}

	if err := s.recipeRepo.Create(ctx, recipe); err != nil {
		return nil, fmt.Errorf("failed to create recipe: %w", err)
	}

	return recipe, nil
}

func (s *recipeService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Recipe, error) {
	return s.recipeRepo.GetByID(ctx, id)
}

func (s *recipeService) Update(ctx context.Context, id primitive.ObjectID, req UpdateRecipeRequest) (*models.Recipe, error) {
	recipe, err := s.recipeRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if recipe == nil {
		return nil, fmt.Errorf("recipe not found")
	}

	// Cannot update published recipes directly
	if recipe.Status == models.RecipeStatusPublished {
		return nil, fmt.Errorf("cannot update published recipe, unpublish first")
	}

	if req.Name != "" {
		recipe.Name = req.Name
	}
	if req.Description != "" {
		recipe.Description = req.Description
	}
	if req.Category != "" {
		recipe.Category = req.Category
	}
	if req.PrepTimeSec > 0 {
		recipe.EstimatedPrepTimeSec = req.PrepTimeSec
	}
	if req.CookTimeSec > 0 {
		recipe.EstimatedCookingTimeSec = req.CookTimeSec
	}
	if req.Servings > 0 {
		recipe.Servings = req.Servings
	}
	if req.Allergens != nil {
		recipe.AllergenWarnings = req.Allergens
	}
	if req.Ingredients != nil {
		recipe.Ingredients = req.Ingredients
	}
	if req.Steps != nil {
		recipe.Steps = req.Steps
	}

	recipe.UpdatedAt = time.Now()
	recipe.Version++

	// Validate the updated recipe
	if err := s.ValidateRecipe(ctx, recipe); err != nil {
		return nil, err
	}

	if err := s.recipeRepo.Update(ctx, recipe); err != nil {
		return nil, err
	}

	return recipe, nil
}

func (s *recipeService) Delete(ctx context.Context, id primitive.ObjectID) error {
	recipe, err := s.recipeRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if recipe == nil {
		return fmt.Errorf("recipe not found")
	}

	// Cannot delete published recipes
	if recipe.Status == models.RecipeStatusPublished {
		return fmt.Errorf("cannot delete published recipe, unpublish first")
	}

	return s.recipeRepo.Delete(ctx, id)
}

func (s *recipeService) List(ctx context.Context, tenantID primitive.ObjectID, filter RecipeListFilter) ([]*models.Recipe, int64, error) {
	return s.recipeRepo.ListByTenant(ctx, tenantID, filter.Status, filter.Page, filter.Limit)
}

func (s *recipeService) Publish(ctx context.Context, id primitive.ObjectID) error {
	recipe, err := s.recipeRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if recipe == nil {
		return fmt.Errorf("recipe not found")
	}

	// Validate before publishing
	if err := s.ValidateRecipe(ctx, recipe); err != nil {
		return fmt.Errorf("recipe validation failed: %w", err)
	}

	recipe.Status = models.RecipeStatusPublished
	recipe.PublishedAt = timePtr(time.Now())
	recipe.UpdatedAt = time.Now()

	return s.recipeRepo.Update(ctx, recipe)
}

func (s *recipeService) Unpublish(ctx context.Context, id primitive.ObjectID) error {
	recipe, err := s.recipeRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if recipe == nil {
		return fmt.Errorf("recipe not found")
	}

	recipe.Status = models.RecipeStatusDraft
	recipe.UpdatedAt = time.Now()

	return s.recipeRepo.Update(ctx, recipe)
}

func (s *recipeService) GetPublishedForSite(ctx context.Context, siteID primitive.ObjectID) ([]*models.Recipe, error) {
	return s.recipeRepo.GetPublishedForSite(ctx, siteID)
}

func (s *recipeService) ValidateRecipe(ctx context.Context, recipe *models.Recipe) error {
	if recipe.Name == "" {
		return fmt.Errorf("recipe name is required")
	}

	if len(recipe.Ingredients) == 0 {
		return fmt.Errorf("recipe must have at least one ingredient")
	}

	if len(recipe.Steps) == 0 {
		return fmt.Errorf("recipe must have at least one step")
	}

	// Validate ingredient references exist
	var ingredientIDs []primitive.ObjectID
	for _, ing := range recipe.Ingredients {
		ingredientIDs = append(ingredientIDs, ing.IngredientID)
	}

	if s.ingredientRepo != nil {
		ingredients, err := s.ingredientRepo.GetByIDs(ctx, ingredientIDs)
		if err != nil {
			return fmt.Errorf("failed to validate ingredients: %w", err)
		}

		existingIDs := make(map[primitive.ObjectID]bool)
		for _, ing := range ingredients {
			existingIDs[ing.ID] = true
		}

		for _, id := range ingredientIDs {
			if !existingIDs[id] {
				return fmt.Errorf("ingredient not found: %s", id.Hex())
			}
		}
	}

	// Validate step dependencies
	stepNumbers := make(map[int]bool)
	for _, step := range recipe.Steps {
		if step.StepNumber <= 0 {
			return fmt.Errorf("step number must be positive")
		}
		if stepNumbers[step.StepNumber] {
			return fmt.Errorf("duplicate step number: %d", step.StepNumber)
		}
		stepNumbers[step.StepNumber] = true
	}

	for _, step := range recipe.Steps {
		for _, dep := range step.DependsOnSteps {
			if !stepNumbers[dep] {
				return fmt.Errorf("step %d depends on non-existent step %d", step.StepNumber, dep)
			}
			if dep >= step.StepNumber {
				return fmt.Errorf("step %d cannot depend on step %d (circular or forward dependency)", step.StepNumber, dep)
			}
		}
	}

	return nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}
