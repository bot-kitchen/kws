package repositories

import (
	"github.com/ak/kws/internal/domain/repositories"
	"github.com/ak/kws/internal/infrastructure/database"
)

// Provider holds all repository instances
type Provider struct {
	Tenant      repositories.TenantRepository
	Region      repositories.RegionRepository
	Site        repositories.SiteRepository
	Kitchen     repositories.KitchenRepository
	KOSInstance repositories.KOSInstanceRepository
	Ingredient  repositories.IngredientRepository
	Recipe      repositories.RecipeRepository
	Order       repositories.OrderRepository
}

// NewProvider creates a new repository provider
func NewProvider(db *database.MongoDB) *Provider {
	return &Provider{
		Tenant:      NewTenantRepository(db),
		Region:      NewRegionRepository(db),
		Site:        NewSiteRepository(db),
		Kitchen:     NewKitchenRepository(db),
		KOSInstance: NewKOSInstanceRepository(db),
		Ingredient:  NewIngredientRepository(db),
		Recipe:      NewRecipeRepository(db),
		Order:       NewOrderRepository(db),
	}
}
