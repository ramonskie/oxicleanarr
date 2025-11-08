#!/bin/bash
# Test script to debug symlink library deletion

echo "=== Testing Symlink Library Debug Logging ==="
echo ""

# Your live Jellyfin URL and API key from config
JELLYFIN_URL="http://jellyfin:8096"
API_KEY="***REMOVED***"

echo "1. Checking Virtual Folders in Jellyfin..."
echo "   GET ${JELLYFIN_URL}/Library/VirtualFolders"
curl -s "${JELLYFIN_URL}/Library/VirtualFolders?api_key=${API_KEY}" | jq -r '.[] | "   - Name: \(.Name), Type: \(.CollectionType), Paths: \(.Locations[])"'
echo ""

echo "2. Expected library names from Prunarr config:"
echo "   - Leaving Soon - Movies"
echo "   - Leaving Soon - TV Shows"
echo ""

echo "3. Checking current retention config:"
curl -s http://localhost:8080/api/config | jq '.rules'
echo ""

echo "4. Checking leaving-soon items:"
curl -s http://localhost:8080/api/media/leaving-soon | jq '{total, movies, tv_shows}'
echo ""

echo "5. Triggering full sync..."
curl -s -X POST http://localhost:8080/api/sync/full | jq .
echo ""

echo "6. Waiting 5 seconds for sync to complete..."
sleep 5
echo ""

echo "7. Check logs for symlink library operations:"
echo "   (Look for: 'Library is empty', 'Fetching virtual folders', 'Checking virtual folder')"
echo ""
echo "   Run: docker logs prunarr 2>&1 | tail -100 | grep -i 'symlink\|virtual\|leaving soon'"
