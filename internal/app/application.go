package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ak/kws/internal/app/middleware"
	"github.com/ak/kws/internal/domain/services"
	"github.com/ak/kws/internal/infrastructure/config"
	"github.com/ak/kws/internal/infrastructure/database"
	"github.com/ak/kws/internal/infrastructure/repositories"
	"github.com/ak/kws/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Application holds all application dependencies and services
type Application struct {
	config        *config.Config
	logger        *logger.Logger
	mongodb       *database.MongoDB
	repos         *repositories.Provider
	tenantService services.TenantService
	router        *gin.Engine
	handlers      *Handlers
	webHandlers   *WebHandlers
	sessionConfig middleware.SessionConfig
}

// New creates a new Application instance
func New(cfg *config.Config, log *logger.Logger, mongodb *database.MongoDB) (*Application, error) {
	repos := repositories.NewProvider(mongodb)

	// Initialize Keycloak service (optional - may not be available in dev)
	var keycloakSvc services.KeycloakService
	if cfg.Keycloak.URL != "" {
		var err error
		keycloakSvc, err = services.NewKeycloakService(cfg.Keycloak)
		if err != nil {
			log.Warn("Keycloak service unavailable, tenant realm management disabled",
				zap.Error(err))
		}
	}

	// Create tenant service with Keycloak integration
	tenantService := services.NewTenantService(repos.Tenant, keycloakSvc)

	app := &Application{
		config:        cfg,
		logger:        log,
		mongodb:       mongodb,
		repos:         repos,
		tenantService: tenantService,
	}

	// Create handlers with repositories
	app.handlers = NewHandlers(repos, log)

	// Configure session for web UI authentication
	sessionConfig := middleware.SessionConfig{
		KeycloakURL:    cfg.Keycloak.URL,
		Realm:          "kws-platform", // Platform realm for web UI
		ClientID:       "kws-web",
		ClientSecret:   "", // Public client, no secret needed
		RedirectURL:    fmt.Sprintf("%s/auth/callback", cfg.Server.ExternalURL),
		CookieName:     "kws_session",
		CookieDomain:   "",
		CookieSecure:   cfg.IsProduction(),
		CookieHTTPOnly: true,
		SessionTTL:     24 * time.Hour,
		DevMode:        cfg.IsDevelopment() && cfg.App.Debug,
	}

	// Store session config for API routes
	app.sessionConfig = sessionConfig

	// Create web handlers for UI
	webHandlers, err := NewWebHandlers(app.handlers, sessionConfig)
	if err != nil {
		return nil, err
	}
	app.webHandlers = webHandlers

	// Set Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	app.router = gin.New()

	// Add middleware
	app.router.Use(gin.Recovery())
	app.router.Use(app.loggerMiddleware())
	app.router.Use(app.corsMiddleware())

	// Setup routes
	app.setupRoutes()

	// Register web UI routes
	app.webHandlers.RegisterRoutes(app.router)

	return app, nil
}

// Router returns the HTTP handler
func (a *Application) Router() http.Handler {
	return a.router
}

// setupRoutes configures all application routes
func (a *Application) setupRoutes() {
	// Health check endpoints
	a.router.GET("/health", a.healthCheck)
	a.router.GET("/ready", a.readinessCheck)

	// API v1 routes - apply session middleware for tenant context
	v1 := a.router.Group("/api/v1")
	v1.Use(middleware.OptionalSession(a.sessionConfig)) // Read session if present, but don't require it
	{
		// Public info endpoint
		v1.GET("/info", a.apiInfo)

		// Tenant management (Platform Admin only)
		tenants := v1.Group("/tenants")
		{
			tenants.GET("", a.listTenants)
			tenants.POST("", a.createTenant)
			tenants.GET("/:id", a.getTenant)
			tenants.PUT("/:id", a.updateTenant)
			tenants.DELETE("/:id", a.deleteTenant)
			tenants.POST("/:id/suspend", a.suspendTenant)
			tenants.POST("/:id/activate", a.activateTenant)
		}

		// Region management
		regions := v1.Group("/regions")
		{
			regions.GET("", a.listRegions)
			regions.POST("", a.createRegion)
			regions.GET("/:id", a.getRegion)
			regions.PUT("/:id", a.updateRegion)
			regions.DELETE("/:id", a.deleteRegion)
		}

		// Site management
		sites := v1.Group("/sites")
		{
			sites.GET("", a.listSites)
			sites.POST("", a.createSite)
			sites.GET("/:id", a.getSite)
			sites.PUT("/:id", a.updateSite)
			sites.DELETE("/:id", a.deleteSite)
		}

		// Kitchen management
		kitchens := v1.Group("/kitchens")
		{
			kitchens.GET("", a.listKitchens)
			kitchens.POST("", a.createKitchen)
			kitchens.GET("/:id", a.getKitchen)
			kitchens.PUT("/:id", a.updateKitchen)
		}

		// KOS instance management
		kos := v1.Group("/kos-instances")
		{
			kos.GET("", a.listKOSInstances)
			kos.POST("", a.createKOSInstance)
			kos.GET("/:id", a.getKOSInstance)
			kos.PUT("/:id", a.updateKOSInstance)
			kos.GET("/:id/provisioning-bundle", a.getKOSProvisioningBundle)
			kos.GET("/:id/provisioning-qrcode", a.getKOSProvisioningQRCode)
			kos.POST("/:id/regenerate-certificate", a.regenerateKOSCertificate)
			kos.DELETE("/:id", a.deleteKOSInstance)
			kos.POST("/:id/deactivate", a.deactivateKOSInstance)
			kos.POST("/:id/activate", a.activateKOSInstance)
		}

		// Ingredient management
		ingredients := v1.Group("/ingredients")
		{
			ingredients.GET("", a.listIngredients)
			ingredients.POST("", a.createIngredient)
			ingredients.GET("/:id", a.getIngredient)
			ingredients.PUT("/:id", a.updateIngredient)
			ingredients.DELETE("/:id", a.deleteIngredient)
			ingredients.POST("/:id/toggle-active", a.toggleIngredientActive)
		}

		// Recipe management
		recipes := v1.Group("/recipes")
		{
			recipes.GET("", a.listRecipes)
			recipes.POST("", a.createRecipe)
			recipes.GET("/:id", a.getRecipe)
			recipes.PUT("/:id", a.updateRecipe)
			recipes.DELETE("/:id", a.deleteRecipe)
			recipes.POST("/:id/publish", a.publishRecipe)
			recipes.POST("/:id/unpublish", a.unpublishRecipe)
		}

		// Order management
		orders := v1.Group("/orders")
		{
			orders.GET("", a.listOrders)
			orders.POST("", a.createOrder)
			orders.GET("/:id", a.getOrder)
			orders.PUT("/:id", a.updateOrder)
			orders.POST("/:id/cancel", a.cancelOrder)
		}

		// KOS API endpoints (authenticated via mTLS + JWT)
		kosAPI := v1.Group("/kos")
		{
			// Registration (one-time)
			kosAPI.POST("/register", a.kosRegister)

			// Heartbeat
			kosAPI.POST("/heartbeat", a.kosHeartbeat)

			// Recipe sync (KOS pulls from KWS)
			kosAPI.GET("/recipes", a.kosGetRecipes)
			kosAPI.GET("/ingredients", a.kosGetIngredients)

			// Order sync
			kosAPI.GET("/orders", a.kosGetOrders)
			kosAPI.POST("/orders/:id/status", a.kosUpdateOrderStatus)
		}
	}
}

// Middleware

func (a *Application) loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip logging for health checks
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ready" {
			c.Next()
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		if raw != "" {
			path = path + "?" + raw
		}

		a.logger.Info("HTTP request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

func (a *Application) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Tenant-ID, X-Client-Cert-CN")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
