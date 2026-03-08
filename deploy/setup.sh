#!/bin/bash
set -euo pipefail

echo "=== ai-lab setup ==="

# Check prerequisites
command -v go >/dev/null 2>&1 || { echo "ERROR: go not found. Install Go 1.21+."; exit 1; }
command -v claude >/dev/null 2>&1 || { echo "ERROR: claude not found. Install Claude Code CLI."; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CURRENT_USER="$(whoami)"
CURRENT_HOME="$HOME"

cd "$PROJECT_DIR"

# Setup .env if not exists
if [ ! -f .env ]; then
    cp .env.example .env
    echo "Created .env from .env.example"
    echo "Edit .env with your Discord bot token and user ID before starting."
fi

# Create data directory
mkdir -p data/cron-logs

# Build
echo "Building..."
go build -o ai-lab .
echo "Build complete."

# Generate and install systemd service
echo "Installing systemd service..."
sed -e "s|__USER__|${CURRENT_USER}|g" \
    -e "s|__HOME__|${CURRENT_HOME}|g" \
    -e "s|__WORKDIR__|${PROJECT_DIR}|g" \
    deploy/systemd/ai-lab.service | sudo tee /etc/systemd/system/ai-lab.service > /dev/null

sudo systemctl daemon-reload
sudo systemctl enable ai-lab
echo "Systemd service installed and enabled."

echo ""
echo "To start:  sudo systemctl start ai-lab"
echo "To check:  sudo systemctl status ai-lab"
echo "Logs:      journalctl -u ai-lab -f"
echo ""
echo "Setup complete. Edit .env and start the service."
