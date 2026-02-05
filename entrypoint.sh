#!/bin/sh
set -e

# Default values if not provided
PUID=${PUID:-1000}
PGID=${PGID:-1000}

# POSIX-compliant validation: ensure PUID/PGID are positive integers
# Using case statement instead of bash-only [[ =~ ]] regex
is_positive_integer() {
    case "$1" in
        ''|*[!0-9]*) return 1 ;;  # Empty or contains non-digits
        0) return 1 ;;             # Zero is not positive
        *) return 0 ;;             # Valid positive integer
    esac
}

if ! is_positive_integer "$PUID"; then
    echo "Error: PUID must be a positive integer, got: $PUID"
    exit 1
fi

if ! is_positive_integer "$PGID"; then
    echo "Error: PGID must be a positive integer, got: $PGID"
    exit 1
fi

# Get current appuser UID/GID
CURRENT_UID=$(id -u appuser 2>/dev/null) || CURRENT_UID=""
CURRENT_GID=$(id -g appuser 2>/dev/null) || CURRENT_GID=""

# Check if appuser exists
if [ -z "$CURRENT_UID" ]; then
    echo "Error: appuser does not exist"
    exit 1
fi

# Adjust appuser UID/GID if different
if [ "$PUID" != "$CURRENT_UID" ] || [ "$PGID" != "$CURRENT_GID" ]; then
    echo "Adjusting appuser UID:GID from $CURRENT_UID:$CURRENT_GID to $PUID:$PGID"
    
    # Modify group first (using shadow's groupmod which is installed in Alpine)
    if ! groupmod -o -g "$PGID" appuser 2>/dev/null; then
        echo "Warning: Could not change appuser GID to $PGID"
    fi
    
    # Modify user (using shadow's usermod which is installed in Alpine)
    if ! usermod -o -u "$PUID" appuser 2>/dev/null; then
        echo "Warning: Could not change appuser UID to $PUID"
    fi
    
    # Update ownership of /app/data
    if [ -d /app/data ]; then
        chown -R appuser:appuser /app/data 2>/dev/null || true
    fi
fi

# Drop privileges and execute the command as appuser
exec su-exec appuser "$@"
