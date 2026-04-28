# Bongo Cat Omarchy

Linux/Omarchy host binary for the ESP32 firmware from [`vostoklabs/bongo_cat_monitor`](https://github.com/vostoklabs/bongo_cat_monitor).

This app sends the same serial protocol as the upstream desktop apps, but uses Linux `/dev/input/event*` devices for typing detection so it works under Arch, Omarchy, Hyprland, and Wayland without X11 global-key APIs.

## Hardware

Use a data-capable USB cable. On a CH340-based ESP32, the device usually appears as:

```text
/dev/serial/by-id/usb-1a86_USB_Serial-if00-port0
```

Check detected ports:

```bash
bongo-cat-omarchy ports
```

## Build

```bash
go build -o bin/bongo-cat-omarchy ./cmd/bongo-cat-omarchy
```

## Install On Omarchy

The installer builds the binary, symlinks it to `/usr/local/bin`, checks USB/input permissions, and can install a user systemd service.

```bash
./scripts/install-omarchy.sh
```

The guided install asks before applying permission fixes or enabling the service.

Install and enable the background service in one command:

```bash
./scripts/install-omarchy.sh --service --port /dev/serial/by-id/usb-1a86_USB_Serial-if00-port0
```

For a mostly unattended install:

```bash
./scripts/install-omarchy.sh --service --fix-permissions -y
```

The script uses `sudo` only when it needs to:

- link `/usr/local/bin/bongo-cat-omarchy`
- add your user to `uucp,input`
- grant a temporary ACL to the currently plugged-in ESP32 serial device

```text
/usr/local/bin/bongo-cat-omarchy -> /path/to/repo/bin/bongo-cat-omarchy
```

## Permissions

You need access to both:

- ESP32 serial device, usually group `uucp` on Arch
- Keyboard input devices, group `input`

Set that up once:

```bash
sudo usermod -aG uucp,input "$USER"
```

Then log out and back in. Until your new group membership is active, temporary per-device access can be granted with:

```bash
sudo setfacl -m u:"$USER":rw /dev/ttyUSB1
```

Use the real tty shown by `bongo-cat-omarchy ports`.

## Run

```bash
bongo-cat-omarchy run --port /dev/serial/by-id/usb-1a86_USB_Serial-if00-port0
```

Useful diagnostics:

```bash
bongo-cat-omarchy ports
bongo-cat-omarchy inputs
bongo-cat-omarchy doctor
```

## Background Service

The install script writes:

```text
~/.config/systemd/user/bongo-cat-omarchy.service
```

Manage it with:

```bash
systemctl --user status bongo-cat-omarchy
systemctl --user restart bongo-cat-omarchy
systemctl --user stop bongo-cat-omarchy
```

Logs:

```bash
journalctl --user -u bongo-cat-omarchy -f
```

If the ESP32 is unplugged when the service starts, the process stays active and retries every 3 seconds until the serial device appears.
