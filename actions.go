package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

func (d *daemon) suspend(percentage float64) error {
	now := time.Now()
	if !d.lastSuspend.IsZero() && now.Sub(d.lastSuspend) < d.cfg.Cooldown {
		return nil
	}

	log.Printf("battery %.1f%% <= %.1f%%: suspending", percentage, d.cfg.SuspendThreshold)
	if d.cfg.DryRun {
		d.lastSuspend = now
		log.Println("dry-run: suspend skipped")
		return nil
	}

	var lastErr error
	for attempt := 1; attempt <= d.cfg.SuspendRetries; attempt++ {
		if err := d.trySuspendOnce(); err == nil {
			d.lastSuspend = now
			return nil
		} else {
			lastErr = err
			if attempt < d.cfg.SuspendRetries {
				log.Printf("suspend attempt %d/%d failed: %v; retrying in %s", attempt, d.cfg.SuspendRetries, err, d.cfg.SuspendRetryDelay)
				time.Sleep(d.cfg.SuspendRetryDelay)
			}
		}
	}
	return lastErr
}

func (d *daemon) trySuspendOnce() error {
	obj := d.bus.Object(login1Service, dbus.ObjectPath(login1Path))
	call := obj.Call(login1Interface+".Suspend", 0, false)
	if call.Err != nil {
		if isBypassablePowerActionError(call.Err) {
			log.Println("suspend via login1 denied, retrying with non-interactive fallback")
			if err := runIgnoreInhibitorsAction("suspend"); err != nil {
				return fmt.Errorf("fallback suspend failed: %w", err)
			}
			return nil
		}
		return fmt.Errorf("login1 suspend failed: %w", call.Err)
	}
	return nil
}

func (d *daemon) muteAndShutdown(percentage float64) error {
	now := time.Now()
	if !d.lastShutdown.IsZero() && now.Sub(d.lastShutdown) < d.cfg.Cooldown {
		return nil
	}

	log.Printf("battery %.1f%% <= %.1f%%: muting audio and shutting down", percentage, d.cfg.ShutdownThreshold)
	if err := muteAudio(d.cfg.DryRun); err != nil {
		log.Printf("audio mute failed: %v", err)
	}
	if d.cfg.DryRun {
		d.lastShutdown = now
		log.Println("dry-run: shutdown skipped")
		return nil
	}

	var lastErr error
	for attempt := 1; attempt <= d.cfg.ShutdownRetries; attempt++ {
		if err := d.tryShutdownOnce(); err == nil {
			time.Sleep(d.cfg.ShutdownSettleDelay)
			if attempt < d.cfg.ShutdownRetries {
				continue
			}
			lastErr = fmt.Errorf("shutdown command accepted but process still alive after %s", d.cfg.ShutdownSettleDelay)
		} else {
			lastErr = err
			if attempt < d.cfg.ShutdownRetries {
				log.Printf("shutdown attempt %d/%d failed: %v; retrying in %s", attempt, d.cfg.ShutdownRetries, err, d.cfg.ShutdownRetryDelay)
				time.Sleep(d.cfg.ShutdownRetryDelay)
			}
		}
	}

	if d.cfg.EmergencyPoweroff {
		log.Printf("shutdown retries exhausted, forcing emergency poweroff")
		if err := runEmergencyPoweroff(); err != nil {
			return fmt.Errorf("emergency poweroff failed: %w", err)
		}
		d.lastShutdown = now
		return nil
	}

	return lastErr
}

func (d *daemon) tryShutdownOnce() error {
	obj := d.bus.Object(login1Service, dbus.ObjectPath(login1Path))
	call := obj.Call(login1Interface+".PowerOff", 0, false)
	if call.Err != nil {
		if isBypassablePowerActionError(call.Err) {
			log.Println("shutdown via login1 denied, retrying with non-interactive fallback")
			if err := runIgnoreInhibitorsAction("poweroff"); err != nil {
				return fmt.Errorf("fallback poweroff failed: %w", err)
			}
			return nil
		}
		return fmt.Errorf("login1 poweroff failed: %w", call.Err)
	}
	return nil
}

func isInhibitorDenied(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "block inhibitor") || strings.Contains(msg, "operation inhibited")
}

func isAuthRequired(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "interactive authentication required") || strings.Contains(msg, "access denied")
}

func isBypassablePowerActionError(err error) bool {
	return isInhibitorDenied(err) || isAuthRequired(err)
}

func runIgnoreInhibitorsAction(action string) error {
	var commands [][]string
	switch action {
	case "suspend":
		commands = [][]string{
			{"loginctl", "suspend", "-i"},
			{"systemctl", "suspend", "-i", "--force"},
			{"systemctl", "start", "suspend.target"},
		}
	case "poweroff":
		commands = [][]string{
			{"loginctl", "poweroff", "-i"},
			{"systemctl", "-i", "poweroff"},
			{"shutdown", "-P", "now"},
			{"systemctl", "poweroff", "-i", "--force", "--no-wall"},
			{"systemctl", "start", "poweroff.target", "--job-mode=replace-irreversibly"},
			{"systemctl", "poweroff", "--force", "--force", "--no-wall"},
		}
	default:
		return fmt.Errorf("unsupported fallback action: %s", action)
	}

	var errs []error
	for _, cmdArgs := range commands {
		out, err := runCommand(cmdArgs)
		if err == nil {
			return nil
		}
		if len(out) > 0 {
			errs = append(errs, fmt.Errorf("%s failed: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out))))
		} else {
			errs = append(errs, fmt.Errorf("%s failed: %w", strings.Join(cmdArgs, " "), err))
		}
	}

	return errors.Join(errs...)
}

func runEmergencyPoweroff() error {
	commands := [][]string{
		{"loginctl", "poweroff", "-i"},
		{"systemctl", "-i", "poweroff"},
		{"shutdown", "-P", "now"},
		{"systemctl", "poweroff", "--force", "--force", "--no-wall"},
		{"/usr/sbin/shutdown", "-P", "now"},
		{"/sbin/shutdown", "-P", "now"},
	}

	var errs []error
	for _, cmdArgs := range commands {
		out, err := runCommand(cmdArgs)
		if err == nil {
			return nil
		}
		if len(out) > 0 {
			errs = append(errs, fmt.Errorf("%s failed: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out))))
		} else {
			errs = append(errs, fmt.Errorf("%s failed: %w", strings.Join(cmdArgs, " "), err))
		}
	}

	return errors.Join(errs...)
}

func muteAudio(dryRun bool) error {
	if dryRun {
		log.Println("dry-run: mute skipped")
		return nil
	}

	if err := muteAudioInUserSessions(); err == nil {
		return nil
	}

	commands := [][]string{
		{"wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "1"},
		{"wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "true"},
		{"wpctl", "set-mute", "@DEFAULT_AUDIO_SOURCE@", "1"},
		{"wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "0"},
		{"pactl", "set-sink-mute", "@DEFAULT_SINK@", "1"},
		{"pactl", "set-sink-volume", "@DEFAULT_SINK@", "0%"},
		{"amixer", "-q", "set", "Master", "mute"},
		{"amixer", "-q", "sset", "Master", "mute"},
		{"amixer", "-q", "-c", "0", "sset", "Master", "mute"},
		{"amixer", "-q", "-c", "0", "sset", "Master", "0%"},
	}

	var errs []error
	for _, cmdArgs := range commands {
		out, err := runCommand(cmdArgs)
		if err == nil {
			return nil
		}
		if len(out) > 0 {
			errs = append(errs, fmt.Errorf("%s failed: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out))))
		} else {
			errs = append(errs, fmt.Errorf("%s failed: %w", strings.Join(cmdArgs, " "), err))
		}
	}

	return errors.Join(errs...)
}

func muteAudioInUserSessions() error {
	entries, err := os.ReadDir("/run/user")
	if err != nil {
		return fmt.Errorf("scan /run/user failed: %w", err)
	}

	var errs []error
	tried := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		uid := entry.Name()
		if _, err := strconv.Atoi(uid); err != nil {
			continue
		}

		u, err := user.LookupId(uid)
		if err != nil {
			continue
		}

		runtimeDir := filepath.Join("/run/user", uid)
		busPath := filepath.Join(runtimeDir, "bus")
		if _, err := os.Stat(busPath); err != nil {
			continue
		}

		envPairs := []string{
			"XDG_RUNTIME_DIR=" + runtimeDir,
			"DBUS_SESSION_BUS_ADDRESS=unix:path=" + busPath,
		}

		sessionCommands := []struct {
			name string
			args []string
		}{
			{
				name: "runuser",
				args: []string{"-u", u.Username, "--", "env", envPairs[0], envPairs[1], "wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "1"},
			},
			{
				name: "runuser",
				args: []string{"-u", u.Username, "--", "env", envPairs[0], envPairs[1], "wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "0"},
			},
			{
				name: "runuser",
				args: []string{"-u", u.Username, "--", "env", envPairs[0], envPairs[1], "pactl", "set-sink-mute", "@DEFAULT_SINK@", "1"},
			},
			{
				name: "runuser",
				args: []string{"-u", u.Username, "--", "env", envPairs[0], envPairs[1], "pactl", "set-sink-volume", "@DEFAULT_SINK@", "0%"},
			},
			{
				name: "sudo",
				args: []string{"-u", u.Username, "env", envPairs[0], envPairs[1], "wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "1"},
			},
			{
				name: "sudo",
				args: []string{"-u", u.Username, "env", envPairs[0], envPairs[1], "wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "0"},
			},
			{
				name: "sudo",
				args: []string{"-u", u.Username, "env", envPairs[0], envPairs[1], "pactl", "set-sink-mute", "@DEFAULT_SINK@", "1"},
			},
			{
				name: "sudo",
				args: []string{"-u", u.Username, "env", envPairs[0], envPairs[1], "pactl", "set-sink-volume", "@DEFAULT_SINK@", "0%"},
			},
		}

		for _, sc := range sessionCommands {
			tried++
			out, err := runCommand(append([]string{sc.name}, sc.args...))
			if err == nil {
				return nil
			}
			if len(out) > 0 {
				errs = append(errs, fmt.Errorf("%s %s failed: %w: %s", sc.name, strings.Join(sc.args, " "), err, strings.TrimSpace(string(out))))
			} else {
				errs = append(errs, fmt.Errorf("%s %s failed: %w", sc.name, strings.Join(sc.args, " "), err))
			}
		}
	}

	if tried == 0 {
		return errors.New("no user session audio bus found under /run/user")
	}

	return errors.Join(errs...)
}

func runCommand(cmdArgs []string) ([]byte, error) {
	if len(cmdArgs) == 0 {
		return nil, errors.New("empty command")
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	out, err := cmd.CombinedOutput()
	return out, err
}
