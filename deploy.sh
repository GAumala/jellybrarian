#!/usr/bin/env bash
set -e

REMOTE="gabriel@raspberry.local"
REMOTE_DIR="/home/gabriel/services/jellybrarian"

echo "Syncing to ${REMOTE}:${REMOTE_DIR} ..."
rsync -av --progress \
  --exclude='/.git' \
  --filter="dir-merge,- .gitignore" \
  ./ "${REMOTE}:${REMOTE_DIR}"

echo "Building and starting container on Pi ..."
ssh "${REMOTE}" "cd ${REMOTE_DIR} && \
  sudo docker stop jellybrarian 2>/dev/null || true && \
  sudo docker rm jellybrarian 2>/dev/null || true && \
  sudo docker build -t jellybrarian . && \
  sudo docker run -d \
    --name jellybrarian \
    --restart unless-stopped \
    --user 1000 \
    -p 8296:8090 \
    -v \"\$(pwd)/config.toml:/app/config.toml:ro\" \
    -v /mnt/hdd0:/mnt/hdd0 \
    jellybrarian"

echo "Done. Jellybrarian is running at http://raspberry.local:8296"
