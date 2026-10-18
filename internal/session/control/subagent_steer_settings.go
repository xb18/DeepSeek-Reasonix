package control

import "reasonix/internal/contract/config"

// SubagentSteerSettings are the bounds on guidance sent to a running
// sub-agent, as the user file records them; zero lifts a bound. A project file
// cannot set them, so there is one value, and the Default fields say what unset
// reads as.
type SubagentSteerSettings struct {
	MaxBytes         int    `json:"maxBytes"`
	PerMinute        int    `json:"perMinute"`
	DefaultMaxBytes  int    `json:"defaultMaxBytes"`
	DefaultPerMinute int    `json:"defaultPerMinute"`
	Path             string `json:"path"`
}

// SubagentSteerSettings reads the limits from the user file.
func (c *Controller) SubagentSteerSettings() SubagentSteerSettings {
	path := config.UserConfigPath()
	bytes, perMinute := config.LoadForEdit(path).SubagentSteerLimits()
	defBytes, defPerMinute := config.Default().SubagentSteerLimits()
	return SubagentSteerSettings{MaxBytes: bytes, PerMinute: perMinute, DefaultMaxBytes: defBytes, DefaultPerMinute: defPerMinute, Path: path}
}

// SaveSubagentSteerSettings persists the limits to the user file. They are read
// at each message, so a running sub-agent takes the change at once.
func (c *Controller) SaveSubagentSteerSettings(maxBytes, perMinute int) error {
	unlock := config.LockUserConfigEdits()
	defer unlock()
	path := config.UserConfigPath()
	cfg := config.LoadForEdit(path)
	cfg.Agent.SubagentSteerBytes, cfg.Agent.SubagentSteerPerMinute = maxBytes, perMinute
	return cfg.SaveTo(path)
}
