package ua

import "github.com/mileusna/useragent"

type Info struct {
	Browser  string
	OS       string
	Device   string
	Original string
}

func Parse(raw string) Info {
	if raw == "" {
		return Info{Original: raw}
	}
	ua := useragent.Parse(raw)
	device := "Desktop"
	if ua.Mobile {
		device = "Mobile"
	} else if ua.Tablet {
		device = "Tablet"
	}
	name := ua.Name
	if name == "" {
		name = "Unknown"
	}
	os := ua.OS
	if os == "" {
		os = "Unknown"
	}
	return Info{Browser: name, OS: os, Device: device, Original: raw}
}
