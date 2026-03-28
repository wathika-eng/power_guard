# Power Guard

Power Guard is a lightweight Go daemon that monitors battery levels and automatically suspends or shuts down the system when thresholds are reached. It is signal-driven (uses UPower/D-Bus events) rather than polling, making it efficient and responsive.

Behavior:

- At battery `<= 5%`: request suspend.
- At battery `<= 3%`: mute audio, then request shutdown.

Battery monitoring is done over D-Bus via `github.com/godbus/dbus/v5` using UPower.

## Requirements

- Linux with systemd
- UPower service running (default on Ubuntu/Fedora desktops)
- `wpctl` (PipeWire) or `pactl` (PulseAudio compatibility) for mute command

## Build

```bash
git clone https://github.com/wathika-eng/power_guard --depth 1
cd power_guard
go mod tidy
go build -o go-power-guardian .
```

## Install using serviceman (preferred)

```bash
curl -sS https://webi.sh/serviceman | sh; \
source ~/.config/envman/PATH.env

make build

make serviceman-add-daemon-best
```

<!-- ## Install Binary

```bash
install -Dm755 ./go-power-guardian ~/.local/bin/go-power-guardian
```

## Install as systemd User Service

```bash
install -Dm644 ./power-guardian.service ~/.config/systemd/user/power-guardian.service
systemctl --user daemon-reload
systemctl --user enable --now power-guardian.service
``` -->

Check logs:

```bash
journalctl --user -u power-guardian.service -f
```

## Configuration

Service environment variables:

- `POWER_SUSPEND_THRESHOLD` (default `5`)
- `POWER_SHUTDOWN_THRESHOLD` (default `3`)
- `POWER_POLL_INTERVAL` (default `20s`)
- `POWER_ACTION_COOLDOWN` (default `2m`)

If suspend/shutdown is denied by policy on your distro, add a polkit rule allowing your local active user to call `org.freedesktop.login1` power actions.
