package models

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Order represents a customer order (single recipe per order for capacity-based scheduling)
// Core fields aligned with KOS order table: OrderReference (order_id), CustomerName, RecipeID,
// PotPercentage, Priority, Status, KitchenID, SpecialInstructions
// KWS-specific fields: TenantID, RegionID, SiteID, Source, KOSSyncStatus, Modifications, etc.
type Order struct {
	ID             primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	TenantID       primitive.ObjectID  `bson:"tenant_id" json:"tenant_id"`
	RegionID       primitive.ObjectID  `bson:"region_id" json:"region_id"`                               // Required per SOP-002
	SiteID         primitive.ObjectID  `bson:"site_id" json:"site_id"`                                   // Required per SOP-002
	KitchenID      *primitive.ObjectID `bson:"kitchen_id,omitempty" json:"kitchen_id,omitempty"`         // Optional, assigned by KOS
	OrderReference string              `bson:"order_reference" json:"order_reference"`                   // External reference (POS ID)
	OrderGroupID   string              `bson:"order_group_id,omitempty" json:"order_group_id,omitempty"` // Groups multiple orders from same customer request
	CustomerName   string              `bson:"customer_name,omitempty" json:"customer_name,omitempty"`

	// Single recipe per order (enables capacity-based fetching by KOS)
	RecipeID      primitive.ObjectID `bson:"recipe_id" json:"recipe_id"`
	RecipeName    string             `bson:"recipe_name" json:"recipe_name"`       // Denormalized for display
	PotPercentage int                `bson:"pot_percentage" json:"pot_percentage"` // 25, 50, 75, 100
	Modifications []Modification     `bson:"modifications,omitempty" json:"modifications,omitempty"`

	Status              OrderStatus    `bson:"status" json:"status"`
	Priority            int            `bson:"priority" json:"priority"`
	ExecutionTime       time.Time      `bson:"execution_time" json:"execution_time"` // When to execute
	EstimatedReadyTime  *time.Time     `bson:"estimated_ready_time,omitempty" json:"estimated_ready_time,omitempty"`
	StartedAt           *time.Time     `bson:"started_at,omitempty" json:"started_at,omitempty"`
	CompletedAt         *time.Time     `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	Notes               string         `bson:"notes,omitempty" json:"notes,omitempty"`
	SpecialInstructions string         `bson:"special_instructions,omitempty" json:"special_instructions,omitempty"`
	Metadata            map[string]any `bson:"metadata,omitempty" json:"metadata,omitempty"`
	Source              OrderSource    `bson:"source" json:"source"` // kws_ui, api, pos_integration, kos_local
	KOSSyncStatus       KOSSyncStatus  `bson:"kos_sync_status" json:"kos_sync_status"`
	KOSSyncedAt         *time.Time     `bson:"kos_synced_at,omitempty" json:"kos_synced_at,omitempty"`
	KOSOrderID          string         `bson:"kos_order_id,omitempty" json:"kos_order_id,omitempty"` // ID in KOS system
	ErrorMessage        string         `bson:"error_message,omitempty" json:"error_message,omitempty"`
	CreatedBy           string         `bson:"created_by,omitempty" json:"created_by,omitempty"`
	CreatedAt           time.Time      `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time      `bson:"updated_at" json:"updated_at"`
}

// OrderItem is used for API requests when creating multiple orders at once
// Each item will be created as a separate Order
type OrderItem struct {
	RecipeID      primitive.ObjectID `bson:"recipe_id" json:"recipe_id"`
	RecipeName    string             `bson:"recipe_name" json:"recipe_name"`       // Denormalized
	Quantity      int                `bson:"quantity" json:"quantity"`             // Creates N separate orders
	PotPercentage int                `bson:"pot_percentage" json:"pot_percentage"` // 25, 50, 75, 100
	Notes         string             `bson:"notes,omitempty" json:"notes,omitempty"`
	Options       map[string]any     `bson:"options,omitempty" json:"options,omitempty"`
	Modifications []Modification     `bson:"modifications,omitempty" json:"modifications,omitempty"`
}

type Modification struct {
	Type       string `bson:"type" json:"type"`             // exclude, substitute, extra
	Ingredient string `bson:"ingredient" json:"ingredient"` // Ingredient name
	Notes      string `bson:"notes,omitempty" json:"notes,omitempty"`
}

type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "pending"     // Created, not yet sent to KOS
	OrderStatusAccepted   OrderStatus = "accepted"    // Accepted by KOS
	OrderStatusScheduled  OrderStatus = "scheduled"   // Scheduled for execution
	OrderStatusInProgress OrderStatus = "in_progress" // Being prepared
	OrderStatusCompleted  OrderStatus = "completed"   // Ready for pickup
	OrderStatusFailed     OrderStatus = "failed"      // Failed during preparation
	OrderStatusCancelled  OrderStatus = "cancelled"   // Cancelled by user or system
)

type OrderPriority string

const (
	OrderPriorityNormal    OrderPriority = "normal"
	OrderPriorityHigh      OrderPriority = "high"
	OrderPriorityVIP       OrderPriority = "vip"
	OrderPriorityEmergency OrderPriority = "emergency"
)

type OrderSource string

const (
	OrderSourceKWSUI          OrderSource = "kws_ui"
	OrderSourceAPI            OrderSource = "api"
	OrderSourcePOSIntegration OrderSource = "pos_integration"
	OrderSourceKOSLocal       OrderSource = "kos_local" // Order created locally on KOS device
)

type KOSSyncStatus string

const (
	KOSSyncStatusPending KOSSyncStatus = "pending"
	KOSSyncStatusSynced  KOSSyncStatus = "synced"
	KOSSyncStatusFailed  KOSSyncStatus = "failed"
	KOSSyncStatusUpdated KOSSyncStatus = "updated" // Status update received from KOS
)

// OrderForKOS is the simplified order format sent to KOS (single recipe per order)
type OrderForKOS struct {
	ID                  string               `json:"id"`
	OrderReference      string               `json:"order_reference"`
	OrderGroupID        string               `json:"order_group_id,omitempty"` // Groups related orders
	CustomerName        string               `json:"customer_name,omitempty"`
	RecipeID            string               `json:"recipe_id"`
	RecipeName          string               `json:"recipe_name"`
	PotPercentage       int                  `json:"pot_percentage"`
	Modifications       []ModificationForKOS `json:"modifications,omitempty"`
	Priority            string               `json:"priority"`
	ExecutionTime       *time.Time           `json:"execution_time,omitempty"`
	SpecialInstructions string               `json:"special_instructions,omitempty"`
}

type ModificationForKOS struct {
	Type       string `json:"type"`
	Ingredient string `json:"ingredient"`
	Notes      string `json:"notes,omitempty"`
}

// OrderStatusUpdate represents a status update from KOS
type OrderStatusUpdate struct {
	KWSOrderID     string     `json:"kws_order_id"`
	KOSOrderID     string     `json:"kos_order_id"`
	Status         string     `json:"status"`
	KitchenID      string     `json:"kitchen_id,omitempty"`
	StartTime      *time.Time `json:"start_time,omitempty"`
	CompletionTime *time.Time `json:"completion_time,omitempty"`
	FailureReason  string     `json:"failure_reason,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// OrderSyncRecord tracks order sync status with KOS
type OrderSyncRecord struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OrderID      primitive.ObjectID `bson:"order_id" json:"order_id"`
	KOSID        primitive.ObjectID `bson:"kos_id" json:"kos_id"`
	SiteID       primitive.ObjectID `bson:"site_id" json:"site_id"`
	Direction    string             `bson:"direction" json:"direction"` // kws_to_kos, kos_to_kws
	SyncStatus   string             `bson:"sync_status" json:"sync_status"`
	RequestBody  string             `bson:"request_body,omitempty" json:"request_body,omitempty"`
	ResponseBody string             `bson:"response_body,omitempty" json:"response_body,omitempty"`
	ErrorMessage string             `bson:"error_message,omitempty" json:"error_message,omitempty"`
	SyncedAt     time.Time          `bson:"synced_at" json:"synced_at"`
}

// ToKOSFormat converts an Order to the simplified KOS format
func (o *Order) ToKOSFormat() OrderForKOS {
	mods := make([]ModificationForKOS, len(o.Modifications))
	for i, mod := range o.Modifications {
		mods[i] = ModificationForKOS{
			Type:       mod.Type,
			Ingredient: mod.Ingredient,
			Notes:      mod.Notes,
		}
	}

	return OrderForKOS{
		ID:                  o.ID.Hex(),
		OrderReference:      o.OrderReference,
		OrderGroupID:        o.OrderGroupID,
		CustomerName:        o.CustomerName,
		RecipeID:            o.RecipeID.Hex(),
		RecipeName:          o.RecipeName,
		PotPercentage:       o.PotPercentage,
		Modifications:       mods,
		Priority:            fmt.Sprintf("%d", o.Priority),
		ExecutionTime:       &o.ExecutionTime,
		SpecialInstructions: o.SpecialInstructions,
	}
}
