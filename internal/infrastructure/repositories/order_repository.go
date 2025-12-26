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

type orderRepository struct {
	collection     *mongo.Collection
	syncCollection *mongo.Collection
}

func NewOrderRepository(db *database.MongoDB) repositories.OrderRepository {
	return &orderRepository{
		collection:     db.Collection(database.CollectionOrders),
		syncCollection: db.Collection(database.CollectionOrderSyncRecords),
	}
}

func (r *orderRepository) Create(ctx context.Context, order *models.Order) error {
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()
	if order.Status == "" {
		order.Status = models.OrderStatusPending
	}
	if order.KOSSyncStatus == "" {
		order.KOSSyncStatus = models.KOSSyncStatusPending
	}

	result, err := r.collection.InsertOne(ctx, order)
	if err != nil {
		return err
	}
	order.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *orderRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	var order models.Order
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&order)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *orderRepository) GetByReference(ctx context.Context, tenantID primitive.ObjectID, reference string) (*models.Order, error) {
	var order models.Order
	err := r.collection.FindOne(ctx, bson.M{
		"tenant_id":       tenantID,
		"order_reference": reference,
	}).Decode(&order)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *orderRepository) Update(ctx context.Context, order *models.Order) error {
	order.UpdatedAt = time.Now()
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": order.ID}, order)
	return err
}

func (r *orderRepository) ListByTenant(ctx context.Context, tenantID primitive.ObjectID, siteID *primitive.ObjectID, status string, page, limit int) ([]*models.Order, int64, error) {
	query := bson.M{"tenant_id": tenantID}

	if status != "" {
		query["status"] = status
	}
	if siteID != nil && !siteID.IsZero() {
		query["site_id"] = *siteID
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
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var orders []*models.Order
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (r *orderRepository) GetPendingForSite(ctx context.Context, siteID primitive.ObjectID) ([]*models.Order, error) {
	now := time.Now()
	cutoff := now.Add(60 * time.Minute) // Get orders due within the next hour

	query := bson.M{
		"site_id": siteID,
		"status": bson.M{
			"$in": []models.OrderStatus{
				models.OrderStatusPending,
				models.OrderStatusAccepted,
			},
		},
		"execution_time": bson.M{"$lte": cutoff},
	}

	opts := options.Find().
		SetSort(bson.D{
			{Key: "priority", Value: -1},
			{Key: "execution_time", Value: 1},
			{Key: "created_at", Value: 1},
		})

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var orders []*models.Order
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, err
	}

	return orders, nil
}

func (r *orderRepository) GetByGroupID(ctx context.Context, tenantID primitive.ObjectID, groupID string) ([]*models.Order, error) {
	query := bson.M{
		"tenant_id":      tenantID,
		"order_group_id": groupID,
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var orders []*models.Order
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, err
	}

	return orders, nil
}

func (r *orderRepository) GetByKOSOrderID(ctx context.Context, kosOrderID string) (*models.Order, error) {
	var order models.Order
	err := r.collection.FindOne(ctx, bson.M{"kos_order_id": kosOrderID}).Decode(&order)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}
