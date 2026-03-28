package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/godbus/dbus/v5"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	bus, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Fatalf("failed to connect system bus: %v", err)
	}
	defer bus.Close()

	d := &daemon{bus: bus, cfg: cfg}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := d.run(ctx); err != nil {
		log.Fatalf("daemon failed: %v", err)
	}
}
