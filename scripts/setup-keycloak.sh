#!/bin/bash
#
# setup-keycloak.sh - Initialize Keycloak for KWS
#
# This script configures Keycloak with the necessary realm, clients, roles,
# and protocol mappers for the KWS platform.
#
# Usage:
#   ./scripts/setup-keycloak.sh [options]
#
# Options:
#   --keycloak-url URL       Keycloak base URL (default: http://localhost:8180)
#   --admin-user USER        Admin username (default: admin)
#   --admin-password PASS    Admin password (default: admin)
#   --kws-realm REALM        Realm name for KWS platform admin (default: kws-platform)
#   --dry-run               Print what would be done without making changes
#   -h, --help              Show this help message
#
# Environment variables:
#   KEYCLOAK_URL             Keycloak base URL
#   KEYCLOAK_ADMIN           Admin username
#   KEYCLOAK_ADMIN_PASSWORD  Admin password
#

set -euo pipefail

# Default configuration
KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8180}"
ADMIN_USER="${KEYCLOAK_ADMIN:-arun}"
ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-pass}"
KWS_REALM="kws-platform"
DRY_RUN=false
RETRY_COUNT=30
RETRY_INTERVAL=2

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
	echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
	echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
	echo -e "${RED}[ERROR]${NC} $1"
}

usage() {
	head -30 "$0" | tail -20
	exit 0
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
	case $1 in
	--keycloak-url)
		KEYCLOAK_URL="$2"
		shift 2
		;;
	--admin-user)
		ADMIN_USER="$2"
		shift 2
		;;
	--admin-password)
		ADMIN_PASSWORD="$2"
		shift 2
		;;
	--kws-realm)
		KWS_REALM="$2"
		shift 2
		;;
	--dry-run)
		DRY_RUN=true
		shift
		;;
	-h | --help)
		usage
		;;
	*)
		log_error "Unknown option: $1"
		usage
		;;
	esac
done

# Wait for Keycloak to be ready
wait_for_keycloak() {
	log_info "Waiting for Keycloak to be ready at ${KEYCLOAK_URL}..."

	for i in $(seq 1 $RETRY_COUNT); do
		if curl -sf "${KEYCLOAK_URL}/health/ready" >/dev/null 2>&1; then
			log_info "Keycloak is ready"
			return 0
		fi
		# Fallback: try the root endpoint
		if curl -sf "${KEYCLOAK_URL}/" >/dev/null 2>&1; then
			log_info "Keycloak is ready"
			return 0
		fi
		echo -n "."
		sleep $RETRY_INTERVAL
	done

	log_error "Keycloak is not ready after $((RETRY_COUNT * RETRY_INTERVAL)) seconds"
	exit 1
}

# Get admin access token
get_admin_token() {
	log_info "Obtaining admin access token..."

	local response
	response=$(curl -sf -X POST "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token" \
		-H "Content-Type: application/x-www-form-urlencoded" \
		-d "client_id=admin-cli" \
		-d "username=${ADMIN_USER}" \
		-d "password=${ADMIN_PASSWORD}" \
		-d "grant_type=password")

	if [[ -z "$response" ]]; then
		log_error "Failed to obtain admin token"
		exit 1
	fi

	ACCESS_TOKEN=$(echo "$response" | jq -r '.access_token')

	if [[ -z "$ACCESS_TOKEN" || "$ACCESS_TOKEN" == "null" ]]; then
		log_error "Failed to parse admin token from response"
		exit 1
	fi

	log_info "Admin token obtained successfully"
}

# Make authenticated API call
keycloak_api() {
	local method="$1"
	local endpoint="$2"
	local data="${3:-}"

	local url="${KEYCLOAK_URL}/admin${endpoint}"

	if [[ "$DRY_RUN" == "true" ]]; then
		log_info "[DRY-RUN] $method $url"
		[[ -n "$data" ]] && echo "$data" | jq . 2>/dev/null || true
		return 0
	fi

	local args=(-sf -X "$method" -H "Authorization: Bearer ${ACCESS_TOKEN}" -H "Content-Type: application/json")

	if [[ -n "$data" ]]; then
		args+=(-d "$data")
	fi

	curl "${args[@]}" "$url"
}

# Check if realm exists
realm_exists() {
	local realm="$1"
	local response
	response=$(keycloak_api GET "/realms/${realm}" 2>/dev/null) || return 1
	[[ -n "$response" ]]
}

# Create KWS platform realm
create_kws_platform_realm() {
	log_info "Creating KWS platform realm: ${KWS_REALM}..."

	if realm_exists "$KWS_REALM"; then
		log_warn "Realm ${KWS_REALM} already exists, skipping creation"
		return 0
	fi

	local realm_config
	realm_config=$(
		cat <<EOF
{
  "realm": "${KWS_REALM}",
  "enabled": true,
  "registrationAllowed": false,
  "loginWithEmailAllowed": true,
  "duplicateEmailsAllowed": false,
  "resetPasswordAllowed": true,
  "editUsernameAllowed": false,
  "bruteForceProtected": true,
  "permanentLockout": false,
  "maxFailureWaitSeconds": 900,
  "minimumQuickLoginWaitSeconds": 60,
  "waitIncrementSeconds": 60,
  "quickLoginCheckMilliSeconds": 1000,
  "maxDeltaTimeSeconds": 43200,
  "failureFactor": 5,
  "sslRequired": "external",
  "defaultSignatureAlgorithm": "RS256",
  "accessTokenLifespan": 900,
  "refreshTokenMaxReuse": 0,
  "ssoSessionIdleTimeout": 1800,
  "ssoSessionMaxLifespan": 36000
}
EOF
	)

	keycloak_api POST "/realms" "$realm_config"
	log_info "Realm ${KWS_REALM} created successfully"
}

# Create realm roles
create_realm_roles() {
	log_info "Creating realm roles..."

	# Define roles in order (composites must come after their components)
	local base_roles=(
		"kitchen_operator"
		"recipe_editor"
		"analytics_viewer"
	)

	# Create base roles first
	for role in "${base_roles[@]}"; do
		log_info "Creating role: $role"
		keycloak_api POST "/realms/${KWS_REALM}/roles" "{\"name\": \"${role}\"}" 2>/dev/null || true
	done

	# Create composite roles
	local composite_roles=(
		"site_manager:kitchen_operator"
		"recipe_manager:recipe_editor"
		"regional_manager:site_manager"
		"tenant_admin:regional_manager,recipe_manager,analytics_viewer"
		"tenant_owner:tenant_admin"
		"platform_admin:tenant_owner"
	)

	for composite in "${composite_roles[@]}"; do
		local role_name="${composite%%:*}"
		local composites="${composite#*:}"

		log_info "Creating composite role: $role_name"
		keycloak_api POST "/realms/${KWS_REALM}/roles" "{\"name\": \"${role_name}\", \"composite\": true}" 2>/dev/null || true

		# Add composite associations
		IFS=',' read -ra COMPOSITE_ARRAY <<<"$composites"
		local composite_refs="["
		for c in "${COMPOSITE_ARRAY[@]}"; do
			composite_refs+="{\"name\": \"${c}\"},"
		done
		composite_refs="${composite_refs%,}]"

		keycloak_api POST "/realms/${KWS_REALM}/roles/${role_name}/composites" "$composite_refs" 2>/dev/null || true
	done

	log_info "Realm roles created successfully"
}

# Create KWS Web Client (public client for web UI)
create_kws_web_client() {
	log_info "Creating KWS web client..."

	local client_config
	client_config=$(
		cat <<EOF
{
  "clientId": "kws-web",
  "name": "KWS Web Application",
  "description": "Public client for KWS web user interface",
  "enabled": true,
  "publicClient": true,
  "redirectUris": [
    "http://localhost:*/*",
    "http://127.0.0.1:*/*",
    "http://kws.local:*/*",
    "https://kws.local:*/*"
  ],
  "webOrigins": [
    "http://localhost:3000",
    "http://localhost:8000",
    "http://localhost:8080",
    "http://127.0.0.1:3000",
    "http://127.0.0.1:8000",
    "http://kws.local",
    "https://kws.local"
  ],
  "standardFlowEnabled": true,
  "implicitFlowEnabled": false,
  "directAccessGrantsEnabled": false,
  "serviceAccountsEnabled": false,
  "protocol": "openid-connect",
  "attributes": {
    "pkce.code.challenge.method": "S256"
  }
}
EOF
	)

	keycloak_api POST "/realms/${KWS_REALM}/clients" "$client_config" 2>/dev/null || true
	log_info "KWS web client created"
}

# Create KWS API Client (confidential client for backend)
create_kws_api_client() {
	log_info "Creating KWS API client..."

	local client_config
	client_config=$(
		cat <<EOF
{
  "clientId": "kws-api",
  "name": "KWS API Service",
  "description": "Confidential client for KWS backend API",
  "enabled": true,
  "publicClient": false,
  "serviceAccountsEnabled": true,
  "authorizationServicesEnabled": true,
  "standardFlowEnabled": false,
  "implicitFlowEnabled": false,
  "directAccessGrantsEnabled": true,
  "clientAuthenticatorType": "client-secret",
  "protocol": "openid-connect"
}
EOF
	)

	keycloak_api POST "/realms/${KWS_REALM}/clients" "$client_config" 2>/dev/null || true
	log_info "KWS API client created"
}

# Create custom protocol mappers for tenant claims
create_protocol_mappers() {
	log_info "Creating protocol mappers for custom claims..."

	# Get the kws-web client UUID
	local clients
	clients=$(keycloak_api GET "/realms/${KWS_REALM}/clients?clientId=kws-web")
	local web_client_uuid
	web_client_uuid=$(echo "$clients" | jq -r '.[0].id // empty')

	if [[ -z "$web_client_uuid" ]]; then
		log_warn "Could not find kws-web client, skipping protocol mapper creation"
		return 0
	fi

	# Tenant ID mapper
	local tenant_mapper
	tenant_mapper=$(
		cat <<EOF
{
  "name": "tenant_id",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-attribute-mapper",
  "config": {
    "user.attribute": "tenant_id",
    "claim.name": "tenant_id",
    "jsonType.label": "String",
    "id.token.claim": "true",
    "access.token.claim": "true",
    "userinfo.token.claim": "true"
  }
}
EOF
	)

	keycloak_api POST "/realms/${KWS_REALM}/clients/${web_client_uuid}/protocol-mappers/models" "$tenant_mapper" 2>/dev/null || true

	# Assigned regions mapper
	local regions_mapper
	regions_mapper=$(
		cat <<EOF
{
  "name": "assigned_regions",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-attribute-mapper",
  "config": {
    "user.attribute": "assigned_regions",
    "claim.name": "assigned_regions",
    "jsonType.label": "JSON",
    "id.token.claim": "true",
    "access.token.claim": "true",
    "userinfo.token.claim": "true",
    "multivalued": "true"
  }
}
EOF
	)

	keycloak_api POST "/realms/${KWS_REALM}/clients/${web_client_uuid}/protocol-mappers/models" "$regions_mapper" 2>/dev/null || true

	# Assigned sites mapper
	local sites_mapper
	sites_mapper=$(
		cat <<EOF
{
  "name": "assigned_sites",
  "protocol": "openid-connect",
  "protocolMapper": "oidc-usermodel-attribute-mapper",
  "config": {
    "user.attribute": "assigned_sites",
    "claim.name": "assigned_sites",
    "jsonType.label": "JSON",
    "id.token.claim": "true",
    "access.token.claim": "true",
    "userinfo.token.claim": "true",
    "multivalued": "true"
  }
}
EOF
	)

	keycloak_api POST "/realms/${KWS_REALM}/clients/${web_client_uuid}/protocol-mappers/models" "$sites_mapper" 2>/dev/null || true

	log_info "Protocol mappers created successfully"
}

# Create a test platform admin user
create_platform_admin_user() {
	log_info "Creating platform admin user for testing..."

	local user_config
	user_config=$(
		cat <<EOF
{
  "username": "platform-admin",
  "email": "admin@kws.local",
  "enabled": true,
  "emailVerified": true,
  "firstName": "Platform",
  "lastName": "Admin",
  "credentials": [
    {
      "type": "password",
      "value": "admin",
      "temporary": false
    }
  ],
  "attributes": {
    "tenant_id": ["platform"]
  }
}
EOF
	)

	keycloak_api POST "/realms/${KWS_REALM}/users" "$user_config" 2>/dev/null || true

	# Get the user ID
	local users
	users=$(keycloak_api GET "/realms/${KWS_REALM}/users?username=platform-admin")
	local user_id
	user_id=$(echo "$users" | jq -r '.[0].id // empty')

	if [[ -n "$user_id" ]]; then
		# Assign platform_admin role
		local role
		role=$(keycloak_api GET "/realms/${KWS_REALM}/roles/platform_admin" 2>/dev/null)
		if [[ -n "$role" ]]; then
			keycloak_api POST "/realms/${KWS_REALM}/users/${user_id}/role-mappings/realm" "[$role]" 2>/dev/null || true
			log_info "Platform admin role assigned"
		fi
	fi

	log_info "Platform admin user created (username: platform-admin, password: admin)"
}

# Print summary
print_summary() {
	echo ""
	log_info "=== Keycloak Setup Complete ==="
	echo ""
	echo "Keycloak URL:     ${KEYCLOAK_URL}"
	echo "Platform Realm:   ${KWS_REALM}"
	echo ""
	echo "Clients created:"
	echo "  - kws-web (public client for web UI)"
	echo "  - kws-api (confidential client for backend)"
	echo ""
	echo "Roles created:"
	echo "  - platform_admin (top-level, for KWS platform operations)"
	echo "  - tenant_owner -> tenant_admin"
	echo "  - tenant_admin -> regional_manager, recipe_manager, analytics_viewer"
	echo "  - regional_manager -> site_manager"
	echo "  - site_manager -> kitchen_operator"
	echo "  - recipe_manager -> recipe_editor"
	echo ""
	echo "Test user:"
	echo "  - Username: platform-admin"
	echo "  - Password: admin"
	echo ""
	echo "Keycloak Admin Console: ${KEYCLOAK_URL}/admin"
	echo ""
}

# Main execution
main() {
	log_info "Starting Keycloak setup for KWS..."

	if [[ "$DRY_RUN" == "true" ]]; then
		log_warn "Running in DRY-RUN mode - no changes will be made"
	fi

	wait_for_keycloak
	get_admin_token
	create_kws_platform_realm
	create_realm_roles
	create_kws_web_client
	create_kws_api_client
	create_protocol_mappers
	create_platform_admin_user
	print_summary

	log_info "Keycloak setup completed successfully!"
}

main
