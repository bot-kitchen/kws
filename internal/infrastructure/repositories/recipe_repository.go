package repositories

import (
	"context"
	"time"

	"github.com/ak/kws/internal/domain/models"
	"github.com/ak/kws/internal/domain/repositories"
	"github.com/ak/kws/internal/infrastructure/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type recipeRepository struct {
	collection     *mongo.Collection
	syncCollection *mongo.Collection
}

func NewRecipeRepository(db *database.MongoDB) repositories.RecipeRepository {
	return &recipeRepository{
		collection:     db.Collection(database.CollectionRecipes),
		syncCollection: db.Collection(database.CollectionRecipeSyncRecords),
	}
}

func (r *recipeRepository) Create(ctx context.Context, recipe *models.Recipe) error {
	recipe.CreatedAt = time.Now()
	recipe.UpdatedAt = time.Now()
	recipe.Version = 1
	if recipe.Status == "" {
		recipe.Status = models.RecipeStatusDraft
	}

	result, err := r.collection.InsertOne(ctx, recipe)
	if err != nil {
		return err
	}
	recipe.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *recipeRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Recipe, error) {
	var recipe models.Recipe
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&recipe)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &recipe, nil
}

func (r *recipeRepository) Update(ctx context.Context, recipe *models.Recipe) error {
	recipe.UpdatedAt = time.Now()
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": recipe.ID}, recipe)
	return err
}

func (r *recipeRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	// Soft delete by archiving
	_, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"status": models.RecipeStatusArchived, "updated_at": time.Now()}},
	)
	return err
}

func (r *recipeRepository) ListByTenant(ctx context.Context, tenantID primitive.ObjectID, status string, page, limit int) ([]*models.Recipe, int64, error) {
	query := bson.M{"tenant_id": tenantID}

	// Exclude archived unless specifically requested
	if status != "" {
		query["status"] = status
	} else {
		query["status"] = bson.M{"$ne": models.RecipeStatusArchived}
	}

	total, err := r.collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	skip := (page - 1) * limit

	opts := options.Find().
		SetSort(bson.D{{Key: "name", Value: 1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var recipes []*models.Recipe
	if err := cursor.All(ctx, &recipes); err != nil {
		return nil, 0, err
	}

	return recipes, total, nil
}

func (r *recipeRepository) GetPublishedForSite(ctx context.Context, siteID primitive.ObjectID) ([]*models.Recipe, error) {
	query := bson.M{
		"status":             models.RecipeStatusPublished,
		"published_to_sites": siteID,
	}

	opts := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var recipes []*models.Recipe
	if err := cursor.All(ctx, &recipes); err != nil {
		return nil, err
	}

	return recipes, nil
}
