# Bongo Cat Omarchy

A tiny Linux sidecar for your tiny typing desk companion.

This is the Omarchy/Arch host app for the ESP32 Bongo Cat monitor firmware from
[`vostoklabs/bongo_cat_monitor`](https://github.com/vostoklabs/bongo_cat_monitor).
It watches your keyboard, sends CPU/RAM/WPM stats over USB serial, and lets the
little screen do its little bongo routine while you work.

It is intentionally small:

- one Go binary
- no Electron
- no Python runtime
- no X11 global-key tricks
- works on Wayland/Hyprland by reading Linux input devices directly

## Quick Start

```bash
git clone https://github.com/bprendie/bongocat_omarchy.git
cd bongocat_omarchy
./scripts/install-omarchy.sh
```

The installer builds the binary, links it into `/usr/local/bin`, checks USB and
keyboard permissions, and asks whether to install the background service.

For the full “just make it live in the background” path:

```bash
./scripts/install-omarchy.sh --service --fix-permissions -y
```

## The Cable Situation

Use a real data USB cable. Charge-only cables are very good at making this
project look broken.

A CH340-based ESP32 usually shows up like this:

```text
/dev/serial/by-id/usb-1a86_USB_Serial-if00-port0
```

Check what your machine sees:

```bash
bongo-cat-omarchy ports
```

If the only serial device looks unrelated to an ESP32, CH340, CP210x, FTDI, or
Espressif board, your ESP32 probably is not enumerating yet.

## Install Modes

Guided install:

```bash
./scripts/install-omarchy.sh
```

Install as a user service:

```bash
./scripts/install-omarchy.sh --service
```

Install as a user service with an explicit ESP32 path:

```bash
./scripts/install-omarchy.sh --service --port /dev/serial/by-id/usb-1a86_USB_Serial-if00-port0
```

Mostly unattended:

```bash
./scripts/install-omarchy.sh --service --fix-permissions -y
```

The installer uses `sudo` only for system-level bits:

- linking `/usr/local/bin/bongo-cat-omarchy`
- adding your user to `uucp,input`
- adding a temporary ACL for the plugged-in ESP32 if your current session needs it
- installing a persistent udev rule so USB serial access survives unplug/replug

## Permissions, The Least Silly Part

The app needs access to two things:

- the ESP32 serial device, usually group `uucp` on Arch
- keyboard input devices, group `input`

Permanent fix:

```bash
sudo usermod -aG uucp,input "$USER"
```

Then log out and back in.

Temporary fix for the current plugged-in ESP32:

```bash
sudo setfacl -m u:"$USER":rw /dev/ttyUSB1
```

Use the real tty from:

```bash
bongo-cat-omarchy ports
```

That temporary ACL disappears when the device node is recreated. The installer
can also install a udev rule for common ESP32 USB serial adapters:

```bash
./scripts/install-omarchy.sh --udev-rule
```

After installing the rule, replug the ESP32.

## Running It By Hand

```bash
bongo-cat-omarchy run --port /dev/serial/by-id/usb-1a86_USB_Serial-if00-port0
```

Diagnostics:

```bash
bongo-cat-omarchy ports
bongo-cat-omarchy inputs
bongo-cat-omarchy doctor
```

## Background Service

The service is a user systemd service:

```text
~/.config/systemd/user/bongo-cat-omarchy.service
```

Manage it:

```bash
systemctl --user status bongo-cat-omarchy
systemctl --user restart bongo-cat-omarchy
systemctl --user stop bongo-cat-omarchy
```

Watch logs:

```bash
journalctl --user -u bongo-cat-omarchy -f
```

If the ESP32 is unplugged when the service starts, the process stays alive and
retries every 3 seconds until the serial device appears.

## Build

```bash
go build -o bin/bongo-cat-omarchy ./cmd/bongo-cat-omarchy
```

## What It Sends

The binary talks to the firmware using the same simple serial commands as the
upstream desktop apps:

```text
PING
TIME:HH:MM
STATS:CPU:12,RAM:34,WPM:56
SPEED:90
STOP
STREAK_ON
STREAK_OFF
IDLE_START
```

That is the whole trick. Keyboard goes click, serial line goes beep, tiny screen
does bongos.

## Credits

Firmware and original desktop app:
[`vostoklabs/bongo_cat_monitor`](https://github.com/vostoklabs/bongo_cat_monitor)

This repo is just the Omarchy-shaped host binary for the same delightful little
desk toy.
