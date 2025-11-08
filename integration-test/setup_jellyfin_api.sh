#!/bin/bash
set -e

JELLYFIN_URL="http://localhost:8096"
USERNAME="admin"
PASSWORD="admin123"

echo "=== Jellyfin Automated Setup Script ==="
echo "URL: $JELLYFIN_URL"
echo "Username: $USERNAME"
echo ""

# Step 1: Create initial admin user
echo "Step 1: Creating admin user..."
RESPONSE=$(curl -s -X POST "$JELLYFIN_URL/Startup/User" \
  -H "Content-Type: application/json" \
  -d "{
    \"Name\": \"$USERNAME\",
    \"Password\": \"$PASSWORD\"
  }")

echo "Response: $RESPONSE"
echo ""

# Step 2: Complete startup wizard
echo "Step 2: Completing startup wizard..."
curl -s -X POST "$JELLYFIN_URL/Startup/Complete" \
  -H "Content-Type: application/json" \
  -d "{}"

echo "Startup wizard completed!"
echo ""

# Step 3: Authenticate and get API key
echo "Step 3: Authenticating to get session..."
AUTH_RESPONSE=$(curl -s -X POST "$JELLYFIN_URL/Users/AuthenticateByName" \
  -H "Content-Type: application/json" \
  -H "X-Emby-Authorization: MediaBrowser Client=\"OxiCleanarr\", Device=\"Setup Script\", DeviceId=\"setup-script\", Version=\"1.0.0\"" \
  -d "{
    \"Username\": \"$USERNAME\",
    \"Pw\": \"$PASSWORD\"
  }")

echo "Auth Response: $AUTH_RESPONSE"
echo ""

# Extract User ID and Access Token
USER_ID=$(echo "$AUTH_RESPONSE" | jq -r '.User.Id')
ACCESS_TOKEN=$(echo "$AUTH_RESPONSE" | jq -r '.AccessToken')

echo "User ID: $USER_ID"
echo "Access Token: $ACCESS_TOKEN"
echo ""

# Step 4: Create API key
echo "Step 4: Creating API key..."
API_KEY_RESPONSE=$(curl -s -X POST "$JELLYFIN_URL/Auth/Keys?app=OxiCleanarr" \
  -H "X-Emby-Token: $ACCESS_TOKEN")

echo "API Key Response: $API_KEY_RESPONSE"
echo ""

API_KEY=$(echo "$API_KEY_RESPONSE" | jq -r '.AccessToken')

echo "=== Setup Complete ==="
echo "API Key: $API_KEY"
echo ""
echo "Add this to your docker-compose.yml:"
echo "  JELLYFIN_API_KEY: \"$API_KEY\""
