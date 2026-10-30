#!/bin/sh
set -e

TZ="${TZ:-UTC}"
if [ -f "/usr/share/zoneinfo/$TZ" ]; then
  ln -snf "/usr/share/zoneinfo/$TZ" /etc/localtime
  echo "$TZ" > /etc/timezone
fi

mkdir -p /app/config /app/data /app/roms

# Seed defaults into the mounted config volume on first boot.
if [ ! -f /app/config/platforms.yaml ] && [ -f /defaults/platforms.yaml ]; then
  cp /defaults/platforms.yaml /app/config/platforms.yaml
fi
if [ ! -f /app/config/config.yaml ] && [ -f /defaults/config.example.yaml ]; then
  cp /defaults/config.example.yaml /app/config/config.yaml
fi

PUID="${PUID:-0}"
PGID="${PGID:-0}"

# Already non-root, or intentionally run as root.
if [ "$(id -u)" != "0" ] || [ "$PUID" = "0" ]; then
  exec /usr/local/bin/retrove
fi

GROUP_NAME="retrove"
USER_NAME="retrove"
if getent group "$PGID" >/dev/null 2>&1; then
  GROUP_NAME="$(getent group "$PGID" | cut -d: -f1)"
else
  addgroup -g "$PGID" "$GROUP_NAME"
fi
if getent passwd "$PUID" >/dev/null 2>&1; then
  USER_NAME="$(getent passwd "$PUID" | cut -d: -f1)"
else
  adduser -u "$PUID" -G "$GROUP_NAME" -D -H "$USER_NAME"
fi

# Only chown writable app state. ROM trees can be huge / read-only.
for d in /app/config /app/data; do
  if [ -d "$d" ]; then
    chown -R "$PUID:$PGID" "$d" || true
  fi
done

export HOME=/tmp
exec su-exec "$PUID:$PGID" /usr/local/bin/retrove
