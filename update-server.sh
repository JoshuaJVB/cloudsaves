#!/usr/bin/env bash
#
# Update and redeploy the CloudSave server container.
# Run it from anywhere — it operates on the repository it lives in:
#
#     ./update-server.sh
#
# The API key is preserved from the running container automatically. To set a
# different one, pass it explicitly:
#
#     API_KEY=mysecretkey ./update-server.sh
#
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_DIR"

echo "==> Pulling latest code..."
git pull --ff-only

# Prefer an explicitly provided API_KEY; otherwise reuse the one the running
# container was started with, so it survives the update.
if [ -z "${API_KEY:-}" ]; then
  API_KEY="$(docker inspect cloudsave \
    --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
    | sed -n 's/^API_KEY=//p' || true)"
fi
if [ -z "${API_KEY:-}" ]; then
  echo "!! Could not determine the API key."
  echo "   Re-run with it set explicitly:  API_KEY=yourkey ./update-server.sh"
  exit 1
fi

echo "==> Rebuilding image..."
docker build -t cloudsave-server "$REPO_DIR/server"

echo "==> Restarting container..."
docker stop cloudsave >/dev/null 2>&1 || true
docker rm cloudsave >/dev/null 2>&1 || true
docker run -d \
  --name cloudsave \
  -p 45231:45231 \
  -v "$REPO_DIR/server/data:/data" \
  -e API_KEY="$API_KEY" \
  --restart unless-stopped \
  cloudsave-server

echo "==> Done. CloudSave server updated and running on port 45231."
