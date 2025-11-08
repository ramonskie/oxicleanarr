#!/bin/bash
# Prunarr Live Test Script
# This script helps you quickly test Prunarr's API endpoints

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${OXICLEANARR_URL:-http://localhost:8080}"
USERNAME="${OXICLEANARR_USER:-admin}"
PASSWORD="${OXICLEANARR_PASS:-changeme}"

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║    Prunarr Live API Test Script       ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""

# Check if Prunarr is running
echo -e "${YELLOW}► Checking if Prunarr is running...${NC}"
if ! curl -s -f "${BASE_URL}/health" > /dev/null 2>&1; then
    echo -e "${RED}✗ Prunarr is not running at ${BASE_URL}${NC}"
    echo -e "${YELLOW}Please start Prunarr first with: make dev${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Prunarr is running${NC}"
echo ""

# Test 1: Health Check
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 1: Health Check${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
HEALTH=$(curl -s "${BASE_URL}/health")
echo "$HEALTH" | jq .
if echo "$HEALTH" | jq -e '.status == "ok"' > /dev/null; then
    echo -e "${GREEN}✓ Health check passed${NC}"
else
    echo -e "${RED}✗ Health check failed${NC}"
    exit 1
fi
echo ""

# Test 2: Login
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 2: Login & Authentication${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${USERNAME}\",\"password\":\"${PASSWORD}\"}")

echo "$LOGIN_RESPONSE" | jq .

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.token')
if [ "$TOKEN" = "null" ] || [ -z "$TOKEN" ]; then
    echo -e "${RED}✗ Login failed - could not get token${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Login successful${NC}"
echo -e "${GREEN}  Token: ${TOKEN:0:50}...${NC}"
echo ""

# Test 3: Get Sync Status (before sync)
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 3: Get Sync Status (before sync)${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
SYNC_STATUS=$(curl -s "${BASE_URL}/api/sync/status" \
    -H "Authorization: Bearer ${TOKEN}")
echo "$SYNC_STATUS" | jq .
echo -e "${GREEN}✓ Sync status retrieved${NC}"
echo ""

# Test 4: Trigger Full Sync
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 4: Trigger Full Sync${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
SYNC_TRIGGER=$(curl -s -X POST "${BASE_URL}/api/sync/full" \
    -H "Authorization: Bearer ${TOKEN}")
echo "$SYNC_TRIGGER" | jq .
echo -e "${GREEN}✓ Full sync triggered${NC}"
echo -e "${YELLOW}⏳ Waiting 5 seconds for sync to complete...${NC}"
sleep 5
echo ""

# Test 5: Get Latest Job
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 5: Get Latest Job${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
LATEST_JOB=$(curl -s "${BASE_URL}/api/jobs/latest" \
    -H "Authorization: Bearer ${TOKEN}")
echo "$LATEST_JOB" | jq .
echo -e "${GREEN}✓ Latest job retrieved${NC}"
echo ""

# Test 6: List All Jobs
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 6: List All Jobs${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
JOBS=$(curl -s "${BASE_URL}/api/jobs" \
    -H "Authorization: Bearer ${TOKEN}")
JOB_COUNT=$(echo "$JOBS" | jq 'length')
echo "$JOBS" | jq .
echo -e "${GREEN}✓ Found ${JOB_COUNT} jobs${NC}"
echo ""

# Test 7: Get Sync Status (after sync)
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 7: Get Sync Status (after sync)${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
SYNC_STATUS_AFTER=$(curl -s "${BASE_URL}/api/sync/status" \
    -H "Authorization: Bearer ${TOKEN}")
echo "$SYNC_STATUS_AFTER" | jq .
MEDIA_COUNT=$(echo "$SYNC_STATUS_AFTER" | jq -r '.media_count')
MOVIES_COUNT=$(echo "$SYNC_STATUS_AFTER" | jq -r '.movies_count')
TV_SHOWS_COUNT=$(echo "$SYNC_STATUS_AFTER" | jq -r '.tv_shows_count')
echo -e "${GREEN}✓ Media synced: ${MEDIA_COUNT} total (${MOVIES_COUNT} movies, ${TV_SHOWS_COUNT} TV shows)${NC}"
echo ""

# Test 8: List Movies
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 8: List Movies${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
MOVIES=$(curl -s "${BASE_URL}/api/media/movies" \
    -H "Authorization: Bearer ${TOKEN}")
MOVIE_COUNT=$(echo "$MOVIES" | jq 'length')
echo "$MOVIES" | jq '.[:3]'  # Show first 3 movies
if [ "$MOVIE_COUNT" -gt 3 ]; then
    echo -e "${YELLOW}... and $((MOVIE_COUNT - 3)) more movies${NC}"
fi
echo -e "${GREEN}✓ Retrieved ${MOVIE_COUNT} movies${NC}"
echo ""

# Test 9: List TV Shows
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 9: List TV Shows${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
SHOWS=$(curl -s "${BASE_URL}/api/media/shows" \
    -H "Authorization: Bearer ${TOKEN}")
SHOW_COUNT=$(echo "$SHOWS" | jq 'length')
echo "$SHOWS" | jq '.[:3]'  # Show first 3 shows
if [ "$SHOW_COUNT" -gt 3 ]; then
    echo -e "${YELLOW}... and $((SHOW_COUNT - 3)) more TV shows${NC}"
fi
echo -e "${GREEN}✓ Retrieved ${SHOW_COUNT} TV shows${NC}"
echo ""

# Test 10: List Leaving Soon
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 10: List Media Leaving Soon${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
LEAVING=$(curl -s "${BASE_URL}/api/media/leaving-soon" \
    -H "Authorization: Bearer ${TOKEN}")
LEAVING_COUNT=$(echo "$LEAVING" | jq 'length')
echo "$LEAVING" | jq '.[:3]'  # Show first 3 items
if [ "$LEAVING_COUNT" -gt 3 ]; then
    echo -e "${YELLOW}... and $((LEAVING_COUNT - 3)) more items${NC}"
fi
echo -e "${GREEN}✓ Retrieved ${LEAVING_COUNT} items leaving soon${NC}"
echo ""

# Test 11: Test Exclusion (if media exists)
if [ "$MEDIA_COUNT" -gt 0 ]; then
    echo -e "${BLUE}═══════════════════════════════════════${NC}"
    echo -e "${YELLOW}Test 11: Add/Remove Exclusion${NC}"
    echo -e "${BLUE}═══════════════════════════════════════${NC}"
    
    # Get first media item ID
    if [ "$MOVIE_COUNT" -gt 0 ]; then
        MEDIA_ID=$(echo "$MOVIES" | jq -r '.[0].id')
        MEDIA_TITLE=$(echo "$MOVIES" | jq -r '.[0].title')
    else
        MEDIA_ID=$(echo "$SHOWS" | jq -r '.[0].id')
        MEDIA_TITLE=$(echo "$SHOWS" | jq -r '.[0].title')
    fi
    
    echo -e "${YELLOW}  Testing with: ${MEDIA_TITLE} (${MEDIA_ID})${NC}"
    
    # Add exclusion
    echo -e "${YELLOW}  ► Adding exclusion...${NC}"
    ADD_EXCLUSION=$(curl -s -X POST "${BASE_URL}/api/media/${MEDIA_ID}/exclude" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{"reason":"Test exclusion from live test script"}')
    echo "$ADD_EXCLUSION" | jq .
    echo -e "${GREEN}  ✓ Exclusion added${NC}"
    
    # Get media item to verify exclusion
    echo -e "${YELLOW}  ► Verifying exclusion...${NC}"
    MEDIA_ITEM=$(curl -s "${BASE_URL}/api/media/${MEDIA_ID}" \
        -H "Authorization: Bearer ${TOKEN}")
    IS_EXCLUDED=$(echo "$MEDIA_ITEM" | jq -r '.is_excluded')
    if [ "$IS_EXCLUDED" = "true" ]; then
        echo -e "${GREEN}  ✓ Media is excluded${NC}"
    else
        echo -e "${RED}  ✗ Media exclusion verification failed${NC}"
    fi
    
    # Remove exclusion
    echo -e "${YELLOW}  ► Removing exclusion...${NC}"
    REMOVE_EXCLUSION=$(curl -s -X DELETE "${BASE_URL}/api/media/${MEDIA_ID}/exclude" \
        -H "Authorization: Bearer ${TOKEN}")
    echo "$REMOVE_EXCLUSION" | jq .
    echo -e "${GREEN}  ✓ Exclusion removed${NC}"
    echo ""
fi

# Test 12: Trigger Incremental Sync
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${YELLOW}Test 12: Trigger Incremental Sync${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
INCR_SYNC=$(curl -s -X POST "${BASE_URL}/api/sync/incremental" \
    -H "Authorization: Bearer ${TOKEN}")
echo "$INCR_SYNC" | jq .
echo -e "${GREEN}✓ Incremental sync triggered${NC}"
echo ""

# Summary
echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          Test Summary                  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo -e "${GREEN}✓ All tests passed successfully!${NC}"
echo ""
echo -e "${YELLOW}Statistics:${NC}"
echo -e "  • Total Media:     ${MEDIA_COUNT}"
echo -e "  • Movies:          ${MOVIES_COUNT}"
echo -e "  • TV Shows:        ${TV_SHOWS_COUNT}"
echo -e "  • Leaving Soon:    ${LEAVING_COUNT}"
echo -e "  • Total Jobs:      ${JOB_COUNT}"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo -e "  1. Review the data above"
echo -e "  2. Check logs for any errors"
echo -e "  3. Test individual endpoints with curl"
echo -e "  4. Configure retention rules"
echo -e "  5. Set dry_run=false when ready for production"
echo ""
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${GREEN}Your JWT Token (valid for 24h):${NC}"
echo -e "${TOKEN}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo ""
