#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

echo "Starting OxiCleanarr as UID:GID = $PUID:$PGID"

# Fix ownership on data directories to match PUID:PGID
# This ensures bind-mounted volumes are writable by the container user
# Only changes ownership if directories exist and current ownership differs
for dir in /app/config /app/data /app/logs; do
    if [ -d "$dir" ] && [ "$(stat -c '%u:%g' "$dir")" != "$PUID:$PGID" ]; then
        echo "Fixing ownership on $dir"
        chown -R "$PUID:$PGID" "$dir" 2>/dev/null || true
    fi
done

# Execute command as the specified UID:GID
exec su-exec "$PUID:$PGID" "$@"
