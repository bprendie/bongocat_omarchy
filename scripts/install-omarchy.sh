#!/usr/bin/env bash
set -euo pipefail

APP_NAME="bongo-cat-omarchy"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY="${REPO_ROOT}/bin/${APP_NAME}"
LINK_PATH="/usr/local/bin/${APP_NAME}"
SERVICE_DIR="${HOME}/.config/systemd/user"
SERVICE_PATH="${SERVICE_DIR}/${APP_NAME}.service"
ENABLE_SERVICE=0
PORT=""

usage() {
  cat <<USAGE
Usage: ./scripts/install-omarchy.sh [options]

Options:
  --enable-service       Install and enable the systemd user service
  --port PATH            Serial port for the service, preferably /dev/serial/by-id/...
  -h, --help             Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --enable-service)
      ENABLE_SERVICE=1
      shift
      ;;
    --port)
      PORT="${2:-}"
      if [[ -z "${PORT}" ]]; then
        echo "--port requires a value" >&2
        exit 2
      fi
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

cd "${REPO_ROOT}"

echo "Building ${APP_NAME}..."
mkdir -p bin
GOCACHE="${REPO_ROOT}/.gocache" GOMODCACHE="${REPO_ROOT}/.gomodcache" go build -o "${BINARY}" ./cmd/bongo-cat-omarchy

echo "Linking ${LINK_PATH}..."
sudo ln -sf "${BINARY}" "${LINK_PATH}"

cat <<INFO

USB/input permissions:
  sudo usermod -aG uucp,input "$USER"

Log out and back in after changing groups.

Detected serial ports:
INFO
"${BINARY}" ports || true

if [[ "${ENABLE_SERVICE}" -eq 1 ]]; then
  mkdir -p "${SERVICE_DIR}"
  EXEC_START="${LINK_PATH} run"
  if [[ -n "${PORT}" ]]; then
    EXEC_START="${EXEC_START} --port ${PORT}"
  fi

  cat > "${SERVICE_PATH}" <<SERVICE
[Unit]
Description=Bongo Cat Omarchy host
After=graphical-session.target

[Service]
Type=simple
ExecStart=${EXEC_START}
Restart=on-failure
RestartSec=3

[Install]
WantedBy=default.target
SERVICE

  systemctl --user daemon-reload
  systemctl --user enable --now "${APP_NAME}.service"

  echo
  echo "Service installed and started:"
  systemctl --user --no-pager status "${APP_NAME}.service" || true
fi

echo
echo "Installed:"
echo "  ${LINK_PATH}"
