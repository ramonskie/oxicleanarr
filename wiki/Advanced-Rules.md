# Advanced Rules

OxiCleanarr provides a powerful rules engine for fine-grained control over media cleanup behavior. Rules are evaluated in priority order to determine retention policies.

## Rule Priority Order

Rules are evaluated in this order (highest to lowest priority):

1. **Tag-based rules** - If media has a matching Radarr/Sonarr tag
2. **User-based rules** - If media was requested by a matching user
3. **Watched-based rules** - If media meets watch criteria
4. **Default retention** - `movie_retention` or `tv_retention` from basic rules

**The first matching rule determines the retention policy.**

## Tag-Based Rules

Target media by Radarr/Sonarr tags for custom retention periods.

### Configuration

```yaml
advanced_rules:
  - name: Kids Content
    type: tag
    enabled: true
    tag: kids
    retention: 180d             # Keep kids content for 6 months
  
  - name: Premium Content
    type: tag
    enabled: true
    tag: premium
    retention: 365d             # Keep for 1 year
  
  - name: Keep Forever
    type: tag
    enabled: true
    tag: keep
    retention: never            # Never delete
```

### How It Works

1. OxiCleanarr syncs tags from Radarr/Sonarr during full sync
2. When evaluating retention, checks if media has a matching tag
3. If tag matches, uses the custom retention period
4. If no tag match, falls through to next rule priority

### Use Cases

- **Preserve collections**: Tag movies in a series with `keep` to preserve them
- **Kids content**: Longer retention for family-friendly content
- **Quality tiers**: Different retention for 4K vs 1080p (using quality tags)
- **Special events**: Tag holiday movies with longer retention

### Setup in Radarr/Sonarr

1. **Radarr**: Settings → Tags → Add tag (e.g., "kids", "premium")
2. Assign tags to movies in Radarr
3. OxiCleanarr will pick up tags during next full sync

## User-Based Rules

Apply different retention policies based on who requested content via Jellyseerr.

### Configuration

```yaml
advanced_rules:
  - name: Trial Users
    type: user
    enabled: true
    users:
      - user_id: 42                    # Match by Jellyseerr user ID
        retention: 30d
      
      - email: guest@example.com       # Match by email
        retention: 7d
        require_watched: true          # Only delete after watched
      
      - username: trial_user           # Match by username
        retention: 14d
```

### User Matching

You can match users by **any ONE** of these identifiers:

1. **user_id** (integer) - Jellyseerr user ID (most reliable)
2. **username** (string) - Jellyseerr username (case-insensitive)
3. **email** (string) - User email address (case-insensitive)

**You only need to provide ONE identifier per user.**

### Watch Tracking (Optional)

When `require_watched: true`, deletion only occurs if:

1. Retention period has passed AND
2. User has watched the content (tracked via Jellystat)

**Example:**
```
Media requested: Jan 1
Retention: 30 days
Watched: Jan 15

If require_watched=true:
  - Jan 31: ✅ DELETE (30 days passed AND watched)

If require_watched=false:
  - Jan 31: ✅ DELETE (30 days passed, watch status ignored)
```

**Not watched behavior:**
```
Media requested: Jan 1
Retention: 30 days
Never watched

If require_watched=true:
  - Jan 31: ❌ KEEP (30 days passed BUT not watched)
  - Keeps indefinitely until watched

If require_watched=false:
  - Jan 31: ✅ DELETE (30 days passed, watch status ignored)
```

### Dependencies

**Required:**
- Jellyseerr integration enabled

**Optional:**
- Jellystat integration (for `require_watched: true`)

### Use Cases

1. **Trial/Temporary Users**
   - Delete content after 7 days
   - Clean up after trial period expires

2. **Watch-and-Delete Policy**
   - User requests content (30 days retention)
   - User watches content → eligible for deletion
   - User doesn't watch → keeps indefinitely

3. **Tiered User Management**
   - Free tier: 14 days retention
   - Premium tier: 90 days retention
   - VIP tier: Excluded from user-based rules

4. **Guest Access**
   - Short retention for guest users
   - Encourages engagement

### Fallback Behavior

If user-based rules are configured but no rule matches:
- Falls through to **standard retention rules** (`movie_retention`, `tv_retention`)
- Does NOT get blanket "requested" protection

**Example:**
```yaml
advanced_rules:
  - name: Specific Users
    type: user
    enabled: true
    users:
      - user_id: 123
        retention: 7d

# Media requested by user_id: 999 (not in rule)
# → Falls through to standard retention (90d for movies, 120d for TV)
```

## Watched-Based Rules

Automatically clean up content based on watch history.

### Configuration

```yaml
advanced_rules:
  - name: Auto Clean Watched Content
    type: watched
    enabled: true
    retention: 30d              # Delete 30 days after last watch
    require_watched: true       # Only delete media that has been watched
```

### How It Works

When `require_watched: true`:

1. Media must have **at least one watch event**
2. Retention period starts from **last watch date**
3. Unwatched content is **never deleted** by this rule

**Example Timeline:**
```
Added: Jan 1
First watch: Jan 15
Last watch: Jan 20
Retention: 30 days

Result:
- Feb 19: ✅ DELETE (30 days since last watch on Jan 20)
```

**Unwatched Protection:**
```
Added: Jan 1
Never watched
Retention: 30 days

Result:
- Feb 1: ❌ KEEP (not watched, protected indefinitely)
- Mar 1: ❌ KEEP (still not watched)
```

### Dependencies

**Required:**
- Jellystat integration enabled

**Fallback:**
- If Jellystat disabled and `require_watched: true` → treat as unwatched (keep indefinitely)

### Use Cases

1. **Auto-cleanup after viewing**
   - Delete movies 30 days after last watch
   - Free up space automatically

2. **Encourage timely viewing**
   - Content stays until watched
   - Incentivizes users to watch sooner

3. **Recently watched protection**
   - Keep recently watched content available
   - Delete after cooling-off period

## Complete Example Configuration

```yaml
rules:
  movie_retention: 90d          # Default for movies
  tv_retention: 120d            # Default for TV shows

advanced_rules:
  # Highest priority: Preserve important content by tag
  - name: Keep Forever
    type: tag
    enabled: true
    tag: keep
    retention: never            # Never delete
  
  - name: Kids Content
    type: tag
    enabled: true
    tag: kids
    retention: 180d             # Keep for 6 months
  
  # Medium priority: Guest users get shorter retention
  - name: Guest Users
    type: user
    enabled: true
    users:
      - username: guest
        retention: 7d
        require_watched: true   # Only delete after watched
      
      - email: trial@example.com
        retention: 14d
  
  # Lower priority: Auto-cleanup watched content
  - name: Watched Cleanup
    type: watched
    enabled: true
    retention: 30d
    require_watched: true       # Only delete watched media
  
  # Fallback: Default retention (90d movies, 120d TV) if no rules match
```

## Rule Evaluation Flow

```
FOR EACH media item:

  1. Check tag-based rules
     IF media has tag matching any enabled tag rule:
       → Apply tag rule retention
       → DONE
  
  2. Check user-based rules
     IF media requested by user matching any enabled user rule:
       → Apply user rule retention (with optional watch check)
       → DONE
  
  3. Check watched-based rules
     IF watched-based rule enabled AND media meets watch criteria:
       → Apply watched rule retention
       → DONE
  
  4. Apply default retention
     → Apply movie_retention or tv_retention
     → DONE
```

## Configuration Validation

On startup/config reload, OxiCleanarr validates:

**Tag Rules:**
- ✅ Valid tag name provided
- ✅ Valid retention duration

**User Rules:**
- ✅ At least one user identifier (user_id, username, or email)
- ✅ Valid retention duration
- ✅ If `require_watched: true`, Jellystat integration configured
- ✅ No duplicate users across rules

**Watched Rules:**
- ✅ Valid retention duration
- ✅ If `require_watched: true`, Jellystat integration configured

**Example validation errors:**
```
ERROR: Configuration validation failed
  - advanced_rules[0]: user rule missing all identifiers
  - advanced_rules[1]: require_watched=true but Jellystat disabled
  - advanced_rules[2]: invalid retention format "30 days" (use "30d")
```

## Disabling Standard Retention

You can disable standard retention rules entirely by setting:

```yaml
rules:
  movie_retention: never        # or "0d"
  tv_retention: never           # or "0d"
```

**Effect:**
- Only advanced rules apply
- Media without matching advanced rule → **not deleted**
- Useful for pure user-based or tag-based management

## Testing Rules

### Preview Rules

Use the API to preview what rules would match:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/rules/preview | jq
```

### Dry Run Mode

Always test rules with `dry_run: true`:

```yaml
app:
  dry_run: true
  enable_deletion: false
```

Then check:
- Timeline page - see scheduled deletions
- Scheduled Deletions page - review all candidates
- Deletion reasons - understand why items are scheduled

## Best Practices

1. **Start simple, add complexity**
   - Begin with default retention
   - Add tag rules for exceptions
   - Add user/watched rules as needed

2. **Use descriptive rule names**
   - `name: "Kids Content"` not `name: "Rule 1"`
   - Makes logs and UI clearer

3. **Order matters in user rules**
   - First matching user wins
   - Put more specific rules first

4. **Test in dry run**
   - Always test new rules with `dry_run: true`
   - Review Timeline and Scheduled Deletions
   - Verify expected behavior

5. **Monitor initially**
   - Check logs after rule changes
   - Review Timeline daily at first
   - Adjust retention as needed

## Troubleshooting

### Rule not matching

**Check:**
1. Rule is `enabled: true`
2. Tag exists in Radarr/Sonarr and sync has run
3. User identifier matches exactly (check Jellyseerr)
4. Required integrations are enabled (Jellyseerr for user rules, Jellystat for watched)

### Items not being deleted

**Check:**
1. `dry_run: false` and `enable_deletion: true`
2. Retention period has actually passed
3. Item not excluded (check Shield icon in UI)
4. Rule evaluation logs for actual applied rule

### Watched rules not working

**Check:**
1. Jellystat integration enabled and connected
2. Watch history is being tracked in Jellystat
3. `require_watched: true` is set correctly
4. Sync has run to pull watch data

## Related Documentation

- [Configuration](Configuration) - Main configuration guide
- [API Reference](API-Reference) - API endpoints for rules
- [Troubleshooting](Troubleshooting) - Common issues

## Next Steps

- Test rules with [Quick Start](Quick-Start)
- Review [Deletion Timeline](Deletion-Timeline) to see rules in action
- Explore [API Reference](API-Reference) for automation
