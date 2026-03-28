package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/godbus/dbus/v5"
)

type daemon struct {
	bus *dbus.Conn
	cfg config

	lastSuspend  time.Time
	lastShutdown time.Time
}

func (d *daemon) run(ctx context.Context) error {
	if d.cfg.OneShot {
		log.Println("running one-shot evaluation")
		return d.evaluateAndAct()
	}

	if err := d.bus.AddMatchSignal(
		dbus.WithMatchObjectPath(dbus.ObjectPath(displayDevicePath)),
		dbus.WithMatchInterface(propertiesInterface),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		return fmt.Errorf("failed to subscribe to UPower signals: %w", err)
	}

	sigCh := make(chan *dbus.Signal, 16)
	d.bus.Signal(sigCh)
	defer d.bus.RemoveSignal(sigCh)

	if d.cfg.PollInterval == 0 {
		log.Printf("started (suspend<=%.1f%% shutdown<=%.1f%% poll=disabled; signal-driven)", d.cfg.SuspendThreshold, d.cfg.ShutdownThreshold)
	} else {
		log.Printf("started (suspend<=%.1f%% shutdown<=%.1f%% poll=%s)", d.cfg.SuspendThreshold, d.cfg.ShutdownThreshold, d.cfg.PollInterval)
	}

	if err := d.evaluateAndAct(); err != nil {
		log.Printf("initial check failed: %v", err)
	}

	var ticker *time.Ticker
	if d.cfg.PollInterval > 0 {
		ticker = time.NewTicker(d.cfg.PollInterval)
		defer ticker.Stop()
	}

	var tick <-chan time.Time
	if ticker != nil {
		tick = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("stopping")
			return nil
		case <-tick:
			if err := d.evaluateAndAct(); err != nil {
				log.Printf("periodic check failed: %v", err)
			}
		case sig := <-sigCh:
			if sig == nil {
				continue
			}
			if err := d.handleSignal(sig); err != nil {
				log.Printf("signal handling failed: %v", err)
			}
		}
	}
}

func (d *daemon) handleSignal(sig *dbus.Signal) error {
	if sig.Name != propertiesInterface+".PropertiesChanged" {
		return nil
	}
	if len(sig.Body) < 2 {
		return nil
	}

	iface, _ := sig.Body[0].(string)
	if iface != upowerDeviceInterface {
		return nil
	}

	changed, _ := sig.Body[1].(map[string]dbus.Variant)
	if len(changed) == 0 {
		return nil
	}

	if _, ok := changed["Percentage"]; !ok {
		if _, stateChanged := changed["State"]; !stateChanged {
			return nil
		}
	}

	return d.evaluateAndAct()
}

func (d *daemon) evaluateAndAct() error {
	if d.cfg.TestPercentage != nil {
		percentage := *d.cfg.TestPercentage
		log.Printf("test mode active: percentage=%.1f%%", percentage)
		if percentage <= d.cfg.ShutdownThreshold {
			return d.muteAndShutdown(percentage)
		}
		if percentage <= d.cfg.SuspendThreshold {
			return d.suspend(percentage)
		}
		return nil
	}

	percentage, state, err := d.getBatteryInfo()
	if err != nil {
		return err
	}
	if state == stateDischarging {
		if percentage <= d.cfg.ShutdownThreshold {
			return d.muteAndShutdown(percentage)
		}
		if percentage <= d.cfg.SuspendThreshold {
			return d.suspend(percentage)
		}
		return nil
	}
	if state == stateCharging {
		return nil
	}

	onBattery, err := d.isOnBattery()
	if err != nil {
		return err
	}
	if !onBattery {
		return nil
	}

	if percentage <= d.cfg.ShutdownThreshold {
		return d.muteAndShutdown(percentage)
	}
	if percentage <= d.cfg.SuspendThreshold {
		return d.suspend(percentage)
	}

	return nil
}

func (d *daemon) isOnBattery() (bool, error) {
	obj := d.bus.Object(upowerService, dbus.ObjectPath(upowerPath))
	v, err := obj.GetProperty(upowerInterface + ".OnBattery")
	if err != nil {
		return false, fmt.Errorf("read OnBattery: %w", err)
	}
	onBattery, ok := v.Value().(bool)
	if !ok {
		return false, fmt.Errorf("unexpected OnBattery type: %T", v.Value())
	}
	return onBattery, nil
}

func (d *daemon) getBatteryInfo() (float64, uint32, error) {
	obj := d.bus.Object(upowerService, dbus.ObjectPath(displayDevicePath))

	vp, err := obj.GetProperty(upowerDeviceInterface + ".Percentage")
	if err != nil {
		return 0, 0, fmt.Errorf("read Percentage: %w", err)
	}
	vstate, err := obj.GetProperty(upowerDeviceInterface + ".State")
	if err != nil {
		return 0, 0, fmt.Errorf("read State: %w", err)
	}

	percentage, ok := vp.Value().(float64)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected Percentage type: %T", vp.Value())
	}

	state, ok := vstate.Value().(uint32)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected State type: %T", vstate.Value())
	}

	return percentage, state, nil
}
