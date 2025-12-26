package app

import (
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/ak/kws/web"
	"github.com/gin-gonic/gin"
)

// WebHandlers handles web UI requests
type WebHandlers struct {
	templates *template.Template
	handlers  *Handlers
}

// NewWebHandlers creates a new web handlers instance
func NewWebHandlers(handlers *Handlers) (*WebHandlers, error) {
	// Create template functions
	funcMap := template.FuncMap{
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

	// Parse templates
	templatesFS := web.Templates()
	tmpl := template.New("").Funcs(funcMap)

	// Parse all templates
	err := fs.WalkDir(templatesFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path == "." {
			return nil
		}
		content, err := fs.ReadFile(templatesFS, path)
		if err != nil {
			return err
		}
		_, err = tmpl.New(path).Parse(string(content))
		return err
	})
	if err != nil {
		return nil, err
	}

	return &WebHandlers{
		templates: tmpl,
		handlers:  handlers,
	}, nil
}

// RegisterRoutes registers web UI routes
func (w *WebHandlers) RegisterRoutes(r *gin.Engine) {
	// Serve static files
	staticFS := web.Static()
	r.StaticFS("/static", http.FS(staticFS))

	// Web pages
	r.GET("/", w.Dashboard)
	r.GET("/dashboard", w.Dashboard)
	r.GET("/tenants", w.Tenants)
	r.GET("/tenants/:id", w.TenantDetail)
	r.GET("/sites", w.Sites)
	r.GET("/sites/:id", w.SiteDetail)
	r.GET("/kos", w.KOSInstances)
	r.GET("/kos/new", w.KOSNew)
	r.GET("/kos/:id", w.KOSDetail)
	r.GET("/recipes", w.Recipes)
	r.GET("/recipes/new", w.RecipeNew)
	r.GET("/recipes/:id", w.RecipeDetail)
	r.GET("/recipes/:id/edit", w.RecipeEdit)
	r.GET("/ingredients", w.Ingredients)
	r.GET("/orders", w.Orders)
	r.GET("/orders/new", w.OrderNew)
	r.GET("/orders/:id", w.OrderDetail)
	r.GET("/settings", w.Settings)
	r.GET("/audit", w.AuditLog)
}

func (w *WebHandlers) render(c *gin.Context, templateName string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := w.templates.ExecuteTemplate(c.Writer, templateName, data); err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// Dashboard renders the dashboard page
func (w *WebHandlers) Dashboard(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "dashboard",
		"Stats": gin.H{
			"TenantCount":    12,
			"ActiveTenants":  10,
			"SiteCount":      45,
			"ActiveSites":    42,
			"KOSCount":       45,
			"OnlineKOS":      40,
			"OfflineKOS":     5,
			"OrdersToday":    156,
			"CompletedToday": 142,
			"PendingOrders":  8,
		},
		"RecentKOS": []gin.H{
			{"Name": "SF Downtown KOS", "SiteName": "San Francisco Downtown", "Status": "online", "LastHeartbeat": "2 min ago"},
			{"Name": "NYC Midtown KOS", "SiteName": "NYC Midtown", "Status": "online", "LastHeartbeat": "1 min ago"},
			{"Name": "LA Venice KOS", "SiteName": "Los Angeles Venice", "Status": "offline", "LastHeartbeat": "15 min ago"},
		},
		"RecentOrders": []gin.H{
			{"OrderReference": "ORD-2024-001234", "SiteName": "SF Downtown", "ItemCount": 3, "Status": "completed", "CreatedAt": "5 min ago"},
			{"OrderReference": "ORD-2024-001235", "SiteName": "NYC Midtown", "ItemCount": 2, "Status": "in_progress", "CreatedAt": "8 min ago"},
			{"OrderReference": "ORD-2024-001236", "SiteName": "LA Venice", "ItemCount": 5, "Status": "pending", "CreatedAt": "12 min ago"},
		},
		"Alerts": []gin.H{
			{"Severity": "warning", "Title": "KOS Offline", "Message": "LA Venice KOS has been offline for 15 minutes", "Timestamp": "15 min ago"},
		},
	}
	w.render(c, "layouts/base.html", data)
}

// Tenants renders the tenants page
func (w *WebHandlers) Tenants(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "tenants",
		"Tenants": []gin.H{
			{"ID": "1", "Name": "ABC Foods Inc.", "Code": "abc-foods", "Plan": "enterprise", "Status": "active", "SiteCount": 12, "CreatedAt": "Jan 15, 2024"},
			{"ID": "2", "Name": "Quick Bites LLC", "Code": "quick-bites", "Plan": "professional", "Status": "active", "SiteCount": 5, "CreatedAt": "Feb 20, 2024"},
			{"ID": "3", "Name": "Gourmet Express", "Code": "gourmet-exp", "Plan": "starter", "Status": "trial", "SiteCount": 1, "CreatedAt": "Mar 10, 2024"},
		},
		"Page":       1,
		"TotalPages": 1,
		"Total":      3,
	}
	w.render(c, "layouts/base.html", data)
}

// TenantDetail renders the tenant detail page
func (w *WebHandlers) TenantDetail(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "tenants",
		"Tenant": gin.H{
			"ID":           c.Param("id"),
			"Name":         "ABC Foods Inc.",
			"Code":         "abc-foods",
			"Plan":         "enterprise",
			"Status":       "active",
			"ContactEmail": "admin@abcfoods.com",
		},
	}
	w.render(c, "layouts/base.html", data)
}

// Sites renders the sites page
func (w *WebHandlers) Sites(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "sites",
	}
	w.render(c, "layouts/base.html", data)
}

// SiteDetail renders the site detail page
func (w *WebHandlers) SiteDetail(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "sites",
	}
	w.render(c, "layouts/base.html", data)
}

// KOSInstances renders the KOS instances page
func (w *WebHandlers) KOSInstances(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "kos",
		"KOSInstances": []gin.H{
			{"ID": "1", "Name": "SF Downtown KOS", "SiteName": "San Francisco Downtown", "Status": "online", "Version": "2.1.0", "LastHeartbeat": "2 min ago", "CPUPercent": 45, "MemoryPercent": 62, "ActiveOrders": 3, "Kitchens": []string{"k1", "k2"}},
			{"ID": "2", "Name": "NYC Midtown KOS", "SiteName": "NYC Midtown", "Status": "online", "Version": "2.1.0", "LastHeartbeat": "1 min ago", "CPUPercent": 32, "MemoryPercent": 48, "ActiveOrders": 2, "Kitchens": []string{"k1"}},
			{"ID": "3", "Name": "LA Venice KOS", "SiteName": "Los Angeles Venice", "Status": "offline", "Version": "2.0.5", "LastHeartbeat": "15 min ago", "Kitchens": []string{"k1", "k2", "k3"}},
			{"ID": "4", "Name": "Chicago Loop KOS", "SiteName": "Chicago Loop", "Status": "pending", "Kitchens": []string{}},
		},
	}
	w.render(c, "layouts/base.html", data)
}

// KOSNew renders the new KOS form
func (w *WebHandlers) KOSNew(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "kos",
	}
	w.render(c, "layouts/base.html", data)
}

// KOSDetail renders the KOS detail page
func (w *WebHandlers) KOSDetail(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "kos",
	}
	w.render(c, "layouts/base.html", data)
}

// Recipes renders the recipes page
func (w *WebHandlers) Recipes(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "recipes",
		"Recipes": []gin.H{
			{"ID": "1", "Name": "Classic Burger", "Category": "Main Course", "Status": "published", "Version": 3, "Description": "Juicy beef patty with fresh vegetables and special sauce", "TotalTime": 25, "Ingredients": []string{"beef", "lettuce", "tomato", "onion", "cheese"}},
			{"ID": "2", "Name": "Caesar Salad", "Category": "Appetizer", "Status": "published", "Version": 2, "Description": "Fresh romaine lettuce with caesar dressing and croutons", "TotalTime": 15, "Ingredients": []string{"lettuce", "croutons", "parmesan"}},
			{"ID": "3", "Name": "Chocolate Lava Cake", "Category": "Dessert", "Status": "draft", "Version": 1, "Description": "Warm chocolate cake with molten center", "TotalTime": 30, "Ingredients": []string{"chocolate", "butter", "eggs", "flour"}},
		},
	}
	w.render(c, "layouts/base.html", data)
}

// RecipeNew renders the new recipe form
func (w *WebHandlers) RecipeNew(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "recipes",
	}
	w.render(c, "layouts/base.html", data)
}

// RecipeDetail renders the recipe detail page
func (w *WebHandlers) RecipeDetail(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "recipes",
	}
	w.render(c, "layouts/base.html", data)
}

// RecipeEdit renders the recipe edit form
func (w *WebHandlers) RecipeEdit(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "recipes",
	}
	w.render(c, "layouts/base.html", data)
}

// Ingredients renders the ingredients page
func (w *WebHandlers) Ingredients(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "ingredients",
	}
	w.render(c, "layouts/base.html", data)
}

// Orders renders the orders page
func (w *WebHandlers) Orders(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "orders",
		"Stats": gin.H{
			"Pending":    8,
			"InProgress": 5,
			"Completed":  142,
			"Failed":     3,
		},
		"Orders": []gin.H{
			{"ID": "1", "OrderReference": "ORD-2024-001234", "CustomerName": "John Doe", "SiteName": "SF Downtown", "RegionName": "US West", "RecipeName": "Classic Burger", "Status": "completed", "Priority": 3, "CreatedAt": "5 min ago"},
			{"ID": "2", "OrderReference": "ORD-2024-001235", "CustomerName": "Jane Smith", "SiteName": "NYC Midtown", "RegionName": "US East", "RecipeName": "Caesar Salad", "Status": "in_progress", "Priority": 2, "CreatedAt": "8 min ago"},
			{"ID": "3", "OrderReference": "ORD-2024-001236", "SiteName": "LA Venice", "RegionName": "US West", "RecipeName": "Chocolate Lava Cake", "Status": "pending", "Priority": 1, "CreatedAt": "12 min ago"},
		},
		"Sites": []gin.H{
			{"ID": "1", "Name": "SF Downtown"},
			{"ID": "2", "Name": "NYC Midtown"},
			{"ID": "3", "Name": "LA Venice"},
		},
	}
	w.render(c, "layouts/base.html", data)
}

// OrderNew renders the new order form
func (w *WebHandlers) OrderNew(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "orders",
	}
	w.render(c, "layouts/base.html", data)
}

// OrderDetail renders the order detail page
func (w *WebHandlers) OrderDetail(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "orders",
	}
	w.render(c, "layouts/base.html", data)
}

// Settings renders the settings page
func (w *WebHandlers) Settings(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "settings",
	}
	w.render(c, "layouts/base.html", data)
}

// AuditLog renders the audit log page
func (w *WebHandlers) AuditLog(c *gin.Context) {
	data := gin.H{
		"CurrentPage": "audit",
	}
	w.render(c, "layouts/base.html", data)
}
