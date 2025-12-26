# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

KWS (Kitchen Web Service) is a B2B cloud platform that manages multiple KOS (Kitchen Operating System) instances across different tenants, regions, and sites. It provides centralized recipe management, order routing, and operational analytics for autonomous kitchen operations.

## Development Commands
- `make` - Format and build (default)
- `make run` - Build and run the server
- `make fmt` - Format code
- `make test` - Run tests
- `make tidy` - Run go mod tidy
- `make clean` - Clean build artifacts

## Docker Commands
- `docker-compose up -d` - Start all services (MongoDB, Keycloak, KWS)
- `docker-compose down` - Stop all services
- `docker-compose logs -f kws` - View KWS logs

## Architecture Overview

### Core Design Principles
- **Multi-tenancy**: Tenant isolation at database and Keycloak realm level
- **MongoDB Only**: Single database for all KWS data (recipes, orders, tenants)
- **Keycloak IAM**: Complete authentication and authorization with admin UI
- **mTLS + JWT**: Dual-layer authentication for KOS devices
- **REST API**: KOS polls KWS for recipes (5 min) and orders (30 sec)

### Directory Structure
```
cmd/kws/           - Application entry point
internal/
  app/             - HTTP handlers and application logic
  domain/
    models/        - Domain models (Tenant, Region, Site, Recipe, Order)
    repositories/  - Data access interfaces
  infrastructure/
    config/        - Viper configuration
    database/      - MongoDB connection and operations
    keycloak/      - Keycloak admin client
    certificate/   - mTLS certificate generation
    http/          - HTTP middleware
  pkg/
    errors/        - Error types and responses
    logger/        - Zap logging wrapper
config/           - YAML configuration files
scripts/
  db/              - MongoDB initialization
  ca/              - Certificate Authority scripts
web/               - Web UI (future)
docs/              - Documentation
```

### Technology Stack
- **Backend**: Go 1.23+, Gin web framework, MongoDB 7.0
- **IAM**: Keycloak 23.0 with realm-per-tenant multi-tenancy
- **Database**: MongoDB for application data, PostgreSQL for Keycloak
- **Security**: mTLS for KOS devices, JWT for API authentication

### Key Domain Concepts

#### Tenant Hierarchy
```
Tenant (B2B Customer)
  └── Region (Geographic area)
      └── Site (Physical location)
          └── Kitchen (Maps to KOS kitchen)
              └── KOS Instance (One per site)
```

#### Recipe Sync (SOP-001)
- KWS is the single source of truth for recipes
- KOS polls `/api/v1/kos/recipes` every 5 minutes
- Recipes flow one-way: KWS → KOS (never KOS → KWS)

#### Order Flow (SOP-002)
- Orders require `region_id` and `site_id`
- KOS polls `/api/v1/kos/orders` every 30 seconds
- Order status updates flow: KOS → KWS via POST

### Configuration
- YAML config in `config/`, env overrides with `KWS_` prefix
- Example: `KWS_MONGODB_URI`, `KWS_KEYCLOAK_URL`

### Logging
- Zap structured logging with contextual fields
- Use `log.WithTenant(id)`, `log.WithSite(id)`, `log.WithOrder(id)`

### API Response Format
```json
{
  "success": true,
  "data": { ... },
  "meta": { "page": 1, "per_page": 20, "total": 100 },
  "timestamp": "2024-12-25T10:30:00Z"
}
```

### KOS Authentication
KOS devices authenticate using dual-layer security:
1. **mTLS (Transport)**: Client certificate with CN=`kos-{site_id}`
2. **JWT (Application)**: Keycloak service account token with tenant/site claims

## Development Guidelines

### Code Organization
- Clean architecture: domain logic independent of infrastructure
- Interfaces in domain, implementations in infrastructure
- Dependency injection for testability

### MongoDB Collections
- `tenants`, `regions`, `sites`, `kitchens`, `kos_instances`
- `recipes`, `ingredients`, `orders`
- `audit_logs`, `kos_heartbeats`

### Key Endpoints
- `/api/v1/tenants` - Tenant management (Platform Admin)
- `/api/v1/regions`, `/sites`, `/kitchens` - Hierarchy management
- `/api/v1/recipes`, `/ingredients` - Recipe management
- `/api/v1/orders` - Order management
- `/api/v1/kos/*` - KOS device endpoints (mTLS required)

## Related Projects
- **KOS**: Kitchen Operating System (on-premise, MariaDB)
  - Location: `~/src/ak/kos`
  - Communication: REST API polling

## Recent Implementation Decisions (Dec 2025)

### KOS-KWS Integration Architecture

#### KOS Identity Management (SOP-003)
- KOS ID is auto-generated (UUID) on first startup and stored in local MariaDB database
- The KOS ID persists across container restarts and application upgrades
- No identity information stored in config files - all in `system_config` table
- KOS uses `X-KOS-ID` header in all API calls to KWS

#### mTLS Authentication
- KOS authenticates to KWS using mutual TLS client certificates
- Certificates are issued during KOS provisioning
- Certificate paths configured in KOS: `certificate_path`, `private_key_path`, `ca_certificate_path`
- No API keys to rotate - certificate-based identity

#### Single Recipe Per Order
- Each order contains exactly one recipe (simplified from multi-item orders)
- `OrderForKOS` struct has `recipe_id` and `recipe_name` fields directly
- Simplifies order processing and KOS queue management

#### Recipe Versioning
- Recipes have monotonically increasing `version` field
- When KOS detects version increase during sync:
  1. Deletes existing recipe ingredients
  2. Deletes existing recipe steps
  3. Updates recipe metadata
  4. Recreates ingredients and steps from KWS data
- This ensures recipe changes are fully propagated

### KWS Configuration Strategy
- `config/config.yaml` - Local development (gitignored)
- `config/config.prod.yaml` - Production defaults (committed to git)
- Docker uses `config.prod.yaml` copied as `config.yaml` in container
- Environment variables override with `KWS_` prefix

### Docker Compose Services
- `kws` - Main application (port 8000:8080)
- `mongodb` - Application data (port 27017)
- `postgres` - Keycloak database
- `keycloak` - IAM (port 8180:8080)
- `nginx` - Reverse proxy (production profile only)

## Documentation

Comprehensive documentation in `docs/`:
- `KWS-Requirements-Document.adoc` - System requirements, SOPs, architecture decisions
- `KWS-Functional-Specification.adoc` - User features, workflows, business rules
- `KWS-Technical-Design.adoc` - MongoDB schemas, API specs, deployment

### Standard Operating Procedures
| SOP | Description |
|-----|-------------|
| SOP-001 | Recipe Authority: KWS is single source of truth. Recipe version increases trigger full re-sync. |
| SOP-002 | Order Routing: Every order must specify region and site. Single recipe per order. |
| SOP-003 | KOS Identity: Auto-generated UUID stored in database. mTLS for authentication. |
