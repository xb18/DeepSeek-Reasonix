package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MinAskTimeoutSeconds     = 10
	MaxAskTimeoutSeconds     = 24 * 60 * 60
	DefaultAskTimeoutSeconds = 300
)

// ErrAskTimeoutOutOfRange marks a timeout outside what an ask can wait.
var ErrAskTimeoutOutOfRange = errors.New("ask timeout out of range")

// ValidateAskTimeoutSeconds is the one rule for the timeout, applied to what is
// saved and to what a hand-edited file holds: 0 (off) or 10 to 86400.
func ValidateAskTimeoutSeconds(seconds int) error {
	if seconds != 0 && (seconds < MinAskTimeoutSeconds || seconds > MaxAskTimeoutSeconds) {
		return fmt.Errorf("%w: seconds must be 0 (off) or between %d and %d, not %d", ErrAskTimeoutOutOfRange, MinAskTimeoutSeconds, MaxAskTimeoutSeconds, seconds)
	}
	return nil
}

// AskTimeoutChecked is how long an `ask` question waits before the host skips
// it. Zero, the default, waits as long as it takes. A value the file holds that
// breaks the rule is an error, never a silent off or a wrapped duration.
func (c *Config) AskTimeoutChecked() (time.Duration, error) {
	if c == nil {
		return 0, nil
	}
	if err := ValidateAskTimeoutSeconds(c.Agent.AskTimeoutSeconds); err != nil {
		return 0, err
	}
	return time.Duration(c.Agent.AskTimeoutSeconds) * time.Second, nil
}

// SetAskTimeoutSeconds validates and stores the timeout; 0 turns it off.
func (c *Config) SetAskTimeoutSeconds(seconds int) error {
	if err := ValidateAskTimeoutSeconds(seconds); err != nil {
		return err
	}
	c.Agent.AskTimeoutSeconds = seconds
	return nil
}

func renderAskTimeout(b *strings.Builder, c *Config) {
	if c.Agent.AskTimeoutSeconds > 0 {
		fmt.Fprintf(b, "ask_timeout_seconds = %d   # skip an ask question nobody answered in this long; the model is told it timed out (0 = wait)\n", c.Agent.AskTimeoutSeconds)
		return
	}
	b.WriteString("# ask_timeout_seconds = 300   # skip an ask question nobody answered in this long; the model is told it timed out (default 0: wait)\n")
}
