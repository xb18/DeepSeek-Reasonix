package tool

import (
	"fmt"
	"strings"
)

// CodeToolDisabled identifies a call refused because the user's configuration
// removed the tool by name ([tools].disabled).
const CodeToolDisabled = "tool.disabled_by_config"

// The features that can require a tool, by identity. A client shows its own
// wording for the id; FeatureLabel is the English one for logs and refusals.
const (
	FeatureGoalMode    = "goal_mode"
	FeatureDeferredMCP = "deferred_mcp_tools"
)

// FeatureLabel is the English phrase for a feature id, or the id itself.
func FeatureLabel(id string) string {
	switch id {
	case FeatureGoalMode:
		return "goal mode"
	case FeatureDeferredMCP:
		return "loading deferred MCP tools"
	}
	return id
}

// HostRequired is implemented by a tool another host feature cannot work
// without. The owner of that feature says so, so which tools stay is declared
// where the dependence lives, not in a list of names kept beside the setting.
type HostRequired interface {
	// HostRequiredBy is the id of the feature that needs the tool.
	HostRequiredBy() string
}

// DisableConflict is a tool the user asked to remove and the host kept.
type DisableConflict struct {
	Tool string
	By   string
}

// DisableTools removes the named tools from every surface and refuses any later
// registration of them, so no path that assembles tools afterwards brings one
// back. A tool a host feature requires is kept and reported. Names nothing has
// registered are remembered for tools that arrive later and change nothing now.
func (r *Registry) DisableTools(names []string) []DisableConflict {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var conflicts []DisableConflict
	drop := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if required, ok := r.tools[name].(HostRequired); ok {
			conflicts = append(conflicts, DisableConflict{Tool: name, By: required.HostRequiredBy()})
			continue
		}
		if r.disabledTools == nil {
			r.disabledTools = map[string]bool{}
		}
		r.disabledTools[name] = true
		if _, registered := r.tools[name]; registered {
			drop[name] = true
		}
	}
	if len(drop) == 0 {
		return conflicts
	}
	kept := r.order[:0]
	for _, name := range r.order {
		if drop[name] {
			delete(r.tools, name)
			delete(r.canon, name)
			continue
		}
		kept = append(kept, name)
	}
	r.order = kept
	r.schemaRev.Add(1)
	return conflicts
}

// DisabledRefusal returns the typed refusal for a tool the user's configuration
// removed. The second result is false for any other name.
func (r *Registry) DisabledRefusal(name string) (Refusal, bool) {
	if r == nil {
		return Refusal{}, false
	}
	name = strings.TrimSpace(name)
	r.mu.RLock()
	_, registered := r.tools[name]
	disabled := r.disabledTools[name] && !registered
	r.mu.RUnlock()
	if !disabled {
		return Refusal{}, false
	}
	return Refusal{
		Code:    CodeToolDisabled,
		Message: fmt.Sprintf("blocked: tool %q is turned off in this configuration ([tools] disabled); it was not run", name),
		Remedy:  &Remedy{Op: RemedyOpen, Setting: "tools.disabled"},
	}, true
}

// refusesLocked reports whether the user's removal list turns t away. A tool the
// host requires is never turned away.
func (r *Registry) refusesLocked(t Tool) bool {
	if !r.disabledTools[t.Name()] {
		return false
	}
	_, required := t.(HostRequired)
	return !required
}

// CallRefusal answers for a tool name the registry does not serve: the user's
// removal first, then an MCP tool the configuration disabled or a definition
// held for re-approval.
func (r *Registry) CallRefusal(name string) (Refusal, bool) {
	if refusal, ok := r.DisabledRefusal(name); ok {
		return refusal, true
	}
	return r.MCPRefusal(name)
}
