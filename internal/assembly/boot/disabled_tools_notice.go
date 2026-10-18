package boot

import (
	"fmt"
	"strings"

	"reasonix/internal/contract/event"
	"reasonix/internal/contract/tool"
)

// reportKeptTools tells the user which tools [tools].disabled named that the host
// kept, and which feature needs each, so a removal that did not happen is not
// mistaken for one that did.
func reportKeptTools(sink event.Sink, kept []tool.DisableConflict) {
	if len(kept) == 0 {
		return
	}
	parts := make([]string, 0, len(kept))
	for _, k := range kept {
		parts = append(parts, fmt.Sprintf("%s (needed for %s)", k.Tool, tool.FeatureLabel(k.By)))
	}
	report(sink, event.Event{
		Level: event.LevelWarn,
		Text:  "[tools] disabled names tools Reasonix needs, so they stay on: " + strings.Join(parts, "; ") + ".",
	})
}
