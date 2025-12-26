package middleware

import (
	"context"
	"crypto/x509"
	"net/http"

	"github.com/ak/kws/internal/domain/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// KOSInstance represents a KOS instance for authentication
type KOSInstance struct {
	ID       primitive.ObjectID
	TenantID primitive.ObjectID
	SiteID   primitive.ObjectID
	Status   models.KOSStatus
}

// KOSLookupFunc is a function type for looking up KOS instances by certificate serial
type KOSLookupFunc func(ctx context.Context, serial string) (*KOSInstance, error)

// MTLSConfig holds mTLS middleware configuration
type MTLSConfig struct {
	// Required: Function to look up KOS instance by certificate serial
	KOSLookup KOSLookupFunc

	// Optional: Root CA pool for additional validation
	RootCAs *x509.CertPool

	// Optional: Skip mTLS in development mode
	DevMode bool
}

// MTLSMiddleware creates a mutual TLS authentication middleware for KOS devices
func MTLSMiddleware(config MTLSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip in dev mode for testing
		if config.DevMode {
			// In dev mode, check for a header-based KOS ID instead
			kosID := c.GetHeader("X-KOS-ID")
			if kosID != "" {
				c.Set("kos_id", kosID)
				c.Next()
				return
			}
		}

		// Get TLS connection state
		if c.Request.TLS == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "TLS connection required",
			})
			return
		}

		// Check for client certificate
		if len(c.Request.TLS.PeerCertificates) == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "client certificate required",
			})
			return
		}

		clientCert := c.Request.TLS.PeerCertificates[0]

		// Validate certificate chain if RootCAs provided
		if config.RootCAs != nil {
			opts := x509.VerifyOptions{
				Roots:         config.RootCAs,
				CurrentTime:   clientCert.NotBefore,
				Intermediates: x509.NewCertPool(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			}

			// Add intermediate certificates
			for _, cert := range c.Request.TLS.PeerCertificates[1:] {
				opts.Intermediates.AddCert(cert)
			}

			if _, err := clientCert.Verify(opts); err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "unauthorized",
					"message": "certificate verification failed",
				})
				return
			}
		}

		// Extract certificate serial number
		serialNumber := clientCert.SerialNumber.String()

		// Look up KOS instance by certificate serial
		if config.KOSLookup == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "KOS lookup not configured",
			})
			return
		}

		kos, err := config.KOSLookup(c.Request.Context(), serialNumber)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "failed to validate certificate",
			})
			return
		}

		if kos == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "unknown certificate",
			})
			return
		}

		// Check KOS status
		if kos.Status == models.KOSStatusDeactivated {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "KOS instance is deactivated",
			})
			return
		}

		// Set KOS information in context
		c.Set("kos_id", kos.ID.Hex())
		c.Set("kos_tenant_id", kos.TenantID.Hex())
		c.Set("kos_site_id", kos.SiteID.Hex())
		c.Set("kos_status", string(kos.Status))
		c.Set("kos_cert_serial", serialNumber)

		c.Next()
	}
}

// GetKOSID extracts KOS ID from context
func GetKOSID(c *gin.Context) string {
	if kosID, exists := c.Get("kos_id"); exists {
		if id, ok := kosID.(string); ok {
			return id
		}
	}
	return ""
}

// GetKOSTenantID extracts KOS tenant ID from context
func GetKOSTenantID(c *gin.Context) string {
	if tenantID, exists := c.Get("kos_tenant_id"); exists {
		if id, ok := tenantID.(string); ok {
			return id
		}
	}
	return ""
}

// GetKOSSiteID extracts KOS site ID from context
func GetKOSSiteID(c *gin.Context) string {
	if siteID, exists := c.Get("kos_site_id"); exists {
		if id, ok := siteID.(string); ok {
			return id
		}
	}
	return ""
}

// RequireKOS ensures the request is from an authenticated KOS device
func RequireKOS() gin.HandlerFunc {
	return func(c *gin.Context) {
		kosID := GetKOSID(c)
		if kosID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "KOS authentication required",
			})
			return
		}
		c.Next()
	}
}

// RequireKOSStatus ensures the KOS device has the required status
func RequireKOSStatus(allowedStatuses ...models.KOSStatus) gin.HandlerFunc {
	return func(c *gin.Context) {
		statusStr, exists := c.Get("kos_status")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "KOS status not found",
			})
			return
		}

		status := models.KOSStatus(statusStr.(string))
		allowed := false
		for _, s := range allowedStatuses {
			if status == s {
				allowed = true
				break
			}
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "KOS status not allowed for this operation",
			})
			return
		}

		c.Next()
	}
}
