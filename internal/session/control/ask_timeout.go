package control

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"reasonix/internal/base/i18n"

	"reasonix/internal/contract/config"
	"reasonix/internal/contract/event"
	"reasonix/internal/runtime/agent"
)

// AskTimeoutSettings is the user's ask-timeout setting: how long an `ask`
// question waits before the host skips it. Seconds 0 means it waits.
type AskTimeoutSettings struct {
	Seconds int `json:"seconds"`
	// Invalid carries the code of a value the file holds that cannot be used;
	// Seconds then reads 0 because nothing is being applied.
	Invalid        string `json:"invalid,omitempty"`
	DefaultSeconds int    `json:"defaultSeconds"`
	MinSeconds     int    `json:"minSeconds"`
	MaxSeconds     int    `json:"maxSeconds"`
	Path           string `json:"path"`
}

func (c *Controller) AskTimeoutSettings() AskTimeoutSettings {
	path := config.UserConfigPath()
	cfg := config.LoadForEdit(path)
	out := AskTimeoutSettings{
		Seconds:        cfg.Agent.AskTimeoutSeconds,
		DefaultSeconds: config.DefaultAskTimeoutSeconds,
		MinSeconds:     config.MinAskTimeoutSeconds,
		MaxSeconds:     config.MaxAskTimeoutSeconds,
		Path:           path,
	}
	if config.ValidateAskTimeoutSeconds(out.Seconds) != nil {
		out.Seconds, out.Invalid = 0, "ask_timeout.out_of_range"
	}
	return out
}

// SaveAskTimeoutSettings persists the timeout. Each ask reads it as it begins
// to wait, so a question already open keeps the wait it started with.
func (c *Controller) SaveAskTimeoutSettings(seconds int) error {
	unlock := config.LockUserConfigEdits()
	defer unlock()
	path := config.UserConfigPath()
	cfg := config.LoadForEdit(path)
	if err := cfg.SetAskTimeoutSeconds(seconds); err != nil {
		return err
	}
	return cfg.SaveTo(path)
}

// askOutcomeTimedOut is the receipt outcome of a question the host skipped
// because nobody answered within the user's ask-timeout setting.
const askOutcomeTimedOut = "timed_out"

// askWaitContext bounds an ask's wait by the user's ask timeout, on top of the
// approval timeout. The deadline's cause is the AskTimedOutError the asker
// returns, so a skip is told apart from a cancel or a headless deadline by
// identity. An external party's form (origin set) is theirs to wait on and is
// never skipped.
func (c *Controller) askWaitContext(ctx context.Context, origin *event.AskOrigin) (context.Context, context.CancelFunc) {
	waitCtx, cancel := c.approval.waitContext(ctx)
	if origin != nil {
		return waitCtx, cancel
	}
	after, err := config.LoadForEdit(config.UserConfigPath()).AskTimeoutChecked()
	if err != nil {
		c.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Code: "ask_timeout.invalid",
			Text:   "ask_timeout_seconds in the user config is not valid, so questions wait as long as it takes until it is fixed",
			Detail: err.Error()})
		return waitCtx, cancel
	}
	if after <= 0 {
		return waitCtx, cancel
	}
	skipCtx, cancelSkip := context.WithTimeoutCause(waitCtx, after, &agent.AskTimedOutError{After: after})
	return skipCtx, func() { cancelSkip(); cancel() }
}

// endUnansweredAsk settles an ask whose wait ended without an answer. A skip
// by the ask timeout lets an answer that landed as the timer fired win;
// otherwise the card is settled on every frontend by a receipt naming the
// timeout, and the asker is told it was one. Any other end is a cancellation.
func (c *Controller) endUnansweredAsk(waitCtx context.Context, id string, reply <-chan []event.AskAnswer) ([]event.AskAnswer, error) {
	timedOut, skipped := errors.AsType[*agent.AskTimedOutError](context.Cause(waitCtx))
	if !skipped {
		c.approval.cancelAsk(id)
		c.closeBarrier(id, barrierCancelled)
		return nil, waitCtx.Err()
	}
	pending, open := c.approval.resolveAsk(id)
	if !open {
		select {
		case ans := <-reply:
			c.closeBarrier(id, barrierResolved)
			return ans, nil
		case <-time.After(time.Second):
		}
	}
	c.closeBarrier(id, barrierTimedOut)
	if open {
		c.recordAskDecisionReceipt(id, pending, nil, askOutcomeTimedOut, nil)
	}
	return nil, timedOut
}

// askTimeoutNotice is /ask-timeout: bare, it reports the setting; with
// <seconds> or off it changes it, through the same save the settings page uses.
func (c *Controller) askTimeoutNotice(fields []string) {
	if len(fields) == 2 {
		seconds := 0
		if arg := strings.ToLower(fields[1]); arg != "off" {
			n, err := strconv.Atoi(arg)
			if err != nil {
				c.notice(fmt.Sprintf(i18n.M.AskTimeoutBadFmt, fields[1]) + "\n" + c.askTimeoutStatus())
				return
			}
			seconds = n
		}
		if err := c.SaveAskTimeoutSettings(seconds); err != nil {
			c.notice("ask-timeout: " + err.Error())
			return
		}
	}
	c.notice(c.askTimeoutStatus())
}

func (c *Controller) askTimeoutStatus() string {
	state := i18n.M.AskTimeoutOff
	if n := c.AskTimeoutSettings().Seconds; n > 0 {
		state = fmt.Sprintf("%d s", n)
	}
	return fmt.Sprintf(i18n.M.AskTimeoutFmt, state)
}

func askTimeoutArgItems(prior []string) []SlashItem {
	if len(prior) != 1 {
		return nil
	}
	return []SlashItem{
		{Label: "off", Insert: "off", Hint: "wait for an answer as long as it takes"},
		{Label: "300", Insert: "300", Hint: "skip after five minutes"},
	}
}
