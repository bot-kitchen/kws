package app

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/ak/kws/internal/app/middleware"
	"github.com/ak/kws/internal/domain/repositories"
	"github.com/ak/kws/web"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WebHandlers handles web UI requests
type WebHandlers struct {
	baseTemplates *template.Template
	pageTemplates map[string]*template.Template
	handlers      *Handlers
	sessionConfig middleware.SessionConfig
}

// templateFuncMap returns the common template functions
func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"dict": func(values ...interface{}) map[string]interface{} {
			if len(values)%2 != 0 {
				return nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil
				}
				dict[key] = values[i+1]
			}
			return dict
		},
		"eq": func(a, b interface{}) bool {
			// Handle nil cases - nil is never equal to a non-nil value
			if a == nil && b == nil {
				return true
			}
			if a == nil || b == nil {
				return false
			}
			return a == b
		},
		"ne": func(a, b interface{}) bool {
			return a != b
		},
		"gt": func(a, b int) bool {
			return a > b
		},
		"lt": func(a, b int) bool {
			return a < b
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"slice": func(s string, start, end int) string {
			if start > len(s) {
				return ""
			}
			if end > len(s) {
				end = len(s)
			}
			return s[start:end]
		},
		"iterate": func(count int) []int {
			result := make([]int, count)
			for i := range result {
				result[i] = i
			}
			return result
		},
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			return t.Format("Jan 02, 15:04")
		},
		"formatDate": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			return t.Format("Jan 02, 2006")
		},
		"relativeTime": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			diff := time.Since(t)
			if diff < time.Minute {
				return "just now"
			}
			if diff < time.Hour {
				return template.HTMLEscapeString(diff.Truncate(time.Minute).String()) + " ago"
			}
			if diff < 24*time.Hour {
				hours := int(diff.Hours())
				return template.HTMLEscapeString(string(rune(hours))) + "h ago"
			}
			days := int(diff.Hours() / 24)
			if days == 1 {
				return "yesterday"
			}
			if days < 7 {
				return template.HTMLEscapeString(string(rune(days))) + "d ago"
			}
			return t.Format("Jan 02")
		},
	}
}

// NewWebHandlers creates a new web handlers instance
func NewWebHandlers(handlers *Handlers, sessionConfig middleware.SessionConfig) (*WebHandlers, error) {
	funcMap := templateFuncMap()
	templatesFS := web.Templates()

	// Parse base/layout templates first
	baseTemplates := template.New("").Funcs(funcMap)

	// Parse layouts
	layoutFiles := []string{"layouts/base.html", "layouts/auth.html", "layouts/sidebar.html", "layouts/header.html"}
	for _, layoutFile := range layoutFiles {
		content, err := fs.ReadFile(templatesFS, layoutFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read layout %s: %w", layoutFile, err)
		}
		_, err = baseTemplates.New(layoutFile).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse layout %s: %w", layoutFile, err)
		}
	}

	// Create page-specific templates by cloning base and adding page content
	pageTemplates := make(map[string]*template.Template)

	err := fs.WalkDir(templatesFS, "pages", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Clone the base templates
		pageTemplate, err := baseTemplates.Clone()
		if err != nil {
			return fmt.Errorf("failed to clone base templates for %s: %w", path, err)
		}

		// Parse the page template into the clone
		content, err := fs.ReadFile(templatesFS, path)
		if err != nil {
			return fmt.Errorf("failed to read page %s: %w", path, err)
		}
		_, err = pageTemplate.New(path).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse page %s: %w", path, err)
		}

		pageTemplates[path] = pageTemplate
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &WebHandlers{
		baseTemplates: baseTemplates,
		pageTemplates: pageTemplates,
		handlers:      handlers,
		sessionConfig: sessionConfig,
	}, nil
}

// RegisterRoutes registers web UI routes
func (w *WebHandlers) RegisterRoutes(r *gin.Engine) {
	// Serve static files
	staticFS := web.Static()
	r.StaticFS("/static", http.FS(staticFS))

	// Public routes (no auth required)
	r.GET("/login", w.Login)
	r.GET("/auth/callback", w.AuthCallback)
	r.GET("/logout", w.Logout)

	// Protected routes (require session)
	protected := r.Group("/")
	protected.Use(middleware.RequireSession(w.sessionConfig))
	{
		protected.GET("/", w.Dashboard)
		protected.GET("/dashboard", w.Dashboard)
		protected.GET("/tenants", w.Tenants)
		protected.GET("/tenants/:id", w.TenantDetail)
		protected.POST("/tenants/:id/select", w.SelectTenant)
		protected.POST("/tenants/clear/select", w.ClearTenantSelection)
		protected.GET("/sites", w.Sites)
		protected.GET("/sites/:id", w.SiteDetail)
		protected.GET("/kos", w.KOSInstances)
		protected.GET("/kos/new", w.KOSNew)
		protected.GET("/kos/:id", w.KOSDetail)
		protected.GET("/recipes", w.Recipes)
		protected.GET("/recipes/new", w.RecipeNew)
		protected.GET("/recipes/:id", w.RecipeDetail)
		protected.GET("/recipes/:id/edit", w.RecipeEdit)
		protected.GET("/ingredients", w.Ingredients)
		protected.GET("/orders", w.Orders)
		protected.GET("/orders/new", w.OrderNew)
		protected.GET("/orders/:id", w.OrderDetail)
		protected.GET("/settings", w.Settings)
		protected.GET("/audit", w.AuditLog)
	}
}

func (w *WebHandlers) render(c *gin.Context, page string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}

	// Add user info to all templates
	if user := middleware.GetUser(c); user != nil {
		data["User"] = user
	}

	// Add tenant context for all pages (for tenant selector dropdown in header)
	ctx := c.Request.Context()
	allTenants, _, _ := w.handlers.repos.Tenant.List(ctx, repositories.TenantFilter{Limit: 100})
	tenantSelectorList := []gin.H{}
	for _, t := range allTenants {
		tenantSelectorList = append(tenantSelectorList, gin.H{
			"ID":   t.ID.Hex(),
			"Code": t.Code,
			"Name": t.Name,
		})
	}
	// Use AllTenants for tenant selector to avoid conflict with page-specific Tenants data
	data["AllTenants"] = tenantSelectorList
	// Only set TenantCount if not already set by the page handler
	if _, exists := data["TenantCount"]; !exists {
		data["TenantCount"] = len(allTenants)
	}

	// Get selected tenant from session
	if selectedID := middleware.GetEffectiveTenantID(c); selectedID != "" {
		data["SelectedTenantID"] = selectedID
		// Find selected tenant name
		for _, t := range tenantSelectorList {
			if t["ID"] == selectedID {
				data["SelectedTenantName"] = t["Name"]
				data["SelectedTenantCode"] = t["Code"]
				break
			}
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	// Get the page-specific template (which includes base layout with page blocks)
	pageTemplate, ok := w.pageTemplates[page]
	if !ok {
		c.String(http.StatusInternalServerError, "Template not found: %s", page)
		return
	}

	// Execute the base layout which will use the page's block definitions
	if err := pageTemplate.ExecuteTemplate(c.Writer, "layouts/base.html", data); err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// renderAuth renders auth pages (login, etc.) using the auth layout
func (w *WebHandlers) renderAuth(c *gin.Context, data gin.H) {
	if data == nil {
		data = gin.H{}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	// Use the login page template which has the auth layout
	pageTemplate, ok := w.pageTemplates["pages/login.html"]
	if !ok {
		c.String(http.StatusInternalServerError, "Login template not found")
		return
	}

	if err := pageTemplate.ExecuteTemplate(c.Writer, "layouts/auth.html", data); err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// Login renders the login page
func (w *WebHandlers) Login(c *gin.Context) {
	// Generate state for OIDC
	state, err := middleware.GenerateState()
	if err != nil {
		w.renderAuth(c, gin.H{
			"Error": "Failed to generate authentication state",
			"Year":  time.Now().Year(),
		})
		return
	}

	// Store state in cookie for verification
	c.SetCookie("oauth_state", state, 300, "/", w.sessionConfig.CookieDomain, w.sessionConfig.CookieSecure, true)

	// Get authorization URL
	authURL := middleware.GetAuthorizationURL(w.sessionConfig, state)

	data := gin.H{
		"AuthURL": authURL,
		"Year":    time.Now().Year(),
	}

	// Check for error message
	if errMsg := c.Query("error"); errMsg != "" {
		data["Error"] = errMsg
	}

	w.renderAuth(c, data)
}

// AuthCallback handles the OIDC callback
func (w *WebHandlers) AuthCallback(c *gin.Context) {
	// Verify state
	state := c.Query("state")
	storedState, err := c.Cookie("oauth_state")
	if err != nil || state != storedState {
		c.Redirect(http.StatusSeeOther, "/login?error=Invalid+authentication+state")
		return
	}

	// Clear state cookie
	c.SetCookie("oauth_state", "", -1, "/", w.sessionConfig.CookieDomain, w.sessionConfig.CookieSecure, true)

	// Check for error from Keycloak
	if errMsg := c.Query("error"); errMsg != "" {
		errDesc := c.Query("error_description")
		c.Redirect(http.StatusSeeOther, "/login?error="+errDesc)
		return
	}

	// Exchange code for tokens
	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusSeeOther, "/login?error=No+authorization+code+received")
		return
	}

	tokenResp, err := middleware.ExchangeCodeForToken(c.Request.Context(), w.sessionConfig, code)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/login?error=Failed+to+exchange+code+for+token")
		return
	}

	// Create session
	if err := middleware.CreateSession(c, w.sessionConfig, tokenResp); err != nil {
		c.Redirect(http.StatusSeeOther, "/login?error=Failed+to+create+session")
		return
	}

	// Redirect to original URL or dashboard
	returnURL, err := c.Cookie("return_url")
	if err != nil || returnURL == "" || returnURL == "/login" {
		returnURL = "/"
	}
	c.SetCookie("return_url", "", -1, "/", w.sessionConfig.CookieDomain, w.sessionConfig.CookieSecure, true)

	c.Redirect(http.StatusSeeOther, returnURL)
}

// Logout handles user logout
func (w *WebHandlers) Logout(c *gin.Context) {
	// Destroy session
	middleware.DestroySession(c, w.sessionConfig)

	// Get Keycloak logout URL
	logoutURL := middleware.GetLogoutURL(w.sessionConfig, w.sessionConfig.RedirectURL)

	// Redirect to Keycloak logout
	c.Redirect(http.StatusSeeOther, logoutURL)
}

// SelectTenant sets the selected tenant context for platform admins
func (w *WebHandlers) SelectTenant(c *gin.Context) {
	tenantID := c.Param("id")

	user := middleware.GetUser(c)
	if user == nil || !user.IsPlatformAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only platform admins can switch tenant context"})
		return
	}

	if err := middleware.SetSelectedTenant(c, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set tenant context"})
		return
	}

	// Redirect to dashboard or referrer
	referer := c.GetHeader("Referer")
	if referer == "" {
		referer = "/"
	}
	c.Redirect(http.StatusSeeOther, referer)
}

// ClearTenantSelection clears the selected tenant (for platform admins)
func (w *WebHandlers) ClearTenantSelection(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil || !user.IsPlatformAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only platform admins can switch tenant context"})
		return
	}

	if err := middleware.SetSelectedTenant(c, ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear tenant context"})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

// Dashboard renders the dashboard page
func (w *WebHandlers) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()

	// Get effective tenant ID for filtering (platform admins may have selected a tenant)
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	// Get counts from repositories
	var tenantCount, siteCount, kosCount, recipeCount, orderCount int64

	if tenantIDStr == "" {
		// Platform admin with no tenant selected - show all tenants
		_, tenantCount, _ = w.handlers.repos.Tenant.List(ctx, repositories.TenantFilter{Limit: 1})
	} else {
		// Parse tenant ID
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			_, siteCount, _ = w.handlers.repos.Site.ListByTenant(ctx, tenantID, 1, 1)
			_, kosCount, _ = w.handlers.repos.KOSInstance.ListByTenant(ctx, tenantID, 1, 1)
			_, recipeCount, _ = w.handlers.repos.Recipe.ListByTenant(ctx, tenantID, "", 1, 1)
			_, orderCount, _ = w.handlers.repos.Order.ListByTenant(ctx, tenantID, nil, "", 1, 1)
		}
	}

	// Get recent KOS instances
	recentKOS := []gin.H{}
	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			instances, _, _ := w.handlers.repos.KOSInstance.ListByTenant(ctx, tenantID, 1, 5)
			for _, kos := range instances {
				lastHeartbeat := "Never"
				if kos.LastHeartbeat != nil && !kos.LastHeartbeat.IsZero() {
					lastHeartbeat = formatRelativeTime(*kos.LastHeartbeat)
				}
				recentKOS = append(recentKOS, gin.H{
					"ID":            kos.ID.Hex(),
					"Name":          kos.Name,
					"SiteName":      "", // TODO: Resolve site name
					"Status":        kos.Status,
					"LastHeartbeat": lastHeartbeat,
				})
			}
		}
	}

	// Get recent orders
	recentOrders := []gin.H{}
	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			orders, _, _ := w.handlers.repos.Order.ListByTenant(ctx, tenantID, nil, "", 1, 5)
			for _, order := range orders {
				recentOrders = append(recentOrders, gin.H{
					"ID":             order.ID.Hex(),
					"OrderReference": order.OrderReference,
					"RecipeName":     order.RecipeName,
					"Status":         order.Status,
					"CreatedAt":      formatRelativeTime(order.CreatedAt),
				})
			}
		}
	}

	data := gin.H{
		"CurrentPage": "dashboard",
		"Stats": gin.H{
			"TenantCount":    int(tenantCount),
			"ActiveTenants":  int(tenantCount), // TODO: Filter by active status
			"SiteCount":      int(siteCount),
			"ActiveSites":    int(siteCount), // TODO: Filter by active status
			"KOSCount":       int(kosCount),
			"OnlineKOS":      int(kosCount), // TODO: Filter by online status
			"OfflineKOS":     0,             // TODO: Count offline KOS
			"RecipeCount":    int(recipeCount),
			"OrderCount":     int(orderCount),
			"OrdersToday":    0, // TODO: Count today's orders
			"CompletedToday": 0, // TODO: Count completed today
			"PendingOrders":  0, // TODO: Count pending orders
		},
		"RecentKOS":    recentKOS,
		"RecentOrders": recentOrders,
		"Alerts":       []gin.H{},
	}
	w.render(c, "pages/dashboard/index.html", data)
}

// formatRelativeTime formats a time as relative (e.g., "5 min ago")
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	diff := time.Since(t)
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%d min ago", mins)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	}
	days := int(diff.Hours() / 24)
	if days == 1 {
		return "yesterday"
	}
	if days < 7 {
		return fmt.Sprintf("%dd ago", days)
	}
	return t.Format("Jan 02")
}

// Tenants renders the tenants page
func (w *WebHandlers) Tenants(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit := 20
	if page < 1 {
		page = 1
	}

	// Get filter
	status := c.Query("status")

	// Fetch tenants from database
	tenants, total, err := w.handlers.repos.Tenant.List(ctx, repositories.TenantFilter{
		Status: status,
		Page:   page,
		Limit:  limit,
	})

	tenantData := []gin.H{}
	if err == nil {
		for _, t := range tenants {
			// Count sites for each tenant
			_, siteCount, _ := w.handlers.repos.Site.ListByTenant(ctx, t.ID, 1, 1)
			tenantData = append(tenantData, gin.H{
				"ID":        t.ID.Hex(),
				"Name":      t.Name,
				"Code":      t.Code,
				"Plan":      t.Plan,
				"Status":    t.Status,
				"SiteCount": siteCount,
				"CreatedAt": t.CreatedAt.Format("Jan 02, 2006"),
			})
		}
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	// Generate page numbers for pagination
	pages := []int{}
	for i := 1; i <= totalPages; i++ {
		pages = append(pages, i)
	}

	data := gin.H{
		"CurrentPage": "tenants",
		"Tenants":     tenantData,
		"Page":        page,
		"TotalPages":  totalPages,
		"Total":       total,
		"Pages":       pages,
		"StartIndex":  (page-1)*limit + 1,
		"EndIndex":    min((page-1)*limit+len(tenantData), int(total)),
	}
	w.render(c, "pages/tenants/list.html", data)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TenantDetail renders the tenant detail page
func (w *WebHandlers) TenantDetail(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")

	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/tenants")
		return
	}

	tenant, err := w.handlers.repos.Tenant.GetByID(ctx, id)
	if err != nil || tenant == nil {
		c.Redirect(http.StatusSeeOther, "/tenants")
		return
	}

	// Get site count
	_, siteCount, _ := w.handlers.repos.Site.ListByTenant(ctx, id, 1, 1)

	// Get region count
	_, regionCount, _ := w.handlers.repos.Region.ListByTenant(ctx, id, 1, 1)

	// Get KOS count
	_, kosCount, _ := w.handlers.repos.KOSInstance.ListByTenant(ctx, id, 1, 1)

	data := gin.H{
		"CurrentPage": "tenants",
		"Tenant": gin.H{
			"ID":                tenant.ID.Hex(),
			"Name":              tenant.Name,
			"Code":              tenant.Code,
			"Plan":              tenant.Plan,
			"Status":            tenant.Status,
			"ContactEmail":      tenant.ContactEmail,
			"ContactPhone":      tenant.ContactPhone,
			"KeycloakRealmName": tenant.KeycloakRealmName,
			"SiteCount":         siteCount,
			"RegionCount":       regionCount,
			"KOSCount":          kosCount,
			"CreatedAt":         tenant.CreatedAt.Format("Jan 02, 2006"),
		},
	}
	w.render(c, "pages/tenants/view.html", data)
}

// Sites renders the sites page
func (w *WebHandlers) Sites(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "sites",
	}
	w.render(c, "pages/sites/list.html", data)
}

// SiteDetail renders the site detail page
func (w *WebHandlers) SiteDetail(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "sites",
	}
	w.render(c, "pages/sites/view.html", data)
}

// KOSInstances renders the KOS instances page
func (w *WebHandlers) KOSInstances(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	kosData := []gin.H{}
	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			instances, _, _ := w.handlers.repos.KOSInstance.ListByTenant(ctx, tenantID, 1, 50)
			for _, kos := range instances {
				lastHeartbeat := "Never"
				if kos.LastHeartbeat != nil && !kos.LastHeartbeat.IsZero() {
					lastHeartbeat = formatRelativeTime(*kos.LastHeartbeat)
				}

				// Get site name
				siteName := ""
				if site, _ := w.handlers.repos.Site.GetByID(ctx, kos.SiteID); site != nil {
					siteName = site.Name
				}

				kosData = append(kosData, gin.H{
					"ID":            kos.ID.Hex(),
					"Name":          kos.Name,
					"SiteName":      siteName,
					"Status":        kos.Status,
					"Version":       kos.Version,
					"LastHeartbeat": lastHeartbeat,
					"Kitchens":      kos.Kitchens,
				})
			}
		}
	}

	data := gin.H{
		"CurrentPage":  "kos",
		"KOSInstances": kosData,
	}
	w.render(c, "pages/kos/list.html", data)
}

// KOSNew renders the new KOS form
func (w *WebHandlers) KOSNew(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "kos",
	}
	w.render(c, "pages/kos/form.html", data)
}

// KOSDetail renders the KOS detail page
func (w *WebHandlers) KOSDetail(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "kos",
	}
	w.render(c, "pages/kos/view.html", data)
}

// Recipes renders the recipes page
func (w *WebHandlers) Recipes(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	recipeData := []gin.H{}
	var publishedCount, draftCount int

	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			status := c.Query("status")
			recipes, _, _ := w.handlers.repos.Recipe.ListByTenant(ctx, tenantID, status, 1, 50)
			for _, r := range recipes {
				// Count by status
				switch r.Status {
				case "published":
					publishedCount++
				case "draft":
					draftCount++
				}

				recipeData = append(recipeData, gin.H{
					"ID":          r.ID.Hex(),
					"Name":        r.Name,
					"Category":    r.Category,
					"Status":      r.Status,
					"Version":     r.Version,
					"Description": r.Description,
					"PrepTime":    r.PrepTime,
					"CookTime":    r.CookTime,
					"TotalTime":   r.PrepTime + r.CookTime,
				})
			}
		}
	}

	data := gin.H{
		"CurrentPage":    "recipes",
		"Recipes":        recipeData,
		"PublishedCount": publishedCount,
		"DraftCount":     draftCount,
	}
	w.render(c, "pages/recipes/list.html", data)
}

// RecipeNew renders the new recipe form
func (w *WebHandlers) RecipeNew(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "recipes",
	}
	w.render(c, "pages/recipes/form.html", data)
}

// RecipeDetail renders the recipe detail page
func (w *WebHandlers) RecipeDetail(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "recipes",
	}
	w.render(c, "pages/recipes/view.html", data)
}

// RecipeEdit renders the recipe edit form
func (w *WebHandlers) RecipeEdit(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "recipes",
	}
	w.render(c, "pages/recipes/edit.html", data)
}

// Ingredients renders the ingredients page
func (w *WebHandlers) Ingredients(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	ingredientData := []gin.H{}
	var dryCount, wetCount, liquidCount int

	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			ingredients, _, _ := w.handlers.repos.Ingredient.ListByTenant(ctx, tenantID, false, 1, 100)
			for _, i := range ingredients {
				ingredientData = append(ingredientData, gin.H{
					"ID":           i.ID.Hex(),
					"Name":         i.Name,
					"MoistureType": i.MoistureType,
					"AllergenInfo": i.AllergenInfo,
					"IsActive":     i.IsActive,
				})

				// Count by moisture type
				switch i.MoistureType {
				case "dry":
					dryCount++
				case "wet":
					wetCount++
				case "liquid":
					liquidCount++
				}
			}
		}
	}

	data := gin.H{
		"CurrentPage": "ingredients",
		"Ingredients": ingredientData,
		"DryCount":    dryCount,
		"WetCount":    wetCount,
		"LiquidCount": liquidCount,
	}
	w.render(c, "pages/ingredients/list.html", data)
}

// Orders renders the orders page
func (w *WebHandlers) Orders(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	orderData := []gin.H{}
	sites := []gin.H{}
	var pendingCount, sentToKOSCount, inProgressCount, completedCount, failedCount int

	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			status := c.Query("status")
			orders, _, _ := w.handlers.repos.Order.ListByTenant(ctx, tenantID, nil, status, 1, 100)
			for _, o := range orders {
				// Count by status
				switch o.Status {
				case "pending":
					pendingCount++
				case "sent_to_kos":
					sentToKOSCount++
				case "in_progress":
					inProgressCount++
				case "completed":
					completedCount++
				case "failed":
					failedCount++
				}

				// Get site name
				siteName := ""
				if site, _ := w.handlers.repos.Site.GetByID(ctx, o.SiteID); site != nil {
					siteName = site.Name
				}

				orderData = append(orderData, gin.H{
					"ID":             o.ID.Hex(),
					"OrderReference": o.OrderReference,
					"RecipeName":     o.RecipeName,
					"SiteID":         o.SiteID.Hex(),
					"SiteName":       siteName,
					"CustomerName":   o.CustomerName,
					"Status":         o.Status,
					"Priority":       o.Priority,
					"CreatedAt":      formatRelativeTime(o.CreatedAt),
					"CreatedAtUnix":  o.CreatedAt.Unix(),
				})
			}

			// Get sites for filter dropdown
			siteList, _, _ := w.handlers.repos.Site.ListByTenant(ctx, tenantID, 1, 100)
			for _, s := range siteList {
				sites = append(sites, gin.H{
					"ID":   s.ID.Hex(),
					"Name": s.Name,
				})
			}
		}
	}

	totalCount := pendingCount + sentToKOSCount + inProgressCount + completedCount + failedCount

	data := gin.H{
		"CurrentPage": "orders",
		"Orders":      orderData,
		"Sites":       sites,
		"Stats": gin.H{
			"Total":      totalCount,
			"Pending":    pendingCount,
			"SentToKOS":  sentToKOSCount,
			"InProgress": inProgressCount,
			"Completed":  completedCount,
			"Failed":     failedCount,
		},
	}
	w.render(c, "pages/orders/list.html", data)
}

// OrderNew renders the new order form
func (w *WebHandlers) OrderNew(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "orders",
	}
	w.render(c, "pages/orders/form.html", data)
}

// OrderDetail renders the order detail page
func (w *WebHandlers) OrderDetail(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "orders",
	}
	w.render(c, "pages/orders/view.html", data)
}

// Settings renders the settings page
func (w *WebHandlers) Settings(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "settings",
	}
	w.render(c, "pages/settings/index.html", data)
}

// AuditLog renders the audit log page
func (w *WebHandlers) AuditLog(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "audit",
	}
	w.render(c, "pages/audit/index.html", data)
}
