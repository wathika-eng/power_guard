BINARY := go-power-guardian
SERVICE := power-guardian
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
ROOT_BINDIR ?= /usr/local/bin
SYSTEMD_USER_DIR ?= $(HOME)/.config/systemd/user
WORKDIR := $(CURDIR)

.PHONY: tidy build run install-bin install-bin-root install-systemd-user enable-systemd-user restart-systemd-user logs clean test-suspend test-shutdown serviceman-dryrun-agent serviceman-add-agent serviceman-add-daemon serviceman-add-daemon-best serviceman-enable serviceman-start serviceman-logs

tidy:
	go mod tidy

build: tidy
	go build -o $(BINARY) .

run: build
	./$(BINARY)

install-bin: build
	install -Dm755 ./$(BINARY) $(BINDIR)/$(BINARY)

install-bin-root: build
	sudo install -Dm755 ./$(BINARY) $(ROOT_BINDIR)/$(BINARY)

install-systemd-user:
	install -Dm644 ./power-guardian.service $(SYSTEMD_USER_DIR)/$(SERVICE).service
	systemctl --user daemon-reload

enable-systemd-user: install-bin install-systemd-user
	systemctl --user enable --now $(SERVICE).service

restart-systemd-user:
	systemctl --user restart $(SERVICE).service

logs:
	journalctl --user -u $(SERVICE).service -f

# Safe test: simulates battery at 5%% in one-shot mode and never suspends.
test-suspend: build
	POWER_DRY_RUN=1 POWER_ONESHOT=1 POWER_TEST_PERCENTAGE=5 ./$(BINARY)

# Safe test: simulates battery at 3%% in one-shot mode and never shuts down.
test-shutdown: build
	POWER_DRY_RUN=1 POWER_ONESHOT=1 POWER_TEST_PERCENTAGE=3 ./$(BINARY)

# Runtime note: POWER_POLL_INTERVAL=0 disables polling (signal-driven only).

# Preview the generated user-login service without installing anything.
serviceman-dryrun-agent:
	serviceman add --agent --dryrun --workdir $(WORKDIR) --name '$(SERVICE)' -- $(BINDIR)/$(BINARY)

# Install as a user-login service (recommended for desktop audio mute behavior).
serviceman-add-agent: install-bin
	serviceman add --agent --workdir $(WORKDIR) --name '$(SERVICE)' -- $(BINDIR)/$(BINARY)

# Install as a system boot service (Linux default, may run as root).
serviceman-add-daemon: install-bin
	serviceman add --daemon --workdir $(WORKDIR) --name '$(SERVICE)' -- $(BINDIR)/$(BINARY)

# Best unattended setup for battery protection while sleeping.
serviceman-add-daemon-best: install-bin-root
	serviceman add --daemon --user root --group root --url 'https://upower.freedesktop.org/' --workdir / --name '$(SERVICE)' -- /usr/bin/env POWER_SUSPEND_THRESHOLD=5 POWER_SHUTDOWN_THRESHOLD=3 POWER_POLL_INTERVAL=0 POWER_ACTION_COOLDOWN=2m POWER_SUSPEND_RETRIES=3 POWER_SUSPEND_RETRY_DELAY=2s POWER_SHUTDOWN_RETRIES=4 POWER_SHUTDOWN_RETRY_DELAY=1s POWER_EMERGENCY_POWEROFF=true $(ROOT_BINDIR)/$(BINARY)

serviceman-enable:
	serviceman enable '$(SERVICE)'

serviceman-start:
	serviceman start '$(SERVICE)'

serviceman-logs:
	serviceman logs '$(SERVICE)'

clean:
	rm -f ./$(BINARY)
