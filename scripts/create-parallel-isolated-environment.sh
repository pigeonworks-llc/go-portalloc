#!/bin/bash
# scripts/create-parallel-isolated-environment.sh
# Enhanced Environment Isolation for Parallel Development
# Task 2.2: Parallel Development Validation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Configuration parameters
WORKTREE_PATH="${1:-$(pwd)}"
PARALLEL_INSTANCE="${2:-$(date +%s%N | cut -c1-10)}"  # Nanosecond timestamp for uniqueness
LOCK_DIR="/tmp/aigis-isolation-locks"
MAX_RETRIES=10
RETRY_DELAY=1

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Create lock directory
mkdir -p "$LOCK_DIR"

# Generate highly unique isolation ID
generate_isolation_id() {
    # Add multiple sources of randomness for parallel execution
    local timestamp=$(date +%s%N)  # Nanosecond timestamp
    local random_component=$RANDOM$RANDOM$RANDOM  # Multiple random numbers
    local process_id=$$
    local base_input="$WORKTREE_PATH-$PARALLEL_INSTANCE-$timestamp-$random_component-$(hostname)-$process_id"
    local full_hash=$(echo "$base_input" | sha256sum | cut -c1-12)

    # Add additional uniqueness checks with expanded range
    local isolation_id="$full_hash"
    local counter=1

    while [ -f "$LOCK_DIR/env-$isolation_id.lock" ] || [ -d "/tmp/aigis-test-$isolation_id" ]; do
        # Add more randomness for collision resolution
        local additional_random=$(date +%N | cut -c1-4)$RANDOM
        isolation_id="${full_hash}${additional_random}$(printf "%03d" $counter)"
        counter=$((counter + 1))

        if [ $counter -gt 999 ]; then
            log_error "Unable to generate unique isolation ID after 999 attempts"
            return 1
        fi

        # Add small delay to ensure different timestamps
        sleep 0.001 2>/dev/null || true
    done

    echo "$isolation_id"
}

# Find available port range
find_available_port_range() {
    local start_port="$1"
    local ports_needed="$2"

    for attempt in $(seq 1 $MAX_RETRIES); do
        local base_port=$((start_port + (RANDOM % 10000)))
        local all_available=true

        # Check if all required ports are available
        for i in $(seq 1 $ports_needed); do
            local port=$((base_port + i))
            if lsof -Pi :$port -sTCP:LISTEN >/dev/null 2>&1; then
                all_available=false
                break
            fi
        done

        if [ "$all_available" = true ]; then
            echo "$base_port"
            return 0
        fi

        # Wait before retry
        sleep $RETRY_DELAY
    done

    log_error "Unable to find available port range after $MAX_RETRIES attempts"
    return 1
}

# Create environment lock
create_environment_lock() {
    local isolation_id="$1"
    local lock_file="$LOCK_DIR/env-$isolation_id.lock"

    # Atomic lock creation
    if (set -C; echo "$$" > "$lock_file") 2>/dev/null; then
        echo "$lock_file"
        return 0
    else
        log_error "Failed to acquire environment lock: $isolation_id"
        return 1
    fi
}

# Release environment lock
release_environment_lock() {
    local lock_file="$1"
    if [ -f "$lock_file" ]; then
        rm -f "$lock_file"
        log_info "Environment lock released: $(basename "$lock_file")"
    fi
}

# Main environment creation function
create_parallel_environment() {
    log_info "=== AIGIS Parallel Environment Isolation ==="
    log_info "Creating isolated environment for: $WORKTREE_PATH"
    log_info "Parallel instance: $PARALLEL_INSTANCE"

    # Generate unique isolation ID
    local isolation_id
    if ! isolation_id=$(generate_isolation_id); then
        return 1
    fi

    log_info "Generated isolation ID: $isolation_id"

    # Create environment lock
    local lock_file
    if ! lock_file=$(create_environment_lock "$isolation_id"); then
        return 1
    fi

    # Set up cleanup trap
    trap "release_environment_lock '$lock_file'" EXIT

    # Find available port range (need 5 ports)
    local base_port
    if ! base_port=$(find_available_port_range 20000 5); then
        return 1
    fi

    log_info "Assigned port range: $base_port-$((base_port + 5))"

    # Calculate individual service ports
    local firestore_port=$((base_port + 1))
    local auth_port=$((base_port + 2))
    local api_port=$((base_port + 3))
    local admin_api_port=$((base_port + 4))
    local user_api_port=$((base_port + 5))

    # Create environment configuration
    cat > ".env.isolation" << EOL
# AIGIS Monolith Isolated Environment Configuration
ISOLATION_ID=$isolation_id
WORKTREE_PATH="$WORKTREE_PATH"
PARALLEL_INSTANCE=$PARALLEL_INSTANCE
ENVIRONMENT=test

# Service Ports
FIRESTORE_EMULATOR_HOST=localhost:$firestore_port
FIREBASE_AUTH_EMULATOR_HOST=localhost:$auth_port
API_SERVER_PORT=$api_port
ADMIN_API_PORT=$admin_api_port
USER_API_PORT=$user_api_port

# Database Configuration
DATABASE_TYPE=memory
USE_FIRESTORE_EMULATOR=true
TEST_DATA_DIR=/tmp/aigis-test-$isolation_id

# Testing Configuration
TEST_TIMEOUT=8m
UNIT_TEST_TIMEOUT=2m
JWT_SECRET=test-secret-$isolation_id
EMAIL_PROVIDER=mock

# Parallel Testing Configuration
TEST_PARALLEL=true
TEST_INSTANCE_ID=$isolation_id
PARALLEL_LOCK_FILE=$lock_file
EOL

    # Create data directories with detailed structure
    local test_data_root="/tmp/aigis-test-$isolation_id"
    mkdir -p "$test_data_root"/{firestore,auth,api,logs,coverage,artifacts,tmp}

    # Set permissions
    chmod 755 "$test_data_root"
    chmod -R 755 "$test_data_root"/*

    # Create environment status file
    cat > "$test_data_root/environment.status" << EOL
ISOLATION_ID=$isolation_id
CREATED_AT=$(date -Iseconds)
CREATED_BY=$$
WORKTREE_PATH=$WORKTREE_PATH
BASE_PORT=$base_port
FIRESTORE_PORT=$firestore_port
AUTH_PORT=$auth_port
API_PORT=$api_port
ADMIN_API_PORT=$admin_api_port
USER_API_PORT=$user_api_port
LOCK_FILE=$lock_file
EOL

    # Create enhanced cleanup script
    mkdir -p scripts
    cat > "scripts/cleanup-environment-$isolation_id.sh" << EOL
#!/bin/bash
# Cleanup script for environment: $isolation_id
set -e

echo "Cleaning up environment: $isolation_id"

# Kill any processes using our ports
for port in $firestore_port $auth_port $api_port $admin_api_port $user_api_port; do
    if lsof -Pi :\$port -sTCP:LISTEN >/dev/null 2>&1; then
        echo "Killing processes on port: \$port"
        lsof -ti :\$port | xargs kill -9 2>/dev/null || true
    fi
done

# Clean up data directory
if [ -d "$test_data_root" ]; then
    echo "Removing data directory: $test_data_root"
    rm -rf "$test_data_root"
fi

# Clean up lock file
if [ -f "$lock_file" ]; then
    echo "Releasing environment lock"
    rm -f "$lock_file"
fi

# Clean up any temporary files
rm -f "/tmp/aigis-*-$isolation_id-*" 2>/dev/null || true

echo "Environment $isolation_id cleaned up successfully"
EOL

    chmod +x "scripts/cleanup-environment-$isolation_id.sh"

    # Create environment validation script
    cat > "scripts/validate-environment-$isolation_id.sh" << EOL
#!/bin/bash
# Validation script for environment: $isolation_id
set -e

echo "Validating environment: $isolation_id"

# Check lock file
if [ ! -f "$lock_file" ]; then
    echo "❌ Lock file missing: $lock_file"
    exit 1
fi

# Check data directory
if [ ! -d "$test_data_root" ]; then
    echo "❌ Data directory missing: $test_data_root"
    exit 1
fi

# Check port availability
for port in $firestore_port $auth_port $api_port $admin_api_port $user_api_port; do
    if lsof -Pi :\$port -sTCP:LISTEN >/dev/null 2>&1; then
        echo "⚠️  Port \$port is in use"
    else
        echo "✅ Port \$port is available"
    fi
done

# Check environment file
if [ ! -f ".env.isolation" ]; then
    echo "❌ Environment file missing"
    exit 1
fi

echo "✅ Environment $isolation_id validation completed"
EOL

    chmod +x "scripts/validate-environment-$isolation_id.sh"

    # Create environment info script
    cat > "scripts/info-environment-$isolation_id.sh" << EOL
#!/bin/bash
# Environment info for: $isolation_id

echo "=== Environment Information ==="
echo "Isolation ID: $isolation_id"
echo "Worktree Path: $WORKTREE_PATH"
echo "Parallel Instance: $PARALLEL_INSTANCE"
echo "Created: \$(date -r $test_data_root/environment.status)"
echo "Base Port: $base_port"
echo ""
echo "=== Service Ports ==="
echo "Firestore Emulator: $firestore_port"
echo "Firebase Auth: $auth_port"
echo "API Server: $api_port"
echo "Admin API: $admin_api_port"
echo "User API: $user_api_port"
echo ""
echo "=== Data Directory ==="
echo "Path: $test_data_root"
echo "Size: \$(du -sh $test_data_root 2>/dev/null | cut -f1 || echo 'N/A')"
echo ""
echo "=== Environment Variables ==="
source .env.isolation
env | grep -E "(ISOLATION_ID|FIRESTORE|API_PORT|TEST_)" | sort
EOL

    chmod +x "scripts/info-environment-$isolation_id.sh"

    # Log success
    log_success "Environment created successfully!"
    log_info "Configuration: .env.isolation"
    log_info "Cleanup: scripts/cleanup-environment-$isolation_id.sh"
    log_info "Validation: scripts/validate-environment-$isolation_id.sh"
    log_info "Info: scripts/info-environment-$isolation_id.sh"
    log_info ""
    log_info "To use: source .env.isolation"

    # Don't release lock on success - it will be released by cleanup
    trap - EXIT

    return 0
}

# Execute main function
create_parallel_environment "$@"