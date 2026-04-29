#!/usr/bin/env bash
set -euo pipefail

SERVICE="bongo-cat-omarchy.service"

echo "Restarting ${SERVICE}..."
systemctl --user restart "${SERVICE}"

echo
systemctl --user status "${SERVICE}" --no-pager

echo
echo "Logs:"
echo "  journalctl --user -u ${SERVICE} -f"
