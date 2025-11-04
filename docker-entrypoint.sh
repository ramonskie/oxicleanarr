#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

# Change user/group IDs if they differ from defaults
groupmod -o -g "$PGID" prunarr
usermod -o -u "$PUID" prunarr

# Fix ownership if IDs were changed
if [ "$PUID" != "1000" ] || [ "$PGID" != "1000" ]; then
    echo "Setting ownership to $PUID:$PGID..."
    chown -R prunarr:prunarr /app
fi

# Execute command as prunarr user
exec su-exec prunarr "$@"
