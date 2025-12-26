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

// Region Repository

type regionRepository struct {
	collection *mongo.Collection
}

func NewRegionRepository(db *database.MongoDB) repositories.RegionRepository {
	return &regionRepository{
		collection: db.Collection(database.CollectionRegions),
	}
}

func (r *regionRepository) Create(ctx context.Context, region *models.Region) error {
	region.CreatedAt = time.Now()
	region.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, region)
	if err != nil {
		return err
	}
	region.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *regionRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Region, error) {
	var region models.Region
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&region)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &region, nil
}

func (r *regionRepository) Update(ctx context.Context, region *models.Region) error {
	region.UpdatedAt = time.Now()
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": region.ID}, region)
	return err
}

func (r *regionRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *regionRepository) ListByTenant(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]*models.Region, int64, error) {
	query := bson.M{"tenant_id": tenantID}

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

	var regions []*models.Region
	if err := cursor.All(ctx, &regions); err != nil {
		return nil, 0, err
	}

	return regions, total, nil
}

// Site Repository

type siteRepository struct {
	collection *mongo.Collection
}

func NewSiteRepository(db *database.MongoDB) repositories.SiteRepository {
	return &siteRepository{
		collection: db.Collection(database.CollectionSites),
	}
}

func (r *siteRepository) Create(ctx context.Context, site *models.Site) error {
	site.CreatedAt = time.Now()
	site.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, site)
	if err != nil {
		return err
	}
	site.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *siteRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Site, error) {
	var site models.Site
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&site)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &site, nil
}

func (r *siteRepository) Update(ctx context.Context, site *models.Site) error {
	site.UpdatedAt = time.Now()
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": site.ID}, site)
	return err
}

func (r *siteRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *siteRepository) ListByTenant(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]*models.Site, int64, error) {
	query := bson.M{"tenant_id": tenantID}
	return r.listWithPagination(ctx, query, page, limit)
}

func (r *siteRepository) ListByRegion(ctx context.Context, regionID primitive.ObjectID, page, limit int) ([]*models.Site, int64, error) {
	query := bson.M{"region_id": regionID}
	return r.listWithPagination(ctx, query, page, limit)
}

func (r *siteRepository) listWithPagination(ctx context.Context, query bson.M, page, limit int) ([]*models.Site, int64, error) {
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

	var sites []*models.Site
	if err := cursor.All(ctx, &sites); err != nil {
		return nil, 0, err
	}

	return sites, total, nil
}

// Kitchen Repository

type kitchenRepository struct {
	collection *mongo.Collection
}

func NewKitchenRepository(db *database.MongoDB) repositories.KitchenRepository {
	return &kitchenRepository{
		collection: db.Collection(database.CollectionKitchens),
	}
}

func (r *kitchenRepository) Create(ctx context.Context, kitchen *models.Kitchen) error {
	kitchen.CreatedAt = time.Now()
	kitchen.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, kitchen)
	if err != nil {
		return err
	}
	kitchen.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *kitchenRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Kitchen, error) {
	var kitchen models.Kitchen
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&kitchen)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &kitchen, nil
}

func (r *kitchenRepository) Update(ctx context.Context, kitchen *models.Kitchen) error {
	kitchen.UpdatedAt = time.Now()
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": kitchen.ID}, kitchen)
	return err
}

func (r *kitchenRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *kitchenRepository) ListBySite(ctx context.Context, siteID primitive.ObjectID) ([]*models.Kitchen, error) {
	query := bson.M{"site_id": siteID}

	opts := options.Find().SetSort(bson.D{{Key: "kitchen_id", Value: 1}})

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var kitchens []*models.Kitchen
	if err := cursor.All(ctx, &kitchens); err != nil {
		return nil, err
	}

	return kitchens, nil
}

// KOS Instance Repository

type kosInstanceRepository struct {
	collection          *mongo.Collection
	heartbeatCollection *mongo.Collection
}

func NewKOSInstanceRepository(db *database.MongoDB) repositories.KOSInstanceRepository {
	return &kosInstanceRepository{
		collection:          db.Collection(database.CollectionKOSInstances),
		heartbeatCollection: db.Collection(database.CollectionKOSHeartbeats),
	}
}

func (r *kosInstanceRepository) Create(ctx context.Context, kos *models.KOSInstance) error {
	kos.CreatedAt = time.Now()
	kos.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, kos)
	if err != nil {
		return err
	}
	kos.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *kosInstanceRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.KOSInstance, error) {
	var kos models.KOSInstance
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&kos)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &kos, nil
}

func (r *kosInstanceRepository) GetBySiteID(ctx context.Context, siteID primitive.ObjectID) (*models.KOSInstance, error) {
	var kos models.KOSInstance
	err := r.collection.FindOne(ctx, bson.M{"site_id": siteID}).Decode(&kos)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &kos, nil
}

func (r *kosInstanceRepository) GetByCertificateSerial(ctx context.Context, serial string) (*models.KOSInstance, error) {
	var kos models.KOSInstance
	err := r.collection.FindOne(ctx, bson.M{"certificate_serial": serial}).Decode(&kos)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &kos, nil
}

func (r *kosInstanceRepository) Update(ctx context.Context, kos *models.KOSInstance) error {
	kos.UpdatedAt = time.Now()
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": kos.ID}, kos)
	return err
}

func (r *kosInstanceRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *kosInstanceRepository) ListByTenant(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]*models.KOSInstance, int64, error) {
	query := bson.M{"tenant_id": tenantID}

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

	var instances []*models.KOSInstance
	if err := cursor.All(ctx, &instances); err != nil {
		return nil, 0, err
	}

	return instances, total, nil
}

func (r *kosInstanceRepository) RecordHeartbeat(ctx context.Context, heartbeat *models.KOSHeartbeat) error {
	heartbeat.ReceivedAt = time.Now()
	_, err := r.heartbeatCollection.InsertOne(ctx, heartbeat)
	return err
}
