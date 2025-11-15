# Deletion Timeline

The **Deletion Timeline** provides a visual calendar view of all media scheduled for deletion. It helps you understand when content will be removed and plan accordingly.

## Overview

The Timeline page shows:

- **Date-grouped deletions** - Media organized by deletion date
- **Aggregate statistics** - Total space to be freed per date
- **Countdown timers** - Days/hours until deletion
- **Media details** - Title, year, file size, deletion reason
- **Quick actions** - Keep button to exclude items

## Timeline View

### Date Grouping

Items are grouped by their scheduled deletion date:

```
┌─ November 20, 2024 (3 items, 25.4 GB) ─────────────┐
│                                                     │
│  🎬 Movie Title (2020)                 8.5 GB      │
│     ⏱️ 5 days remaining  ℹ️  🛡️ Keep               │
│                                                     │
│  📺 TV Show Name                      12.3 GB      │
│     ⏱️ 5 days remaining  ℹ️  🛡️ Keep               │
│                                                     │
│  🎬 Another Movie (2019)               4.6 GB      │
│     ⏱️ 5 days remaining  ℹ️  🛡️ Keep               │
│                                                     │
└─────────────────────────────────────────────────────┘

┌─ November 27, 2024 (2 items, 15.2 GB) ─────────────┐
│  ...                                                │
└─────────────────────────────────────────────────────┘
```

### Date Headers

Each date section shows:
- **Date** - Deletion date
- **Item count** - Number of items scheduled
- **Total size** - Combined file size of all items

### Media Cards

Each item displays:
- **Poster/Icon** - Movie (🎬) or TV Show (📺) indicator
- **Title + Year** - Full media name
- **File size** - Disk space to be freed
- **Countdown timer** - Time remaining until deletion
- **Info icon (ℹ️)** - Hover to see deletion reason
- **Keep button (🛡️)** - Click to exclude from deletion

## How Deletion Dates Are Calculated

### Basic Retention

```
Added Date + Retention Period = Deletion Date
```

**Example (Movie):**
```
Added: Jan 1, 2024
Retention: 90d (movie_retention: 90d)
Delete After: Apr 1, 2024
```

### Watched-Based Retention

```
Last Watched Date + Retention Period = Deletion Date
```

**Example:**
```
Added: Jan 1, 2024
Last Watched: Mar 15, 2024
Retention: 30d (watched-based rule)
Delete After: Apr 14, 2024
```

### User-Based Retention

```
Import Date + User Retention = Deletion Date
```

**Example:**
```
Requested By: guest_user
Import Date: Feb 1, 2024
User Retention: 14d
Delete After: Feb 15, 2024
```

### Tag-Based Retention

```
Added Date + Tag Retention = Deletion Date
```

**Example:**
```
Added: Jan 1, 2024
Tag: demo-content
Tag Retention: 7d
Delete After: Jan 8, 2024
```

## Timeline States

### Safe (Green)

```
Status: More than leaving_soon_days until deletion
Color: Green / Normal
Location: Not shown in timeline (optional filter)
```

**Example:**
```
Added: Jan 1
Retention: 90d
Today: Jan 15
Days Until Deletion: 75 days
Status: Safe
```

### Leaving Soon (Yellow)

```
Status: Within leaving_soon_days window
Color: Yellow / Warning
Location: Shown in timeline, "Leaving Soon" dashboard
```

**Example:**
```
Added: Jan 1
Retention: 90d
leaving_soon_days: 14
Today: Mar 20
Days Until Deletion: 12 days
Status: Leaving Soon
```

### Overdue (Red)

```
Status: Past retention deadline
Color: Red / Danger
Location: Top of timeline, "Scheduled Deletions" page
```

**Example:**
```
Added: Jan 1
Retention: 90d
Today: Apr 5
Days Overdue: 4 days
Status: Overdue (pending deletion)
```

**Note:** In dry-run mode, overdue items remain visible. When dry-run is disabled, they're deleted on next sync.

### Excluded (Blue/Gray)

```
Status: User excluded from deletion
Color: Blue badge / Muted
Location: Library browser (filtered view)
```

**Example:**
```
Added: Jan 1
Retention: 90d
Excluded: Yes
Status: Protected (will never be deleted)
```

## Timeline vs. Other Views

### Timeline Page

**Purpose:** Calendar view of deletion schedule

**Shows:**
- All items grouped by deletion date
- Aggregate statistics per date
- Chronological order (soonest first)

**Use case:** "What's being deleted this week?"

### Leaving Soon Dashboard

**Purpose:** Quick overview of imminent deletions

**Shows:**
- Items within leaving_soon_days window
- Countdown timers
- Limited to ~10-20 items

**Use case:** "What should I watch before it's gone?"

### Scheduled Deletions Page

**Purpose:** Complete list of deletion candidates

**Shows:**
- All items with daysUntilDue > 0
- Sortable by various fields
- Filterable by media type
- Full pagination

**Use case:** "Review everything scheduled for deletion"

### Library Browser

**Purpose:** Explore entire media collection

**Shows:**
- All media (safe + leaving soon + excluded)
- Advanced filtering and sorting
- Search by title/year

**Use case:** "Browse my entire library"

## Filtering and Sorting

### Timeline Filters

```
Media Type:
  ☑ Movies
  ☑ TV Shows

Status:
  ☑ Leaving Soon
  ☑ Overdue
  ☐ Safe (hidden by default)

Date Range:
  From: [Today]
  To: [+30 days]
```

### Default Behavior

Timeline shows:
- Items scheduled for deletion in next 30 days
- Sorted by deletion date (ascending)
- Grouped by date
- Safe items hidden (optional toggle)

## Interactive Features

### Countdown Timers

Live countdown updates every minute:

```
⏱️ 5 days, 14 hours remaining
⏱️ 2 days, 8 hours remaining
⏱️ 12 hours remaining
⏱️ 45 minutes remaining
⏱️ Overdue (4 days)
```

### Deletion Reason Tooltips

Hover over the info icon (ℹ️) to see why content is scheduled:

```
"This movie was added 95 days ago. 
The retention policy for movies is 90 days."
```

```
"This show was last watched 35 days ago. 
The watched-based cleanup rule deletes 
content 30 days after last watch."
```

```
"This content was requested by guest_user. 
User-based cleanup removes their content 
after 14 days."
```

### Keep Button

Click the shield icon (🛡️) to exclude from deletion:

```
Before: 🛡️ Keep
         ↓ (click)
After:  🛡️ Excluded
        Item removed from timeline
        Added to exclusions.json
```

### Date Navigation

Jump to specific dates:

```
Timeline Navigation:
[Today] [This Week] [This Month] [Next Month]
```

## Sync Behavior

### Timeline Updates

Timeline refreshes automatically:

| Event | Timeline Action |
|-------|----------------|
| Full sync completes | Recalculate all deletion dates, rebuild timeline |
| Incremental sync | Update watch dates, adjust deletion dates |
| User adds exclusion | Remove item from timeline immediately |
| User removes exclusion | Add item back to timeline |
| Config change (retention) | Recalculate dates, hot-reload timeline |

### Performance

- **Calculation time:** <1 second for 10,000 items
- **Page load:** <100ms (cached)
- **Live updates:** Every 60 seconds (countdown timers)

## API Endpoint

### Get Deletion Timeline

**GET** `/api/deletions/timeline`

Query parameters:
- `from`: Start date (ISO 8601, default: today)
- `to`: End date (ISO 8601, default: +30 days)
- `type`: Filter by media type (`movie`, `show`)

Response:
```json
{
  "timeline": [
    {
      "date": "2024-11-20",
      "items": [
        {
          "id": "radarr-123",
          "type": "movie",
          "title": "Movie Title",
          "year": 2020,
          "file_size": 9126805504,
          "delete_after": "2024-11-20T00:00:00Z",
          "days_until_due": 5,
          "deletion_reason": "Retention period expired (90d)"
        }
      ],
      "total_items": 3,
      "total_size": 27305287680
    }
  ],
  "summary": {
    "total_items": 15,
    "total_size": 161061273600,
    "date_range": {
      "from": "2024-11-15",
      "to": "2024-12-15"
    }
  }
}
```

### Date Format

All dates in ISO 8601 format:
```
2024-11-20T00:00:00Z
```

## Configuration

### Leaving Soon Window

```yaml
app:
  leaving_soon_days: 14
```

Controls:
- Which items appear in "Leaving Soon" section
- Timeline color coding (yellow badge)
- Dashboard alerts

### Retention Periods

```yaml
rules:
  movie_retention: 90d
  tv_retention: 120d

advanced_rules:
  - name: Watched Cleanup
    type: watched
    enabled: true
    retention: 30d
```

Controls:
- Base deletion dates
- Timeline position of items

## Use Cases

### 1. Weekly Review

**Scenario:** Check what's being deleted this week

**Steps:**
1. Open Timeline page
2. Review items scheduled for next 7 days
3. Watch items you want to keep
4. Click "Keep" on favorites

### 2. Storage Planning

**Scenario:** Estimate disk space freed per week

**Steps:**
1. Open Timeline page
2. Look at date group headers
3. Note total size per date
4. Plan for storage reclamation

**Example:**
```
Nov 20: 25.4 GB (3 items)
Nov 21: 42.1 GB (5 items)
Nov 22: 18.7 GB (2 items)
---------------------------
This Week: 86.2 GB to be freed
```

### 3. Retention Tuning

**Scenario:** Adjust retention periods based on deletion patterns

**Steps:**
1. Observe timeline over 2-3 weeks
2. Note if too many/too few items scheduled
3. Adjust retention in config:
   ```yaml
   rules:
     movie_retention: 120d  # Increased from 90d
   ```
4. Hot-reload applies new retention
5. Observe timeline changes

### 4. User Communication

**Scenario:** Notify users of upcoming deletions

**Steps:**
1. Export timeline data via API
2. Generate email/notification
3. Include deletion dates and titles
4. Allow users to click Keep button

## Troubleshooting

### Problem: Timeline Shows No Items

**Check 1: Are items actually scheduled?**
```bash
# Check sync status
curl http://localhost:8080/api/sync/status | jq

# Should show:
# "media_count": > 0
```

**Check 2: Is retention configured?**
```yaml
# Check config.yaml
rules:
  movie_retention: 90d    # Must be set (not "never")
  tv_retention: 120d      # Must be set (not "never")
```

**Check 3: Is media old enough?**
```
Added: Nov 1
Retention: 90d
Today: Nov 10
Days Until Deletion: 81 days (not in default 30-day timeline range)
```

### Problem: Wrong Deletion Dates

**Check 1: Verify retention config**
```yaml
rules:
  movie_retention: 90d  # Check this matches expectation
```

**Check 2: Check for advanced rules**
```yaml
advanced_rules:
  - name: Override Rule
    type: tag
    tag: some-tag
    retention: 30d  # This overrides default if media has tag
```

**Check 3: Trigger manual sync**
```bash
# Force recalculation
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/sync/full
```

### Problem: Timeline Not Updating

**Check 1: Browser cache**
```
Hard refresh: Ctrl+Shift+R (Windows/Linux) or Cmd+Shift+R (Mac)
```

**Check 2: Check last sync time**
```bash
curl http://localhost:8080/api/sync/status | jq '.last_full_sync'
```

**Check 3: Check logs for errors**
```bash
docker logs oxicleanarr 2>&1 | grep -i error | tail -20
```

## Best Practices

### 1. Review Timeline Weekly

Set a recurring reminder to review the timeline every week. This ensures:
- You don't miss content you want to watch
- You can adjust retention if needed
- You spot any configuration issues early

### 2. Use Timeline for Storage Planning

Before adding new content, check timeline to see how much space will be freed soon.

### 3. Monitor Overdue Items

If items are consistently showing as "overdue" for long periods:
- Enable deletions: `dry_run: false`
- Or increase retention periods
- Or add exclusions for content you want to keep

### 4. Communicate with Users

Share timeline view with family/users:
- Weekly email with items scheduled for deletion
- Shared access to OxiCleanarr web UI
- Instructions for using Keep button

## Related Pages

- [Leaving Soon Library](Leaving-Soon-Library.md) - Symlink libraries and countdown feature
- [Configuration](Configuration.md) - Retention period configuration
- [Advanced Rules](Advanced-Rules.md) - Custom retention logic
- [Scheduled Deletions](../web/src/pages/ScheduledDeletionsPage.tsx) - Full deletion candidate list
