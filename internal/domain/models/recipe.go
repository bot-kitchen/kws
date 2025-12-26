package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Ingredient represents a cooking ingredient
// Aligned with KOS ingredient table: Name, MoistureType, ShelfLifeMinutes, AllergenInfo,
// Nutrition fields (CaloriesPer100g, ProteinPer100g, etc.), Parameters
// KWS-specific fields: TenantID, IsActive
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
// Core fields aligned with KOS recipe table: Name, EstimatedPrepTimeSec, EstimatedCookingTimeSec,
// AllergenWarnings, Version, and IsActive (mapped to Status == published)
// KWS-specific fields: TenantID, Description, Category, CuisineType, PrepTime, CookTime, Servings, etc.
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
// Aligned with KOS recipe_ingredient table
type RecipeIngredient struct {
	IngredientID     primitive.ObjectID   `bson:"ingredient_id" json:"ingredient_id"`
	IngredientName   string               `bson:"ingredient_name" json:"ingredient_name"` // Denormalized for display
	QuantityRequired float64              `bson:"quantity_required" json:"quantity_required"`
	Unit             string               `bson:"unit" json:"unit"` // grams, ml
	PrepNotes        string               `bson:"prep_notes,omitempty" json:"prep_notes,omitempty"`
	TimingStep       int                  `bson:"timing_step" json:"timing_step"`
	IsCritical       bool                 `bson:"is_critical" json:"is_critical"`
	Substitutes      []primitive.ObjectID `bson:"substitutes,omitempty" json:"substitutes,omitempty"`
}

// L4Action represents the valid L4 action types for recipe steps
// These are the actions that KOS orchestrator understands
type L4Action string

const (
	// Pot lifecycle actions
	L4ActionAcquirePotFromStaging L4Action = "acquire_pot_from_staging" // Pick pot from staging conveyor, place on pyro
	L4ActionDeliverPotToServing   L4Action = "deliver_pot_to_serving"   // Pick pot from pyro, place on serving conveyor

	// Ingredient actions
	L4ActionAddLiquid       L4Action = "add_liquid"       // Dispense liquid from hydra to pot
	L4ActionAddSolid        L4Action = "add_solid"        // Add solid ingredient from canister to pot
	L4ActionPickIngredient  L4Action = "pick_ingredient"  // Pick canister from storage (hulk operation)
	L4ActionPlaceIngredient L4Action = "place_ingredient" // Return canister to storage (hulk operation)

	// Pot manipulation actions
	L4ActionOpenPotLid  L4Action = "open_pot_lid"  // Thor opens pot lid
	L4ActionClosePotLid L4Action = "close_pot_lid" // Thor closes pot lid
	L4ActionAgitate     L4Action = "agitate"       // Spiral grinder stirs/grinds contents

	// Heating actions
	L4ActionHeat L4Action = "heat" // Pyro heats pot at specified power level
)

// RecipeStep represents a step in recipe execution
// Aligned with KOS recipe_step table: step_number, action, parameters (JSON), depends_on_steps
//
// Parameters JSON structure varies by action type:
//   - add_liquid:  {"ingredient_id": int, "ingredient_name": string, "metric": "ml", "quantity": float}
//   - add_solid:   {"ingredient_id": int, "ingredient_name": string, "metric": "grams", "quantity": float}
//   - agitate:     {"speed": string, "duration_sec": int, "direction": string}
//     speeds: "slow_stir", "med_stir", "fast_stir", "coarse_grind"
//     directions: "scraping", "cutting"
//   - heat:        {"power_level": int, "on_duration_sec": int}
//   - open_pot_lid, close_pot_lid: {} (no parameters)
//   - acquire_pot_from_staging, deliver_pot_to_serving: {} (generated by KOS, not in recipes)
type RecipeStep struct {
	StepNumber     int            `bson:"step_number" json:"step_number"`                               // Sequential order (1,2,3...), must be unique per recipe
	Action         L4Action       `bson:"action" json:"action"`                                         // L4 action type (add_liquid, add_solid, agitate, heat, etc.)
	Parameters     map[string]any `bson:"parameters,omitempty" json:"parameters,omitempty"`             // Action-specific parameters (see struct docs)
	DependsOnSteps []int          `bson:"depends_on_steps,omitempty" json:"depends_on_steps,omitempty"` // Parent step numbers; this step starts after all parents complete

	// KWS-only fields for recipe authoring UI (not synced to KOS)
	Name        string `bson:"name,omitempty" json:"name,omitempty"`               // Human-readable step name for UI
	Description string `bson:"description,omitempty" json:"description,omitempty"` // Detailed description for recipe authors
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
	Action         L4Action       `json:"action"`
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
