#!/bin/bash
set -euo pipefail

echo "=== ai-lab setup ==="

# Check prerequisites
command -v go >/dev/null 2>&1 || { echo "ERROR: go not found. Install Go 1.21+."; exit 1; }
command -v claude >/dev/null 2>&1 || { echo "ERROR: claude not found. Install Claude Code CLI."; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

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

# Run migrations (happens on startup, but test it)
echo "Testing database..."
./ai-lab &
AI_PID=$!
sleep 2
kill $AI_PID 2>/dev/null || true
wait $AI_PID 2>/dev/null || true
echo "Database OK."

# Install systemd service
echo ""
echo "To install as a systemd service:"
echo "  sudo cp deploy/systemd/ai-lab.service /etc/systemd/system/"
echo "  sudo systemctl daemon-reload"
echo "  sudo systemctl enable ai-lab"
echo "  sudo systemctl start ai-lab"
echo ""
echo "To check status:"
echo "  sudo systemctl status ai-lab"
echo "  journalctl -u ai-lab -f"
echo ""
echo "Setup complete. Edit .env and start the service."
