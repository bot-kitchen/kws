// MongoDB initialization script for KWS
// This script runs when MongoDB container starts for the first time

db = db.getSiblingDB('kws');

// Create collections with validation
db.createCollection('tenants', {
  validator: {
    $jsonSchema: {
      bsonType: 'object',
      required: ['code', 'name', 'status', 'keycloak_realm_name'],
      properties: {
        code: { bsonType: 'string', description: 'Unique tenant code' },
        name: { bsonType: 'string', description: 'Tenant display name' },
        status: { enum: ['active', 'suspended', 'trial'], description: 'Tenant status' },
        keycloak_realm_name: { bsonType: 'string', description: 'Keycloak realm name' }
      }
    }
  }
});

db.createCollection('regions');
db.createCollection('sites');
db.createCollection('kitchens');
db.createCollection('kos_instances');
db.createCollection('kos_heartbeats');
db.createCollection('ingredients');
db.createCollection('recipes');
db.createCollection('recipe_sync_records');
db.createCollection('orders');
db.createCollection('order_sync_records');
db.createCollection('audit_logs');

print('KWS collections created successfully');
