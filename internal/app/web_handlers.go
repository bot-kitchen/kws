package app

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ak/kws/internal/app/middleware"
	"github.com/ak/kws/internal/domain/models"
	"github.com/ak/kws/internal/domain/repositories"
	"github.com/ak/kws/web"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Template cache (matching KOS pattern)
var templates *template.Template

// WebHandlers handles web UI requests
type WebHandlers struct {
	handlers      *Handlers
	sessionConfig middleware.SessionConfig
}

// templateFuncMap returns the common template functions (matching KOS)
func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"divFloat": func(a, b interface{}) float64 {
			aVal, _ := strconv.ParseFloat(fmt.Sprint(a), 64)
			bVal, _ := strconv.ParseFloat(fmt.Sprint(b), 64)
			if bVal == 0 {
				return 0
			}
			return aVal / bVal
		},
		"add": func(a, b interface{}) int {
			aVal, _ := strconv.Atoi(fmt.Sprint(a))
			bVal, _ := strconv.Atoi(fmt.Sprint(b))
			return aVal + bVal
		},
		"json": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("[]")
			}
			return template.JS(b)
		},
		"replace": func(old, new, s string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"title": func(s string) string {
			return strings.Title(s)
		},
		"split": func(s, sep string) []string {
			if s == "" {
				return []string{}
			}
			return strings.Split(s, sep)
		},
		"formatTimeAgo": func(t *time.Time) string {
			if t == nil {
				return "N/A"
			}
			duration := time.Since(*t)
			if duration < time.Minute {
				return fmt.Sprintf("%ds", int(duration.Seconds()))
			} else if duration < time.Hour {
				return fmt.Sprintf("%dm", int(duration.Minutes()))
			} else if duration < 24*time.Hour {
				return fmt.Sprintf("%dh", int(duration.Hours()))
			}
			return fmt.Sprintf("%dd", int(duration.Hours()/24))
		},
		"deref": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
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
		},
	}
}

// initTemplates initializes the template cache (matching KOS pattern)
func initTemplates() error {
	funcMap := templateFuncMap()
	templatesFS := web.Templates()

	// Create base template with custom functions
	tmpl := template.New("").Funcs(funcMap)

	// Collect all template files
	var templateFiles []string
	err := fs.WalkDir(templatesFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".html") {
			templateFiles = append(templateFiles, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Sort files to ensure layouts are parsed first
	sort.Slice(templateFiles, func(i, j int) bool {
		// Layouts come first
		iLayout := strings.HasPrefix(templateFiles[i], "layouts/")
		jLayout := strings.HasPrefix(templateFiles[j], "layouts/")
		if iLayout && !jLayout {
			return true
		}
		if !iLayout && jLayout {
			return false
		}
		return templateFiles[i] < templateFiles[j]
	})

	// Parse all template files
	for _, path := range templateFiles {
		content, readErr := fs.ReadFile(templatesFS, path)
		if readErr != nil {
			return fmt.Errorf("error reading template %s: %w", path, readErr)
		}
		_, parseErr := tmpl.Parse(string(content))
		if parseErr != nil {
			return fmt.Errorf("error parsing template %s: %w", path, parseErr)
		}
	}

	templates = tmpl
	return nil
}

// NewWebHandlers creates a new web handlers instance
func NewWebHandlers(handlers *Handlers, sessionConfig middleware.SessionConfig) (*WebHandlers, error) {
	// Initialize templates if not already done
	if templates == nil {
		if err := initTemplates(); err != nil {
			return nil, fmt.Errorf("failed to initialize templates: %w", err)
		}
	}

	return &WebHandlers{
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
		protected.GET("/sites/new", w.SiteNew)
		protected.GET("/sites/:id", w.SiteDetail)
		protected.GET("/sites/:id/edit", w.SiteEdit)
		protected.GET("/kos", w.KOSInstances)
		protected.GET("/kos/new", w.KOSNew)
		protected.GET("/kos/:id", w.KOSDetail)
		protected.GET("/recipes", w.Recipes)
		protected.GET("/recipes/new", w.RecipeNew)
		protected.GET("/recipes/:id", w.RecipeDetail)
		protected.GET("/recipes/:id/edit", w.RecipeEdit)
		protected.GET("/ingredients", w.Ingredients)
		protected.GET("/ingredients/new", w.IngredientNew)
		protected.GET("/ingredients/:id/edit", w.IngredientEdit)
		protected.GET("/orders", w.Orders)
		protected.GET("/orders/new", w.OrderNew)
		protected.GET("/orders/:id", w.OrderDetail)
		protected.GET("/settings", w.Settings)
		protected.GET("/audit", w.AuditLog)
	}
}

// renderTemplate renders a named template with the given data (matching KOS pattern)
func (w *WebHandlers) renderTemplate(c *gin.Context, tmplName string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}

	// Add common data available to all templates
	data["AppName"] = "KWS"

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
	data["AllTenants"] = tenantSelectorList
	if _, exists := data["TenantCount"]; !exists {
		data["TenantCount"] = len(allTenants)
	}

	// Get selected tenant from session
	if selectedID := middleware.GetEffectiveTenantID(c); selectedID != "" {
		data["SelectedTenantID"] = selectedID
		for _, t := range tenantSelectorList {
			if t["ID"] == selectedID {
				data["SelectedTenantName"] = t["Name"]
				data["SelectedTenantCode"] = t["Code"]
				break
			}
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	// Check if the page template exists
	pageTemplate := templates.Lookup(tmplName)
	if pageTemplate == nil {
		c.String(http.StatusInternalServerError, "Template not found: %s", tmplName)
		return
	}

	// Create a wrapper that defines "content" as the page template (KOS pattern)
	funcMap := templateFuncMap()
	contentWrapper := template.Must(template.New("content").Funcs(funcMap).Parse(`{{template "` + tmplName + `" .}}`))

	// Add all templates to the wrapper
	for _, t := range templates.Templates() {
		if t.Name() != "content" {
			contentWrapper.AddParseTree(t.Name(), t.Tree)
		}
	}

	// Execute the base template with the wrapper
	if err := contentWrapper.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// renderAuth renders auth pages (login, etc.) using the auth layout
func (w *WebHandlers) renderAuth(c *gin.Context, data gin.H) {
	if data == nil {
		data = gin.H{}
	}

	data["AppName"] = "KWS"

	c.Header("Content-Type", "text/html; charset=utf-8")

	// Check if the auth-login template exists
	if templates.Lookup("auth-login") == nil {
		c.String(http.StatusInternalServerError, "Login template not found")
		return
	}

	// Create wrapper for auth layout
	funcMap := templateFuncMap()
	contentWrapper := template.Must(template.New("content").Funcs(funcMap).Parse(`{{template "auth-login" .}}`))

	for _, t := range templates.Templates() {
		if t.Name() != "content" {
			contentWrapper.AddParseTree(t.Name(), t.Tree)
		}
	}

	if err := contentWrapper.ExecuteTemplate(c.Writer, "auth", data); err != nil {
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
					"Status":        string(kos.Status),
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
					"Status":         string(order.Status),
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
	w.renderTemplate(c, "dashboard", data)
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
	w.renderTemplate(c, "tenants-list", data)
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
	w.renderTemplate(c, "tenants-view", data)
}

// Sites renders the sites page with region tag management
func (w *WebHandlers) Sites(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	if tenantIDStr == "" {
		c.Redirect(http.StatusFound, "/tenants")
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		c.Redirect(http.StatusFound, "/tenants")
		return
	}

	// Load regions for this tenant (for tag pills)
	regionData := []gin.H{}
	regions, _, _ := w.handlers.repos.Region.ListByTenant(ctx, tenantID, 1, 100)
	regionMap := make(map[string]string) // ID -> Name
	for _, r := range regions {
		regionMap[r.ID.Hex()] = r.Name
		// Count sites per region
		siteCount, _, _ := w.handlers.repos.Site.ListByRegion(ctx, r.ID, 1, 1)
		regionData = append(regionData, gin.H{
			"ID":        r.ID.Hex(),
			"Name":      r.Name,
			"Code":      r.Code,
			"Timezone":  r.Timezone,
			"SiteCount": len(siteCount),
		})
	}

	// Load sites for this tenant
	siteData := []gin.H{}
	sites, total, _ := w.handlers.repos.Site.ListByTenant(ctx, tenantID, 1, 100)

	var withKOS, withoutKOS int
	for _, s := range sites {
		// Check if site has KOS instance
		kos, _ := w.handlers.repos.KOSInstance.GetBySiteID(ctx, s.ID)
		hasKOS := kos != nil
		var kosStatus string
		if kos != nil {
			kosStatus = string(kos.Status)
			withKOS++
		} else {
			withoutKOS++
		}

		// Get region name
		regionName := regionMap[s.RegionID.Hex()]

		siteItem := gin.H{
			"ID":         s.ID.Hex(),
			"Name":       s.Name,
			"Code":       s.Code,
			"RegionID":   s.RegionID.Hex(),
			"RegionName": regionName,
			"Timezone":   s.Timezone,
			"Status":     s.Status,
			"HasKOS":     hasKOS,
			"KOSStatus":  kosStatus,
		}
		if s.Address != nil {
			siteItem["Address"] = gin.H{
				"Street":     s.Address.Street,
				"City":       s.Address.City,
				"State":      s.Address.State,
				"PostalCode": s.Address.PostalCode,
				"Country":    s.Address.Country,
			}
			siteItem["AddressLine"] = s.Address.City + ", " + s.Address.State
		}
		siteData = append(siteData, siteItem)
	}

	data := gin.H{
		"CurrentPage": "sites",
		"TenantID":    tenantIDStr,
		"Regions":     regionData,
		"Sites":       siteData,
		"Total":       total,
		"WithKOS":     withKOS,
		"WithoutKOS":  withoutKOS,
		"HasRegions":  len(regions) > 0,
		"RegionCount": len(regions),
	}
	w.renderTemplate(c, "sites-list", data)
}

// SiteNew renders the new site form
func (w *WebHandlers) SiteNew(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	if tenantIDStr == "" {
		c.Redirect(http.StatusFound, "/tenants")
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		c.Redirect(http.StatusFound, "/tenants")
		return
	}

	// Load regions for dropdown
	regionData := []gin.H{}
	regions, _, _ := w.handlers.repos.Region.ListByTenant(ctx, tenantID, 1, 100)
	for _, r := range regions {
		regionData = append(regionData, gin.H{
			"ID":   r.ID.Hex(),
			"Name": r.Name,
			"Code": r.Code,
		})
	}

	data := gin.H{
		"CurrentPage": "sites",
		"TenantID":    tenantIDStr,
		"Regions":     regionData,
		"Site":        gin.H{},
		"IsNew":       true,
	}
	w.renderTemplate(c, "sites-form", data)
}

// SiteEdit renders the site edit form
func (w *WebHandlers) SiteEdit(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)
	siteIDStr := c.Param("id")

	if tenantIDStr == "" {
		c.Redirect(http.StatusFound, "/tenants")
		return
	}

	tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		c.Redirect(http.StatusFound, "/tenants")
		return
	}

	siteID, err := primitive.ObjectIDFromHex(siteIDStr)
	if err != nil {
		c.Redirect(http.StatusFound, "/sites")
		return
	}

	site, err := w.handlers.repos.Site.GetByID(ctx, siteID)
	if err != nil || site == nil {
		c.Redirect(http.StatusFound, "/sites")
		return
	}

	// Load regions for dropdown
	regionData := []gin.H{}
	regions, _, _ := w.handlers.repos.Region.ListByTenant(ctx, tenantID, 1, 100)
	for _, r := range regions {
		regionData = append(regionData, gin.H{
			"ID":   r.ID.Hex(),
			"Name": r.Name,
			"Code": r.Code,
		})
	}

	siteItem := gin.H{
		"ID":       site.ID.Hex(),
		"Name":     site.Name,
		"Code":     site.Code,
		"RegionID": site.RegionID.Hex(),
		"Timezone": site.Timezone,
	}
	if site.Address != nil {
		siteItem["Street"] = site.Address.Street
		siteItem["City"] = site.Address.City
		siteItem["State"] = site.Address.State
		siteItem["PostalCode"] = site.Address.PostalCode
		siteItem["Country"] = site.Address.Country
	}

	data := gin.H{
		"CurrentPage": "sites",
		"TenantID":    tenantIDStr,
		"Regions":     regionData,
		"Site":        siteItem,
		"IsNew":       false,
	}
	w.renderTemplate(c, "sites-form", data)
}

// SiteDetail renders the site detail page
func (w *WebHandlers) SiteDetail(c *gin.Context) {
	ctx := c.Request.Context()
	siteIDStr := c.Param("id")

	siteID, err := primitive.ObjectIDFromHex(siteIDStr)
	if err != nil {
		c.Redirect(http.StatusFound, "/sites")
		return
	}

	site, err := w.handlers.repos.Site.GetByID(ctx, siteID)
	if err != nil || site == nil {
		c.Redirect(http.StatusFound, "/sites")
		return
	}

	// Get region info
	region, _ := w.handlers.repos.Region.GetByID(ctx, site.RegionID)
	var regionName string
	if region != nil {
		regionName = region.Name
	}

	// Get KOS instance if any
	kos, _ := w.handlers.repos.KOSInstance.GetBySiteID(ctx, site.ID)
	var kosData gin.H
	if kos != nil {
		kosData = gin.H{
			"ID":                kos.ID.Hex(),
			"Name":              kos.Name,
			"Status":            string(kos.Status),
			"Version":           kos.Version,
			"LastHeartbeat":     kos.LastHeartbeat,
			"CertificateExpiry": kos.CertificateExpiry,
		}
	}

	siteItem := gin.H{
		"ID":         site.ID.Hex(),
		"Name":       site.Name,
		"Code":       site.Code,
		"RegionID":   site.RegionID.Hex(),
		"RegionName": regionName,
		"Timezone":   site.Timezone,
		"Status":     site.Status,
		"CreatedAt":  site.CreatedAt.Format("Jan 02, 2006"),
	}
	if site.Address != nil {
		siteItem["Address"] = gin.H{
			"Street":     site.Address.Street,
			"City":       site.Address.City,
			"State":      site.Address.State,
			"PostalCode": site.Address.PostalCode,
			"Country":    site.Address.Country,
		}
	}

	data := gin.H{
		"CurrentPage": "sites",
		"Site":        siteItem,
		"KOS":         kosData,
		"HasKOS":      kos != nil,
	}
	w.renderTemplate(c, "sites-view", data)
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
					"Status":        string(kos.Status),
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
	w.renderTemplate(c, "kos-list", data)
}

// KOSNew renders the new KOS form
func (w *WebHandlers) KOSNew(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	// Get pre-selected site if passed as query param
	preselectedSiteID := c.Query("site_id")

	// Load sites without KOS instances (available for assignment)
	availableSites := []gin.H{}
	if tenantIDStr != "" && tenantIDStr != "platform" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			// Get all sites for the tenant
			sites, _, _ := w.handlers.repos.Site.ListByTenant(ctx, tenantID, 1, 100)
			for _, site := range sites {
				// Check if site already has a KOS instance
				existingKOS, _ := w.handlers.repos.KOSInstance.GetBySiteID(ctx, site.ID)
				if existingKOS == nil {
					// Get region name
					regionName := ""
					if region, _ := w.handlers.repos.Region.GetByID(ctx, site.RegionID); region != nil {
						regionName = region.Name
					}

					availableSites = append(availableSites, gin.H{
						"ID":         site.ID.Hex(),
						"Name":       site.Name,
						"Code":       site.Code,
						"RegionID":   site.RegionID.Hex(),
						"RegionName": regionName,
					})
				}
			}
		}
	}

	data := gin.H{
		"CurrentPage":       "kos",
		"TenantID":          tenantIDStr,
		"AvailableSites":    availableSites,
		"PreselectedSiteID": preselectedSiteID,
		"KOS":               gin.H{},
		"IsNew":             true,
	}
	w.renderTemplate(c, "kos-form", data)
}

// KOSDetail renders the KOS detail page
func (w *WebHandlers) KOSDetail(c *gin.Context) {
	ctx := c.Request.Context()
	kosIDStr := c.Param("id")

	kosID, err := primitive.ObjectIDFromHex(kosIDStr)
	if err != nil {
		c.Redirect(http.StatusFound, "/kos")
		return
	}

	kos, err := w.handlers.repos.KOSInstance.GetByID(ctx, kosID)
	if err != nil || kos == nil {
		c.Redirect(http.StatusFound, "/kos")
		return
	}

	// Get site info
	var siteName, siteCode string
	if site, _ := w.handlers.repos.Site.GetByID(ctx, kos.SiteID); site != nil {
		siteName = site.Name
		siteCode = site.Code
	}

	// Get region info
	var regionName string
	if region, _ := w.handlers.repos.Region.GetByID(ctx, kos.RegionID); region != nil {
		regionName = region.Name
	}

	// Format last heartbeat
	lastHeartbeat := "Never"
	if kos.LastHeartbeat != nil && !kos.LastHeartbeat.IsZero() {
		lastHeartbeat = formatRelativeTime(*kos.LastHeartbeat)
	}

	// Format certificate expiry
	certExpiry := ""
	certExpiryDays := 0
	if !kos.CertificateExpiry.IsZero() {
		certExpiry = kos.CertificateExpiry.Format("Jan 02, 2006")
		certExpiryDays = int(time.Until(kos.CertificateExpiry).Hours() / 24)
	}

	// Format registered at
	registeredAt := ""
	if kos.RegisteredAt != nil && !kos.RegisteredAt.IsZero() {
		registeredAt = kos.RegisteredAt.Format("Jan 02, 2006 15:04")
	}

	kosItem := gin.H{
		"ID":                kos.ID.Hex(),
		"Name":              kos.Name,
		"SiteID":            kos.SiteID.Hex(),
		"SiteName":          siteName,
		"SiteCode":          siteCode,
		"RegionName":        regionName,
		"Status":            string(kos.Status),
		"Version":           kos.Version,
		"LastHeartbeat":     lastHeartbeat,
		"CertificateSerial": kos.CertificateSerial,
		"CertificateExpiry": certExpiry,
		"CertExpiryDays":    certExpiryDays,
		"RegisteredAt":      registeredAt,
		"HasCertificate":    kos.CertificatePEM != "",
		"CreatedAt":         kos.CreatedAt.Format("Jan 02, 2006"),
	}

	data := gin.H{
		"CurrentPage": "kos",
		"KOS":         kosItem,
	}
	w.renderTemplate(c, "kos-view", data)
}

// Recipes renders the recipes page
func (w *WebHandlers) Recipes(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	recipeData := []gin.H{}
	siteData := []gin.H{}
	var publishedCount, draftCount int

	// Only load recipes if we have a valid tenant ObjectID (not "platform")
	if tenantIDStr != "" && tenantIDStr != "platform" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			status := c.Query("status")
			recipes, _, _ := w.handlers.repos.Recipe.ListByTenant(ctx, tenantID, status, 1, 50)
			for _, r := range recipes {
				// Count by status
				switch r.Status {
				case models.RecipeStatusPublished:
					publishedCount++
				case models.RecipeStatusDraft:
					draftCount++
				}

				recipeData = append(recipeData, gin.H{
					"ID":          r.ID.Hex(),
					"Name":        r.Name,
					"Category":    r.Category,
					"Status":      string(r.Status), // Convert to string for template comparison
					"Version":     r.Version,
					"Description": r.Description,
					"PrepTime":    r.PrepTime,
					"CookTime":    r.CookTime,
					"TotalTime":   r.PrepTime + r.CookTime,
				})
			}

			// Load sites for the Cook modal
			sites, _, _ := w.handlers.repos.Site.ListByTenant(ctx, tenantID, 1, 100)
			for _, s := range sites {
				siteData = append(siteData, gin.H{
					"ID":       s.ID.Hex(),
					"Name":     s.Name,
					"RegionID": s.RegionID.Hex(),
				})
			}
		}
	}

	data := gin.H{
		"CurrentPage":    "recipes",
		"Recipes":        recipeData,
		"Sites":          siteData,
		"TenantID":       tenantIDStr,
		"PublishedCount": publishedCount,
		"DraftCount":     draftCount,
	}
	w.renderTemplate(c, "recipes-list", data)
}

// RecipeNew renders the new recipe form
func (w *WebHandlers) RecipeNew(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	// Get ingredients for selector (use snake_case for JS compatibility)
	ingredientData := []gin.H{}

	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			ingredients, _, _ := w.handlers.repos.Ingredient.ListByTenant(ctx, tenantID, true, 1, 500) // Only active ingredients
			for _, i := range ingredients {
				ingredientData = append(ingredientData, gin.H{
					"id":            i.ID.Hex(),
					"name":          i.Name,
					"moisture_type": i.MoistureType,
				})
			}
		}
	}

	data := gin.H{
		"CurrentPage": "recipes",
		"IsNew":       true,
		"Recipe": gin.H{
			"Name":                    "",
			"EstimatedPrepTimeSec":    0,
			"EstimatedCookingTimeSec": 0,
		},
		"Steps":       []gin.H{},
		"Ingredients": ingredientData,
	}
	w.renderTemplate(c, "recipes-form", data)
}

// RecipeDetail renders the recipe detail page
func (w *WebHandlers) RecipeDetail(c *gin.Context) {
	ctx := c.Request.Context()
	recipeID := c.Param("id")

	recipeOID, err := primitive.ObjectIDFromHex(recipeID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid recipe ID")
		return
	}

	recipe, err := w.handlers.repos.Recipe.GetByID(ctx, recipeOID)
	if err != nil || recipe == nil {
		c.String(http.StatusNotFound, "Recipe not found")
		return
	}

	// Get recipe steps from embedded array
	stepData := []gin.H{}
	for _, s := range recipe.Steps {
		stepData = append(stepData, gin.H{
			"StepNumber":     s.StepNumber,
			"Action":         s.Action,
			"Parameters":     s.Parameters,
			"DependsOnSteps": s.DependsOnSteps,
			"Name":           s.Name,
			"Description":    s.Description,
		})
	}

	data := gin.H{
		"CurrentPage": "recipes",
		"Recipe": gin.H{
			"ID":                      recipe.ID.Hex(),
			"Name":                    recipe.Name,
			"Status":                  recipe.Status,
			"IsActive":                recipe.Status == "published",
			"EstimatedPrepTimeSec":    recipe.EstimatedPrepTimeSec,
			"EstimatedCookingTimeSec": recipe.EstimatedCookingTimeSec,
			"RecipeSteps":             stepData,
		},
		"Steps": stepData,
	}
	w.renderTemplate(c, "recipes-view", data)
}

// RecipeEdit renders the recipe edit form
func (w *WebHandlers) RecipeEdit(c *gin.Context) {
	ctx := c.Request.Context()
	recipeID := c.Param("id")
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	recipeOID, err := primitive.ObjectIDFromHex(recipeID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid recipe ID")
		return
	}

	recipe, err := w.handlers.repos.Recipe.GetByID(ctx, recipeOID)
	if err != nil || recipe == nil {
		c.String(http.StatusNotFound, "Recipe not found")
		return
	}

	// Get recipe steps from embedded array
	stepData := []gin.H{}
	for _, s := range recipe.Steps {
		stepData = append(stepData, gin.H{
			"StepNumber":     s.StepNumber,
			"Action":         s.Action,
			"Parameters":     s.Parameters,
			"DependsOnSteps": s.DependsOnSteps,
			"Name":           s.Name,
			"Description":    s.Description,
		})
	}

	// Get ingredients for selector (use snake_case for JS compatibility)
	ingredientData := []gin.H{}
	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			ingredients, _, _ := w.handlers.repos.Ingredient.ListByTenant(ctx, tenantID, true, 1, 500) // Only active ingredients
			for _, i := range ingredients {
				ingredientData = append(ingredientData, gin.H{
					"id":            i.ID.Hex(),
					"name":          i.Name,
					"moisture_type": i.MoistureType,
				})
			}
		}
	}

	data := gin.H{
		"CurrentPage": "recipes",
		"IsNew":       false,
		"Recipe": gin.H{
			"ID":                      recipe.ID.Hex(),
			"Name":                    recipe.Name,
			"Status":                  recipe.Status,
			"EstimatedPrepTimeSec":    recipe.EstimatedPrepTimeSec,
			"EstimatedCookingTimeSec": recipe.EstimatedCookingTimeSec,
			"RecipeSteps":             stepData,
		},
		"Steps":       stepData,
		"Ingredients": ingredientData,
	}
	w.renderTemplate(c, "recipes-form", data)
}

// Ingredients renders the ingredients page
func (w *WebHandlers) Ingredients(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	ingredientData := []gin.H{}
	var dryCount, wetCount, liquidCount, activeCount, inactiveCount int

	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			// Get all ingredients (including inactive)
			ingredients, _, _ := w.handlers.repos.Ingredient.ListByTenant(ctx, tenantID, false, 1, 100)
			for _, i := range ingredients {
				ingredientItem := gin.H{
					"ID":               i.ID.Hex(),
					"Name":             i.Name,
					"MoistureType":     i.MoistureType,
					"AllergenInfo":     strings.Join(i.AllergenInfo, ", "),
					"IsActive":         i.IsActive,
					"ShelfLifeMinutes": i.ShelfLifeMinutes,
				}

				// Add nutrition fields with defaults
				if i.Nutrition != nil {
					ingredientItem["CaloriesPer100g"] = i.Nutrition.CaloriesPer100g
					ingredientItem["ProteinPer100g"] = i.Nutrition.ProteinPer100g
					ingredientItem["FatPer100g"] = i.Nutrition.FatPer100g
					ingredientItem["CarbsPer100g"] = i.Nutrition.CarbsPer100g
					ingredientItem["SugarPer100g"] = i.Nutrition.SugarPer100g
					ingredientItem["FiberPer100g"] = i.Nutrition.FiberPer100g
					ingredientItem["SodiumPer100g"] = i.Nutrition.SodiumPer100g
				} else {
					ingredientItem["CaloriesPer100g"] = 0.0
					ingredientItem["ProteinPer100g"] = 0.0
					ingredientItem["FatPer100g"] = 0.0
					ingredientItem["CarbsPer100g"] = 0.0
					ingredientItem["SugarPer100g"] = 0.0
					ingredientItem["FiberPer100g"] = 0.0
					ingredientItem["SodiumPer100g"] = 0.0
				}
				ingredientData = append(ingredientData, ingredientItem)

				// Count by moisture type
				switch i.MoistureType {
				case "dry":
					dryCount++
				case "wet":
					wetCount++
				case "liquid":
					liquidCount++
				}

				// Count by active status
				if i.IsActive {
					activeCount++
				} else {
					inactiveCount++
				}
			}
		}
	}

	data := gin.H{
		"CurrentPage":   "ingredients",
		"Ingredients":   ingredientData,
		"DryCount":      dryCount,
		"WetCount":      wetCount,
		"LiquidCount":   liquidCount,
		"ActiveCount":   activeCount,
		"InactiveCount": inactiveCount,
	}
	w.renderTemplate(c, "ingredients-list", data)
}

// IngredientNew renders the new ingredient form
func (w *WebHandlers) IngredientNew(c *gin.Context) {
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	data := gin.H{
		"CurrentPage": "ingredients",
		"Ingredient":  gin.H{},
		"TenantID":    tenantIDStr,
	}
	w.renderTemplate(c, "ingredients-form", data)
}

// IngredientEdit renders the ingredient edit form
func (w *WebHandlers) IngredientEdit(c *gin.Context) {
	ctx := c.Request.Context()
	ingredientID := c.Param("id")
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	ingredientOID, err := primitive.ObjectIDFromHex(ingredientID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ingredient ID")
		return
	}

	ingredient, err := w.handlers.repos.Ingredient.GetByID(ctx, ingredientOID)
	if err != nil || ingredient == nil {
		c.String(http.StatusNotFound, "Ingredient not found")
		return
	}

	// Build ingredient data for template including nutrition fields
	ingredientData := gin.H{
		"ID":               ingredient.ID.Hex(),
		"Name":             ingredient.Name,
		"MoistureType":     string(ingredient.MoistureType),
		"ShelfLifeMinutes": ingredient.ShelfLifeMinutes,
		"AllergenInfo":     strings.Join(ingredient.AllergenInfo, ", "),
		"IsActive":         ingredient.IsActive,
	}

	// Add nutrition fields if present
	if ingredient.Nutrition != nil {
		ingredientData["CaloriesPer100g"] = ingredient.Nutrition.CaloriesPer100g
		ingredientData["ProteinPer100g"] = ingredient.Nutrition.ProteinPer100g
		ingredientData["FatPer100g"] = ingredient.Nutrition.FatPer100g
		ingredientData["CarbsPer100g"] = ingredient.Nutrition.CarbsPer100g
		ingredientData["SugarPer100g"] = ingredient.Nutrition.SugarPer100g
		ingredientData["FiberPer100g"] = ingredient.Nutrition.FiberPer100g
		ingredientData["SodiumPer100g"] = ingredient.Nutrition.SodiumPer100g
	}

	data := gin.H{
		"CurrentPage": "ingredients",
		"Ingredient":  ingredientData,
		"TenantID":    tenantIDStr,
	}
	w.renderTemplate(c, "ingredients-form", data)
}

// Orders renders the orders page
func (w *WebHandlers) Orders(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	orderData := []gin.H{}
	sites := []gin.H{}
	var pendingCount, acceptedCount, scheduledCount, inProgressCount, completedCount, failedCount int

	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			status := c.Query("status")
			orders, _, _ := w.handlers.repos.Order.ListByTenant(ctx, tenantID, nil, status, 1, 100)
			for _, o := range orders {
				// Count by status
				switch o.Status {
				case models.OrderStatusPending:
					pendingCount++
				case models.OrderStatusAccepted:
					acceptedCount++
				case models.OrderStatusScheduled:
					scheduledCount++
				case models.OrderStatusInProgress:
					inProgressCount++
				case models.OrderStatusCompleted:
					completedCount++
				case models.OrderStatusFailed:
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
					"Status":         string(o.Status),
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

	totalCount := pendingCount + acceptedCount + scheduledCount + inProgressCount + completedCount + failedCount

	data := gin.H{
		"CurrentPage": "orders",
		"Orders":      orderData,
		"Sites":       sites,
		"Stats": gin.H{
			"Total":      totalCount,
			"Pending":    pendingCount,
			"Accepted":   acceptedCount,
			"Scheduled":  scheduledCount,
			"InProgress": inProgressCount,
			"Completed":  completedCount,
			"Failed":     failedCount,
		},
	}
	w.renderTemplate(c, "orders-list", data)
}

// OrderNew renders the new order form
func (w *WebHandlers) OrderNew(c *gin.Context) {
	ctx := c.Request.Context()
	tenantIDStr := middleware.GetEffectiveTenantID(c)

	recipeData := []gin.H{}
	siteData := []gin.H{}

	if tenantIDStr != "" {
		tenantID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err == nil {
			// Get published recipes
			recipes, _, _ := w.handlers.repos.Recipe.ListByTenant(ctx, tenantID, "published", 1, 100)
			for _, r := range recipes {
				recipeData = append(recipeData, gin.H{
					"ID":   r.ID.Hex(),
					"Name": r.Name,
				})
			}

			// Get sites with region info for order routing
			sites, _, _ := w.handlers.repos.Site.ListByTenant(ctx, tenantID, 1, 100)
			for _, s := range sites {
				siteData = append(siteData, gin.H{
					"ID":       s.ID.Hex(),
					"Name":     s.Name,
					"RegionID": s.RegionID.Hex(),
				})
			}
		}
	}

	data := gin.H{
		"CurrentPage": "orders",
		"TenantID":    tenantIDStr,
		"Recipes":     recipeData,
		"Sites":       siteData,
	}
	w.renderTemplate(c, "orders-form", data)
}

// OrderDetail renders the order detail page
func (w *WebHandlers) OrderDetail(c *gin.Context) {
	ctx := c.Request.Context()
	orderID := c.Param("id")

	orderOID, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid order ID")
		return
	}

	order, err := w.handlers.repos.Order.GetByID(ctx, orderOID)
	if err != nil || order == nil {
		c.String(http.StatusNotFound, "Order not found")
		return
	}

	data := gin.H{
		"CurrentPage": "orders",
		"Order": gin.H{
			"ID":                  order.ID.Hex(),
			"OrderReference":      order.OrderReference,
			"RecipeID":            order.RecipeID.Hex(),
			"RecipeName":          order.RecipeName,
			"SiteID":              order.SiteID.Hex(),
			"CustomerName":        order.CustomerName,
			"Status":              order.Status,
			"Priority":            order.Priority,
			"PotPercentage":       order.PotPercentage,
			"SpecialInstructions": order.SpecialInstructions,
			"CreatedAt":           order.CreatedAt,
			"UpdatedAt":           order.UpdatedAt,
		},
	}
	w.renderTemplate(c, "orders-view", data)
}

// Settings renders the settings page
func (w *WebHandlers) Settings(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "settings",
	}
	w.renderTemplate(c, "settings", data)
}

// AuditLog renders the audit log page
func (w *WebHandlers) AuditLog(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "audit",
	}
	w.renderTemplate(c, "audit", data)
}
