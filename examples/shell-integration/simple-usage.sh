#!/bin/bash
# Example: Simple usage of go-parallel-test-env

set -euo pipefail

echo "📦 Creating isolated test environment..."

# Create environment with JSON output for easy parsing
ENV_JSON=$(go-parallel-test-env create --ports 5 --json)

# Parse environment details
ISOLATION_ID=$(echo "$ENV_JSON" | jq -r '.isolation_id')
BASE_PORT=$(echo "$ENV_JSON" | jq -r '.ports.base_port')
TEMP_DIR=$(echo "$ENV_JSON" | jq -r '.temp_dir')

echo "✅ Environment created:"
echo "   ID: $ISOLATION_ID"
echo "   Base Port: $BASE_PORT"
echo "   Temp Dir: $TEMP_DIR"
echo ""

# Cleanup on exit
trap "go-parallel-test-env cleanup --id $ISOLATION_ID" EXIT

# Load environment variables
source .env.isolation

echo "🧪 Running tests with isolated environment..."
echo "   FIRESTORE_PORT: $FIRESTORE_PORT"
echo "   AUTH_PORT: $AUTH_PORT"
echo "   API_PORT: $API_PORT"
echo ""

# Your test commands here
# go test ./...
# npm test
# pytest

echo "✅ Tests completed successfully!"
echo ""
echo "🔍 Validating environment..."
go-parallel-test-env validate --id "$ISOLATION_ID"

echo ""
echo "✅ Validation passed!"
