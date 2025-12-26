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

type ingredientRepository struct {
	collection *mongo.Collection
}

func NewIngredientRepository(db *database.MongoDB) repositories.IngredientRepository {
	return &ingredientRepository{
		collection: db.Collection(database.CollectionIngredients),
	}
}

func (r *ingredientRepository) Create(ctx context.Context, ingredient *models.Ingredient) error {
	ingredient.CreatedAt = time.Now()
	ingredient.UpdatedAt = time.Now()
	ingredient.IsActive = true

	result, err := r.collection.InsertOne(ctx, ingredient)
	if err != nil {
		return err
	}
	ingredient.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ingredientRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Ingredient, error) {
	var ingredient models.Ingredient
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&ingredient)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &ingredient, nil
}

func (r *ingredientRepository) Update(ctx context.Context, ingredient *models.Ingredient) error {
	ingredient.UpdatedAt = time.Now()
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": ingredient.ID}, ingredient)
	return err
}

func (r *ingredientRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	// Soft delete by setting is_active to false
	_, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"is_active": false, "updated_at": time.Now()}},
	)
	return err
}

func (r *ingredientRepository) ListByTenant(ctx context.Context, tenantID primitive.ObjectID, activeOnly bool, page, limit int) ([]*models.Ingredient, int64, error) {
	query := bson.M{"tenant_id": tenantID}
	if activeOnly {
		query["is_active"] = true
	}

	total, err := r.collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
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

	var ingredients []*models.Ingredient
	if err := cursor.All(ctx, &ingredients); err != nil {
		return nil, 0, err
	}

	return ingredients, total, nil
}

func (r *ingredientRepository) GetByIDs(ctx context.Context, ids []primitive.ObjectID) ([]*models.Ingredient, error) {
	query := bson.M{"_id": bson.M{"$in": ids}}

	cursor, err := r.collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var ingredients []*models.Ingredient
	if err := cursor.All(ctx, &ingredients); err != nil {
		return nil, err
	}

	return ingredients, nil
}
