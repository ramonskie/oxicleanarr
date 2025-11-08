#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JELLYFIN_URL="http://localhost:8096"
RADARR_URL="http://localhost:7878"
OXICLEANARR_URL="http://localhost:8080"
PLUGIN_DIR="$SCRIPT_DIR/../../jellyfin-plugin-oxicleanarr"

print_header() {
    echo ""
    echo -e "${BLUE}=========================================="
    echo -e "$1"
    echo -e "==========================================${NC}"
    echo ""
}

print_status() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

print_info() {
    echo -e "${YELLOW}[i]${NC} $1"
}

print_step() {
    echo ""
    echo -e "${BLUE}Step $1: $2${NC}"
    echo "----------------------------------------"
}

wait_for_service() {
    local url=$1
    local name=$2
    local max_attempts=30
    local attempt=1
    
    print_info "Waiting for $name to be ready..."
    while [ $attempt -le $max_attempts ]; do
        if curl -s "$url" >/dev/null 2>&1; then
            print_status "$name is ready!"
            return 0
        fi
        echo -n "."
        sleep 2
        ((attempt++))
    done
    
    print_error "$name failed to start after $max_attempts attempts"
    return 1
}

cleanup() {
    local keep_media="${1:-true}"  # Default: preserve test-media (51MB)
    local keep_containers="${2:-false}"  # Default: stop containers
    
    print_header "Cleaning Up Test Environment"
    cd "$SCRIPT_DIR"
    
    # Stop containers unless keeping for debug
    if [ "$keep_containers" = "false" ]; then
        print_info "Stopping Docker containers..."
        docker compose down -v 2>/dev/null || true
    else
        print_info "Keeping containers running for debugging"
    fi
    
    # Always clean runtime data (configs, caches, databases)
    print_info "Cleaning runtime data..."
    rm -rf jellyfin-config jellyfin-cache radarr-config
    rm -rf oxicleanarr-config oxicleanarr-data oxicleanarr-logs
    rm -rf leaving-soon jellyfin.db
    
    # Optionally clean test media (50MB+ of dummy files)
    if [ "$keep_media" = "false" ]; then
        print_info "Removing test-media directory..."
        rm -rf test-media
    else
        print_info "Preserving test-media directory (use cleanup false false to remove)"
    fi
    
    print_status "Cleanup complete"
}

create_test_media() {
    print_info "Creating test media directory structure..."
    mkdir -p test-media/movies
    
    # Create 5 test movies
    print_info "Creating test movie files..."
    local created_count=0
    local skipped_count=0
    
    for i in {1..5}; do
        movie_dir="test-media/movies/Test Movie ${i} (2024)"
        movie_file="$movie_dir/Test Movie ${i} (2024).mkv"
        nfo_file="$movie_dir/Test Movie ${i} (2024).nfo"
        
        if [ ! -f "$movie_file" ]; then
            mkdir -p "$movie_dir"
            # Create a 10MB dummy video file
            dd if=/dev/zero of="$movie_file" bs=1M count=10 2>/dev/null
            ((created_count++))
            print_status "Created: Test Movie ${i} (2024).mkv"
        else
            ((skipped_count++))
            print_info "Test Movie ${i} already exists"
        fi
        
        # Create movie metadata (nfo files for better Jellyfin parsing)
        if [ ! -f "$nfo_file" ]; then
            cat > "$nfo_file" << EOF
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie>
    <title>Test Movie ${i}</title>
    <year>2024</year>
    <plot>This is a test movie for OxiCleanarr integration testing.</plot>
    <genre>Test</genre>
</movie>
EOF
        fi
    done
    
    if [ $created_count -gt 0 ]; then
        print_status "Created $created_count test movies (10MB each)"
    fi
    if [ $skipped_count -gt 0 ]; then
        print_status "Skipped $skipped_count existing movies"
    fi
}

# Main test flow
print_header "OxiCleanarr Integration Test"
echo "This test will:"
echo "  1. Build the Jellyfin plugin"
echo "  2. Create test movie files"
echo "  3. Start Jellyfin, Radarr, and OxiCleanarr"
echo "  4. Configure all services"
echo "  5. Test symlink library creation"
echo ""

# Ask if user wants to clean up first
read -p "Do you want to clean up any existing test environment? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    cleanup
fi

print_step 1 "Downloading Jellyfin Plugin"
PLUGIN_VERSION="v3.0.0"
PLUGIN_URL="https://github.com/ramonskie/jellyfin-plugin-oxicleanarr/releases/download/${PLUGIN_VERSION}/jellyfin-plugin-oxicleanarr.zip"
PLUGIN_TEMP_DIR="$SCRIPT_DIR/.plugin-temp"

print_info "Downloading OxiCleanarr plugin ${PLUGIN_VERSION}..."
rm -rf "$PLUGIN_TEMP_DIR"
mkdir -p "$PLUGIN_TEMP_DIR"

if curl -L -f -o "$PLUGIN_TEMP_DIR/plugin.zip" "$PLUGIN_URL" 2>/dev/null; then
    print_status "Plugin downloaded successfully"
    
    # Extract plugin
    print_info "Extracting plugin..."
    cd "$PLUGIN_TEMP_DIR"
    unzip -q plugin.zip
    
    # Find the DLL file
    PLUGIN_DLL=$(find . -name "Jellyfin.Plugin.*.dll" | head -1)
    
    if [ -z "$PLUGIN_DLL" ]; then
        print_error "Failed to find plugin DLL in archive"
        exit 1
    fi
    
    print_status "Plugin DLL found: $PLUGIN_DLL"
    
    # Copy plugin to test environment
    mkdir -p "$SCRIPT_DIR/jellyfin-config/plugins/OxiCleanarr"
    cp -r * "$SCRIPT_DIR/jellyfin-config/plugins/OxiCleanarr/"
    print_status "Plugin installed to test environment"
    
    cd "$SCRIPT_DIR"
    rm -rf "$PLUGIN_TEMP_DIR"
else
    print_error "Failed to download plugin from GitHub"
    print_info "URL: $PLUGIN_URL"
    exit 1
fi

print_step 2 "Creating Test Media"
create_test_media

print_step 3 "Creating Configuration Files"

# Create OxiCleanarr config
mkdir -p oxicleanarr-config
cat > oxicleanarr-config/config.yaml << 'EOF'
admin:
  username: admin
  password: admin123
  disable_auth: false

app:
  dry_run: true
  enable_deletion: false
  leaving_soon_days: 7

sync:
  full_interval: 3600
  incremental_interval: 900
  auto_start: false

retention:
  movie_retention: 0d
  tv_retention: 0d
  advanced_rules: []

integrations:
  jellyfin:
    enabled: true
    url: http://oxicleanarr-test-jellyfin:8096
    api_key: ""  # Will be set after Jellyfin setup
    symlink_library:
      enabled: true
      base_path: /data/leaving-soon
      movies:
        enabled: true
        name: "Leaving Soon - Movies"
        collection_type: movies
      tv_shows:
        enabled: true
        name: "Leaving Soon - TV Shows"
        collection_type: tvshows
      hide_when_empty: true
    
  radarr:
    enabled: true
    url: http://oxicleanarr-test-radarr:7878
    api_key: ""  # Will be set after Radarr setup
    
  sonarr:
    enabled: false
    
  jellyseerr:
    enabled: false
    
  jellystat:
    enabled: false
EOF
print_status "Created OxiCleanarr config"

print_step 4 "Starting Docker Services"
docker compose up -d

# Wait for services to be ready
wait_for_service "$JELLYFIN_URL/health" "Jellyfin"
wait_for_service "$RADARR_URL/ping" "Radarr"
wait_for_service "$OXICLEANARR_URL/health" "OxiCleanarr"

print_step 5 "Jellyfin Automated Setup"
print_info "Running automated Jellyfin setup script..."

# Run automated Jellyfin setup and capture output
SETUP_OUTPUT=$("$SCRIPT_DIR/setup_jellyfin.sh" 2>&1)
SETUP_EXIT_CODE=$?

# Display setup output to user
echo "$SETUP_OUTPUT"

if [ $SETUP_EXIT_CODE -ne 0 ]; then
    print_error "Jellyfin setup failed"
    exit 1
fi

# Extract API key from setup script output (format: "export JELLYFIN_API_KEY=...")
JELLYFIN_API_KEY=$(echo "$SETUP_OUTPUT" | grep "^export JELLYFIN_API_KEY=" | cut -d'=' -f2 | tr -d '"')

if [ -z "$JELLYFIN_API_KEY" ]; then
    print_error "Failed to extract Jellyfin API key from setup script"
    exit 1
fi

print_status "Jellyfin API key obtained: ${JELLYFIN_API_KEY:0:8}..."

# Update OxiCleanarr config with Jellyfin API key
# Use yq if available, otherwise use awk for precise replacement
if command -v yq &> /dev/null; then
    yq eval ".integrations.jellyfin.api_key = \"$JELLYFIN_API_KEY\"" -i oxicleanarr-config/config.yaml
else
    # Fallback to awk - only update api_key within jellyfin section
    awk -v key="$JELLYFIN_API_KEY" '
    /^  jellyfin:/ { in_jellyfin=1 }
    /^  radarr:/ { in_jellyfin=0 }
    in_jellyfin && /^    api_key:/ { sub(/api_key: .*/, "api_key: \"" key "\"") }
    { print }
    ' oxicleanarr-config/config.yaml > oxicleanarr-config/config.yaml.tmp && mv oxicleanarr-config/config.yaml.tmp oxicleanarr-config/config.yaml
fi
print_status "OxiCleanarr config updated with Jellyfin API key"

print_step 6 "Radarr Initial Setup"
print_info "Open $RADARR_URL in your browser"
print_info "Complete the following steps:"
echo ""
echo "  1. Complete initial setup wizard"
echo ""
echo "  2. Add root folder:"
echo "     - Go to Settings → Media Management"
echo "     - Add Root Folder: /media/movies"
echo "     - Save"
echo ""
echo "  3. Get API key:"
echo "     - Go to Settings → General"
echo "     - Copy the API Key"
echo ""
read -p "Press Enter after completing Radarr setup and getting API key..."

# Ask for Radarr API key
echo ""
read -p "Enter Radarr API key: " RADARR_API_KEY
if [ -z "$RADARR_API_KEY" ]; then
    print_error "Radarr API key is required"
    exit 1
fi

# Update OxiCleanarr config with Radarr API key
sed -i "s/api_key: \"\"  # Will be set after Radarr setup/api_key: \"$RADARR_API_KEY\"/" oxicleanarr-config/config.yaml

# Restart OxiCleanarr to pick up new config
print_info "Restarting OxiCleanarr to load API keys..."
docker compose restart oxicleanarr
sleep 5
wait_for_service "$OXICLEANARR_URL/health" "OxiCleanarr"

print_step 7 "Adding Movies to Radarr"
print_info "Manually adding test movies to Radarr..."
print_info "Open $RADARR_URL in your browser"
echo ""
echo "For each test movie in /media/movies:"
echo "  1. Go to Movies → Add New"
echo "  2. Type 'Test Movie' in search"
echo "  3. Add the test movies manually (use 'Add Existing Movie' if needed)"
echo "  4. Monitor: None (already have files)"
echo "  5. Root Folder: /media/movies"
echo ""
echo "OR run library scan:"
echo "  1. Settings → Media Management"
echo "  2. Click 'Scan Media Files' under the root folder"
echo ""
read -p "Press Enter after adding movies to Radarr..."

print_step 8 "Adding Movies to Jellyfin"
print_info "Setting up Jellyfin media library..."
echo ""
echo "  1. Go to Dashboard → Libraries"
echo "  2. Add Media Library:"
echo "     - Content type: Movies"
echo "     - Display name: Test Movies"
echo "     - Folders: /media/movies"
echo "     - Save"
echo ""
echo "  3. Wait for library scan to complete"
echo "     - Check Dashboard → Scheduled Tasks"
echo "     - Look for 'Scan Media Library' task"
echo ""
read -p "Press Enter after Jellyfin has scanned the library..."

print_step 9 "Testing OxiCleanarr Sync"
print_info "Triggering full sync from OxiCleanarr..."

# Login to OxiCleanarr
TOKEN=$(curl -s -X POST "$OXICLEANARR_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    print_error "Failed to login to OxiCleanarr"
    exit 1
fi

print_status "Logged in to OxiCleanarr"

# Trigger full sync
print_info "Starting full sync..."
SYNC_RESPONSE=$(curl -s -X POST "$OXICLEANARR_URL/api/sync/full" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json")

echo "$SYNC_RESPONSE" | jq '.'

if echo "$SYNC_RESPONSE" | jq -e '.message' | grep -q "started"; then
    print_status "Full sync started successfully"
else
    print_error "Failed to start sync"
    echo "$SYNC_RESPONSE"
fi

# Wait for sync to complete
print_info "Waiting for sync to complete (this may take 30-60 seconds)..."
sleep 30

# Check sync status
print_info "Checking sync status..."
STATUS=$(curl -s "$OXICLEANARR_URL/api/sync/status" \
    -H "Authorization: Bearer $TOKEN")
echo "$STATUS" | jq '.'

print_step 10 "Checking Media Data"
print_info "Fetching movie data from OxiCleanarr..."
MOVIES=$(curl -s "$OXICLEANARR_URL/api/media/movies" \
    -H "Authorization: Bearer $TOKEN")

MOVIE_COUNT=$(echo "$MOVIES" | jq '.items | length')
print_status "Found $MOVIE_COUNT movies in OxiCleanarr"

if [ "$MOVIE_COUNT" -gt 0 ]; then
    echo ""
    echo "Movies:"
    echo "$MOVIES" | jq '.items[] | {title: .title, jellyfin_id: .jellyfin_id, radarr_id: .radarr_id}'
fi

print_step 11 "Testing Symlink Library Feature"
print_info "This will test if OxiCleanarr can create symlinks..."

# Change retention to trigger symlink creation
print_info "Updating retention policy to 7 days..."
CONFIG=$(cat oxicleanarr-config/config.yaml | sed 's/movie_retention: 0d/movie_retention: 7d/')
echo "$CONFIG" > oxicleanarr-config/config.yaml

# Restart to reload config
docker compose restart oxicleanarr
sleep 5
wait_for_service "$OXICLEANARR_URL/health" "OxiCleanarr"

# Re-login
TOKEN=$(curl -s -X POST "$OXICLEANARR_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

# Trigger another sync
print_info "Triggering sync with new retention policy..."
curl -s -X POST "$OXICLEANARR_URL/api/sync/full" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" | jq '.'

sleep 30

# Check for symlinks
print_info "Checking for symlinks..."
if [ -d "leaving-soon/movies" ]; then
    SYMLINK_COUNT=$(ls -1 leaving-soon/movies/ 2>/dev/null | wc -l)
    if [ "$SYMLINK_COUNT" -gt 0 ]; then
        print_status "Found $SYMLINK_COUNT symlinks in leaving-soon/movies/"
        echo ""
        echo "Symlinks:"
        ls -lah leaving-soon/movies/
    else
        print_info "No symlinks created yet (movies may not meet retention criteria)"
    fi
else
    print_info "Leaving soon directory not created yet"
fi

print_step 12 "Checking Jellyfin Plugin"
print_info "Verify the plugin is working in Jellyfin:"
echo ""
echo "  1. Go to Dashboard → Plugins"
echo "  2. Check if 'OxiCleanarr Bridge' plugin is listed"
echo ""
echo "  3. Check if plugin API is responding:"
curl -s "$JELLYFIN_URL/api/oxicleanarr/status" | jq '.' 2>/dev/null || echo "Plugin API not responding"
echo ""

print_step 13 "Manual Verification"
print_info "Please verify the following:"
echo ""
echo "  1. Check Jellyfin Libraries:"
echo "     → Open $JELLYFIN_URL"
echo "     → Go to Home → Libraries"
echo "     → Look for 'Leaving Soon - Movies' library"
echo ""
echo "  2. Check OxiCleanarr Dashboard:"
echo "     → Open $OXICLEANARR_URL"
echo "     → Login: admin / admin123"
echo "     → Check Dashboard for statistics"
echo "     → Check Timeline for scheduled deletions"
echo ""
echo "  3. Check Radarr Integration:"
echo "     → Open $RADARR_URL"
echo "     → Verify movies are listed"
echo "     → Check if Radarr can see the media files"
echo ""

print_header "Test Complete!"
print_info "Services are running:"
echo "  - Jellyfin:     $JELLYFIN_URL"
echo "  - Radarr:       $RADARR_URL"
echo "  - OxiCleanarr:  $OXICLEANARR_URL"
echo ""
print_info "Useful commands:"
echo "  View logs:          docker compose logs -f [service]"
echo "  Check symlinks:     ls -lah leaving-soon/"
echo "  Restart services:   docker compose restart"
echo "  Stop services:      docker compose down"
echo "  Clean up:           $0 --cleanup"
echo ""

# Offer to keep running or clean up
read -p "Do you want to keep the test environment running? (Y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]] && [[ ! -z $REPLY ]]; then
    cleanup
fi
