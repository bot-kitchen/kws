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

type tenantRepository struct {
	collection *mongo.Collection
}

// NewTenantRepository creates a new tenant repository
func NewTenantRepository(db *database.MongoDB) repositories.TenantRepository {
	return &tenantRepository{
		collection: db.Collection(database.CollectionTenants),
	}
}

func (r *tenantRepository) Create(ctx context.Context, tenant *models.Tenant) error {
	tenant.CreatedAt = time.Now()
	tenant.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, tenant)
	if err != nil {
		return err
	}
	tenant.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *tenantRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Tenant, error) {
	var tenant models.Tenant
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&tenant)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &tenant, nil
}

func (r *tenantRepository) GetByCode(ctx context.Context, code string) (*models.Tenant, error) {
	var tenant models.Tenant
	err := r.collection.FindOne(ctx, bson.M{"code": code}).Decode(&tenant)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &tenant, nil
}

func (r *tenantRepository) Update(ctx context.Context, tenant *models.Tenant) error {
	tenant.UpdatedAt = time.Now()

	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": tenant.ID}, tenant)
	return err
}

func (r *tenantRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *tenantRepository) List(ctx context.Context, filter repositories.TenantFilter) ([]*models.Tenant, int64, error) {
	query := bson.M{}
	if filter.Status != "" {
		query["status"] = filter.Status
	}

	// Count total
	total, err := r.collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	// Set pagination
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	}
	skip := (page - 1) * limit

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var tenants []*models.Tenant
	if err := cursor.All(ctx, &tenants); err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}
