package app

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ak/kws/internal/domain/models"
	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ==================== KOS Instance Management handlers ====================

type CreateKOSInstanceRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
	SiteID   string `json:"site_id" binding:"required"`
	Name     string `json:"name" binding:"required"`
}

type UpdateKOSInstanceRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (a *Application) listKOSInstances(c *gin.Context) {
	tenantIDStr := c.Query("tenant_id")
	if tenantIDStr == "" {
		errorResponse(c, http.StatusBadRequest, "MISSING_PARAM", "tenant_id is required")
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid tenant_id format")
		return
	}

	page, limit := getPagination(c)

	instances, total, err := a.repos.KOSInstance.ListByTenant(c.Request.Context(), tenantID, page, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to list KOS instances")
		return
	}

	paginatedResponse(c, instances, page, limit, total)
}

func (a *Application) createKOSInstance(c *gin.Context) {
	var req CreateKOSInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(req.TenantID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid tenant_id format")
		return
	}

	siteID, err := primitive.ObjectIDFromHex(req.SiteID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid site_id format")
		return
	}

	// Check if site already has a KOS instance
	existing, err := a.repos.KOSInstance.GetBySiteID(c.Request.Context(), siteID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to check existing KOS instance")
		return
	}
	if existing != nil {
		errorResponse(c, http.StatusConflict, "SITE_HAS_KOS", "Site already has a KOS instance")
		return
	}

	instance := &models.KOSInstance{
		TenantID: tenantID,
		SiteID:   siteID,
		Name:     req.Name,
		Status:   models.KOSStatusPending,
	}

	if err := a.repos.KOSInstance.Create(c.Request.Context(), instance); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to create KOS instance")
		return
	}

	createdResponse(c, instance)
}

func (a *Application) getKOSInstance(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get KOS instance")
		return
	}
	if instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	successResponse(c, instance)
}

func (a *Application) updateKOSInstance(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get KOS instance")
		return
	}
	if instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	var req UpdateKOSInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	if req.Name != "" {
		instance.Name = req.Name
	}
	if req.Status != "" {
		instance.Status = models.KOSStatus(req.Status)
	}

	if err := a.repos.KOSInstance.Update(c.Request.Context(), instance); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update KOS instance")
		return
	}

	successResponse(c, instance)
}

func (a *Application) deleteKOSInstance(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get KOS instance")
		return
	}
	if instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	// Only allow deletion if status is pending or deactivated
	if instance.Status != models.KOSStatusPending && instance.Status != models.KOSStatusDeactivated {
		errorResponse(c, http.StatusConflict, "CANNOT_DELETE", "Can only delete pending or deactivated KOS instances. Deactivate first.")
		return
	}

	if err := a.repos.KOSInstance.Delete(c.Request.Context(), id); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to delete KOS instance")
		return
	}

	successResponse(c, gin.H{"deleted": true})
}

func (a *Application) deactivateKOSInstance(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get KOS instance")
		return
	}
	if instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	instance.Status = models.KOSStatusDeactivated

	if err := a.repos.KOSInstance.Update(c.Request.Context(), instance); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to deactivate KOS instance")
		return
	}

	successResponse(c, instance)
}

func (a *Application) activateKOSInstance(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get KOS instance")
		return
	}
	if instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	// Reset to pending - requires re-provisioning
	instance.Status = models.KOSStatusPending
	instance.CertificatePEM = ""
	instance.PrivateKeyPEM = ""
	instance.CertificateSerial = ""
	instance.RegisteredAt = nil

	if err := a.repos.KOSInstance.Update(c.Request.Context(), instance); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to activate KOS instance")
		return
	}

	successResponse(c, instance)
}

// ProvisioningBundle contains all data needed to provision a KOS instance
type ProvisioningBundle struct {
	KOSID          string `json:"kos_id"`
	TenantID       string `json:"tenant_id"`
	SiteID         string `json:"site_id"`
	KWSEndpoint    string `json:"kws_endpoint"`
	Certificate    string `json:"certificate"`
	PrivateKey     string `json:"private_key"`
	CACertificate  string `json:"ca_certificate"`
	JWTSecret      string `json:"jwt_secret"`
	RecipePollSecs int    `json:"recipe_poll_secs"`
	OrderPollSecs  int    `json:"order_poll_secs"`
}

func (a *Application) getKOSProvisioningBundle(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get KOS instance")
		return
	}
	if instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	// Generate certificate if not already generated
	if instance.CertificatePEM == "" {
		cert, key, serial, err := a.generateKOSCertificate(instance)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "CERT_ERROR", "Failed to generate certificate")
			return
		}
		instance.CertificatePEM = cert
		instance.PrivateKeyPEM = key
		instance.CertificateSerial = serial
		instance.CertificateExpiry = time.Now().AddDate(1, 0, 0) // 1 year
		instance.Status = models.KOSStatusProvisioned

		if err := a.repos.KOSInstance.Update(c.Request.Context(), instance); err != nil {
			errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to save certificate")
			return
		}
	}

	bundle := ProvisioningBundle{
		KOSID:          instance.ID.Hex(),
		TenantID:       instance.TenantID.Hex(),
		SiteID:         instance.SiteID.Hex(),
		KWSEndpoint:    a.config.Server.ExternalURL + "/api/v1",
		Certificate:    instance.CertificatePEM,
		PrivateKey:     instance.PrivateKeyPEM,
		CACertificate:  a.config.Certificate.CACert,
		JWTSecret:      a.config.JWT.Secret,
		RecipePollSecs: 300, // 5 minutes
		OrderPollSecs:  30,  // 30 seconds
	}

	// Return as downloadable JSON file
	bundleJSON, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "JSON_ERROR", "Failed to serialize bundle")
		return
	}

	filename := fmt.Sprintf("kos-provisioning-%s.json", instance.ID.Hex())
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", bundleJSON)
}

func (a *Application) getKOSProvisioningQRCode(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get KOS instance")
		return
	}
	if instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	// Generate certificate if not already generated (same as provisioning bundle)
	if instance.CertificatePEM == "" {
		cert, key, serial, err := a.generateKOSCertificate(instance)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "CERT_ERROR", "Failed to generate certificate")
			return
		}
		instance.CertificatePEM = cert
		instance.PrivateKeyPEM = key
		instance.CertificateSerial = serial
		instance.CertificateExpiry = time.Now().AddDate(1, 0, 0) // 1 year
		instance.Status = models.KOSStatusProvisioned

		if err := a.repos.KOSInstance.Update(c.Request.Context(), instance); err != nil {
			errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to save certificate")
			return
		}
	}

	// Generate URL that points to the provisioning bundle
	// The URL is dynamic - bundle content can change without regenerating QR
	bundleURL := fmt.Sprintf("%s/api/v1/kos-instances/%s/provisioning-bundle",
		a.config.Server.ExternalURL, instance.ID.Hex())

	// Generate QR code as PNG
	// Size 256x256 is good balance between scannability and file size
	qr, err := qrcode.Encode(bundleURL, qrcode.Medium, 256)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "QR_ERROR", "Failed to generate QR code")
		return
	}

	c.Header("Content-Type", "image/png")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Data(http.StatusOK, "image/png", qr)
}

func (a *Application) regenerateKOSCertificate(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get KOS instance")
		return
	}
	if instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	cert, key, serial, err := a.generateKOSCertificate(instance)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "CERT_ERROR", "Failed to generate certificate")
		return
	}

	instance.CertificatePEM = cert
	instance.PrivateKeyPEM = key
	instance.CertificateSerial = serial
	instance.CertificateExpiry = time.Now().AddDate(1, 0, 0)

	if err := a.repos.KOSInstance.Update(c.Request.Context(), instance); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to save certificate")
		return
	}

	successResponse(c, gin.H{
		"certificate_serial": serial,
		"expires_at":         instance.CertificateExpiry,
	})
}

// generateKOSCertificate creates a client certificate for KOS mTLS authentication
func (a *Application) generateKOSCertificate(instance *models.KOSInstance) (certPEM, keyPEM, serial string, err error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", err
	}

	// Generate serial number
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", "", err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   instance.ID.Hex(),
			Organization: []string{"KWS"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // 1 year validity
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Self-sign for now (in production, use CA)
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", "", err
	}

	// Encode to PEM
	certPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return string(certPEMBytes), string(keyPEMBytes), serialNumber.String(), nil
}

// ==================== KOS Device API handlers ====================
// These endpoints are called by KOS instances to communicate with KWS

type KOSRegisterRequest struct {
	KOSID   string `json:"kos_id" binding:"required"`
	Version string `json:"version" binding:"required"`
}

type KOSHeartbeatRequest struct {
	KOSID        string         `json:"kos_id" binding:"required"`
	Status       string         `json:"status" binding:"required"`
	Version      string         `json:"version"`
	Metrics      map[string]any `json:"metrics"`
	ActiveTasks  int            `json:"active_tasks"`
	ActiveOrders []string       `json:"active_orders"` // KWS order IDs currently active on this KOS
}

// L2TaskReport represents an L2 subtask reported from KOS
type L2TaskReport struct {
	L4TaskID        string            `json:"l4_task_id"`
	L4Action        string            `json:"l4_action"`
	L2Action        string            `json:"l2_action"`
	DeviceTypes     []string          `json:"device_types"`
	SelectedDevices map[string]string `json:"selected_devices"`
	IsCompleted     bool              `json:"is_completed"`
	IsInProgress    bool              `json:"is_in_progress"`
}

// TaskReport represents an L4 task reported from KOS
type TaskReport struct {
	TaskID          string         `json:"task_id"`
	StepNumber      int            `json:"step_number"`
	Action          string         `json:"action"`
	Status          string         `json:"status"`
	Parameters      string         `json:"parameters"`
	DependsOnTasks  []string       `json:"depends_on_tasks"`
	ActualStartTime *time.Time     `json:"actual_start_time"`
	ActualEndTime   *time.Time     `json:"actual_end_time"`
	ErrorMessage    string         `json:"error_message"`
	ErrorCode       string         `json:"error_code"`
	L2Tasks         []L2TaskReport `json:"l2_tasks"`
}

// EquipmentReport represents equipment info from KOS
type EquipmentReport struct {
	KitchenName string   `json:"kitchen_name"`
	Pots        []string `json:"pots"`
	PyroID      string   `json:"pyro_id"`
}

type KOSOrderStatusRequest struct {
	Status      string           `json:"status" binding:"required"`
	KOSOrderID  string           `json:"kos_order_id"`
	StartedAt   *time.Time       `json:"started_at"`
	CompletedAt *time.Time       `json:"completed_at"`
	ErrorMsg    string           `json:"error_msg"`
	Tasks       []TaskReport     `json:"tasks"`     // All tasks with L2 subtasks
	Equipment   *EquipmentReport `json:"equipment"` // Kitchen, pots, pyro
}

func (a *Application) kosRegister(c *gin.Context) {
	var req KOSRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	kosID, err := primitive.ObjectIDFromHex(req.KOSID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid kos_id format")
		return
	}

	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), kosID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get KOS instance")
		return
	}
	if instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	// Update status to online
	instance.Status = models.KOSStatusOnline
	instance.Version = req.Version
	now := time.Now()
	instance.LastHeartbeat = &now
	instance.RegisteredAt = &now

	if err := a.repos.KOSInstance.Update(c.Request.Context(), instance); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update KOS instance")
		return
	}

	successResponse(c, gin.H{
		"registered": true,
		"kos_id":     instance.ID.Hex(),
		"site_id":    instance.SiteID.Hex(),
		"tenant_id":  instance.TenantID.Hex(),
	})
}

func (a *Application) kosHeartbeat(c *gin.Context) {
	var req KOSHeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	kosID, err := primitive.ObjectIDFromHex(req.KOSID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid kos_id format")
		return
	}

	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), kosID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get KOS instance")
		return
	}
	if instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	// Record heartbeat
	heartbeat := &models.KOSHeartbeat{
		KOSID:      kosID,
		Status:     req.Status,
		ReceivedAt: time.Now(),
		Metrics:    req.Metrics,
	}

	if err := a.repos.KOSInstance.RecordHeartbeat(c.Request.Context(), heartbeat); err != nil {
		a.logger.Warn("Failed to record heartbeat")
	}

	// Update instance status
	now := time.Now()
	instance.LastHeartbeat = &now
	if req.Version != "" {
		instance.Version = req.Version
	}
	instance.Status = models.KOSStatus(req.Status)

	if err := a.repos.KOSInstance.Update(c.Request.Context(), instance); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update KOS instance")
		return
	}

	// Order reconciliation: reset orphaned orders that KOS no longer has
	// This handles cases where KOS lost its database (e.g., drop_db_on_start)
	resetCount, err := a.repos.Order.ResetOrphanedOrders(c.Request.Context(), instance.SiteID, req.ActiveOrders)
	if err != nil {
		a.logger.Warn("Failed to reset orphaned orders")
	} else if resetCount > 0 {
		a.logger.Info("Reset orphaned orders to pending")
	}

	successResponse(c, gin.H{"acknowledged": true, "orders_reset": resetCount})
}

func (a *Application) kosGetRecipes(c *gin.Context) {
	// Get KOS ID from mTLS certificate CN or JWT
	kosIDStr := c.GetHeader("X-KOS-ID")
	if kosIDStr == "" {
		errorResponse(c, http.StatusUnauthorized, "MISSING_KOS_ID", "KOS ID required")
		return
	}

	kosID, err := primitive.ObjectIDFromHex(kosIDStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid kos_id format")
		return
	}

	// Get KOS instance to find site
	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), kosID)
	if err != nil || instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	// Get recipes published to this site
	recipes, err := a.repos.Recipe.GetPublishedForSite(c.Request.Context(), instance.SiteID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get recipes")
		return
	}

	// Convert to KOS format
	kosRecipes := make([]models.RecipeForKOS, len(recipes))
	for i, r := range recipes {
		kosRecipes[i] = r.ToKOSFormat()
	}

	successResponse(c, kosRecipes)
}

func (a *Application) kosGetIngredients(c *gin.Context) {
	// Get KOS ID from header
	kosIDStr := c.GetHeader("X-KOS-ID")
	if kosIDStr == "" {
		errorResponse(c, http.StatusUnauthorized, "MISSING_KOS_ID", "KOS ID required")
		return
	}

	kosID, err := primitive.ObjectIDFromHex(kosIDStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid kos_id format")
		return
	}

	// Get KOS instance to find tenant
	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), kosID)
	if err != nil || instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	// Get all active ingredients for this tenant
	ingredients, _, err := a.repos.Ingredient.ListByTenant(c.Request.Context(), instance.TenantID, true, 1, 10000)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get ingredients")
		return
	}

	// Convert to KOS format
	kosIngredients := make([]models.IngredientForKOS, len(ingredients))
	for i, ing := range ingredients {
		kosIngredients[i] = ing.ToKOSFormat()
	}

	successResponse(c, kosIngredients)
}

func (a *Application) kosGetOrders(c *gin.Context) {
	// Get KOS ID from header
	kosIDStr := c.GetHeader("X-KOS-ID")
	if kosIDStr == "" {
		errorResponse(c, http.StatusUnauthorized, "MISSING_KOS_ID", "KOS ID required")
		return
	}

	kosID, err := primitive.ObjectIDFromHex(kosIDStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid kos_id format")
		return
	}

	// Get KOS instance to find site
	instance, err := a.repos.KOSInstance.GetByID(c.Request.Context(), kosID)
	if err != nil || instance == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "KOS instance not found")
		return
	}

	// Get pending orders for this site
	orders, err := a.repos.Order.GetPendingForSite(c.Request.Context(), instance.SiteID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get orders")
		return
	}

	// Convert to KOS format
	kosOrders := make([]models.OrderForKOS, len(orders))
	for i, o := range orders {
		kosOrders[i] = o.ToKOSFormat()
	}

	successResponse(c, kosOrders)
}

func (a *Application) kosUpdateOrderStatus(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	var req KOSOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	order, err := a.repos.Order.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get order")
		return
	}
	if order == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Order not found")
		return
	}

	// Update order status based on KOS feedback
	order.Status = models.OrderStatus(req.Status)
	order.KOSSyncStatus = models.KOSSyncStatusSynced
	if req.KOSOrderID != "" {
		order.KOSOrderID = req.KOSOrderID
	}
	if req.StartedAt != nil {
		order.StartedAt = req.StartedAt
	}
	if req.CompletedAt != nil {
		order.CompletedAt = req.CompletedAt
	}
	if req.ErrorMsg != "" {
		order.ErrorMessage = req.ErrorMsg
	}

	// Convert and store tasks from KOS
	if len(req.Tasks) > 0 {
		order.Tasks = make([]models.OrderTask, len(req.Tasks))
		for i, t := range req.Tasks {
			// Convert L2 tasks
			l2Tasks := make([]models.L2Task, len(t.L2Tasks))
			for j, l2 := range t.L2Tasks {
				l2Tasks[j] = models.L2Task{
					L4TaskID:        l2.L4TaskID,
					L4Action:        l2.L4Action,
					L2Action:        l2.L2Action,
					DeviceTypes:     l2.DeviceTypes,
					SelectedDevices: l2.SelectedDevices,
					IsCompleted:     l2.IsCompleted,
					IsInProgress:    l2.IsInProgress,
				}
			}

			order.Tasks[i] = models.OrderTask{
				TaskID:          t.TaskID,
				StepNumber:      t.StepNumber,
				Action:          t.Action,
				Status:          t.Status,
				Parameters:      t.Parameters,
				DependsOnTasks:  t.DependsOnTasks,
				ActualStartTime: t.ActualStartTime,
				ActualEndTime:   t.ActualEndTime,
				ErrorMessage:    t.ErrorMessage,
				ErrorCode:       t.ErrorCode,
				L2Tasks:         l2Tasks,
			}
		}
	}

	// Convert and store equipment info from KOS
	if req.Equipment != nil {
		order.Equipment = &models.OrderEquipment{
			KitchenName: req.Equipment.KitchenName,
			Pots:        req.Equipment.Pots,
			PyroID:      req.Equipment.PyroID,
		}
	}

	now := time.Now()
	order.KOSSyncedAt = &now

	if err := a.repos.Order.Update(c.Request.Context(), order); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update order")
		return
	}

	successResponse(c, gin.H{"updated": true})
}
