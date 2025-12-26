package app

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ak/kws/internal/domain/models"
	"github.com/ak/kws/internal/domain/repositories"
	infrarepos "github.com/ak/kws/internal/infrastructure/repositories"
	"github.com/ak/kws/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Handlers holds API handler dependencies
type Handlers struct {
	repos  *infrarepos.Provider
	logger *logger.Logger
}

// NewHandlers creates a new Handlers instance
func NewHandlers(repos *infrarepos.Provider, log *logger.Logger) *Handlers {
	return &Handlers{
		repos:  repos,
		logger: log,
	}
}

// APIResponse is the standard API response format
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Meta      *APIMeta    `json:"meta,omitempty"`
	Timestamp string      `json:"timestamp"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type APIMeta struct {
	Page       int   `json:"page,omitempty"`
	PerPage    int   `json:"per_page,omitempty"`
	Total      int64 `json:"total,omitempty"`
	TotalPages int   `json:"total_pages,omitempty"`
}

func successResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func createdResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func paginatedResponse(c *gin.Context, data interface{}, page, perPage int, total int64) {
	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
		Meta: &APIMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func errorResponse(c *gin.Context, status int, code, message string) {
	c.JSON(status, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func getObjectID(c *gin.Context, param string) (primitive.ObjectID, bool) {
	idStr := c.Param(param)
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid ID format")
		return primitive.NilObjectID, false
	}
	return id, true
}

func getPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}

// Health and info endpoints

func (a *Application) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *Application) readinessCheck(c *gin.Context) {
	if err := a.mongodb.Health(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "not ready",
			"reason":    "database unavailable",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ready",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *Application) apiInfo(c *gin.Context) {
	successResponse(c, gin.H{
		"name":        "KWS - Kitchen Web Service",
		"version":     "0.1.0",
		"description": "Cloud orchestrator for KOS (Kitchen Operating System) instances",
	})
}

// ==================== Tenant handlers ====================

type CreateTenantRequest struct {
	Code         string                 `json:"code" binding:"required"`
	Name         string                 `json:"name" binding:"required"`
	ContactEmail string                 `json:"contact_email" binding:"required,email"`
	ContactPhone string                 `json:"contact_phone"`
	Plan         string                 `json:"plan"`
	Address      *models.Address        `json:"address"`
	Settings     *models.TenantSettings `json:"settings"`
}

func (a *Application) listTenants(c *gin.Context) {
	page, limit := getPagination(c)
	status := c.Query("status")

	tenants, total, err := a.repos.Tenant.List(c.Request.Context(), repositories.TenantFilter{
		Status: status,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to list tenants")
		return
	}

	paginatedResponse(c, tenants, page, limit, total)
}

func (a *Application) createTenant(c *gin.Context) {
	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	// Check if code already exists
	existing, _ := a.repos.Tenant.GetByCode(c.Request.Context(), req.Code)
	if existing != nil {
		errorResponse(c, http.StatusConflict, "ALREADY_EXISTS", "Tenant with this code already exists")
		return
	}

	tenant := &models.Tenant{
		Code:              req.Code,
		Name:              req.Name,
		Status:            models.TenantStatusActive,
		Plan:              req.Plan,
		KeycloakRealmName: "tenant-" + req.Code,
		ContactEmail:      req.ContactEmail,
		ContactPhone:      req.ContactPhone,
		Address:           req.Address,
		Settings:          req.Settings,
	}

	if tenant.Plan == "" {
		tenant.Plan = "starter"
	}

	if err := a.repos.Tenant.Create(c.Request.Context(), tenant); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to create tenant")
		return
	}

	createdResponse(c, tenant)
}

func (a *Application) getTenant(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	tenant, err := a.repos.Tenant.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get tenant")
		return
	}
	if tenant == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
		return
	}

	successResponse(c, tenant)
}

func (a *Application) updateTenant(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	tenant, err := a.repos.Tenant.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get tenant")
		return
	}
	if tenant == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Tenant not found")
		return
	}

	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	tenant.Name = req.Name
	tenant.ContactEmail = req.ContactEmail
	tenant.ContactPhone = req.ContactPhone
	if req.Plan != "" {
		tenant.Plan = req.Plan
	}
	if req.Address != nil {
		tenant.Address = req.Address
	}
	if req.Settings != nil {
		tenant.Settings = req.Settings
	}

	if err := a.repos.Tenant.Update(c.Request.Context(), tenant); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update tenant")
		return
	}

	successResponse(c, tenant)
}

// ==================== Region handlers ====================

type CreateRegionRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
	Code     string `json:"code" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Timezone string `json:"timezone"`
}

func (a *Application) listRegions(c *gin.Context) {
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

	regions, total, err := a.repos.Region.ListByTenant(c.Request.Context(), tenantID, page, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to list regions")
		return
	}

	paginatedResponse(c, regions, page, limit, total)
}

func (a *Application) createRegion(c *gin.Context) {
	var req CreateRegionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(req.TenantID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid tenant_id format")
		return
	}

	region := &models.Region{
		TenantID: tenantID,
		Code:     req.Code,
		Name:     req.Name,
		Timezone: req.Timezone,
		Status:   "active",
	}

	if err := a.repos.Region.Create(c.Request.Context(), region); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to create region")
		return
	}

	createdResponse(c, region)
}

func (a *Application) getRegion(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	region, err := a.repos.Region.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get region")
		return
	}
	if region == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Region not found")
		return
	}

	successResponse(c, region)
}

func (a *Application) updateRegion(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	region, err := a.repos.Region.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get region")
		return
	}
	if region == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Region not found")
		return
	}

	var req CreateRegionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	region.Name = req.Name
	if req.Timezone != "" {
		region.Timezone = req.Timezone
	}

	if err := a.repos.Region.Update(c.Request.Context(), region); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update region")
		return
	}

	successResponse(c, region)
}

// ==================== Site handlers ====================

type CreateSiteRequest struct {
	TenantID string          `json:"tenant_id" binding:"required"`
	RegionID string          `json:"region_id" binding:"required"`
	Code     string          `json:"code" binding:"required"`
	Name     string          `json:"name" binding:"required"`
	Timezone string          `json:"timezone"`
	Address  *models.Address `json:"address"`
}

func (a *Application) listSites(c *gin.Context) {
	tenantIDStr := c.Query("tenant_id")
	regionIDStr := c.Query("region_id")

	page, limit := getPagination(c)

	if regionIDStr != "" {
		regionID, err := primitive.ObjectIDFromHex(regionIDStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid region_id format")
			return
		}
		sites, total, err := a.repos.Site.ListByRegion(c.Request.Context(), regionID, page, limit)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to list sites")
			return
		}
		paginatedResponse(c, sites, page, limit, total)
		return
	}

	if tenantIDStr == "" {
		errorResponse(c, http.StatusBadRequest, "MISSING_PARAM", "tenant_id or region_id is required")
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid tenant_id format")
		return
	}

	sites, total, err := a.repos.Site.ListByTenant(c.Request.Context(), tenantID, page, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to list sites")
		return
	}

	paginatedResponse(c, sites, page, limit, total)
}

func (a *Application) createSite(c *gin.Context) {
	var req CreateSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(req.TenantID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid tenant_id format")
		return
	}

	regionID, err := primitive.ObjectIDFromHex(req.RegionID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid region_id format")
		return
	}

	site := &models.Site{
		TenantID: tenantID,
		RegionID: regionID,
		Code:     req.Code,
		Name:     req.Name,
		Timezone: req.Timezone,
		Address:  req.Address,
		Status:   "active",
	}

	if err := a.repos.Site.Create(c.Request.Context(), site); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to create site")
		return
	}

	createdResponse(c, site)
}

func (a *Application) getSite(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	site, err := a.repos.Site.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get site")
		return
	}
	if site == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Site not found")
		return
	}

	successResponse(c, site)
}

func (a *Application) updateSite(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	site, err := a.repos.Site.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get site")
		return
	}
	if site == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Site not found")
		return
	}

	var req CreateSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	site.Name = req.Name
	if req.Timezone != "" {
		site.Timezone = req.Timezone
	}
	if req.Address != nil {
		site.Address = req.Address
	}

	if err := a.repos.Site.Update(c.Request.Context(), site); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update site")
		return
	}

	successResponse(c, site)
}

// ==================== Kitchen handlers ====================

type CreateKitchenRequest struct {
	TenantID            string `json:"tenant_id" binding:"required"`
	RegionID            string `json:"region_id" binding:"required"`
	SiteID              string `json:"site_id" binding:"required"`
	KitchenID           string `json:"kitchen_id" binding:"required"`
	Name                string `json:"name" binding:"required"`
	MaxConcurrentOrders int    `json:"max_concurrent_orders"`
}

func (a *Application) listKitchens(c *gin.Context) {
	siteIDStr := c.Query("site_id")
	if siteIDStr == "" {
		errorResponse(c, http.StatusBadRequest, "MISSING_PARAM", "site_id is required")
		return
	}

	siteID, err := primitive.ObjectIDFromHex(siteIDStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_ID", "Invalid site_id format")
		return
	}

	kitchens, err := a.repos.Kitchen.ListBySite(c.Request.Context(), siteID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to list kitchens")
		return
	}

	successResponse(c, kitchens)
}

func (a *Application) createKitchen(c *gin.Context) {
	var req CreateKitchenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	tenantID, _ := primitive.ObjectIDFromHex(req.TenantID)
	regionID, _ := primitive.ObjectIDFromHex(req.RegionID)
	siteID, _ := primitive.ObjectIDFromHex(req.SiteID)

	kitchen := &models.Kitchen{
		TenantID:            tenantID,
		RegionID:            regionID,
		SiteID:              siteID,
		KitchenID:           req.KitchenID,
		Name:                req.Name,
		MaxConcurrentOrders: req.MaxConcurrentOrders,
		Status:              "online",
	}

	if kitchen.MaxConcurrentOrders == 0 {
		kitchen.MaxConcurrentOrders = 13
	}

	if err := a.repos.Kitchen.Create(c.Request.Context(), kitchen); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to create kitchen")
		return
	}

	createdResponse(c, kitchen)
}

func (a *Application) getKitchen(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	kitchen, err := a.repos.Kitchen.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get kitchen")
		return
	}
	if kitchen == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Kitchen not found")
		return
	}

	successResponse(c, kitchen)
}

func (a *Application) updateKitchen(c *gin.Context) {
	id, ok := getObjectID(c, "id")
	if !ok {
		return
	}

	kitchen, err := a.repos.Kitchen.GetByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to get kitchen")
		return
	}
	if kitchen == nil {
		errorResponse(c, http.StatusNotFound, "NOT_FOUND", "Kitchen not found")
		return
	}

	var req CreateKitchenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	kitchen.Name = req.Name
	if req.MaxConcurrentOrders > 0 {
		kitchen.MaxConcurrentOrders = req.MaxConcurrentOrders
	}

	if err := a.repos.Kitchen.Update(c.Request.Context(), kitchen); err != nil {
		errorResponse(c, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to update kitchen")
		return
	}

	successResponse(c, kitchen)
}
