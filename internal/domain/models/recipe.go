package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Ingredient represents a cooking ingredient
type Ingredient struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID         primitive.ObjectID `bson:"tenant_id" json:"tenant_id"`
	Name             string             `bson:"name" json:"name"`
	MoistureType     MoistureType       `bson:"moisture_type" json:"moisture_type"` // dry, wet, liquid
	ShelfLifeMinutes int                `bson:"shelf_life_minutes,omitempty" json:"shelf_life_minutes,omitempty"`
	AllergenInfo     []string           `bson:"allergen_info,omitempty" json:"allergen_info,omitempty"`
	Nutrition        *NutritionInfo     `bson:"nutrition,omitempty" json:"nutrition,omitempty"`
	Parameters       map[string]any     `bson:"parameters,omitempty" json:"parameters,omitempty"` // e.g., ml_per_sec for liquids
	IsActive         bool               `bson:"is_active" json:"is_active"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
}

type MoistureType string

const (
	MoistureTypeDry    MoistureType = "dry"
	MoistureTypeWet    MoistureType = "wet"
	MoistureTypeLiquid MoistureType = "liquid"
)

type NutritionInfo struct {
	CaloriesPer100g float64 `bson:"calories_per_100g" json:"calories_per_100g"`
	ProteinPer100g  float64 `bson:"protein_per_100g" json:"protein_per_100g"`
	FatPer100g      float64 `bson:"fat_per_100g" json:"fat_per_100g"`
	CarbsPer100g    float64 `bson:"carbs_per_100g" json:"carbs_per_100g"`
	FiberPer100g    float64 `bson:"fiber_per_100g" json:"fiber_per_100g"`
	SodiumPer100g   float64 `bson:"sodium_per_100g" json:"sodium_per_100g"`
	SugarPer100g    float64 `bson:"sugar_per_100g" json:"sugar_per_100g"`
}

// Recipe represents a cooking recipe
type Recipe struct {
	ID                      primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	TenantID                primitive.ObjectID   `bson:"tenant_id" json:"tenant_id"`
	Name                    string               `bson:"name" json:"name"`
	Description             string               `bson:"description,omitempty" json:"description,omitempty"`
	Category                string               `bson:"category,omitempty" json:"category,omitempty"` // appetizer, main, dessert, etc.
	CuisineType             string               `bson:"cuisine_type,omitempty" json:"cuisine_type,omitempty"`
	PrepTime                int                  `bson:"prep_time" json:"prep_time"` // in minutes
	CookTime                int                  `bson:"cook_time" json:"cook_time"` // in minutes
	Servings                int                  `bson:"servings" json:"servings"`
	EstimatedPrepTimeSec    int                  `bson:"estimated_prep_time_sec" json:"estimated_prep_time_sec"`
	EstimatedCookingTimeSec int                  `bson:"estimated_cooking_time_sec" json:"estimated_cooking_time_sec"`
	AllergenWarnings        []string             `bson:"allergen_warnings,omitempty" json:"allergen_warnings,omitempty"`
	Ingredients             []RecipeIngredient   `bson:"ingredients" json:"ingredients"`
	Steps                   []RecipeStep         `bson:"steps" json:"steps"`
	Parameters              map[string]any       `bson:"parameters,omitempty" json:"parameters,omitempty"`
	Status                  RecipeStatus         `bson:"status" json:"status"`
	Version                 int                  `bson:"version" json:"version"`
	PublishedAt             *time.Time           `bson:"published_at,omitempty" json:"published_at,omitempty"`
	PublishedToSites        []primitive.ObjectID `bson:"published_to_sites,omitempty" json:"published_to_sites,omitempty"`
	CreatedBy               string               `bson:"created_by,omitempty" json:"created_by,omitempty"`
	UpdatedBy               string               `bson:"updated_by,omitempty" json:"updated_by,omitempty"`
	CreatedAt               time.Time            `bson:"created_at" json:"created_at"`
	UpdatedAt               time.Time            `bson:"updated_at" json:"updated_at"`
}

type RecipeStatus string

const (
	RecipeStatusDraft     RecipeStatus = "draft"
	RecipeStatusReview    RecipeStatus = "review"
	RecipeStatusApproved  RecipeStatus = "approved"
	RecipeStatusPublished RecipeStatus = "published"
	RecipeStatusArchived  RecipeStatus = "archived"
)

// RecipeIngredient represents an ingredient requirement in a recipe
type RecipeIngredient struct {
	IngredientID     primitive.ObjectID   `bson:"ingredient_id" json:"ingredient_id"`
	IngredientName   string               `bson:"ingredient_name" json:"ingredient_name"` // Denormalized for display
	Quantity         float64              `bson:"quantity" json:"quantity"`
	QuantityRequired float64              `bson:"quantity_required" json:"quantity_required"`
	Unit             string               `bson:"unit" json:"unit"` // grams, ml
	PrepNotes        string               `bson:"prep_notes,omitempty" json:"prep_notes,omitempty"`
	TimingStep       int                  `bson:"timing_step" json:"timing_step"`
	IsCritical       bool                 `bson:"is_critical" json:"is_critical"`
	Substitutes      []primitive.ObjectID `bson:"substitutes,omitempty" json:"substitutes,omitempty"`
}

// RecipeStep represents a step in recipe execution
type RecipeStep struct {
	Order          int            `bson:"order" json:"order"`
	StepNumber     int            `bson:"step_number" json:"step_number"`
	Name           string         `bson:"name" json:"name"`
	Action         string         `bson:"action" json:"action"` // prep, cook, add_ingredient, add_liquid, stir, grind, wait
	Description    string         `bson:"description,omitempty" json:"description,omitempty"`
	Duration       int            `bson:"duration" json:"duration"` // in seconds
	DeviceType     string         `bson:"device_type,omitempty" json:"device_type,omitempty"`
	Parameters     map[string]any `bson:"parameters,omitempty" json:"parameters,omitempty"`
	DependsOnSteps []int          `bson:"depends_on_steps,omitempty" json:"depends_on_steps,omitempty"`
	DurationSec    int            `bson:"duration_sec,omitempty" json:"duration_sec,omitempty"`
}

// RecipeSyncRecord tracks recipe sync status to KOS instances
type RecipeSyncRecord struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RecipeID     primitive.ObjectID `bson:"recipe_id" json:"recipe_id"`
	KOSID        primitive.ObjectID `bson:"kos_id" json:"kos_id"`
	SiteID       primitive.ObjectID `bson:"site_id" json:"site_id"`
	Version      int                `bson:"version" json:"version"`
	SyncedAt     time.Time          `bson:"synced_at" json:"synced_at"`
	SyncStatus   string             `bson:"sync_status" json:"sync_status"` // pending, synced, failed
	ErrorMessage string             `bson:"error_message,omitempty" json:"error_message,omitempty"`
}

// RecipeForKOS is the simplified recipe format sent to KOS
type RecipeForKOS struct {
	ID                      string                   `json:"id"`
	Name                    string                   `json:"name"`
	EstimatedPrepTimeSec    int                      `json:"estimated_prep_time_sec"`
	EstimatedCookingTimeSec int                      `json:"estimated_cooking_time_sec"`
	AllergenWarnings        []string                 `json:"allergen_warnings,omitempty"`
	Ingredients             []RecipeIngredientForKOS `json:"ingredients"`
	Steps                   []RecipeStepForKOS       `json:"steps"`
	Version                 int                      `json:"version"`
}

type RecipeIngredientForKOS struct {
	IngredientID     string   `json:"ingredient_id"`
	Name             string   `json:"name"`
	QuantityRequired float64  `json:"quantity_required"`
	Unit             string   `json:"unit"`
	TimingStep       int      `json:"timing_step"`
	IsCritical       bool     `json:"is_critical"`
	Substitutes      []string `json:"substitutes,omitempty"`
}

type RecipeStepForKOS struct {
	StepNumber     int            `json:"step_number"`
	Action         string         `json:"action"`
	Parameters     map[string]any `json:"parameters,omitempty"`
	DependsOnSteps []int          `json:"depends_on_steps,omitempty"`
}

// IngredientForKOS is the simplified ingredient format sent to KOS
type IngredientForKOS struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	MoistureType     string         `json:"moisture_type"`
	ShelfLifeMinutes int            `json:"shelf_life_minutes,omitempty"`
	AllergenInfo     []string       `json:"allergen_info,omitempty"`
	Nutrition        *NutritionInfo `json:"nutrition,omitempty"`
	Parameters       map[string]any `json:"parameters,omitempty"`
}

// ToKOSFormat converts a Recipe to the simplified KOS format
func (r *Recipe) ToKOSFormat() RecipeForKOS {
	ingredients := make([]RecipeIngredientForKOS, len(r.Ingredients))
	for i, ing := range r.Ingredients {
		subs := make([]string, len(ing.Substitutes))
		for j, s := range ing.Substitutes {
			subs[j] = s.Hex()
		}
		ingredients[i] = RecipeIngredientForKOS{
			IngredientID:     ing.IngredientID.Hex(),
			Name:             ing.IngredientName,
			QuantityRequired: ing.QuantityRequired,
			Unit:             ing.Unit,
			TimingStep:       ing.TimingStep,
			IsCritical:       ing.IsCritical,
			Substitutes:      subs,
		}
	}

	steps := make([]RecipeStepForKOS, len(r.Steps))
	for i, s := range r.Steps {
		steps[i] = RecipeStepForKOS{
			StepNumber:     s.StepNumber,
			Action:         s.Action,
			Parameters:     s.Parameters,
			DependsOnSteps: s.DependsOnSteps,
		}
	}

	return RecipeForKOS{
		ID:                      r.ID.Hex(),
		Name:                    r.Name,
		EstimatedPrepTimeSec:    r.EstimatedPrepTimeSec,
		EstimatedCookingTimeSec: r.EstimatedCookingTimeSec,
		AllergenWarnings:        r.AllergenWarnings,
		Ingredients:             ingredients,
		Steps:                   steps,
		Version:                 r.Version,
	}
}

// ToKOSFormat converts an Ingredient to the simplified KOS format
func (i *Ingredient) ToKOSFormat() IngredientForKOS {
	return IngredientForKOS{
		ID:               i.ID.Hex(),
		Name:             i.Name,
		MoistureType:     string(i.MoistureType),
		ShelfLifeMinutes: i.ShelfLifeMinutes,
		AllergenInfo:     i.AllergenInfo,
		Nutrition:        i.Nutrition,
		Parameters:       i.Parameters,
	}
}
