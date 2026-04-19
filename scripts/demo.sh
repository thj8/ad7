#!/usr/bin/env bash
# demo.sh — Run all API test scripts
# Usage: BASE_URL=http://host:8080 ./scripts/demo.sh
#         ./scripts/demo.sh [competitions|challenges|submissions|leaderboard|notifications|hints|analytics]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Export env vars for sub-scripts
export BASE_URL="${BASE_URL:-http://localhost:8080}"
export JWT_SECRET="${JWT_SECRET:-change-me-in-production}"

SCRIPTS=(
  competitions
  challenges
  submissions
  leaderboard
  notifications
  hints
  analytics
)

run_one() {
  local name="$1"
  local script="$SCRIPT_DIR/test-${name}.sh"
  if [ -f "$script" ]; then
    echo ""
    echo ">>>>>>>>>> Running: $name <<<<<<<<<<"
    bash "$script"
  else
    echo "ERROR: $script not found"
    return 1
  fi
}

if [ $# -gt 0 ]; then
  # Run specific scripts by name
  for name in "$@"; do
    run_one "$name"
  done
else
  # Run all
  for name in "${SCRIPTS[@]}"; do
    run_one "$name"
  done
fi

echo ""
echo "============================================"
echo "  All tests complete."
echo "============================================"
