#!/bin/bash
#
# migrate-users.sh - Migrate users from standard local connector to enhanced local connector
#
# Usage:
#   DRY_RUN=1 ./migrate-users.sh  # Preview migration without writing files
#   ./migrate-users.sh            # Perform actual migration
#
# Requirements:
#   - jq (JSON processor)
#   - sha256sum
#   - Exported user data from old connector (JSON format)
#
# Configuration:
#   EXPORT_FILE - Path to exported users JSON file
#   DATA_DIR    - Enhanced local connector data directory
#   DRY_RUN     - Set to 1 to preview without writing files
#

set -euo pipefail

# Configuration
EXPORT_FILE="${EXPORT_FILE:-/tmp/dex-users-export.json}"
DATA_DIR="${DATA_DIR:-/var/lib/dex/data}"
USERS_DIR="$DATA_DIR/users"
DRY_RUN="${DRY_RUN:-0}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check for jq
    if ! command -v jq &> /dev/null; then
        log_error "jq is not installed. Install it with: sudo apt-get install jq (or brew install jq)"
        exit 1
    fi

    # Check for sha256sum (or shasum on macOS)
    if ! command -v sha256sum &> /dev/null && ! command -v shasum &> /dev/null; then
        log_error "sha256sum/shasum is not installed."
        exit 1
    fi

    # Check if export file exists
    if [ ! -f "$EXPORT_FILE" ]; then
        log_error "Export file not found: $EXPORT_FILE"
        log_info "Please export users from old connector first."
        exit 1
    fi

    # Validate JSON format
    if ! jq empty "$EXPORT_FILE" 2>/dev/null; then
        log_error "Export file is not valid JSON: $EXPORT_FILE"
        exit 1
    fi

    log_success "Prerequisites check passed"
}

# Generate deterministic user ID from email
generate_user_id() {
    local email="$1"
    local hash

    # Use sha256sum or shasum depending on platform
    if command -v sha256sum &> /dev/null; then
        hash=$(echo -n "$email" | sha256sum | awk '{print $1}')
    else
        hash=$(echo -n "$email" | shasum -a 256 | awk '{print $1}')
    fi

    # Format as UUID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    echo "$hash" | sed 's/\(........\)\(....\)\(....\)\(....\)\(............\)/\1-\2-\3-\4-\5/'
}

# Create directories
create_directories() {
    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY RUN] Would create directories:"
        log_info "  - $DATA_DIR"
        log_info "  - $USERS_DIR"
        return
    fi

    log_info "Creating directories..."
    mkdir -p "$DATA_DIR"
    mkdir -p "$USERS_DIR"

    # Set permissions
    chmod 700 "$DATA_DIR"
    chmod 700 "$USERS_DIR"

    log_success "Directories created"
}

# Migrate single user
migrate_user() {
    local user_json="$1"
    local email username display_name hash user_id output_file

    # Extract user data
    email=$(echo "$user_json" | jq -r '.email')
    username=$(echo "$user_json" | jq -r '.username // .email')
    hash=$(echo "$user_json" | jq -r '.hash // ""')

    # Generate deterministic user ID
    user_id=$(generate_user_id "$email")
    output_file="$USERS_DIR/$user_id.json"

    # Use username as display name if available, otherwise email
    display_name="$username"

    # Create timestamp
    created_at=$(date -Iseconds)

    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY RUN] Would migrate: $email → $user_id"
        return 0
    fi

    # Create user JSON file
    cat > "$output_file" <<EOF
{
  "id": "$user_id",
  "email": "$email",
  "username": "$username",
  "display_name": "$display_name",
  "email_verified": true,
  "password_hash": $(if [ -n "$hash" ]; then echo "\"$hash\""; else echo "null"; fi),
  "passkeys": [],
  "totp_secret": null,
  "totp_enabled": false,
  "backup_codes": [],
  "magic_link_enabled": true,
  "require_2fa": false,
  "created_at": "$created_at",
  "updated_at": "$created_at",
  "last_login_at": null
}
EOF

    # Set file permissions
    chmod 600 "$output_file"

    log_success "Migrated: $email → $user_id"
    return 0
}

# Migrate all users
migrate_all_users() {
    local total_users migrated_users failed_users

    total_users=$(jq -r '. | length' "$EXPORT_FILE")
    migrated_users=0
    failed_users=0

    log_info "Migrating $total_users users from $EXPORT_FILE..."

    # Read users array and process each user
    jq -c '.[]' "$EXPORT_FILE" | while read -r user; do
        if migrate_user "$user"; then
            ((migrated_users++)) || true
        else
            ((failed_users++)) || true
        fi
    done

    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY RUN] Migration preview complete"
        log_info "  Would migrate: $total_users users"
        return
    fi

    # Count actual migrated users
    migrated_count=$(ls -1 "$USERS_DIR"/*.json 2>/dev/null | wc -l | tr -d ' ')

    log_success "Migration complete!"
    log_info "  Total users in export: $total_users"
    log_info "  Users migrated: $migrated_count"

    if [ "$migrated_count" -ne "$total_users" ]; then
        log_warning "  Mismatch detected! Please review migration."
    fi
}

# Verify migration
verify_migration() {
    if [ "$DRY_RUN" = "1" ]; then
        return
    fi

    log_info "Verifying migration..."

    # Check directory permissions
    data_perm=$(stat -c '%a' "$DATA_DIR" 2>/dev/null || stat -f '%A' "$DATA_DIR")
    users_perm=$(stat -c '%a' "$USERS_DIR" 2>/dev/null || stat -f '%A' "$USERS_DIR")

    if [ "$data_perm" != "700" ]; then
        log_warning "Data directory permissions: $data_perm (expected 700)"
    fi

    if [ "$users_perm" != "700" ]; then
        log_warning "Users directory permissions: $users_perm (expected 700)"
    fi

    # Verify all user files have correct permissions
    log_info "Checking user file permissions..."
    local bad_perms=0
    for file in "$USERS_DIR"/*.json; do
        if [ -f "$file" ]; then
            file_perm=$(stat -c '%a' "$file" 2>/dev/null || stat -f '%A' "$file")
            if [ "$file_perm" != "600" ]; then
                log_warning "File $file has permissions $file_perm (expected 600)"
                ((bad_perms++)) || true
            fi
        fi
    done

    if [ $bad_perms -eq 0 ]; then
        log_success "All files have correct permissions"
    else
        log_warning "$bad_perms files have incorrect permissions"
    fi

    # Validate JSON syntax for all user files
    log_info "Validating JSON syntax..."
    local invalid_json=0
    for file in "$USERS_DIR"/*.json; do
        if [ -f "$file" ]; then
            if ! jq empty "$file" 2>/dev/null; then
                log_error "Invalid JSON in file: $file"
                ((invalid_json++)) || true
            fi
        fi
    done

    if [ $invalid_json -eq 0 ]; then
        log_success "All user files have valid JSON"
    else
        log_error "$invalid_json files have invalid JSON"
    fi

    # Sample a few users to display
    log_info "Sample migrated users:"
    local count=0
    for file in "$USERS_DIR"/*.json; do
        if [ -f "$file" ] && [ $count -lt 3 ]; then
            email=$(jq -r '.email' "$file")
            user_id=$(jq -r '.id' "$file")
            has_password=$(jq -r '.password_hash != null' "$file")
            log_info "  - $email ($user_id) [password: $has_password]"
            ((count++)) || true
        fi
    done
}

# Print summary
print_summary() {
    local user_count

    echo ""
    echo "=========================================="
    echo "Migration Summary"
    echo "=========================================="
    echo ""

    if [ "$DRY_RUN" = "1" ]; then
        echo "Mode: DRY RUN (no files written)"
    else
        echo "Mode: LIVE MIGRATION"
        user_count=$(ls -1 "$USERS_DIR"/*.json 2>/dev/null | wc -l | tr -d ' ')
        echo "Users migrated: $user_count"
        echo "Data directory: $DATA_DIR"
        echo "Users directory: $USERS_DIR"
    fi

    echo ""
    echo "Next steps:"
    echo "1. Verify migrated data: ls -la $USERS_DIR"
    echo "2. Inspect sample user: cat $USERS_DIR/*.json | jq . | head -30"
    echo "3. Update Dex configuration to use enhanced local connector"
    echo "4. Restart Dex: sudo systemctl restart dex"
    echo "5. Test authentication with migrated users"
    echo ""
    echo "=========================================="
}

# Main execution
main() {
    echo ""
    echo "=========================================="
    echo "Dex User Migration Tool"
    echo "Enhanced Local Connector"
    echo "=========================================="
    echo ""

    if [ "$DRY_RUN" = "1" ]; then
        log_warning "Running in DRY RUN mode - no files will be written"
    fi

    check_prerequisites
    create_directories
    migrate_all_users
    verify_migration
    print_summary
}

# Run main function
main
