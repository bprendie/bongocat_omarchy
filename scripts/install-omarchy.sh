#!/usr/bin/env bash
set -euo pipefail

APP_NAME="bongo-cat-omarchy"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY="${REPO_ROOT}/bin/${APP_NAME}"
LINK_PATH="/usr/local/bin/${APP_NAME}"
SERVICE_DIR="${HOME}/.config/systemd/user"
SERVICE_PATH="${SERVICE_DIR}/${APP_NAME}.service"

PORT=""
INSTALL_SERVICE=""
FIX_PERMISSIONS=""
ASSUME_YES=0

usage() {
  cat <<USAGE
Usage: ./scripts/install-omarchy.sh [options]

Builds bongo-cat-omarchy, links it into /usr/local/bin, checks USB/input
permissions, and optionally installs a user systemd service.

Options:
  --service, --enable-service   Install and enable the user systemd service
  --no-service                  Do not install the user systemd service
  --fix-permissions             Run sudo permission setup when needed
  --no-fix-permissions          Only print permission commands
  --port PATH                   ESP32 serial path, preferably /dev/serial/by-id/...
  -y, --yes                     Use yes for installer prompts
  -h, --help                    Show this help

Examples:
  ./scripts/install-omarchy.sh
  ./scripts/install-omarchy.sh --service
  ./scripts/install-omarchy.sh --service --port /dev/serial/by-id/usb-1a86_USB_Serial-if00-port0
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --service|--enable-service)
      INSTALL_SERVICE=1
      shift
      ;;
    --no-service)
      INSTALL_SERVICE=0
      shift
      ;;
    --fix-permissions)
      FIX_PERMISSIONS=1
      shift
      ;;
    --no-fix-permissions)
      FIX_PERMISSIONS=0
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
    -y|--yes)
      ASSUME_YES=1
      shift
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

ask_yes_no() {
  local prompt="$1"
  local default="${2:-y}"

  if [[ "${ASSUME_YES}" -eq 1 ]]; then
    return 0
  fi

  if [[ ! -t 0 ]]; then
    [[ "${default}" == "y" ]]
    return
  fi

  local suffix="[y/N]"
  if [[ "${default}" == "y" ]]; then
    suffix="[Y/n]"
  fi

  local answer
  read -r -p "${prompt} ${suffix} " answer
  answer="${answer:-${default}}"
  [[ "${answer}" =~ ^[Yy]$ ]]
}

run_sudo() {
  echo "+ sudo $*"
  sudo "$@"
}

has_group() {
  local group="$1"
  id -nG "${USER}" | tr ' ' '\n' | grep -qx "${group}"
}

detect_port() {
  local detected=""
  detected="$("${BINARY}" ports 2>/dev/null | awk '/^\*/ { print $2; exit }' || true)"
  if [[ -n "${detected}" ]]; then
    echo "${detected}"
    return
  fi

  if [[ -e /dev/serial/by-id/usb-1a86_USB_Serial-if00-port0 ]]; then
    echo "/dev/serial/by-id/usb-1a86_USB_Serial-if00-port0"
    return
  fi
}

write_service() {
  local exec_start="${LINK_PATH} run"
  if [[ -n "${PORT}" ]]; then
    exec_start="${exec_start} --port ${PORT}"
  fi

  mkdir -p "${SERVICE_DIR}"
  cat > "${SERVICE_PATH}" <<SERVICE
[Unit]
Description=Bongo Cat Omarchy host
After=graphical-session.target

[Service]
Type=simple
ExecStart=${exec_start}
Restart=on-failure
RestartSec=3

[Install]
WantedBy=default.target
SERVICE
}

cd "${REPO_ROOT}"

echo "==> Building ${APP_NAME}"
mkdir -p bin
GOCACHE="${REPO_ROOT}/.gocache" GOMODCACHE="${REPO_ROOT}/.gomodcache" go build -o "${BINARY}" ./cmd/bongo-cat-omarchy

echo "==> Installing command"
if [[ "$(readlink "${LINK_PATH}" 2>/dev/null || true)" == "${BINARY}" ]]; then
  echo "Already linked: ${LINK_PATH} -> ${BINARY}"
else
  run_sudo ln -sf "${BINARY}" "${LINK_PATH}"
  echo "Linked: ${LINK_PATH} -> ${BINARY}"
fi

if [[ -z "${PORT}" ]]; then
  PORT="$(detect_port || true)"
fi

echo
echo "==> Detected serial ports"
"${BINARY}" ports || true

if [[ -n "${PORT}" ]]; then
  echo
  echo "Selected ESP32 port: ${PORT}"
else
  echo
  echo "No ESP32 serial port was auto-detected."
  echo "Plug in the ESP32 with a data USB cable, then run:"
  echo "  ${APP_NAME} ports"
  echo "  ./scripts/install-omarchy.sh --service --port /dev/serial/by-id/..."
fi

echo
echo "==> Permission check"
missing_groups=()
has_group input || missing_groups+=("input")
has_group uucp || missing_groups+=("uucp")

if [[ "${#missing_groups[@]}" -eq 0 ]]; then
  echo "User ${USER} is already in input and uucp."
else
  echo "User ${USER} is missing group(s): ${missing_groups[*]}"
  echo "Permanent fix:"
  echo "  sudo usermod -aG ${missing_groups[*]// /,} ${USER}"
  echo "Then log out and back in."

  should_fix=0
  if [[ "${FIX_PERMISSIONS}" == "1" ]]; then
    should_fix=1
  elif [[ "${FIX_PERMISSIONS}" != "0" ]] && ask_yes_no "Apply permanent group fix with sudo usermod now?" "y"; then
    should_fix=1
  fi

  if [[ "${should_fix}" -eq 1 ]]; then
    groups_csv="$(IFS=,; echo "${missing_groups[*]}")"
    run_sudo usermod -aG "${groups_csv}" "${USER}"
    echo "Group membership updated. You still need to log out and back in for it to apply."
  fi
fi

if [[ -n "${PORT}" && -e "${PORT}" ]]; then
  tty_path="$(readlink -f "${PORT}")"
  if [[ -r "${tty_path}" && -w "${tty_path}" ]]; then
    echo "Current session can access ${tty_path}."
  else
    echo "Current session cannot access ${tty_path}."
    echo "Temporary fix until next unplug/reboot:"
    echo "  sudo setfacl -m u:${USER}:rw ${tty_path}"

    should_acl=0
    if [[ "${FIX_PERMISSIONS}" == "1" ]]; then
      should_acl=1
    elif [[ "${FIX_PERMISSIONS}" != "0" ]] && ask_yes_no "Apply temporary ACL for this plugged-in ESP32 now?" "y"; then
      should_acl=1
    fi

    if [[ "${should_acl}" -eq 1 ]]; then
      run_sudo setfacl -m "u:${USER}:rw" "${tty_path}"
    fi
  fi
fi

echo
echo "==> Service"
if [[ -z "${INSTALL_SERVICE}" ]]; then
  if ask_yes_no "Install and enable the user systemd service?" "y"; then
    INSTALL_SERVICE=1
  else
    INSTALL_SERVICE=0
  fi
fi

if [[ "${INSTALL_SERVICE}" -eq 1 ]]; then
  write_service
  systemctl --user daemon-reload
  systemctl --user enable "${APP_NAME}.service"
  systemctl --user restart "${APP_NAME}.service"

  echo
  echo "Service installed:"
  echo "  ${SERVICE_PATH}"
  systemctl --user --no-pager status "${APP_NAME}.service" || true
else
  echo "Service not installed. Manual run:"
  if [[ -n "${PORT}" ]]; then
    echo "  ${APP_NAME} run --port ${PORT}"
  else
    echo "  ${APP_NAME} run"
  fi
fi

echo
echo "Done."
