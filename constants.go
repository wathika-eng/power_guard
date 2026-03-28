package main

const (
	upowerService         = "org.freedesktop.UPower"
	upowerPath            = "/org/freedesktop/UPower"
	displayDevicePath     = "/org/freedesktop/UPower/devices/DisplayDevice"
	upowerInterface       = "org.freedesktop.UPower"
	upowerDeviceInterface = "org.freedesktop.UPower.Device"
	propertiesInterface   = "org.freedesktop.DBus.Properties"

	login1Service   = "org.freedesktop.login1"
	login1Path      = "/org/freedesktop/login1"
	login1Interface = "org.freedesktop.login1.Manager"

	stateCharging    = uint32(1)
	stateDischarging = uint32(2)
)
