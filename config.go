package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type config struct {
	SuspendThreshold   float64
	ShutdownThreshold  float64
	PollInterval       time.Duration
	Cooldown           time.Duration
	SuspendRetries     int
	SuspendRetryDelay  time.Duration
	ShutdownRetries    int
	ShutdownRetryDelay time.Duration
	EmergencyPoweroff  bool
	DryRun             bool
	OneShot            bool
	TestPercentage     *float64
}

func loadConfig() (config, error) {
	cfg := config{
		SuspendThreshold:   5,
		ShutdownThreshold:  3,
		PollInterval:       60 * time.Second,
		Cooldown:           2 * time.Minute,
		SuspendRetries:     3,
		SuspendRetryDelay:  2 * time.Second,
		ShutdownRetries:    4,
		ShutdownRetryDelay: 1 * time.Second,
		EmergencyPoweroff:  true,
	}

	if v := os.Getenv("POWER_SUSPEND_RETRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("POWER_SUSPEND_RETRIES: %w", err)
		}
		cfg.SuspendRetries = n
	}

	if v := os.Getenv("POWER_SUSPEND_RETRY_DELAY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("POWER_SUSPEND_RETRY_DELAY: %w", err)
		}
		cfg.SuspendRetryDelay = d
	}

	if v := os.Getenv("POWER_SHUTDOWN_RETRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("POWER_SHUTDOWN_RETRIES: %w", err)
		}
		cfg.ShutdownRetries = n
	}

	if v := os.Getenv("POWER_SHUTDOWN_RETRY_DELAY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("POWER_SHUTDOWN_RETRY_DELAY: %w", err)
		}
		cfg.ShutdownRetryDelay = d
	}

	if v := os.Getenv("POWER_EMERGENCY_POWEROFF"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("POWER_EMERGENCY_POWEROFF: %w", err)
		}
		cfg.EmergencyPoweroff = b
	}

	if v := os.Getenv("POWER_DRY_RUN"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("POWER_DRY_RUN: %w", err)
		}
		cfg.DryRun = b
	}

	if v := os.Getenv("POWER_ONESHOT"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("POWER_ONESHOT: %w", err)
		}
		cfg.OneShot = b
	}

	if v := os.Getenv("POWER_TEST_PERCENTAGE"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return cfg, fmt.Errorf("POWER_TEST_PERCENTAGE: %w", err)
		}
		cfg.TestPercentage = &f
	}

	if v := os.Getenv("POWER_SUSPEND_THRESHOLD"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return cfg, fmt.Errorf("POWER_SUSPEND_THRESHOLD: %w", err)
		}
		cfg.SuspendThreshold = f
	}

	if v := os.Getenv("POWER_SHUTDOWN_THRESHOLD"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return cfg, fmt.Errorf("POWER_SHUTDOWN_THRESHOLD: %w", err)
		}
		cfg.ShutdownThreshold = f
	}

	if cfg.ShutdownThreshold >= cfg.SuspendThreshold {
		return cfg, errors.New("shutdown threshold must be lower than suspend threshold")
	}

	if v := os.Getenv("POWER_POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("POWER_POLL_INTERVAL: %w", err)
		}
		cfg.PollInterval = d
	}

	if cfg.PollInterval < 0 {
		return cfg, errors.New("POWER_POLL_INTERVAL cannot be negative")
	}
	if cfg.PollInterval > 0 && cfg.PollInterval < 5*time.Second {
		cfg.PollInterval = 5 * time.Second
	}

	if v := os.Getenv("POWER_ACTION_COOLDOWN"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("POWER_ACTION_COOLDOWN: %w", err)
		}
		cfg.Cooldown = d
	}

	if cfg.SuspendRetries < 1 {
		cfg.SuspendRetries = 1
	}
	if cfg.SuspendRetryDelay < 0 {
		return cfg, errors.New("POWER_SUSPEND_RETRY_DELAY cannot be negative")
	}
	if cfg.ShutdownRetries < 1 {
		cfg.ShutdownRetries = 1
	}
	if cfg.ShutdownRetryDelay < 0 {
		return cfg, errors.New("POWER_SHUTDOWN_RETRY_DELAY cannot be negative")
	}

	return cfg, nil
}
