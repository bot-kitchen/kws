package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/ak/kws/internal/domain/models"
	"github.com/ak/kws/internal/domain/repositories"
	"github.com/ak/kws/internal/infrastructure/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// KOSService handles KOS instance business logic
type KOSService interface {
	Create(ctx context.Context, req CreateKOSRequest) (*models.KOSInstance, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.KOSInstance, error)
	GetBySiteID(ctx context.Context, siteID primitive.ObjectID) (*models.KOSInstance, error)
	GetByCertificateSerial(ctx context.Context, serial string) (*models.KOSInstance, error)
	Update(ctx context.Context, id primitive.ObjectID, req UpdateKOSRequest) (*models.KOSInstance, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]*models.KOSInstance, int64, error)

	// Certificate management
	ProvisionCertificate(ctx context.Context, id primitive.ObjectID) (*KOSProvisioningResult, error)
	RevokeCertificate(ctx context.Context, id primitive.ObjectID) error

	// Registration and heartbeat
	Register(ctx context.Context, kosID string, version string) error
	RecordHeartbeat(ctx context.Context, heartbeat *models.KOSHeartbeat) error

	// Status management
	SetStatus(ctx context.Context, id primitive.ObjectID, status models.KOSStatus) error
	GetOfflineInstances(ctx context.Context, threshold time.Duration) ([]*models.KOSInstance, error)
}

type CreateKOSRequest struct {
	TenantID primitive.ObjectID   `json:"tenant_id" binding:"required"`
	RegionID primitive.ObjectID   `json:"region_id" binding:"required"`
	SiteID   primitive.ObjectID   `json:"site_id" binding:"required"`
	Name     string               `json:"name" binding:"required"`
	Kitchens []primitive.ObjectID `json:"kitchens"`
}

type UpdateKOSRequest struct {
	Name     string               `json:"name"`
	Kitchens []primitive.ObjectID `json:"kitchens"`
}

type KOSProvisioningResult struct {
	KOSID          string `json:"kos_id"`
	CertificatePEM string `json:"certificate_pem"`
	PrivateKeyPEM  string `json:"private_key_pem"`
	CACertPEM      string `json:"ca_cert_pem"`
	Endpoint       string `json:"endpoint"`
}

type kosService struct {
	kosRepo     repositories.KOSInstanceRepository
	siteRepo    repositories.SiteRepository
	keycloakSvc KeycloakService
	certConfig  config.CertificateConfig
	serverURL   string
}

// NewKOSService creates a new KOS service
func NewKOSService(
	kosRepo repositories.KOSInstanceRepository,
	siteRepo repositories.SiteRepository,
	keycloakSvc KeycloakService,
	certConfig config.CertificateConfig,
	serverURL string,
) KOSService {
	return &kosService{
		kosRepo:     kosRepo,
		siteRepo:    siteRepo,
		keycloakSvc: keycloakSvc,
		certConfig:  certConfig,
		serverURL:   serverURL,
	}
}

func (s *kosService) Create(ctx context.Context, req CreateKOSRequest) (*models.KOSInstance, error) {
	// Validate site exists
	if s.siteRepo != nil {
		site, err := s.siteRepo.GetByID(ctx, req.SiteID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate site: %w", err)
		}
		if site == nil {
			return nil, fmt.Errorf("site not found: %s", req.SiteID.Hex())
		}
		if site.TenantID != req.TenantID {
			return nil, fmt.Errorf("site does not belong to tenant")
		}
	}

	// Check if KOS already exists for this site
	existing, err := s.kosRepo.GetBySiteID(ctx, req.SiteID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing KOS: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("KOS instance already exists for site")
	}

	kos := &models.KOSInstance{
		TenantID:  req.TenantID,
		RegionID:  req.RegionID,
		SiteID:    req.SiteID,
		Name:      req.Name,
		Status:    models.KOSStatusPending,
		Kitchens:  req.Kitchens,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.kosRepo.Create(ctx, kos); err != nil {
		return nil, fmt.Errorf("failed to create KOS instance: %w", err)
	}

	return kos, nil
}

func (s *kosService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.KOSInstance, error) {
	return s.kosRepo.GetByID(ctx, id)
}

func (s *kosService) GetBySiteID(ctx context.Context, siteID primitive.ObjectID) (*models.KOSInstance, error) {
	return s.kosRepo.GetBySiteID(ctx, siteID)
}

func (s *kosService) GetByCertificateSerial(ctx context.Context, serial string) (*models.KOSInstance, error) {
	return s.kosRepo.GetByCertificateSerial(ctx, serial)
}

func (s *kosService) Update(ctx context.Context, id primitive.ObjectID, req UpdateKOSRequest) (*models.KOSInstance, error) {
	kos, err := s.kosRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if kos == nil {
		return nil, fmt.Errorf("KOS instance not found")
	}

	if req.Name != "" {
		kos.Name = req.Name
	}
	if req.Kitchens != nil {
		kos.Kitchens = req.Kitchens
	}

	kos.UpdatedAt = time.Now()

	if err := s.kosRepo.Update(ctx, kos); err != nil {
		return nil, err
	}

	return kos, nil
}

func (s *kosService) Delete(ctx context.Context, id primitive.ObjectID) error {
	kos, err := s.kosRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if kos == nil {
		return fmt.Errorf("KOS instance not found")
	}

	// Revoke certificate if provisioned
	if kos.CertificateSerial != "" {
		_ = s.RevokeCertificate(ctx, id)
	}

	return s.kosRepo.Delete(ctx, id)
}

func (s *kosService) List(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]*models.KOSInstance, int64, error) {
	return s.kosRepo.ListByTenant(ctx, tenantID, page, limit)
}

func (s *kosService) ProvisionCertificate(ctx context.Context, id primitive.ObjectID) (*KOSProvisioningResult, error) {
	kos, err := s.kosRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if kos == nil {
		return nil, fmt.Errorf("KOS instance not found")
	}

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Generate certificate serial number
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	// Load CA certificate and key
	caCert, caKey, err := s.loadCA()
	if err != nil {
		return nil, fmt.Errorf("failed to load CA: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.AddDate(0, 0, s.certConfig.CertValidityDays)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("kos-%s", kos.ID.Hex()),
			Organization: []string{"KWS"},
			Country:      []string{"US"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})

	// Update KOS instance
	kos.CertificatePEM = string(certPEM)
	kos.PrivateKeyPEM = string(keyPEM)
	kos.CertificateSerial = serialNumber.String()
	kos.CertificateExpiry = notAfter
	kos.Status = models.KOSStatusProvisioned
	kos.UpdatedAt = time.Now()

	if err := s.kosRepo.Update(ctx, kos); err != nil {
		return nil, fmt.Errorf("failed to update KOS instance: %w", err)
	}

	return &KOSProvisioningResult{
		KOSID:          kos.ID.Hex(),
		CertificatePEM: string(certPEM),
		PrivateKeyPEM:  string(keyPEM),
		CACertPEM:      string(caCertPEM),
		Endpoint:       s.serverURL + "/api/v1/kos",
	}, nil
}

func (s *kosService) RevokeCertificate(ctx context.Context, id primitive.ObjectID) error {
	kos, err := s.kosRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if kos == nil {
		return fmt.Errorf("KOS instance not found")
	}

	kos.CertificatePEM = ""
	kos.PrivateKeyPEM = ""
	kos.CertificateSerial = ""
	kos.CertificateExpiry = time.Time{}
	kos.Status = models.KOSStatusDeactivated
	kos.UpdatedAt = time.Now()

	return s.kosRepo.Update(ctx, kos)
}

func (s *kosService) Register(ctx context.Context, kosID string, version string) error {
	id, err := primitive.ObjectIDFromHex(kosID)
	if err != nil {
		return fmt.Errorf("invalid KOS ID: %w", err)
	}

	kos, err := s.kosRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if kos == nil {
		return fmt.Errorf("KOS instance not found")
	}

	now := time.Now()
	kos.Version = version
	kos.Status = models.KOSStatusRegistered
	kos.RegisteredAt = &now
	kos.UpdatedAt = now

	return s.kosRepo.Update(ctx, kos)
}

func (s *kosService) RecordHeartbeat(ctx context.Context, heartbeat *models.KOSHeartbeat) error {
	heartbeat.ReceivedAt = time.Now()

	// Update KOS instance last heartbeat
	kos, err := s.kosRepo.GetByID(ctx, heartbeat.KOSID)
	if err != nil {
		return err
	}
	if kos == nil {
		return fmt.Errorf("KOS instance not found")
	}

	now := time.Now()
	kos.LastHeartbeat = &now
	kos.Version = heartbeat.Version
	kos.Status = models.KOSStatusOnline
	kos.UpdatedAt = now

	if err := s.kosRepo.Update(ctx, kos); err != nil {
		return err
	}

	return s.kosRepo.RecordHeartbeat(ctx, heartbeat)
}

func (s *kosService) SetStatus(ctx context.Context, id primitive.ObjectID, status models.KOSStatus) error {
	kos, err := s.kosRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if kos == nil {
		return fmt.Errorf("KOS instance not found")
	}

	kos.Status = status
	kos.UpdatedAt = time.Now()

	return s.kosRepo.Update(ctx, kos)
}

func (s *kosService) GetOfflineInstances(ctx context.Context, threshold time.Duration) ([]*models.KOSInstance, error) {
	instances, _, err := s.kosRepo.ListByTenant(ctx, primitive.NilObjectID, 1, 1000)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-threshold)
	var offline []*models.KOSInstance

	for _, kos := range instances {
		if kos.Status == models.KOSStatusOnline || kos.Status == models.KOSStatusRegistered {
			if kos.LastHeartbeat == nil || kos.LastHeartbeat.Before(cutoff) {
				offline = append(offline, kos)
			}
		}
	}

	return offline, nil
}

func (s *kosService) loadCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	// Parse CA certificate
	caCertBlock, _ := pem.Decode([]byte(s.certConfig.CACert))
	if caCertBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
	}

	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Parse CA private key
	caKeyBlock, _ := pem.Decode([]byte(s.certConfig.CAKey))
	if caKeyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA key PEM")
	}

	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		// Try PKCS8
		key, err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse CA key: %w", err)
		}
		var ok bool
		caKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("CA key is not RSA")
		}
	}

	return caCert, caKey, nil
}
