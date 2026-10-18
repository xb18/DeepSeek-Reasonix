package config

import (
	"fmt"
	"slices"
	"strings"
)

// DisabledTools is the tool names the configuration removes, trimmed, without
// repeats, in a stable order.
func (c *Config) DisabledTools() []string {
	if c == nil {
		return nil
	}
	var out []string
	for _, name := range c.Tools.Disabled {
		if name = strings.TrimSpace(name); name != "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// SetToolDisabled adds or removes one name from the list.
func (c *Config) SetToolDisabled(name string, disabled bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	names := c.DisabledTools()
	if i := slices.Index(names, name); disabled && i < 0 {
		names = append(names, name)
	} else if !disabled && i >= 0 {
		names = slices.Delete(names, i, i+1)
	}
	slices.Sort(names)
	c.Tools.Disabled = names
}

// listsSet reports whether either tool list is set.
func (t ToolsConfig) listsSet() bool { return len(t.Enabled) > 0 || len(t.Disabled) > 0 }

// renderToolLists writes the [tools] enabled and disabled lists. The full file
// spells out an empty enabled list; a delta writes only what is set.
func renderToolLists(b *strings.Builder, c *Config, full bool) {
	switch {
	case len(c.Tools.Enabled) > 0:
		fmt.Fprintf(b, "enabled = %s\n", renderStringArray(c.Tools.Enabled))
	case full:
		b.WriteString("enabled = []   # empty = all built-in tools\n")
	}
	if disabled := c.DisabledTools(); len(disabled) > 0 {
		fmt.Fprintf(b, "disabled = %s\n", renderStringArray(disabled))
	}
}
