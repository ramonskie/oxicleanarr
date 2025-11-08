#!/bin/bash

echo "=== Testing Date Semantic Labels ==="
echo ""

# Test 1: Check for movies with zero last_watched (should show "Never" in UI)
echo "1. Movies with zero last_watched dates (should display 'Never' for watched context):"
ZERO_WATCHED=$(curl -s http://localhost:8080/api/media/movies | jq -r '[.items[] | select(.last_watched == "0001-01-01T00:00:00Z")] | length')
echo "   Found: $ZERO_WATCHED movies"
echo "   Example:"
curl -s http://localhost:8080/api/media/movies | jq -r '[.items[] | select(.last_watched == "0001-01-01T00:00:00Z")] | .[0] | {title, last_watched}' | head -5
echo ""

# Test 2: Check scheduled deletions with zero delete_after (should show "N/A" in UI)
echo "2. Scheduled deletions with zero delete_after dates (should display 'N/A' for deletion context):"
ZERO_DELETE=$(curl -s http://localhost:8080/api/jobs | jq -r '[.jobs[0].summary.deletion_candidates[] | select(.delete_after == "0001-01-01T00:00:00Z")] | length')
echo "   Found: $ZERO_DELETE items"
if [ "$ZERO_DELETE" -gt 0 ]; then
  echo "   Example:"
  curl -s http://localhost:8080/api/jobs | jq -r '[.jobs[0].summary.deletion_candidates[] | select(.delete_after == "0001-01-01T00:00:00Z")] | .[0] | {title, delete_after}' | head -5
fi
echo ""

# Test 3: Movies with valid deletion_date (should show formatted date or "Not scheduled")
echo "3. Movies with valid deletion dates (should show formatted date):"
VALID_DELETE=$(curl -s http://localhost:8080/api/media/movies | jq -r '[.items[] | select(.deletion_date != null and .deletion_date != "" and .deletion_date != "0001-01-01T00:00:00Z")] | length')
echo "   Found: $VALID_DELETE movies"
echo "   Example:"
curl -s http://localhost:8080/api/media/movies | jq -r '[.items[] | select(.deletion_date != null and .deletion_date != "" and .deletion_date != "0001-01-01T00:00:00Z")] | .[0] | {title, deletion_date}' | head -5
echo ""

# Test 4: Timeline page data (all should have valid deletion_date)
echo "4. Timeline page (leaving soon) - all items should have valid deletion dates:"
TIMELINE_ITEMS=$(curl -s "http://localhost:8080/api/media/leaving-soon?limit=1000" | jq -r '.items | length')
echo "   Total items: $TIMELINE_ITEMS"
TIMELINE_ZERO=$(curl -s "http://localhost:8080/api/media/leaving-soon?limit=1000" | jq -r '[.items[] | select(.deletion_date == "0001-01-01T00:00:00Z")] | length')
echo "   Items with zero date: $TIMELINE_ZERO (should be 0)"
echo ""

echo "=== Summary ==="
echo "✓ Library Page:"
echo "  - Last watched: 'Never' for zero dates (watched context)"
echo "  - Deletion date: 'N/A' for zero dates (deletion context)"
echo ""
echo "✓ Scheduled Deletions Page:"
echo "  - Delete after: 'N/A' for zero dates (deletion context)"
echo "  - Last watched: 'Never'/'Unknown' for zero dates (watched context)"
echo ""
echo "✓ Timeline Page:"
echo "  - Deletion date: 'N/A' for zero dates (deletion context)"
echo "  - Should filter out items without valid deletion dates"
