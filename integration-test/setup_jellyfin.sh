#!/bin/bash
set -e

# Configuration
JELLYFIN_URL="${JELLYFIN_URL:-http://localhost:8096}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123}"
LANGUAGE="${LANGUAGE:-en-US}"
MAX_RETRIES=60
RETRY_DELAY=2

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

print_info() {
    echo -e "${BLUE}[i]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

# Wait for Jellyfin to be ready
wait_for_jellyfin() {
    print_info "Waiting for Jellyfin to be ready at $JELLYFIN_URL..."
    
    for i in $(seq 1 $MAX_RETRIES); do
        if curl -sf "$JELLYFIN_URL/health" >/dev/null 2>&1 || \
           curl -sf "$JELLYFIN_URL/System/Info/Public" >/dev/null 2>&1; then
            print_status "Jellyfin is ready!"
            return 0
        fi
        
        if [ $((i % 10)) -eq 0 ]; then
            print_info "Still waiting... ($i/$MAX_RETRIES)"
        fi
        sleep $RETRY_DELAY
    done
    
    print_error "Jellyfin failed to start after $((MAX_RETRIES * RETRY_DELAY)) seconds"
    return 1
}

# Check if setup wizard is needed
check_setup_status() {
    print_info "Checking if setup wizard is needed..."
    
    # Try to get startup configuration
    RESPONSE=$(curl -sf "$JELLYFIN_URL/Startup/Configuration" 2>/dev/null || echo "")
    
    if [ -z "$RESPONSE" ]; then
        # If Configuration endpoint fails, check if we can get system info without auth
        if curl -sf "$JELLYFIN_URL/System/Info/Public" >/dev/null 2>&1; then
            print_status "Setup wizard already completed"
            return 1
        fi
    fi
    
    # Check if startup/user endpoint exists (means wizard not completed)
    if curl -sf -X GET "$JELLYFIN_URL/Startup/User" >/dev/null 2>&1; then
        print_info "Setup wizard needs to be completed"
        return 0
    fi
    
    print_status "Setup wizard already completed"
    return 1
}

# Step 1: Set preferred language
set_language() {
    print_info "Setting preferred language to $LANGUAGE..."
    
    RESPONSE=$(curl -s -X POST "$JELLYFIN_URL/Startup/Configuration" \
        -H "Content-Type: application/json" \
        -d "{
            \"UICulture\": \"$LANGUAGE\",
            \"MetadataCountryCode\": \"US\",
            \"PreferredMetadataLanguage\": \"en\"
        }")
    
    if [ $? -eq 0 ]; then
        print_status "Language set successfully"
        return 0
    else
        print_warning "Failed to set language (may not be critical)"
        return 0  # Don't fail on this
    fi
}

# Step 2: Create admin user
create_admin_user() {
    print_info "Creating admin user: $USERNAME"
    
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$JELLYFIN_URL/Startup/User" \
        -H "Content-Type: application/json" \
        -d "{
            \"Name\": \"$USERNAME\",
            \"Password\": \"$PASSWORD\"
        }")
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n-1)
    
    if [ "$HTTP_CODE" = "204" ] || [ "$HTTP_CODE" = "200" ]; then
        print_status "Admin user created successfully"
        return 0
    else
        print_error "Failed to create admin user (HTTP $HTTP_CODE)"
        echo "Response: $BODY"
        return 1
    fi
}

# Step 3: Complete startup wizard
complete_wizard() {
    print_info "Completing startup wizard..."
    
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$JELLYFIN_URL/Startup/Complete" \
        -H "Content-Type: application/json" \
        -d "{}")
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    
    if [ "$HTTP_CODE" = "204" ] || [ "$HTTP_CODE" = "200" ]; then
        print_status "Startup wizard completed"
        return 0
    else
        print_error "Failed to complete wizard (HTTP $HTTP_CODE)"
        return 1
    fi
}

# Step 4: Authenticate and get access token
authenticate() {
    print_info "Authenticating as $USERNAME..." >&2
    
    RESPONSE=$(curl -s -X POST "$JELLYFIN_URL/Users/AuthenticateByName" \
        -H "Content-Type: application/json" \
        -H "X-Emby-Authorization: MediaBrowser Client=\"OxiCleanarr-Setup\", Device=\"IntegrationTest\", DeviceId=\"setup-script\", Version=\"1.0.0\"" \
        -d "{
            \"Username\": \"$USERNAME\",
            \"Pw\": \"$PASSWORD\"
        }")
    
    USER_ID=$(echo "$RESPONSE" | jq -r '.User.Id // empty' 2>/dev/null)
    ACCESS_TOKEN=$(echo "$RESPONSE" | jq -r '.AccessToken // empty' 2>/dev/null)
    
    if [ -z "$ACCESS_TOKEN" ] || [ "$ACCESS_TOKEN" = "null" ]; then
        print_error "Failed to authenticate" >&2
        echo "Response: $RESPONSE" >&2
        return 1
    fi
    
    print_status "Authentication successful" >&2
    echo "$USER_ID"
    echo "$ACCESS_TOKEN"
    return 0
}

# Step 5: Create API key
create_api_key() {
    local ACCESS_TOKEN=$1
    
    print_info "Creating API key for OxiCleanarr..." >&2
    
    # First check if API key already exists
    EXISTING_KEYS=$(curl -s -X GET "$JELLYFIN_URL/Auth/Keys" \
        -H "X-Emby-Token: $ACCESS_TOKEN")
    
    API_KEY=$(echo "$EXISTING_KEYS" | jq -r '.Items[] | select(.AppName == "OxiCleanarr") | .AccessToken' 2>/dev/null)
    
    if [ ! -z "$API_KEY" ] && [ "$API_KEY" != "null" ]; then
        print_status "API key already exists (reusing existing key)" >&2
        echo "$API_KEY"
        return 0
    fi
    
    # Create new API key (POST returns empty response in Jellyfin 10.11+)
    curl -s -X POST "$JELLYFIN_URL/Auth/Keys?app=OxiCleanarr" \
        -H "X-Emby-Token: $ACCESS_TOKEN" > /dev/null
    
    # Query the key list to get the newly created key
    sleep 1  # Give Jellyfin a moment to create the key
    ALL_KEYS=$(curl -s -X GET "$JELLYFIN_URL/Auth/Keys" \
        -H "X-Emby-Token: $ACCESS_TOKEN")
    
    API_KEY=$(echo "$ALL_KEYS" | jq -r '.Items[] | select(.AppName == "OxiCleanarr") | .AccessToken' 2>/dev/null)
    
    if [ -z "$API_KEY" ] || [ "$API_KEY" = "null" ]; then
        print_error "Failed to create API key" >&2
        echo "All keys: $ALL_KEYS" >&2
        return 1
    fi
    
    print_status "API key created successfully" >&2
    echo "$API_KEY"
    return 0
}

# Step 6: Add media library
add_media_library() {
    local ACCESS_TOKEN=$1
    local LIBRARY_NAME="${2:-Test Movies}"
    local LIBRARY_PATH="${3:-/media/movies}"
    local CONTENT_TYPE="${4:-movies}"
    
    print_info "Adding media library: $LIBRARY_NAME ($LIBRARY_PATH)..."
    
    # Get library options first (required for proper setup)
    OPTIONS=$(curl -s -X GET "$JELLYFIN_URL/Library/VirtualFolders/LibraryOptions?CollectionType=$CONTENT_TYPE" \
        -H "X-Emby-Token: $ACCESS_TOKEN")
    
    # Create the library
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$JELLYFIN_URL/Library/VirtualFolders?collectionType=$CONTENT_TYPE&name=$(echo $LIBRARY_NAME | jq -sRr @uri)&refreshLibrary=true" \
        -H "Content-Type: application/json" \
        -H "X-Emby-Token: $ACCESS_TOKEN" \
        -d "{
            \"LibraryOptions\": {
                \"EnablePhotos\": true,
                \"EnableRealtimeMonitor\": false,
                \"EnableChapterImageExtraction\": false,
                \"ExtractChapterImagesDuringLibraryScan\": false,
                \"PathInfos\": [
                    {
                        \"Path\": \"$LIBRARY_PATH\"
                    }
                ],
                \"SaveLocalMetadata\": false,
                \"EnableAutomaticSeriesGrouping\": false
            }
        }")
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    
    if [ "$HTTP_CODE" = "204" ] || [ "$HTTP_CODE" = "200" ]; then
        print_status "Media library '$LIBRARY_NAME' created"
        return 0
    else
        print_warning "Failed to create media library (HTTP $HTTP_CODE)"
        return 0  # Don't fail if library creation fails
    fi
}

# Step 7: Wait for library scan to complete (optional)
wait_for_library_scan() {
    local ACCESS_TOKEN=$1
    local MAX_WAIT=30
    
    print_info "Waiting for initial library scan..."
    
    for i in $(seq 1 $MAX_WAIT); do
        # Check if any scan tasks are running
        TASKS=$(curl -s "$JELLYFIN_URL/ScheduledTasks" \
            -H "X-Emby-Token: $ACCESS_TOKEN")
        
        SCANNING=$(echo "$TASKS" | jq -r '[.[] | select(.Name | contains("Scan")) | select(.State == "Running")] | length')
        
        if [ "$SCANNING" = "0" ]; then
            print_status "Library scan completed"
            return 0
        fi
        
        if [ $((i % 5)) -eq 0 ]; then
            print_info "Still scanning... ($i/$MAX_WAIT seconds)"
        fi
        sleep 1
    done
    
    print_warning "Library scan still in progress (continuing anyway)"
    return 0
}

# Main execution
main() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║         Jellyfin Automated Setup Script                   ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo ""
    echo "Configuration:"
    echo "  URL:      $JELLYFIN_URL"
    echo "  Username: $USERNAME"
    echo "  Language: $LANGUAGE"
    echo ""
    
    # Wait for Jellyfin to start
    if ! wait_for_jellyfin; then
        exit 1
    fi
    
    # Check if setup is needed
    if ! check_setup_status; then
        print_warning "Jellyfin is already set up"
        print_info "Attempting to authenticate with existing credentials..."
        
        AUTH_RESULT=$(authenticate)
        if [ $? -eq 0 ]; then
            USER_ID=$(echo "$AUTH_RESULT" | sed -n '1p')
            ACCESS_TOKEN=$(echo "$AUTH_RESULT" | sed -n '2p')
            
            print_status "Successfully authenticated with existing setup"
            
            # Try to create API key if it doesn't exist
            API_KEY=$(create_api_key "$ACCESS_TOKEN")
            if [ $? -eq 0 ]; then
                echo ""
                echo "═══════════════════════════════════════════════════════════"
                echo "Setup Summary:"
                echo "═══════════════════════════════════════════════════════════"
                echo "  URL:       $JELLYFIN_URL"
                echo "  Username:  $USERNAME"
                echo "  User ID:   $USER_ID"
                echo "  API Key:   $API_KEY"
                echo "═══════════════════════════════════════════════════════════"
                echo ""
                
                # Export for use by other scripts
                echo "export JELLYFIN_API_KEY=\"$API_KEY\""
                echo "export JELLYFIN_USER_ID=\"$USER_ID\""
                exit 0
            fi
        fi
        
        print_error "Cannot proceed with existing setup and authentication failed"
        exit 1
    fi
    
    # Run setup wizard
    echo ""
    print_info "Running automated setup wizard..."
    echo ""
    
    # Set language (optional, can fail)
    set_language
    
    # Create admin user (required)
    if ! create_admin_user; then
        exit 1
    fi
    
    # Complete wizard (required)
    if ! complete_wizard; then
        exit 1
    fi
    
    # Give Jellyfin a moment to finish setup
    sleep 2
    
    # Authenticate (required for API key creation)
    AUTH_RESULT=$(authenticate)
    if [ $? -ne 0 ]; then
        exit 1
    fi
    
    USER_ID=$(echo "$AUTH_RESULT" | sed -n '1p')
    ACCESS_TOKEN=$(echo "$AUTH_RESULT" | sed -n '2p')
    
    # Create API key (required for OxiCleanarr)
    API_KEY=$(create_api_key "$ACCESS_TOKEN")
    if [ $? -ne 0 ]; then
        exit 1
    fi
    
    # Add media library (optional)
    if [ ! -z "$JELLYFIN_LIBRARY_PATH" ]; then
        add_media_library "$ACCESS_TOKEN" "Test Movies" "$JELLYFIN_LIBRARY_PATH" "movies"
        
        # Optionally wait for scan
        if [ "$JELLYFIN_WAIT_FOR_SCAN" = "true" ]; then
            wait_for_library_scan "$ACCESS_TOKEN"
        fi
    fi
    
    echo ""
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║              Setup Completed Successfully! ✓              ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo ""
    echo "═══════════════════════════════════════════════════════════"
    echo "Setup Summary:"
    echo "═══════════════════════════════════════════════════════════"
    echo "  URL:       $JELLYFIN_URL"
    echo "  Username:  $USERNAME"
    echo "  Password:  $PASSWORD"
    echo "  User ID:   $USER_ID"
    echo "  API Key:   $API_KEY"
    echo "═══════════════════════════════════════════════════════════"
    echo ""
    echo "Environment variables for integration scripts:"
    echo "  export JELLYFIN_API_KEY=\"$API_KEY\""
    echo "  export JELLYFIN_USER_ID=\"$USER_ID\""
    echo ""
    
    # Save to file for later use
    if [ ! -z "$JELLYFIN_CONFIG_OUTPUT" ]; then
        cat > "$JELLYFIN_CONFIG_OUTPUT" << EOF
JELLYFIN_URL=$JELLYFIN_URL
JELLYFIN_USERNAME=$USERNAME
JELLYFIN_PASSWORD=$PASSWORD
JELLYFIN_USER_ID=$USER_ID
JELLYFIN_API_KEY=$API_KEY
EOF
        print_status "Configuration saved to: $JELLYFIN_CONFIG_OUTPUT"
    fi
}

# Run main function
main "$@"
