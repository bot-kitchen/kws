package database

import (
	"context"
	"fmt"
	"time"

	"github.com/ak/kws/internal/infrastructure/config"
	"github.com/ak/kws/internal/pkg/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

// MongoDB wraps the MongoDB client and database
type MongoDB struct {
	client   *mongo.Client
	database *mongo.Database
	config   config.MongoDBConfig
	logger   *logger.Logger
}

// NewMongoDB creates a new MongoDB connection
func NewMongoDB(cfg config.MongoDBConfig, log *logger.Logger) (*MongoDB, error) {
	return &MongoDB{
		config: cfg,
		logger: log.WithComponent("mongodb"),
	}, nil
}

// Connect establishes connection to MongoDB
func (m *MongoDB) Connect(ctx context.Context) error {
	clientOpts := options.Client().
		ApplyURI(m.config.URI).
		SetMaxPoolSize(m.config.MaxPoolSize).
		SetMinPoolSize(m.config.MinPoolSize).
		SetConnectTimeout(m.config.ConnectTimeout)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Verify connection
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	m.client = client
	m.database = client.Database(m.config.Database)
	m.logger.Info("Connected to MongoDB", zap.String("database", m.config.Database))

	// Create indexes
	if err := m.createIndexes(ctx); err != nil {
		m.logger.Warn("Failed to create some indexes", zap.Error(err))
	}

	return nil
}

// Close closes the MongoDB connection
func (m *MongoDB) Close(ctx context.Context) error {
	if m.client != nil {
		return m.client.Disconnect(ctx)
	}
	return nil
}

// Database returns the database instance
func (m *MongoDB) Database() *mongo.Database {
	return m.database
}

// Client returns the client instance
func (m *MongoDB) Client() *mongo.Client {
	return m.client
}

// Collection returns a collection by name
func (m *MongoDB) Collection(name string) *mongo.Collection {
	return m.database.Collection(name)
}

// Collections
const (
	CollectionTenants           = "tenants"
	CollectionRegions           = "regions"
	CollectionSites             = "sites"
	CollectionKitchens          = "kitchens"
	CollectionKOSInstances      = "kos_instances"
	CollectionKOSHeartbeats     = "kos_heartbeats"
	CollectionIngredients       = "ingredients"
	CollectionRecipes           = "recipes"
	CollectionRecipeSyncRecords = "recipe_sync_records"
	CollectionOrders            = "orders"
	CollectionOrderSyncRecords  = "order_sync_records"
	CollectionAuditLogs         = "audit_logs"
	CollectionAPIKeys           = "api_keys"
)

// createIndexes creates necessary indexes for all collections
func (m *MongoDB) createIndexes(ctx context.Context) error {
	indexes := map[string][]mongo.IndexModel{
		CollectionTenants: {
			{Keys: bson.D{{Key: "code", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "keycloak_realm_name", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "status", Value: 1}}},
		},
		CollectionRegions: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "code", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}}},
		},
		CollectionSites: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "code", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "region_id", Value: 1}}},
			{Keys: bson.D{{Key: "region_id", Value: 1}}},
		},
		CollectionKitchens: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "site_id", Value: 1}, {Key: "kitchen_id", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "site_id", Value: 1}}},
		},
		CollectionKOSInstances: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "site_id", Value: 1}}},
			{Keys: bson.D{{Key: "site_id", Value: 1}}, Options: options.Index().SetUnique(true)}, // One KOS per site
			{Keys: bson.D{{Key: "certificate_serial", Value: 1}}, Options: options.Index().SetSparse(true)},
			{Keys: bson.D{{Key: "status", Value: 1}}},
		},
		CollectionKOSHeartbeats: {
			{Keys: bson.D{{Key: "kos_id", Value: 1}, {Key: "received_at", Value: -1}}},
			{Keys: bson.D{{Key: "received_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(86400 * 7)}, // TTL: 7 days
		},
		CollectionIngredients: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "name", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "is_active", Value: 1}}},
		},
		CollectionRecipes: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "name", Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "status", Value: 1}}},
			{Keys: bson.D{{Key: "published_to_sites", Value: 1}}},
		},
		CollectionRecipeSyncRecords: {
			{Keys: bson.D{{Key: "recipe_id", Value: 1}, {Key: "kos_id", Value: 1}}},
			{Keys: bson.D{{Key: "kos_id", Value: 1}, {Key: "sync_status", Value: 1}}},
		},
		CollectionOrders: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "status", Value: 1}, {Key: "created_at", Value: -1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "region_id", Value: 1}, {Key: "site_id", Value: 1}}},
			{Keys: bson.D{{Key: "site_id", Value: 1}, {Key: "status", Value: 1}, {Key: "execution_time", Value: 1}}},
			{Keys: bson.D{{Key: "order_reference", Value: 1}}},
			{Keys: bson.D{{Key: "kos_order_id", Value: 1}}, Options: options.Index().SetSparse(true)},
			{Keys: bson.D{{Key: "kos_sync_status", Value: 1}}},
		},
		CollectionOrderSyncRecords: {
			{Keys: bson.D{{Key: "order_id", Value: 1}, {Key: "synced_at", Value: -1}}},
			{Keys: bson.D{{Key: "kos_id", Value: 1}}},
		},
		CollectionAuditLogs: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}}},
			{Keys: bson.D{{Key: "resource_type", Value: 1}, {Key: "resource_id", Value: 1}}},
			{Keys: bson.D{{Key: "user_id", Value: 1}}},
			{Keys: bson.D{{Key: "created_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(86400 * 90)}, // TTL: 90 days
		},
	}

	for collection, idxModels := range indexes {
		coll := m.database.Collection(collection)
		for _, idx := range idxModels {
			_, err := coll.Indexes().CreateOne(ctx, idx)
			if err != nil {
				m.logger.Warn("Failed to create index",
					zap.String("collection", collection),
					zap.Error(err))
			}
		}
	}

	return nil
}

// Health checks if MongoDB is healthy
func (m *MongoDB) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return m.client.Ping(ctx, readpref.Primary())
}
