#!/bin/bash

# Test script to verify requester information is displayed
# Uses prunarr.test.yaml which has Jellyseerr enabled

echo "Testing Requester Information Display"
echo "======================================"
echo ""

CONFIG_FILE="config/prunarr.test.yaml"

# Check if test config exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "✗ Test config not found: $CONFIG_FILE"
    exit 1
fi

echo "Using config: $CONFIG_FILE"
echo ""

# Check if Jellyseerr is enabled in test config
echo "1. Checking Jellyseerr configuration..."
JELLYSEERR_ENABLED=$(grep -A3 "jellyseerr:" "$CONFIG_FILE" | grep "enabled:" | awk '{print $2}')
JELLYSEERR_URL=$(grep -A3 "jellyseerr:" "$CONFIG_FILE" | grep "url:" | awk '{print $2}' | tr -d '"')

if [ "$JELLYSEERR_ENABLED" = "true" ]; then
    echo "   ✓ Jellyseerr is enabled"
    echo "   → URL: $JELLYSEERR_URL"
else
    echo "   ✗ Jellyseerr is not enabled in test config"
    exit 1
fi
echo ""

# Test API endpoint for requester fields
echo "2. Testing API for requester fields..."
echo "   → GET http://localhost:8080/api/media/movies"
echo ""

# Make API call (requires app to be running)
RESPONSE=$(curl -s http://localhost:8080/api/media/movies 2>/dev/null)

if [ $? -ne 0 ] || [ -z "$RESPONSE" ]; then
    echo "   ✗ Could not connect to API (is Prunarr running?)"
    echo ""
    echo "   To start Prunarr with test config:"
    echo "   → ./prunarr --config $CONFIG_FILE"
    echo ""
    echo "   Or use the test binary:"
    echo "   → ./prunarr-test"
    exit 1
fi

# Parse response and check for requester info
echo "   ✓ API responded successfully"
echo ""

TOTAL_ITEMS=$(echo "$RESPONSE" | jq '.total' 2>/dev/null)
echo "   Total media items: $TOTAL_ITEMS"

# Count items with requester info
REQUESTED_COUNT=$(echo "$RESPONSE" | jq '[.items[] | select(.is_requested == true)] | length' 2>/dev/null)
echo "   Items marked as requested: $REQUESTED_COUNT"
echo ""

if [ "$REQUESTED_COUNT" -gt 0 ]; then
    echo "3. ✓ SUCCESS! Found media with requester information:"
    echo "   ================================================"
    echo ""
    echo "$RESPONSE" | jq -r '.items[] | select(.is_requested == true) | "   Title: \(.title) (\(.year // "N/A"))\n   Requested by: \(.requested_by_username // "N/A")\n   Email: \(.requested_by_email // "N/A")\n   User ID: \(.requested_by_user_id // "N/A")\n"' 2>/dev/null | head -50
else
    echo "3. ⚠ No media found with requester information"
    echo ""
    echo "   Possible reasons:"
    echo "   - Jellyseerr has no approved requests"
    echo "   - No requests match media in Radarr/Sonarr"
    echo "   - Sync hasn't completed yet"
    echo ""
    echo "   Sample media (first 3):"
    echo "$RESPONSE" | jq -r '.items[0:3] | .[] | "   - \(.title) (\(.year // "N/A")) [Requested: \(.is_requested)]"' 2>/dev/null
fi

echo ""
echo "4. Testing Jellyseerr API directly..."
JELLYSEERR_RESPONSE=$(curl -s -H "X-Api-Key: MTcwNTkyOTE2NDc5MmMzOGYzZjJkLWM2NzgtNDhmOS1iZjQyLWFmY2Q3ZWE1Y2FkYw==" "$JELLYSEERR_URL/api/v1/request?take=5" 2>/dev/null)

if [ $? -eq 0 ] && [ ! -z "$JELLYSEERR_RESPONSE" ]; then
    REQUEST_COUNT=$(echo "$JELLYSEERR_RESPONSE" | jq '.results | length' 2>/dev/null)
    echo "   ✓ Jellyseerr responded: $REQUEST_COUNT requests found"
    
    if [ "$REQUEST_COUNT" -gt 0 ]; then
        echo "   Sample requests:"
        echo "$JELLYSEERR_RESPONSE" | jq -r '.results[0:3] | .[] | "   - \(.media.tmdbId): \(.media.title // .media.name) (Requested by: \(.requestedBy.displayName))"' 2>/dev/null
    fi
else
    echo "   ✗ Could not connect to Jellyseerr"
fi

echo ""
echo "5. Expected behavior in UI:"
echo "   ========================="
echo "   When viewing media in the frontend:"
echo "   - Library page: Shows 'Requested by: username (email)' below media info"
echo "   - Timeline page: Shows requester in compact format"
echo "   - Scheduled Deletions: Shows requester info for deletion candidates"
echo ""
echo "   The requester info will ONLY appear for items where:"
echo "   - Jellyseerr integration is enabled ✓"
echo "   - A request exists in Jellyseerr for that media ?"
echo "   - The media matches by TMDB ID (movies) or TVDB ID (shows)"
echo "   - Sync has completed successfully"
echo ""
