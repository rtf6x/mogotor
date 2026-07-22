#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="${INSTALL_DIR:-/opt/mogotor}"
DATA_DIR="${DATA_DIR:-/var/lib/mogotor}"
SERVICE_USER="${SERVICE_USER:-$USER}"
PORT="${PORT:-8188}"

echo "Building mogotor..."
cd "$ROOT"
go build -o "$ROOT/mogotor" ./cmd/mogotor

echo "Installing binary to $INSTALL_DIR"
sudo install -d -o "$SERVICE_USER" -g "$SERVICE_USER" "$INSTALL_DIR"
sudo install -o "$SERVICE_USER" -g "$SERVICE_USER" -m 755 "$ROOT/mogotor" "$INSTALL_DIR/mogotor"

echo "Preparing data dir $DATA_DIR"
sudo install -d -o "$SERVICE_USER" -g "$SERVICE_USER" "$DATA_DIR"

echo "Installing systemd unit"
sudo sed \
  -e "s|@USER@|$SERVICE_USER|g" \
  -e "s|/opt/mogotor|$INSTALL_DIR|g" \
  -e "s|/var/lib/mogotor|$DATA_DIR|g" \
  -e "s|:4342|:$PORT|g" \
  "$ROOT/deploy/mogotor.service" | sudo tee /etc/systemd/system/mogotor.service >/dev/null

sudo systemctl daemon-reload
sudo systemctl enable --now mogotor.service
sudo systemctl status mogotor.service --no-pager

echo "Done. Mogotor should listen on :$PORT"
