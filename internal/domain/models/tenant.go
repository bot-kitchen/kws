package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Tenant represents a B2B customer organization
type Tenant struct {
	ID                primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Code              string             `bson:"code" json:"code"`                               // Unique short code: "abc-foods"
	Name              string             `bson:"name" json:"name"`                               // Display name: "ABC Foods Inc."
	Status            TenantStatus       `bson:"status" json:"status"`                           // active, suspended, trial
	Plan              string             `bson:"plan" json:"plan"`                               // starter, professional, enterprise
	KeycloakRealmName string             `bson:"keycloak_realm_name" json:"keycloak_realm_name"` // Keycloak realm: "tenant-abc-foods"
	ContactEmail      string             `bson:"contact_email" json:"contact_email"`
	ContactPhone      string             `bson:"contact_phone,omitempty" json:"contact_phone,omitempty"`
	Address           *Address           `bson:"address,omitempty" json:"address,omitempty"`
	Settings          *TenantSettings    `bson:"settings,omitempty" json:"settings,omitempty"`
	CreatedAt         time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt         time.Time          `bson:"updated_at" json:"updated_at"`
}

type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusTrial     TenantStatus = "trial"
)

type TenantSettings struct {
	DefaultTimezone   string `bson:"default_timezone" json:"default_timezone"`
	DefaultCurrency   string `bson:"default_currency" json:"default_currency"`
	RecipeSyncEnabled bool   `bson:"recipe_sync_enabled" json:"recipe_sync_enabled"`
	OrderSyncEnabled  bool   `bson:"order_sync_enabled" json:"order_sync_enabled"`
}

type Address struct {
	Street     string `bson:"street,omitempty" json:"street,omitempty"`
	City       string `bson:"city,omitempty" json:"city,omitempty"`
	State      string `bson:"state,omitempty" json:"state,omitempty"`
	PostalCode string `bson:"postal_code,omitempty" json:"postal_code,omitempty"`
	Country    string `bson:"country,omitempty" json:"country,omitempty"`
}

// Region represents a geographical region within a tenant
type Region struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID  primitive.ObjectID `bson:"tenant_id" json:"tenant_id"`
	Code      string             `bson:"code" json:"code"` // "us-west", "eu-central"
	Name      string             `bson:"name" json:"name"` // "US West Coast", "EU Central"
	Timezone  string             `bson:"timezone" json:"timezone"`
	Status    string             `bson:"status" json:"status"` // active, inactive
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// Site represents a physical location within a region
type Site struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID  primitive.ObjectID `bson:"tenant_id" json:"tenant_id"`
	RegionID  primitive.ObjectID `bson:"region_id" json:"region_id"`
	Code      string             `bson:"code" json:"code"` // "sf-downtown", "nyc-midtown"
	Name      string             `bson:"name" json:"name"` // "San Francisco Downtown"
	Address   *Address           `bson:"address,omitempty" json:"address,omitempty"`
	Timezone  string             `bson:"timezone" json:"timezone"`
	Status    string             `bson:"status" json:"status"` // active, inactive, maintenance
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// Kitchen represents a kitchen within a site (maps to KOS kitchen concept)
type Kitchen struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID            primitive.ObjectID `bson:"tenant_id" json:"tenant_id"`
	RegionID            primitive.ObjectID `bson:"region_id" json:"region_id"`
	SiteID              primitive.ObjectID `bson:"site_id" json:"site_id"`
	KitchenID           string             `bson:"kitchen_id" json:"kitchen_id"` // Matches KOS kitchen_id: "kitchen_1"
	Name                string             `bson:"name" json:"name"`
	MaxConcurrentOrders int                `bson:"max_concurrent_orders" json:"max_concurrent_orders"`
	Status              string             `bson:"status" json:"status"` // online, maintenance, emergency, offline
	CreatedAt           time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time          `bson:"updated_at" json:"updated_at"`
}

// KOSInstance represents a registered KOS device
type KOSInstance struct {
	ID                primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	TenantID          primitive.ObjectID   `bson:"tenant_id" json:"tenant_id"`
	RegionID          primitive.ObjectID   `bson:"region_id" json:"region_id"`
	SiteID            primitive.ObjectID   `bson:"site_id" json:"site_id"`
	Name              string               `bson:"name" json:"name"` // "Downtown Kitchen KOS"
	Version           string               `bson:"version" json:"version"`
	Status            KOSStatus            `bson:"status" json:"status"`
	LastHeartbeat     *time.Time           `bson:"last_heartbeat,omitempty" json:"last_heartbeat,omitempty"`
	Kitchens          []primitive.ObjectID `bson:"kitchens" json:"kitchens"` // Kitchen IDs managed by this KOS
	KeycloakClientID  string               `bson:"keycloak_client_id" json:"keycloak_client_id"`
	CertificatePEM    string               `bson:"certificate_pem,omitempty" json:"certificate_pem,omitempty"`
	PrivateKeyPEM     string               `bson:"private_key_pem,omitempty" json:"-"` // Never expose in JSON
	CertificateSerial string               `bson:"certificate_serial,omitempty" json:"certificate_serial,omitempty"`
	CertificateExpiry time.Time            `bson:"certificate_expiry,omitempty" json:"certificate_expiry,omitempty"`
	RegisteredAt      *time.Time           `bson:"registered_at,omitempty" json:"registered_at,omitempty"`
	CreatedAt         time.Time            `bson:"created_at" json:"created_at"`
	UpdatedAt         time.Time            `bson:"updated_at" json:"updated_at"`
}

type KOSStatus string

const (
	KOSStatusPending     KOSStatus = "pending"     // Created, awaiting registration
	KOSStatusProvisioned KOSStatus = "provisioned" // Certificate generated, not yet registered
	KOSStatusOnline      KOSStatus = "online"      // Registered and receiving heartbeats
	KOSStatusRegistered  KOSStatus = "registered"  // Registered and active
	KOSStatusOffline     KOSStatus = "offline"     // No recent heartbeat
	KOSStatusMaintenance KOSStatus = "maintenance" // Under maintenance
	KOSStatusDeactivated KOSStatus = "deactivated" // Deactivated by admin
)

// KOSHeartbeat represents a heartbeat from a KOS instance
type KOSHeartbeat struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	KOSID                primitive.ObjectID `bson:"kos_id" json:"kos_id"`
	Version              string             `bson:"version" json:"version"`
	Status               string             `bson:"status" json:"status"`
	Metrics              map[string]any     `bson:"metrics,omitempty" json:"metrics,omitempty"`
	ActiveOrders         int                `bson:"active_orders" json:"active_orders"`
	DevicesOnline        int                `bson:"devices_online" json:"devices_online"`
	DevicesTotal         int                `bson:"devices_total" json:"devices_total"`
	CPUPercent           float64            `bson:"cpu_percent" json:"cpu_percent"`
	MemoryPercent        float64            `bson:"memory_percent" json:"memory_percent"`
	DiskPercent          float64            `bson:"disk_percent" json:"disk_percent"`
	OrdersCompletedToday int                `bson:"orders_completed_today" json:"orders_completed_today"`
	ReceivedAt           time.Time          `bson:"received_at" json:"received_at"`
}
